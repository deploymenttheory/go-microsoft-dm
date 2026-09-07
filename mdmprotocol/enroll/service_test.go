package enroll

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
	"github.com/deploymenttheory/go-microsoft-dm/testpki"
)

var t0 = time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)

// fakeIssuer signs whatever public key the CSR carries with a testpki CA.
type fakeIssuer struct {
	ca   *testpki.CA
	err  error
	nilR bool
	last *Request
}

func (f *fakeIssuer) Issue(_ context.Context, req *Request) (*Issued, error) {
	f.last = req
	if f.err != nil {
		return nil, f.err
	}
	if f.nilR {
		return &Issued{}, nil
	}
	csr, err := x509.ParseCertificateRequest(req.CSR)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: csr.Subject, NotBefore: t0, NotAfter: t0.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, f.ca.Cert, csr.PublicKey, f.ca.Key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &Issued{Certificate: cert, Chain: []*x509.Certificate{f.ca.Cert}}, nil
}

type fakeRecorder struct {
	err  error
	last *Enrollment
	doc  *wapprov.Document
}

func (r *fakeRecorder) Record(_ context.Context, e *Enrollment, doc *wapprov.Document) error {
	r.last, r.doc = e, doc
	return r.err
}

type fakeProvisioner struct {
	err error
	doc *wapprov.Document
}

func (p *fakeProvisioner) Provision(_ context.Context, _ *Enrollment) (*wapprov.Document, error) {
	return p.doc, p.err
}

var users = AuthenticatorFunc(func(_ context.Context, c Credentials) (Principal, error) {
	switch {
	case c.Username == "user@contoso.com" && c.Password == "mypassword":
		return Principal{UPN: "user@contoso.com"}, nil
	case c.Username == "blocked@contoso.com":
		return Principal{}, ErrUnauthorized
	case c.Username == "fault@contoso.com":
		return Principal{}, soap.NewFault(soap.SubcodeAuthorization, "cap").WithDetail(soap.ErrorDeviceCapReached, "")
	case c.Username == "broken@contoso.com":
		return Principal{}, errors.New("directory down")
	}
	return Principal{}, ErrUnauthenticated
})

type env struct {
	svc    *Service
	issuer *fakeIssuer
	rec    *fakeRecorder
	cfg    Config
}

const (
	policyURL = "https://enrolltest.contoso.com/ENROLLMENTSERVER/DEVICEENROLLMENTWEBSERVICE.SVC"
	enrollURL = "https://enrolltest.contoso.com/ENROLLMENTSERVER/DEVICEENROLLMENTWEBSERVICE.SVC"
)

func staticCreds(_ context.Context, _ *Enrollment) (wapprov.Credential, wapprov.Credential, error) {
	return wapprov.Credential{Type: wapprov.AuthBasic, Name: "dev", Secret: "s1"}, wapprov.Credential{Type: wapprov.AuthDigest, Secret: "s2", Nonce: []byte{1}}, nil
}

func newEnv(t testing.TB, mutate func(*Config)) *env {
	t.Helper()
	ca, err := testpki.NewCA("Test CA")
	if err != nil {
		t.Fatal(err)
	}
	return newEnvWithIssuer(t, &fakeIssuer{ca: ca}, mutate)
}

