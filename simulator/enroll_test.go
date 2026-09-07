package simulator

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/soap"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
	"github.com/deploymenttheory/go-microsoft-dm/pki/ca"
	"github.com/deploymenttheory/go-microsoft-dm/pki/wstep"
	"github.com/deploymenttheory/go-microsoft-dm/pki/xcep"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
)

var t0 = time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

// testServer is the in-memory enrollment server: CA, WSTEP issuer,
// in-memory store and the enrollment handler behind an httptest TLS server.
type testServer struct {
	srv       *httptest.Server
	ca        *ca.Local
	store     *inmem.Store
	conflicts []storage.Conflict
	mu        sync.Mutex
	cfg       *enroll.Config
}

func newTestServer(t *testing.T, mutate func(*enroll.Config)) *testServer {
	t.Helper()
	ts := &testServer{store: inmem.New()}
	var handler http.Handler
	ts.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
	t.Cleanup(ts.srv.Close)
	authority, err := ca.NewSelfSigned(ca.SelfSignedOptions{Clock: clock.NewFake(t0)})
	if err != nil {
		t.Fatal(err)
	}
	ts.ca = authority
	issuer, err := wstep.NewIssuer(authority)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := xcep.FromPolicy(authority.Policy(), xcep.Options{CommonName: "Sim CA"})
	if err != nil {
		t.Fatal(err)
	}
	prov, err := enroll.NewProvisioner(enroll.ProvisionConfig{
		ManagementURL: ts.srv.URL + "/ManagementServer/MDM.svc", ProviderID: "SimServer", Name: "Simulator",
		Credentials: enroll.CredentialSourceFunc(func(_ context.Context, e *enroll.Enrollment) (wapprov.Credential, wapprov.Credential, error) {
			return wapprov.Credential{Type: wapprov.AuthDigest, Name: e.Request.Context.DeviceID, Secret: "device-secret", Nonce: []byte{1, 2, 3}},
				wapprov.Credential{Type: wapprov.AuthDigest, Secret: "server-secret", Nonce: []byte{4, 5, 6}}, nil
		}),
		EntDMID: func(e *enroll.Enrollment) string { return "dm-" + e.Request.Context.DeviceID },
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := enroll.Config{
		EnrollmentServiceURL:       ts.srv.URL + "/EnrollmentServer/Enrollment.svc",
		EnrollmentPolicyServiceURL: ts.srv.URL + "/EnrollmentServer/Policy.svc",
		Authenticator: enroll.AuthenticatorFunc(func(_ context.Context, c enroll.Credentials) (enroll.Principal, error) {
			if c.Username == "user@contoso.com" && c.Password == "pw" {
				return enroll.Principal{UPN: c.Username}, nil
			}
			return enroll.Principal{}, enroll.ErrUnauthenticated
		}),
		Issuer: issuer, Provisioner: prov, Policy: policy, Clock: clock.NewFake(t0),
		Recorder: &storage.Recorder{Store: ts.store, Conflicts: storage.ConflictSinkFunc(func(_ context.Context, c storage.Conflict) error {
			ts.mu.Lock()
			defer ts.mu.Unlock()
			ts.conflicts = append(ts.conflicts, c)
			return nil
		})},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	ts.cfg = &cfg
	svc, err := enroll.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handler = enroll.NewHandler(svc)
	return ts
}

func (ts *testServer) client(deviceID string) Client {
	return Client{
		HTTP: ts.srv.Client(), DiscoveryURL: ts.srv.URL + enroll.DiscoveryPath,
		Email: "user@contoso.com", Password: "pw", DeviceID: deviceID, HWDevID: strings.Repeat("A", 64),
	}
}

func TestEnrollFull(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, nil)
	c := ts.client("7BA748C8-703E-4DF2-A74A-92984117346A")
	c.WindowsSubject = true
	e, err := Enroll(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if e.Discovery.AuthPolicy != enroll.AuthPolicyOnPremise || e.Discovery.EnrollmentVersion != "3.0" {
		t.Errorf("discovery = %+v", e.Discovery)
	}
	if e.Policy == nil || e.Policy.CommonName != "Sim CA" || e.Policy.MinimalKeyLength != 2048 {
		t.Errorf("policy = %+v", e.Policy)
	}
	if e.Certificate == nil || e.Store != wapprov.StoreUser || !strings.Contains(e.Certificate.Subject.CommonName, "!") {
		t.Errorf("certificate = %+v store %s", e.Certificate.Subject, e.Store)
	}
	if !e.Certificate.NotBefore.Equal(t0) {
		t.Errorf("NotBefore = %s", e.Certificate.NotBefore)
	}
	if err := e.Certificate.CheckSignatureFrom(ts.ca.Certificate()); err != nil {
		t.Error(err)
	}
	if len(e.Roots) != 1 || !e.Roots[0].Equal(ts.ca.Certificate()) || len(e.Intermediates) != 0 {
		t.Errorf("chain = %d roots %d intermediates", len(e.Roots), len(e.Intermediates))
	}
	if e.Renew == nil || !e.Renew.ROBOSupport || e.Renew.RenewPeriod != 60 || e.Renew.RetryInterval != 4 {
		t.Errorf("renew = %+v", e.Renew)
	}
	acct := e.Account
	if acct.ProviderID != "SimServer" || acct.Name != "Simulator" || acct.Address != ts.srv.URL+"/ManagementServer/MDM.svc" {
		t.Errorf("account = %+v", acct)
	}
	if acct.ServerAuth.Type != wapprov.AuthDigest || acct.ServerAuth.Name != c.DeviceID || acct.ServerAuth.Secret != "device-secret" || string(acct.ServerAuth.Nonce) != "\x01\x02\x03" {
		t.Errorf("server auth = %+v", acct.ServerAuth)
	}
	if acct.ClientAuth.Type != wapprov.AuthDigest || acct.ClientAuth.Secret != "server-secret" || string(acct.ClientAuth.Nonce) != "\x04\x05\x06" {
		t.Errorf("client auth = %+v", acct.ClientAuth)
	}
	dm := e.DMClient
	if dm.ProviderID != "SimServer" || dm.UPN != "user@contoso.com" || dm.EntDeviceName != "DESKTOP-SIM" || dm.EntDMID != "dm-"+c.DeviceID {
		t.Errorf("dmclient = %+v", dm)
	}
	if dm.Poll != wapprov.DefaultPoll() {
		t.Errorf("poll = %+v", dm.Poll)
	}
	// The server recorded it.
	rec, err := ts.store.Get(context.Background(), c.DeviceID)
	if err != nil || rec.Serial != e.Certificate.SerialNumber.String() || rec.UPN != "user@contoso.com" || rec.HWDevID != c.HWDevID {
		t.Errorf("stored = %+v, %v", rec, err)
	}
	cert, err := ts.store.CertificateByThumbprint(context.Background(), wapprov.Thumbprint(e.Certificate.Raw))
	if err != nil || cert.DeviceID != c.DeviceID {
		t.Errorf("stored certificate = %+v, %v", cert, err)
	}
}

func TestEnrollDeviceAndDuplicateHWDevID(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, nil)
	c := ts.client("DEVICE-A")
	c.EnrollmentType = enroll.EnrollmentTypeDevice
	c.SkipPolicy = true
	c.Key = func() *ecdsa.PrivateKey {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}()
	e, err := Enroll(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if e.Policy != nil || e.Store != wapprov.StoreSystem || e.DMClient.UPN != "" {
		t.Errorf("device enrollment = policy %v store %s upn %q", e.Policy, e.Store, e.DMClient.UPN)
	}
	if _, ok := e.Certificate.PublicKey.(*ecdsa.PublicKey); !ok {
		t.Errorf("key type %T", e.Certificate.PublicKey)
	}
	// Same hardware, different DeviceID: recorded and reported.
	c2 := ts.client("DEVICE-B")
	if _, err := Enroll(context.Background(), c2); err != nil {
		t.Fatal(err)
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.conflicts) != 1 || ts.conflicts[0].New.DeviceID != "DEVICE-B" || ts.conflicts[0].Existing[0].DeviceID != "DEVICE-A" {
		t.Errorf("conflicts = %+v", ts.conflicts)
	}
	if _, err := ts.store.Get(context.Background(), "DEVICE-B"); err != nil {
		t.Error("conflicting device was not enrolled")
	}
}

func TestEnrollFaults(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, nil)
	c := ts.client("DEVICE-A")
	c.Password = "wrong"
	_, err := Enroll(context.Background(), c)
	var f *soap.DecodedFault
	if !errors.As(err, &f) || f.Subcode != string(soap.SubcodeAuthentication) {
		t.Errorf("wrong password: %v", err)
	}
	c = ts.client("DEVICE-A")
	c.SkipPolicy = true
	c.Password = "wrong"
	if _, err := Enroll(context.Background(), c); !errors.As(err, &f) || f.Subcode != string(soap.SubcodeAuthentication) {
		t.Errorf("wrong password at RST: %v", err)
	}
	c = ts.client("DEVICE-A")
	c.RequestVersion = "3.0"
	c.Extra = []enroll.ContextItem{{Name: "EnrollmentType", Value: "Partial"}}
	_, err = Enroll(context.Background(), c)
	if !errors.As(err, &f) || f.Detail == nil || f.Detail.ErrorType != string(soap.ErrorInvalidEnrollmentData) {
		t.Errorf("invalid enrollment data: %v", err)
	}
	// Discovery refuses a version outside the range.
	c = ts.client("DEVICE-A")
	c.RequestVersion = "3"
	if _, err := Enroll(context.Background(), c); !errors.Is(err, ErrConfig) {
		t.Errorf("bad version: %v", err)
	}
}

func TestEnrollConfigErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]Client{
		"no email":    {Password: "p", DeviceID: "d"},
		"no password": {Email: "u@c", DeviceID: "d"},
		"no device":   {Email: "u@c", Password: "p"},
		"bad email":   {Email: "user", Password: "p", DeviceID: "d"},
		"bad type":    {Email: "u@c", Password: "p", DeviceID: "d", EnrollmentType: "Half", DiscoveryURL: "https://x"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := Enroll(context.Background(), c); !errors.Is(err, ErrConfig) {
				t.Errorf("err = %v", err)
			}
		})
	}
	c := Client{Email: "user@Contoso.COM", Password: "p", DeviceID: "d"}
	if err := c.defaults(); err != nil || c.DiscoveryURL != "https://enterpriseenrollment.contoso.com/EnrollmentServer/Discovery.svc" {
		t.Errorf("derived URL = %q, %v", c.DiscoveryURL, err)
	}
}

