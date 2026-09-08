package simulator

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
	"github.com/deploymenttheory/go-microsoft-dm/testpki"
)

// Errors returned by Enroll.
var (
	// ErrConfig reports an unusable Client.
	ErrConfig = errors.New("simulator: invalid configuration")
	// ErrProtocol reports a server answer the client cannot use.
	ErrProtocol = errors.New("simulator: unexpected server behavior")
	// ErrProvisioning reports a provisioning document missing what the
	// client needs.
	ErrProvisioning = errors.New("simulator: provisioning document")
)

// Client describes the device and user that enroll.
type Client struct {
	// HTTP performs the requests; default http.DefaultClient.
	HTTP *http.Client
	// DiscoveryURL is the Discovery Service URL. When empty it is derived
	// from Email as https://enterpriseenrollment.<domain>/EnrollmentServer/Discovery.svc.
	DiscoveryURL string
	// Email and Password are the on-premise credentials.
	Email    string
	Password string
	// DeviceID is the MDE2 DeviceID; required.
	DeviceID string
	// HWDevID, DeviceName, OSVersion, OSEdition and ApplicationVersion
	// populate AdditionalContext; defaults describe a Windows 11 26H2 PC.
	HWDevID            string
	DeviceName         string
	OSVersion          string
	OSEdition          string
	ApplicationVersion string
	// EnrollmentType; default Full.
	EnrollmentType enroll.EnrollmentType
	// DeviceType; default CIMClient_Windows.
	DeviceType enroll.DeviceType
	// RequestVersion; default 5.0.
	RequestVersion string
	// Key signs the request; default an RSA key of the advertised length.
	Key crypto.Signer
	// WindowsSubject encodes the CSR subject as Windows does, as a
	// PrintableString containing '!', to exercise the server's parser.
	WindowsSubject bool
	// SkipPolicy skips GetPolicies even when discovery advertises it.
	SkipPolicy bool
	// Extra context items are appended to the catalogue items.
	Extra []enroll.ContextItem
}

// Credential is one OMA DM account credential from the w7 APPLICATION.
type Credential struct {
	Type   wapprov.AuthType
	Name   string
	Secret string
	Nonce  []byte
}

// Account is the OMA DM account the provisioning document created.
type Account struct {
	ProviderID string
	Name       string
	Address    string
	ServerAuth Credential
	ClientAuth Credential
	// Parms holds every APPLICATION parm for settings not modelled here.
	Parms []wapprov.Parm
}

// DMClient is the DMClient/Provider/<ID> state.
type DMClient struct {
	PushPFN       string
	ProviderID    string
	UPN           string
	EntDeviceName string
	EntDMID       string
	Poll          wapprov.Poll
}

// Enrollment is the outcome of Enroll.
type Enrollment struct {
	Discovery   *enroll.DiscoverResponse
	Policy      *enroll.PolicyResponse
	Certificate *x509.Certificate
	Key         crypto.Signer
	// Store is where the document put the certificate.
	Store         wapprov.Store
	Roots         []*x509.Certificate
	Intermediates []*x509.Certificate
	Renew         *wapprov.Renew
	Document      *wapprov.Document
	Account       Account
	DMClient      DMClient
}

func (c *Client) defaults() error {
	if c.Email == "" || c.Password == "" || c.DeviceID == "" {
		return fmt.Errorf("%w: Email, Password and DeviceID are required", ErrConfig)
	}
	if c.DiscoveryURL == "" {
		_, domain, ok := strings.Cut(c.Email, "@")
		if !ok || domain == "" {
			return fmt.Errorf("%w: cannot derive a discovery URL from %q", ErrConfig, c.Email)
		}
		c.DiscoveryURL = "https://enterpriseenrollment." + strings.ToLower(domain) + enroll.DiscoveryPath
	}
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}
	if c.DeviceName == "" {
		c.DeviceName = "DESKTOP-SIM"
	}
	if c.OSVersion == "" {
		c.OSVersion = "10.0.26100.1"
	}
	if c.OSEdition == "" {
		c.OSEdition = "4"
	}
	if c.ApplicationVersion == "" {
		c.ApplicationVersion = c.OSVersion
	}
	if c.EnrollmentType == "" {
		c.EnrollmentType = enroll.EnrollmentTypeFull
	}
	if !c.EnrollmentType.Valid() {
		return fmt.Errorf("%w: EnrollmentType %q", ErrConfig, c.EnrollmentType)
	}
	if c.DeviceType == "" {
		c.DeviceType = enroll.DeviceTypeWindows
	}
	if c.RequestVersion == "" {
		c.RequestVersion = "5.0"
	}
	if _, err := enroll.ParseVersion(c.RequestVersion); err != nil {
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}
	return nil
}