// newEnvWithIssuer builds the service around a given issuer; the fuzz
// target uses it to avoid generating an RSA CA in every worker process.
func newEnvWithIssuer(t testing.TB, issuer *fakeIssuer, mutate func(*Config)) *env {
	t.Helper()
	prov, err := NewProvisioner(ProvisionConfig{ManagementURL: "https://mdm.contoso.com/ManagementServer/MDM.svc", ProviderID: "TestServer", Name: "Test",
		Credentials: CredentialSourceFunc(staticCreds)})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{issuer: issuer, rec: &fakeRecorder{}}
	e.cfg = Config{EnrollmentServiceURL: enrollURL, EnrollmentPolicyServiceURL: policyURL, Authenticator: users, Issuer: e.issuer,
		Provisioner: prov, Recorder: e.rec, Clock: clock.NewFake(t0), TraceID: func() string { return "trace-1" }}
	if mutate != nil {
		mutate(&e.cfg)
	}
	e.svc, err = New(e.cfg)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func wantFault(t *testing.T, out []byte, err error, sub soap.Subcode, detail soap.ErrorType, cause error) *soap.Fault {
	t.Helper()
	f, ok := IsFault(err)
	if !ok {
		t.Fatalf("err = %v, want a fault", err)
	}
	if f.Subcode != sub || f.Detail != detail {
		t.Errorf("fault = %+v, want %s %s", f, sub, detail)
	}
	if cause != nil && !errors.Is(err, cause) {
		t.Errorf("cause = %v, want %v", f.Cause, cause)
	}
	d, derr := soap.DecodeFault(out)
	if derr != nil || d == nil || d.Subcode != string(sub) {
		t.Errorf("wire fault = %+v, %v", d, derr)
	}
	if detail != "" && (d.Detail == nil || d.Detail.ErrorType != string(detail) || d.Detail.TraceID != "trace-1") {
		t.Errorf("wire detail = %+v", d.Detail)
	}
	return f
}

func TestNewRejectsBadConfig(t *testing.T) {
	t.Parallel()
	good := newEnv(t, nil).cfg
	cases := map[string]func(c *Config){
		"no enrollment url":  func(c *Config) { c.EnrollmentServiceURL = "" },
		"http enrollment":    func(c *Config) { c.EnrollmentServiceURL = "http://x/e" },
		"relative policy":    func(c *Config) { c.EnrollmentPolicyServiceURL = "/policy" },
		"version 2.0":        func(c *Config) { c.EnrollmentVersion = "2.0" },
		"version junk":       func(c *Config) { c.EnrollmentVersion = "three" },
		"federated":          func(c *Config) { c.AuthPolicy = AuthPolicyFederated },
		"no authenticator":   func(c *Config) { c.Authenticator = nil },
		"no issuer":          func(c *Config) { c.Issuer = nil },
		"no provisioner":     func(c *Config) { c.Provisioner = nil },
		"bad policy":         func(c *Config) { c.Policy = &PolicyResponse{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := good
			mutate(&c)
			if _, err := New(c); !errors.Is(err, ErrConfig) {
				t.Errorf("err = %v", err)
			}
		})
	}
	c := good
	c.AuthPolicy = AuthPolicyFederated
	if _, err := New(c); !errors.Is(err, ErrPolicyUnsupported) {
		t.Errorf("federated cause: %v", err)
	}
	// Defaults are filled in.
	c = good
	c.EnrollmentVersion, c.Clock, c.MaxRequestSize, c.TraceID, c.Policy, c.AuthPolicy = "", nil, 0, nil, nil, ""
	svc, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	got := svc.Config()
	if got.EnrollmentVersion != "3.0" || got.Clock == nil || got.MaxRequestSize != soap.DefaultMaxSize || got.TraceID == nil || got.Policy == nil || got.AuthPolicy != AuthPolicyOnPremise {
		t.Errorf("defaults = %+v", got)
	}
	id := got.TraceID()
	if len(id) != 36 || id[14] != '4' || !strings.ContainsRune("89ab", rune(id[19])) {
		t.Errorf("uuid = %q", id)
	}
	if NewUUID() == NewUUID() {
		t.Error("uuids repeat")
	}
}

func TestDiscoverSuccess(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	out, err := e.svc.Discover(context.Background(), fixture(t, "discover-onprem-request.xml"))
	if err != nil {
		t.Fatal(err)
	}
	h, resp, err := DecodeDiscoverResponse(out)
	if err != nil {
		t.Fatal(err)
	}
	if h.Action != ActionDiscoverResponse || !strings.Contains(string(out), `<a:RelatesTo>urn:uuid: 748132ec-a575-4329-b01b-6171a9cf8478</a:RelatesTo>`) {
		t.Errorf("header: %s", out)
	}
	want := DiscoverResponse{AuthPolicy: AuthPolicyOnPremise, EnrollmentVersion: "3.0", EnrollmentPolicyServiceURL: policyURL, EnrollmentServiceURL: enrollURL}
	if *resp != want {
		t.Errorf("resp = %+v", *resp)
	}
	if !strings.Contains(string(out), `CorrelationId="trace-1"`) {
		t.Error("ActivityId missing")
	}
}

func discoverWith(t *testing.T, version string, policies ...AuthPolicy) []byte {
	t.Helper()
	data, err := EncodeDiscover(&DiscoverRequest{EmailAddress: "user@contoso.com", RequestVersion: version, DeviceType: DeviceTypeWindows,
		ApplicationVersion: "10.0.26100.0", OSEdition: "4", AuthPolicies: policies}, "urn:uuid:d", "https://enterpriseenrollment.contoso.com"+DiscoveryPath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDiscoverVersionNegotiation(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *Config) { c.EnrollmentVersion = "5.0"; c.EnrollmentPolicyServiceURL = "" })
	cases := map[string]string{"9.0": "5.0", "5.0": "5.0", "4.0": "4.0", "3.0": "3.0", "2.0": "", "1.0": ""}
	for req, want := range cases {
		out, err := e.svc.Discover(context.Background(), discoverWith(t, req, AuthPolicyFederated, AuthPolicyOnPremise))
		if err != nil {
			t.Fatalf("%s: %v", req, err)
		}
		_, resp, err := DecodeDiscoverResponse(out)
		if err != nil {
			t.Fatal(err)
		}
		if resp.EnrollmentVersion != want || resp.EnrollmentPolicyServiceURL != "" {
			t.Errorf("request %s: EnrollmentVersion = %q, want %q (policy url %q)", req, resp.EnrollmentVersion, want, resp.EnrollmentPolicyServiceURL)
		}
	}
}

func TestDiscoverFaults(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	wrongAction := strings.Replace(string(discoverWith(t, "3.0", AuthPolicyOnPremise)), ActionDiscover, ActionGetPolicies, 1)
	noEmail := strings.Replace(string(discoverWith(t, "3.0", AuthPolicyOnPremise)), "user@contoso.com", "", 1)
	cases := map[string]struct {
		body  []byte
		sub   soap.Subcode
		cause error
	}{
		"malformed":       {body: []byte("<s:Envelope"), sub: soap.SubcodeMessageFormat, cause: soap.ErrSyntax},
		"wrong body":      {body: fixture(t, "getpolicies-onprem-request.xml"), sub: soap.SubcodeMessageFormat, cause: ErrMessage},
		"wrong action":    {body: []byte(wrongAction), sub: soap.SubcodeMessageFormat},
		"no email":        {body: []byte(noEmail), sub: soap.SubcodeMessageFormat},
		"version 10.0":    {body: discoverWith(t, "10.0", AuthPolicyOnPremise), sub: soap.SubcodeMessageFormat, cause: ErrVersion},
		"version 3":       {body: discoverWith(t, "3", AuthPolicyOnPremise), sub: soap.SubcodeMessageFormat, cause: ErrVersion},
		"federated only":  {body: discoverWith(t, "3.0", AuthPolicyFederated, AuthPolicyCertificate), sub: soap.SubcodeAuthentication, cause: ErrPolicyUnsupported},
		"too large":       {body: append(discoverWith(t, "3.0", AuthPolicyOnPremise), make([]byte, soap.DefaultMaxSize)...), sub: soap.SubcodeMessageFormat, cause: soap.ErrTooLarge},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := e.svc.Discover(context.Background(), tc.body)
			wantFault(t, out, err, tc.sub, "", tc.cause)
			if !strings.Contains(string(out), ActionDiscoverResponse) {
				t.Error("fault action")
			}
		})
	}
	// A request whose header could not be read gets the placeholder RelatesTo.
	out, _ := e.svc.Discover(context.Background(), []byte("<s:Envelope"))
	if !strings.Contains(string(out), "<a:RelatesTo>urn:uuid:00000000-0000-0000-0000-000000000000</a:RelatesTo>") {
		t.Errorf("placeholder RelatesTo missing: %s", out)
	}
}