func TestEnrollProtocolErrors(t *testing.T) {
	t.Parallel()
	// A server that answers the probe with 404.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	t.Cleanup(bad.Close)
	c := Client{HTTP: bad.Client(), DiscoveryURL: bad.URL + enroll.DiscoveryPath, Email: "u@c", Password: "p", DeviceID: "d"}
	if _, err := Enroll(context.Background(), c); !errors.Is(err, ErrProtocol) {
		t.Errorf("probe 404: %v", err)
	}
	// A server that chunks its discovery answer.
	chunked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			return
		}
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Content-Type", enroll.ContentType)
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte(strings.Repeat("y", 100)))
	}))
	t.Cleanup(chunked.Close)
	c.HTTP, c.DiscoveryURL = chunked.Client(), chunked.URL+enroll.DiscoveryPath
	if _, err := Enroll(context.Background(), c); !errors.Is(err, ErrProtocol) {
		t.Errorf("chunked: %v", err)
	}
	// A server that returns garbage with status 200.
	garbage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Length", "5")
			_, _ = w.Write([]byte("<nope"))
		}
	}))
	t.Cleanup(garbage.Close)
	c.HTTP, c.DiscoveryURL = garbage.Client(), garbage.URL+enroll.DiscoveryPath
	if _, err := Enroll(context.Background(), c); !errors.Is(err, soap.ErrSyntax) {
		t.Errorf("garbage: %v", err)
	}
	// A server that answers discovery with a status the client does not expect.
	teapot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	t.Cleanup(teapot.Close)
	c.HTTP, c.DiscoveryURL = teapot.Client(), teapot.URL+enroll.DiscoveryPath
	if _, err := Enroll(context.Background(), c); !errors.Is(err, ErrProtocol) {
		t.Errorf("teapot: %v", err)
	}
	// A server whose discovery advertises federated authentication.
	fedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			return
		}
		out, _ := enroll.EncodeDiscoverResponse(&enroll.DiscoverResponse{AuthPolicy: enroll.AuthPolicyFederated, EnrollmentServiceURL: "https://x", AuthenticationServiceURL: "https://sts"}, "r", "")
		w.Header().Set("Content-Length", strconv.Itoa(len(out)))
		_, _ = w.Write(out)
	}))
	t.Cleanup(fedSrv.Close)
	c.HTTP, c.DiscoveryURL = fedSrv.Client(), fedSrv.URL+enroll.DiscoveryPath
	if _, err := Enroll(context.Background(), c); !errors.Is(err, ErrProtocol) {
		t.Errorf("federated: %v", err)
	}
	// Discovery without an enrollment URL.
	noURL := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			return
		}
		out, _ := soap.Encode(soap.Response{Action: enroll.ActionDiscoverResponse, RelatesTo: "r", Body: []byte(`<DiscoverResponse xmlns="` + enroll.NamespaceDiscovery + `"><DiscoverResult><AuthPolicy>OnPremise</AuthPolicy></DiscoverResult></DiscoverResponse>`)})
		w.Header().Set("Content-Length", strconv.Itoa(len(out)))
		_, _ = w.Write(out)
	}))
	t.Cleanup(noURL.Close)
	c.HTTP, c.DiscoveryURL = noURL.Client(), noURL.URL+enroll.DiscoveryPath
	if _, err := Enroll(context.Background(), c); !errors.Is(err, ErrProtocol) {
		t.Errorf("no enrollment url: %v", err)
	}
	// An unreachable server.
	c.HTTP, c.DiscoveryURL = http.DefaultClient, "https://127.0.0.1:1"+enroll.DiscoveryPath
	if _, err := Enroll(context.Background(), c); err == nil {
		t.Error("unreachable server succeeded")
	}
}