// Enroll runs the enrollment and returns what the device now holds.
func Enroll(ctx context.Context, c Client) (*Enrollment, error) {
	if err := c.defaults(); err != nil {
		return nil, err
	}
	if err := c.probe(ctx); err != nil {
		return nil, err
	}
	disc, err := c.discover(ctx)
	if err != nil {
		return nil, err
	}
	if disc.AuthPolicy != enroll.AuthPolicyOnPremise {
		return nil, fmt.Errorf("%w: server wants %s authentication", ErrProtocol, disc.AuthPolicy)
	}
	out := &Enrollment{Discovery: disc}
	keyBits := 2048
	if disc.EnrollmentPolicyServiceURL != "" && !c.SkipPolicy {
		policy, err := c.getPolicies(ctx, disc.EnrollmentPolicyServiceURL)
		if err != nil {
			return nil, err
		}
		out.Policy = policy
		if policy.MinimalKeyLength > keyBits {
			keyBits = policy.MinimalKeyLength
		}
	}
	key := c.Key
	if key == nil {
		k, err := rsa.GenerateKey(randReader, keyBits)
		if err != nil {
			return nil, fmt.Errorf("simulator: generate key: %w", err)
		}
		key = k
	}
	out.Key = key
	csr, err := c.csr(key)
	if err != nil {
		return nil, err
	}
	docBytes, err := c.requestToken(ctx, disc.EnrollmentServiceURL, csr)
	if err != nil {
		return nil, err
	}
	doc, err := wapprov.Decode(docBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrProvisioning, err)
	}
	out.Document = doc
	if err := out.apply(doc, key); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) csr(key crypto.Signer) ([]byte, error) {
	if c.WindowsSubject {
		half := strings.ToUpper(strings.ReplaceAll(c.DeviceID, "-", ""))
		subject := c.DeviceID + "!" + half
		der, err := testpki.WindowsCSR(subject, key)
		if err != nil {
			return nil, fmt.Errorf("simulator: %w", err)
		}
		return der, nil
	}
	der, err := testpki.CSR(c.DeviceID, key)
	if err != nil {
		return nil, fmt.Errorf("simulator: %w", err)
	}
	return der, nil
}

func (c *Client) items() []enroll.ContextItem {
	items := []enroll.ContextItem{
		{Name: "OSEdition", Value: c.OSEdition},
		{Name: "OSVersion", Value: c.OSVersion},
		{Name: "DeviceName", Value: c.DeviceName},
		{Name: "EnrollmentType", Value: string(c.EnrollmentType)},
		{Name: "DeviceType", Value: string(c.DeviceType)},
		{Name: "ApplicationVersion", Value: c.ApplicationVersion},
		{Name: "DeviceID", Value: c.DeviceID},
		{Name: "TargetedUserLoggedIn", Value: "True"},
		{Name: "Locale", Value: "en-US"},
		{Name: "RequestVersion", Value: c.RequestVersion},
	}
	if c.HWDevID != "" {
		items = append(items, enroll.ContextItem{Name: "HWDevID", Value: c.HWDevID})
	}
	return append(items, c.Extra...)
}

// probe is the GET the client sends before the SOAP exchange.
func (c *Client) probe(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.DiscoveryURL, nil)
	if err != nil {
		return fmt.Errorf("simulator: %w", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("simulator: discovery probe: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: discovery probe returned %d", ErrProtocol, resp.StatusCode)
	}
	return nil
}

func (c *Client) discover(ctx context.Context) (*enroll.DiscoverResponse, error) {
	body, err := enroll.EncodeDiscover(&enroll.DiscoverRequest{
		EmailAddress: c.Email, RequestVersion: c.RequestVersion, DeviceType: c.DeviceType,
		ApplicationVersion: c.ApplicationVersion, OSEdition: c.OSEdition,
		AuthPolicies: []enroll.AuthPolicy{enroll.AuthPolicyOnPremise, enroll.AuthPolicyFederated, enroll.AuthPolicyCertificate},
	}, "urn:uuid:"+enroll.NewUUID(), c.DiscoveryURL)
	if err != nil {
		return nil, fmt.Errorf("simulator: %w", err)
	}
	out, err := c.post(ctx, c.DiscoveryURL, body)
	if err != nil {
		return nil, err
	}
	_, resp, err := enroll.DecodeDiscoverResponse(out)
	if err != nil {
		return nil, fmt.Errorf("simulator: discover: %w", err)
	}
	if resp.EnrollmentServiceURL == "" {
		return nil, fmt.Errorf("%w: discovery gave no EnrollmentServiceUrl", ErrProtocol)
	}
	return resp, nil
}