func policyRequest(t *testing.T, user, pass string) []byte {
	t.Helper()
	var creds *Credentials
	if user != "" {
		creds = &Credentials{Username: user, Password: pass}
	}
	data, err := EncodeGetPolicies("urn:uuid:p", policyURL, creds)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestGetPoliciesSuccess(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	out, err := e.svc.GetPolicies(context.Background(), fixture(t, "getpolicies-onprem-request.xml"))
	if err != nil {
		t.Fatal(err)
	}
	h, p, err := DecodePolicyResponse(out)
	if err != nil {
		t.Fatal(err)
	}
	if h.Action != ActionGetPoliciesResponse || p.MinimalKeyLength != 2048 || p.HashAlgorithm != SHA256 {
		t.Errorf("policy = %+v", p)
	}
	if !strings.Contains(string(out), "<a:RelatesTo>urn:uuid:72048B64-0F19-448F-8C2E-B4C661860AA0</a:RelatesTo>") {
		t.Error("RelatesTo")
	}
}

func TestGetPoliciesFaults(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	badType := strings.Replace(string(policyRequest(t, "user@contoso.com", "mypassword")), soap.PasswordText, "urn:digest", 1)
	bst := strings.Replace(string(policyRequest(t, "user@contoso.com", "mypassword")),
		`<wsse:UsernameToken`, `<wsse:BinarySecurityToken ValueType="x" EncodingType="y">AA==</wsse:BinarySecurityToken><wsse:UsernameToken`, 1)
	bst = strings.Replace(bst, `<wsse:UsernameToken u:Id="uuid-cc1ccc1f-2fba-4bcf-b063-ffc0cac77917-4"><wsse:Username>user@contoso.com</wsse:Username>`, "", 1)
	bst = strings.Replace(bst, `<wsse:Password wsse:Type="`+soap.PasswordText+`">mypassword</wsse:Password></wsse:UsernameToken>`, "", 1)
	wrongAction := strings.Replace(string(policyRequest(t, "user@contoso.com", "mypassword")), ActionGetPolicies, ActionRST, 1)
	cases := map[string]struct {
		body   []byte
		sub    soap.Subcode
		detail soap.ErrorType
		cause  error
	}{
		"malformed":       {body: []byte("<"), sub: soap.SubcodeMessageFormat, cause: soap.ErrSyntax},
		"wrong body":      {body: fixture(t, "discover-onprem-request.xml"), sub: soap.SubcodeMessageFormat, cause: ErrMessage},
		"wrong action":    {body: []byte(wrongAction), sub: soap.SubcodeMessageFormat},
		"no security":     {body: policyRequest(t, "", ""), sub: soap.SubcodeInvalidSecurity},
		"token not user":  {body: []byte(bst), sub: soap.SubcodeInvalidSecurity, cause: ErrPolicyUnsupported},
		"password type":   {body: []byte(badType), sub: soap.SubcodeInvalidSecurity},
		"empty username":  {body: policyRequest(t, " ", "x"), sub: soap.SubcodeInvalidSecurity},
		"wrong password":  {body: policyRequest(t, "user@contoso.com", "nope"), sub: soap.SubcodeAuthentication, cause: ErrUnauthenticated},
		"blocked user":    {body: policyRequest(t, "blocked@contoso.com", "x"), sub: soap.SubcodeAuthorization, cause: ErrUnauthorized},
		"fault from auth": {body: policyRequest(t, "fault@contoso.com", "x"), sub: soap.SubcodeAuthorization, detail: soap.ErrorDeviceCapReached},
		"directory down":  {body: policyRequest(t, "broken@contoso.com", "x"), sub: soap.SubcodeEnrollmentServer},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := e.svc.GetPolicies(context.Background(), tc.body)
			wantFault(t, out, err, tc.sub, tc.detail, tc.cause)
			if !strings.Contains(string(out), ActionGetPoliciesResponse) {
				t.Error("fault action")
			}
		})
	}
	disabled := newEnv(t, func(c *Config) { c.EnrollmentPolicyServiceURL = "" })
	out, err := disabled.svc.GetPolicies(context.Background(), policyRequest(t, "user@contoso.com", "mypassword"))
	wantFault(t, out, err, soap.SubcodeEnrollmentServer, "", ErrPolicyDisabled)
}