func TestApplyProvisioningErrors(t *testing.T) {
	t.Parallel()
	authority, err := ca.NewSelfSigned(ca.SelfSignedOptions{Clock: clock.NewFake(t0)})
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := authority.Issue(context.Background(), ca.Request{PublicKey: key.Public()})
	if err != nil {
		t.Fatal(err)
	}
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cs := func(client []byte) wapprov.Characteristic {
		c, err := wapprov.CertificateStore(wapprov.CertificateStoreConfig{Roots: [][]byte{authority.Certificate().Raw}, Client: client, Store: wapprov.StoreUser})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	app := func(mutate func(*wapprov.Characteristic)) wapprov.Characteristic {
		a, err := wapprov.Application(wapprov.ApplicationConfig{ProviderID: "P", Address: "https://m",
			ServerAuth: wapprov.Credential{Type: wapprov.AuthBasic, Name: "n", Secret: "s"}, ClientAuth: wapprov.Credential{Type: wapprov.AuthDigest, Secret: "s", Nonce: []byte{1}}})
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(&a)
		}
		return a
	}
	dm := func(mutate func(*wapprov.Characteristic)) wapprov.Characteristic {
		d, err := wapprov.DMClient(wapprov.DMClientConfig{ProviderID: "P"})
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(&d)
		}
		return d
	}
	setParm := func(c *wapprov.Characteristic, name, value string) {
		for i := range c.Parms {
			if c.Parms[i].Name == name {
				c.Parms[i].Value = value
				return
			}
		}
		c.Parms = append(c.Parms, wapprov.Parm{Name: name, Value: value})
	}
	good := &wapprov.Document{Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(nil), dm(nil)}}
	var e Enrollment
	if err := e.apply(good, key); err != nil {
		t.Fatalf("good document: %v", err)
	}
	if e.Certificate == nil || e.Renew != nil || e.Account.ClientAuth.Nonce[0] != 1 {
		t.Errorf("applied = %+v", e)
	}
	cases := map[string]*wapprov.Document{
		"wrong key":       {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(mustCert(t, authority, other)), app(nil), dm(nil)}},
		"no application":  {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), dm(nil)}},
		"no dmclient":     {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(nil)}},
		"wrong provider":  {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(func(a *wapprov.Characteristic) { setParm(a, "PROVIDER-ID", "Q") }), dm(nil)}},
		"no address":      {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(func(a *wapprov.Characteristic) { setParm(a, "ADDR", "") }), dm(nil)}},
		"bad level":       {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(func(a *wapprov.Characteristic) { setParm(&a.Children[0], "AAUTHLEVEL", "OTHER") }), dm(nil)}},
		"missing client":  {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(func(a *wapprov.Characteristic) { a.Children = a.Children[:1] }), dm(nil)}},
		"no secret":       {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(func(a *wapprov.Characteristic) { setParm(&a.Children[0], "AAUTHSECRET", "") }), dm(nil)}},
		"bad nonce":       {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(func(a *wapprov.Characteristic) { setParm(&a.Children[1], "AAUTHDATA", "!!!") }), dm(nil)}},
		"no poll":         {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(nil), dm(func(d *wapprov.Characteristic) { d.Children[0].Children[0].Children = nil })}},
		"bad poll int":    {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(nil), dm(func(d *wapprov.Characteristic) { setParm(&d.Children[0].Children[0].Children[0], "NumberOfFirstRetries", "x") })}},
		"bad poll bool":   {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(nil), dm(func(d *wapprov.Characteristic) { setParm(&d.Children[0].Children[0].Children[0], "PollOnLogin", "x") })}},
		"unknown poll":    {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(nil), dm(func(d *wapprov.Characteristic) { setParm(&d.Children[0].Children[0].Children[0], "Mystery", "1") })}},
		"no root":         {Version: "1.1", Characteristics: []wapprov.Characteristic{{Type: wapprov.TypeCertificateStore, Children: cs(leaf.Raw).Children[1:]}, app(nil), dm(nil)}},
		"bad root base64": {Version: "1.1", Characteristics: []wapprov.Characteristic{cs(leaf.Raw), app(nil), dm(nil)}},
	}
	setParm(&cases["bad root base64"].Characteristics[0].Children[0].Children[0].Children[0], "EncodedCertificate", "!!!")
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var e Enrollment
			if err := e.apply(doc, key); !errors.Is(err, ErrProvisioning) {
				t.Errorf("err = %v", err)
			}
		})
	}
	// A store entry whose name is not its thumbprint, and one whose DER is not a certificate.
	mismatch := cs(leaf.Raw)
	mismatch.Children[0].Children[0].Children[0].Type = "0000"
	var e2 Enrollment
	if err := e2.apply(&wapprov.Document{Version: "1.1", Characteristics: []wapprov.Characteristic{mismatch, app(nil), dm(nil)}}, key); !errors.Is(err, ErrProvisioning) {
		t.Errorf("thumbprint mismatch: %v", err)
	}
	junk := cs(leaf.Raw)
	setParm(&junk.Children[0].Children[0].Children[0], "EncodedCertificate", "AAAA")
	if err := e2.apply(&wapprov.Document{Version: "1.1", Characteristics: []wapprov.Characteristic{junk, app(nil), dm(nil)}}, key); !errors.Is(err, ErrProvisioning) {
		t.Errorf("junk der: %v", err)
	}
	// Poll booleans and intermediates are read.
	full := cs(leaf.Raw)
	inter, err := wapprov.CertificateStore(wapprov.CertificateStoreConfig{Roots: [][]byte{authority.Certificate().Raw}, Intermediates: [][]byte{authority.Certificate().Raw}, Client: leaf.Raw, Store: wapprov.StoreUser,
		Renew: &wapprov.Renew{ROBOSupport: true, RenewPeriod: 42, RetryInterval: 5, ServerURL: "https://renew"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = full
	poll := wapprov.DefaultPoll()
	poll.PollOnLogin, poll.AllUsersPollOnFirstLogin = true, true
	dmFull, err := wapprov.DMClient(wapprov.DMClientConfig{ProviderID: "P", Poll: &poll})
	if err != nil {
		t.Fatal(err)
	}
	var e3 Enrollment
	if err := e3.apply(&wapprov.Document{Version: "1.1", Characteristics: []wapprov.Characteristic{inter, app(nil), dmFull}}, key); err != nil {
		t.Fatal(err)
	}
	if len(e3.Intermediates) != 1 || e3.Renew == nil || e3.Renew.RenewPeriod != 42 || e3.Renew.ServerURL != "https://renew" || !e3.DMClient.Poll.PollOnLogin || !e3.DMClient.Poll.AllUsersPollOnFirstLogin {
		t.Errorf("full = %+v renew %+v poll %+v", e3.Intermediates, e3.Renew, e3.DMClient.Poll)
	}
}

func mustCert(t *testing.T, authority *ca.Local, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	c, err := authority.Issue(context.Background(), ca.Request{PublicKey: key.Public()})
	if err != nil {
		t.Fatal(err)
	}
	return c.Raw
}