func (c *Client) getPolicies(ctx context.Context, url string) (*enroll.PolicyResponse, error) {
	body, err := enroll.EncodeGetPolicies("urn:uuid:"+enroll.NewUUID(), url, &enroll.Credentials{Username: c.Email, Password: c.Password})
	if err != nil {
		return nil, fmt.Errorf("simulator: %w", err)
	}
	out, err := c.post(ctx, url, body)
	if err != nil {
		return nil, err
	}
	_, policy, err := enroll.DecodePolicyResponse(out)
	if err != nil {
		return nil, fmt.Errorf("simulator: policy: %w", err)
	}
	return policy, nil
}

func (c *Client) requestToken(ctx context.Context, url string, csr []byte) ([]byte, error) {
	body, err := enroll.EncodeRequestSecurityToken("urn:uuid:"+enroll.NewUUID(), url, &enroll.Credentials{Username: c.Email, Password: c.Password}, csr, c.items())
	if err != nil {
		return nil, fmt.Errorf("simulator: %w", err)
	}
	out, err := c.post(ctx, url, body)
	if err != nil {
		return nil, err
	}
	_, doc, err := enroll.DecodeTokenResponse(out)
	if err != nil {
		return nil, fmt.Errorf("simulator: enroll: %w", err)
	}
	return doc, nil
}

// post sends a SOAP request and returns the body whatever the status, so
// that a fault envelope reaches the decoders. It refuses a chunked reply
// because the Windows client does (research pitfall "Transport").
func (c *Client) post(ctx context.Context, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("simulator: %w", err)
	}
	req.Header.Set("Content-Type", enroll.ContentType)
	req.Header.Set("User-Agent", "ENROLLClient")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("simulator: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, soap.DefaultMaxSize))
	if err != nil {
		return nil, fmt.Errorf("simulator: read response: %w", err)
	}
	for _, te := range resp.TransferEncoding {
		if te == "chunked" {
			return nil, fmt.Errorf("%w: chunked response", ErrProtocol)
		}
	}
	if resp.ContentLength >= 0 && resp.ContentLength != int64(len(out)) {
		return nil, fmt.Errorf("%w: Content-Length %d but %d bytes", ErrProtocol, resp.ContentLength, len(out))
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		return nil, fmt.Errorf("%w: status %d", ErrProtocol, resp.StatusCode)
	}
	return out, nil
}

// apply reads the provisioning document into the enrollment.
func (e *Enrollment) apply(doc *wapprov.Document, key crypto.Signer) error {
	if err := e.applyCertificates(doc, key); err != nil {
		return err
	}
	app := doc.Find(wapprov.TypeApplication)
	if app == nil || app.Value("APPID") != "w7" {
		return fmt.Errorf("%w: no w7 APPLICATION", ErrProvisioning)
	}
	e.Account = Account{ProviderID: app.Value("PROVIDER-ID"), Name: app.Value("NAME"), Address: app.Value("ADDR"), Parms: app.Parms}
	if e.Account.ProviderID == "" || e.Account.Address == "" {
		return fmt.Errorf("%w: APPLICATION without PROVIDER-ID or ADDR", ErrProvisioning)
	}
	var haveServer, haveClient bool
	for i := range app.Children {
		ch := &app.Children[i]
		if ch.Type != "APPAUTH" {
			continue
		}
		cred, err := credential(ch)
		if err != nil {
			return err
		}
		switch ch.Value("AAUTHLEVEL") {
		case "APPSRV":
			e.Account.ServerAuth, haveServer = cred, true
		case "CLIENT":
			e.Account.ClientAuth, haveClient = cred, true
		default:
			return fmt.Errorf("%w: APPAUTH level %q", ErrProvisioning, ch.Value("AAUTHLEVEL"))
		}
	}
	if !haveServer || !haveClient {
		return fmt.Errorf("%w: both APPSRV and CLIENT credentials are required", ErrProvisioning)
	}
	dm := doc.Find(wapprov.TypeDMClient)
	if dm == nil {
		return fmt.Errorf("%w: no DMClient", ErrProvisioning)
	}
	prov := dm.Path("Provider", e.Account.ProviderID)
	if prov == nil {
		return fmt.Errorf("%w: DMClient has no provider %q", ErrProvisioning, e.Account.ProviderID)
	}
	e.DMClient = DMClient{ProviderID: e.Account.ProviderID, UPN: prov.Value("UPN"), EntDeviceName: prov.Value("EntDeviceName"), EntDMID: prov.Value("EntDMID")}
	if push := prov.Child("Push"); push != nil {
		e.DMClient.PushPFN = push.Value("PFN")
	}
	poll := prov.Child("Poll")
	if poll == nil {
		return fmt.Errorf("%w: DMClient provider without Poll", ErrProvisioning)
	}
	for _, p := range poll.Parms {
		if err := setPoll(&e.DMClient.Poll, p); err != nil {
			return err
		}
	}
	return nil
}