func TestGetPoliciesEmptyUsername(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	body := strings.Replace(string(policyRequest(t, "user@contoso.com", "mypassword")), "<wsse:Username>user@contoso.com</wsse:Username>", "<wsse:Username></wsse:Username>", 1)
	out, err := e.svc.GetPolicies(context.Background(), []byte(body))
	wantFault(t, out, err, soap.SubcodeInvalidSecurity, "", nil)
	// A UsernameToken without a Type attribute on Password is accepted.
	body = strings.Replace(string(policyRequest(t, "user@contoso.com", "mypassword")), ` wsse:Type="`+soap.PasswordText+`"`, "", 1)
	if _, err := e.svc.GetPolicies(context.Background(), []byte(body)); err != nil {
		t.Errorf("untyped password: %v", err)
	}
}

func enrollRequest(t *testing.T, items map[string]string, mutate func(body string) string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csr, err := testpki.CSR("7BA748C8-703E-4DF2-A74A-92984117346A", key)
	if err != nil {
		t.Fatal(err)
	}
	ctx := []ContextItem{{Name: "OSEdition", Value: "4"}, {Name: "OSVersion", Value: "10.0.26100.0"}, {Name: "DeviceName", Value: "PC-1"},
		{Name: "EnrollmentType", Value: "Full"}, {Name: "DeviceType", Value: "CIMClient_Windows"}, {Name: "ApplicationVersion", Value: "10.0.26100.0"},
		{Name: "DeviceID", Value: "7BA748C8-703E-4DF2-A74A-92984117346A"}, {Name: "HWDevID", Value: strings.Repeat("A", 64)}, {Name: "RequestVersion", Value: "5.0"}}
	for i := range ctx {
		if v, ok := items[ctx[i].Name]; ok {
			ctx[i].Value = v
		}
	}
	var kept []ContextItem
	for _, it := range ctx {
		if v, ok := items[it.Name]; !ok || v != "-" {
			kept = append(kept, it)
		}
	}
	data, err := EncodeRequestSecurityToken("urn:uuid:e", enrollURL, &Credentials{Username: "user@contoso.com", Password: "mypassword"}, csr, kept)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		return []byte(mutate(string(data)))
	}
	return data
}

func TestEnrollSuccessFixture(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	body, _ := rstFixture(t)
	out, err := e.svc.Enroll(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	h, docBytes, err := DecodeTokenResponse(out)
	if err != nil {
		t.Fatal(err)
	}
	if h.Action != ActionRSTRC || !strings.Contains(string(out), "<a:RelatesTo>urn:uuid:0d5a1441-5891-453b-becf-a2e5f6ea3749</a:RelatesTo>") {
		t.Errorf("header: %s", out)
	}
	if !strings.Contains(string(out), "<u:Created>2026-09-07T10:00:00.000Z</u:Created><u:Expires>2026-09-07T10:05:00.000Z</u:Expires>") {
		t.Error("timestamp")
	}
	doc, err := wapprov.Decode(docBytes)
	if err != nil {
		t.Fatal(err)
	}
	cert := e.rec.last.Certificate
	cs := doc.Find(wapprov.TypeCertificateStore)
	if cs.Path("Root", "System", wapprov.Thumbprint(e.issuer.ca.Cert.Raw)) == nil {
		t.Error("root missing")
	}
	entry := cs.Path("My", "User", wapprov.Thumbprint(cert.Raw))
	if entry == nil || entry.Value("EncodedCertificate") != base64.StdEncoding.EncodeToString(cert.Raw) {
		t.Errorf("client certificate missing under My/User: %+v", cs)
	}
	if cs.Path("My", "WSTEP", "Renew").Value("RenewPeriod") != "60" {
		t.Error("renew")
	}
	app := doc.Find(wapprov.TypeApplication)
	if app.Value("PROVIDER-ID") != "TestServer" || app.Value("ADDR") != "https://mdm.contoso.com/ManagementServer/MDM.svc" || len(app.Children) != 2 {
		t.Errorf("application: %+v", app)
	}
	dm := doc.Find(wapprov.TypeDMClient).Path("Provider", "TestServer")
	if dm.Value("UPN") != "user@contoso.com" || dm.Value("EntDeviceName") != "MY_WINDOWS_DEVICE" || dm.Child("Poll").Value("NumberOfFirstRetries") != "5" {
		t.Errorf("dmclient: %+v", dm)
	}
	rec := e.rec.last
	if rec.Request.Context.DeviceID != "7BA748C8-703E-4DF2-A74A-92984117346A" || rec.Request.Principal.UPN != "user@contoso.com" ||
		rec.Store != wapprov.StoreUser || !rec.EnrolledAt.Equal(t0) || rec.TraceID != "trace-1" || len(rec.Chain) != 1 || e.rec.doc != nil && e.rec.doc.Find(wapprov.TypeDMClient) == nil {
		t.Errorf("recorded = %+v", rec)
	}
	if cert.Subject.CommonName != "7BA748C8-703E-4DF2-A74A-92984117346A" {
		t.Errorf("subject = %v", cert.Subject)
	}
	if e.issuer.last.Header.MessageID != "urn:uuid:0d5a1441-5891-453b-becf-a2e5f6ea3749" {
		t.Error("issuer did not get the header")
	}
}

func TestEnrollDeviceGoesToSystemStore(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	out, err := e.svc.Enroll(context.Background(), enrollRequest(t, map[string]string{"EnrollmentType": "Device"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	_, docBytes, err := DecodeTokenResponse(out)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := wapprov.Decode(docBytes)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Find(wapprov.TypeCertificateStore).Path("My", "System", "PrivateKeyContainer") == nil {
		t.Error("device certificate not under My/System")
	}
	if _, ok := doc.Find(wapprov.TypeDMClient).Path("Provider", "TestServer").Parm("UPN"); ok {
		t.Error("device enrollment carries a UPN")
	}
	if e.rec.last.Store != wapprov.StoreSystem {
		t.Error("recorded store")
	}
}

func TestEnrollWithoutRecorder(t *testing.T) {
	t.Parallel()
	e := newEnv(t, func(c *Config) { c.Recorder = nil })
	if _, err := e.svc.Enroll(context.Background(), enrollRequest(t, nil, nil)); err != nil {
		t.Fatal(err)
	}
}

func TestEnrollFaults(t *testing.T) {
	t.Parallel()
	replace := func(old, repl string) func(string) string {
		return func(s string) string { return strings.Replace(s, old, repl, 1) }
	}
	e := newEnv(t, nil)
	cases := map[string]struct {
		body   []byte
		sub    soap.Subcode
		detail soap.ErrorType
		cause  error
	}{
		"malformed":         {body: []byte("<"), sub: soap.SubcodeMessageFormat, cause: soap.ErrSyntax},
		"wrong body":        {body: fixture(t, "discover-onprem-request.xml"), sub: soap.SubcodeMessageFormat, cause: ErrMessage},
		"wrong action":      {body: enrollRequest(t, nil, replace(ActionRST, ActionGetPolicies)), sub: soap.SubcodeMessageFormat},
		"bad password":      {body: enrollRequest(t, nil, replace(">mypassword<", ">nope<")), sub: soap.SubcodeAuthentication, cause: ErrUnauthenticated},
		"token type":        {body: enrollRequest(t, nil, replace(TokenTypeDeviceEnrollment, "urn:other")), sub: soap.SubcodeMessageFormat},
		"renew":             {body: enrollRequest(t, nil, replace(RequestTypeIssue, RequestTypeRenew)), sub: soap.SubcodeCertificateRequest, detail: soap.ErrorNotEligibleToRenew, cause: ErrRenew},
		"request type":      {body: enrollRequest(t, nil, replace(RequestTypeIssue, "urn:other")), sub: soap.SubcodeMessageFormat},
		"pkcs7":             {body: enrollRequest(t, nil, replace(ValueTypePKCS10, ValueTypePKCS7)), sub: soap.SubcodeCertificateRequest, detail: soap.ErrorNotEligibleToRenew, cause: ErrPKCS7},
		"value type":        {body: enrollRequest(t, nil, replace(ValueTypePKCS10, "urn:other")), sub: soap.SubcodeMessageFormat},
		"no token":          {body: enrollRequest(t, nil, replace(`ValueType="`+ValueTypePKCS10+`"`, `ValueType=""`)), sub: soap.SubcodeMessageFormat},
		"bad base64":        {body: enrollRequest(t, nil, replace(`#base64binary">`, `#base64binary">!!!`)), sub: soap.SubcodeMessageFormat, cause: soap.ErrEncoding},
		"no device id":      {body: enrollRequest(t, map[string]string{"DeviceID": "-"}, nil), sub: soap.SubcodeMessageFormat, detail: soap.ErrorInvalidEnrollmentData},
		"enrollment type":   {body: enrollRequest(t, map[string]string{"EnrollmentType": "Partial"}, nil), sub: soap.SubcodeMessageFormat, detail: soap.ErrorInvalidEnrollmentData},
		"no enrollment type": {body: enrollRequest(t, map[string]string{"EnrollmentType": "-"}, nil), sub: soap.SubcodeMessageFormat, detail: soap.ErrorInvalidEnrollmentData},
		"request version":   {body: enrollRequest(t, map[string]string{"RequestVersion": "12.0"}, nil), sub: soap.SubcodeMessageFormat, detail: soap.ErrorInvalidEnrollmentData, cause: ErrVersion},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := e.svc.Enroll(context.Background(), tc.body)
			wantFault(t, out, err, tc.sub, tc.detail, tc.cause)
			if !strings.Contains(string(out), ActionRSTRC) {
				t.Error("fault action")
			}
		})
	}
}

func TestEnrollIssuerAndBackendFaults(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	cases := map[string]struct {
		mutate func(e *env)
		sub    soap.Subcode
		detail soap.ErrorType
		cause  error
	}{
		"issuer rejects":    {mutate: func(e *env) { e.issuer.err = boom }, sub: soap.SubcodeCertificateRequest, cause: boom},
		"issuer forbids":    {mutate: func(e *env) { e.issuer.err = ErrUnauthorized }, sub: soap.SubcodeAuthorization, cause: ErrUnauthorized},
		"issuer fault":      {mutate: func(e *env) { e.issuer.err = soap.NewFault(soap.SubcodeAuthorization, "cap").WithDetail(soap.ErrorDeviceCapReached, "") }, sub: soap.SubcodeAuthorization, detail: soap.ErrorDeviceCapReached},
		"issuer nil":        {mutate: func(e *env) { e.issuer.nilR = true }, sub: soap.SubcodeCertificateRequest},
		"recorder fails":    {mutate: func(e *env) { e.rec.err = boom }, sub: soap.SubcodeEnrollmentServer, cause: boom},
		"recorder fault":    {mutate: func(e *env) { e.rec.err = soap.NewFault(soap.SubcodeAuthorization, "x").WithDetail(soap.ErrorUserLicense, "") }, sub: soap.SubcodeAuthorization, detail: soap.ErrorUserLicense},
		"provisioner fails": {mutate: func(e *env) { e.svc.cfg.Provisioner = &fakeProvisioner{err: boom} }, sub: soap.SubcodeEnrollmentServer, cause: boom},
		"provisioner fault": {mutate: func(e *env) { e.svc.cfg.Provisioner = &fakeProvisioner{err: soap.NewFault(soap.SubcodeEnrollmentServer, "x").WithDetail(soap.ErrorInMaintenance, "")} }, sub: soap.SubcodeEnrollmentServer, detail: soap.ErrorInMaintenance},
		"invalid document":  {mutate: func(e *env) { e.svc.cfg.Provisioner = &fakeProvisioner{doc: &wapprov.Document{}} }, sub: soap.SubcodeEnrollmentServer, cause: wapprov.ErrInvalid},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, nil)
			tc.mutate(e)
			out, err := e.svc.Enroll(context.Background(), enrollRequest(t, nil, nil))
			wantFault(t, out, err, tc.sub, tc.detail, tc.cause)
		})
	}
}

func TestFaultFallsBackOnUnencodableFault(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	out, err := e.svc.fault(&soap.Fault{Subcode: "s:Bogus", Reason: "x"}, ActionRSTRC, nil, "trace-1")
	f, ok := IsFault(err)
	if !ok || f.Subcode != soap.SubcodeInternalServiceFault {
		t.Errorf("fault = %v", err)
	}
	d, derr := soap.DecodeFault(out)
	if derr != nil || d.Subcode != string(soap.SubcodeInternalServiceFault) {
		t.Errorf("wire = %+v, %v", d, derr)
	}
	if _, ok := IsFault(errors.New("plain")); ok {
		t.Error("plain error is not a fault")
	}
}