func credential(ch *wapprov.Characteristic) (Credential, error) {
	c := Credential{Type: wapprov.AuthType(ch.Value("AAUTHTYPE")), Name: ch.Value("AAUTHNAME"), Secret: ch.Value("AAUTHSECRET")}
	if c.Secret == "" {
		return Credential{}, fmt.Errorf("%w: APPAUTH without AAUTHSECRET", ErrProvisioning)
	}
	if v := ch.Value("AAUTHDATA"); v != "" {
		nonce, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return Credential{}, fmt.Errorf("%w: AAUTHDATA: %w", ErrProvisioning, err)
		}
		c.Nonce = nonce
	}
	return c, nil
}

func (e *Enrollment) applyCertificates(doc *wapprov.Document, key crypto.Signer) error {
	pub, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return fmt.Errorf("simulator: %w", err)
	}
	for _, cs := range doc.FindAll(wapprov.TypeCertificateStore) {
		if root := cs.Path("Root", "System"); root != nil {
			certs, err := entries(root)
			if err != nil {
				return err
			}
			e.Roots = append(e.Roots, certs...)
		}
		if ca := cs.Path("CA", "System"); ca != nil {
			certs, err := entries(ca)
			if err != nil {
				return err
			}
			e.Intermediates = append(e.Intermediates, certs...)
		}
		for _, store := range []wapprov.Store{wapprov.StoreUser, wapprov.StoreSystem} {
			my := cs.Path("My", string(store))
			if my == nil {
				continue
			}
			certs, err := entries(my)
			if err != nil {
				return err
			}
			for _, cert := range certs {
				got, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
				if err == nil && bytes.Equal(got, pub) {
					e.Certificate, e.Store = cert, store
				}
			}
		}
		if renew := cs.Path("My", "WSTEP", "Renew"); renew != nil {
			r := &wapprov.Renew{ServerURL: renew.Value("ServerURL")}
			r.ROBOSupport, _ = strconv.ParseBool(renew.Value("ROBOSupport"))
			r.RenewPeriod, _ = strconv.Atoi(renew.Value("RenewPeriod"))
			r.RetryInterval, _ = strconv.Atoi(renew.Value("RetryInterval"))
			e.Renew = r
		}
	}
	if e.Certificate == nil {
		return fmt.Errorf("%w: no certificate for the enrollment key under My/User or My/System", ErrProvisioning)
	}
	if len(e.Roots) == 0 {
		return fmt.Errorf("%w: no root certificate", ErrProvisioning)
	}
	return nil
}

// entries decodes the certificate entries under a store node.
func entries(node *wapprov.Characteristic) ([]*x509.Certificate, error) {
	var out []*x509.Certificate
	for i := range node.Children {
		ch := &node.Children[i]
		enc, ok := ch.Parm("EncodedCertificate")
		if !ok {
			continue
		}
		der, err := base64.StdEncoding.DecodeString(enc.Value)
		if err != nil {
			return nil, fmt.Errorf("%w: EncodedCertificate under %s: %w", ErrProvisioning, ch.Type, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("%w: certificate under %s: %w", ErrProvisioning, ch.Type, err)
		}
		if wapprov.Thumbprint(der) != ch.Type {
			return nil, fmt.Errorf("%w: entry %s does not match its certificate thumbprint", ErrProvisioning, ch.Type)
		}
		out = append(out, cert)
	}
	return out, nil
}
