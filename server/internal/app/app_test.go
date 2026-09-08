package app_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
	"github.com/deploymenttheory/go-microsoft-dm/pki/ca"
	"github.com/deploymenttheory/go-microsoft-dm/server/internal/app"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
	"github.com/deploymenttheory/go-microsoft-dm/simulator"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

type lazy struct {
	mu sync.RWMutex
	h  http.Handler
}

func (l *lazy) set(h http.Handler) { l.mu.Lock(); l.h = h; l.mu.Unlock() }
func (l *lazy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.mu.RLock()
	h := l.h
	l.mu.RUnlock()
	if h == nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	h.ServeHTTP(w, r)
}

const deviceID = "F717C0F0-5F68-4AC3-A341-01B2544219DF"

// newApp starts an app with an isolated SQLite database behind a TLS test server.
func newApp(t *testing.T) (*app.App, *httptest.Server) {
	t.Helper()
	l := &lazy{}
	srv := httptest.NewTLSServer(l)
	t.Cleanup(srv.Close)
	var logged int
	a, err := app.New(context.Background(), app.Config{
		Store: sqlstore.SQLite, DSN: "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "app.db")),
		BaseURL: srv.URL, ProviderID: "app-test", Name: "App Test",
		Role: app.RoleAll, EnrollAllowAny: true,
	}, app.Options{Clock: clock.Real{}, Log: func(string, string, int) { logged++ }})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	l.set(a.Handler)
	return a, srv
}

func TestAppEndToEnd(t *testing.T) {
	t.Parallel()
	a, srv := newApp(t)
	ctx := context.Background()

	res, err := simulator.Enroll(ctx, simulator.Client{
		HTTP: srv.Client(), DiscoveryURL: srv.URL + enroll.DiscoveryPath,
		Email: "u@contoso.com", Password: "pw", DeviceID: deviceID, HWDevID: strings.Repeat("A", 64), WindowsSubject: true,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if res.Account.ProviderID != "app-test" {
		t.Errorf("provider = %s", res.Account.ProviderID)
	}
	// Queue a policy and drive a session.
	policy, _ := mdm.NewReplace("./Vendor/MSFT/Policy/Config/Camera/AllowCamera", "0", mdm.WithFormat(syncml.FormatInt))
	pq, err := a.Store.Queue().Enqueue(ctx, deviceID, policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	dev := &simulator.Device{
		HTTP: srv.Client(), ManagementURL: res.Account.Address, DeviceID: deviceID,
		Username: res.Account.ServerAuth.Name, Password: res.Account.ServerAuth.Secret, LoginStatus: syncml.LoginStatusUser,
	}
	if _, err := dev.RunSession(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	if c, err := a.Store.Queue().Get(ctx, deviceID, pq.ID); err != nil || c.State != mdm.StateAcknowledged {
		t.Errorf("policy state = %v, %v", c.State, err)
	}
	if facts, err := a.Store.Facts(ctx, deviceID); err != nil || facts.LoginStatus != syncml.LoginStatusUser {
		t.Errorf("facts = %+v, %v", facts, err)
	}
	evs, err := a.Store.Events(ctx, deviceID, 0)
	if err != nil || len(evs) == 0 {
		t.Errorf("events = %d, %v", len(evs), err)
	}
	// Unenroll via a generic alert.
	dev.Generics = []simulator.GenericAlert{{Type: syncml.AlertTypeUnenrollmentUserRequest, Format: syncml.FormatInt, Data: "1"}}
	if _, err := dev.RunSession(ctx, "2"); err != nil {
		t.Fatal(err)
	}
	list, _ := a.Store.ListByHWDevID(ctx, strings.Repeat("A", 64))
	if len(list) == 0 || list[len(list)-1].State != storage.StateUnenrolled {
		t.Errorf("not unenrolled: %+v", list)
	}
}

// Enrollment event 56 on Windows 11 build 26200.9278 rejects a top-level
// RootCATrustedCertificates characteristic. The root is already installed
// through CertificateStore, as in MS-MDE2 2.2.9.1 and Fleet's provisioning.
func TestEnrollmentErrorCodesProvisioningUsesCertificateStore(t *testing.T) {
	t.Parallel()
	_, srv := newApp(t)
	res, err := simulator.Enroll(context.Background(), simulator.Client{
		HTTP: srv.Client(), DiscoveryURL: srv.URL + enroll.DiscoveryPath,
		Email: "u@contoso.com", Password: "pw", DeviceID: deviceID, WindowsSubject: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{wapprov.TypeCertificateStore, wapprov.TypeApplication, wapprov.TypeDMClient}
	if len(res.Document.Characteristics) != len(want) {
		t.Fatalf("provisioning has %d characteristics, want CertificateStore, APPLICATION, DMClient", len(res.Document.Characteristics))
	}
	for i, typ := range want {
		if res.Document.Characteristics[i].Type != typ {
			t.Errorf("characteristic %d = %q, want %q", i, res.Document.Characteristics[i].Type, typ)
		}
	}
	if len(res.Roots) != 1 || res.Document.Find(wapprov.TypeCertificateStore).Path("Root", "System", wapprov.Thumbprint(res.Roots[0].Raw)) == nil {
		t.Fatal("enrollment root is missing from CertificateStore/Root/System")
	}
	// Without these initial nonces the native Windows client rejected account
	// configuration with 0x80070057, although simulator enrollment succeeded.
	for _, auth := range res.Document.Find(wapprov.TypeApplication).Children {
		nonce, err := base64.StdEncoding.DecodeString(auth.Value("AAUTHDATA"))
		if err != nil || len(nonce) != 16 {
			t.Fatalf("%s needs a base64-encoded 16-byte nonce", auth.Value("AAUTHLEVEL"))
		}
		parsed := res.Account.ServerAuth.Nonce
		if auth.Value("AAUTHLEVEL") == "CLIENT" {
			parsed = res.Account.ClientAuth.Nonce
		}
		if !bytes.Equal(nonce, parsed) {
			t.Fatal("simulator did not preserve the provisioned nonce")
		}
	}
	if bytes.Equal(res.Account.ServerAuth.Nonce, res.Account.ClientAuth.Nonce) {
		t.Fatal("CLIENT and APPSRV must receive independent nonces")
	}
}

func TestNewErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Invalid config (no enroll auth configured).
	if _, err := app.New(ctx, app.Config{Store: sqlstore.SQLite, Memory: true, BaseURL: "https://x", Role: app.RoleAll}, app.Options{}); err == nil {
		t.Error("missing enroll auth accepted")
	}
	// Mismatched CA files.
	_, err := app.New(ctx, app.Config{Store: sqlstore.SQLite, Memory: true, BaseURL: "https://x", Role: app.RoleAll, EnrollAllowAny: true, CACert: "only-cert"}, app.Options{})
	if err == nil {
		t.Error("half a CA accepted")
	}
	// A store that cannot open.
	if _, err := app.New(ctx, app.Config{Store: sqlstore.SQLite, DSN: "file:/no/such/dir/x.db?mode=rw", BaseURL: "https://x", Role: app.RoleAll, EnrollAllowAny: true}, app.Options{}); err == nil {
		t.Error("unopenable store accepted")
	}
}

func TestBuildCAFromFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Bad CA files fail.
	if _, err := app.New(context.Background(), app.Config{
		Store: sqlstore.SQLite, Memory: true, BaseURL: "https://x", Role: app.RoleAll, EnrollAllowAny: true,
		CACert: dir + "/nope.crt", CAKey: dir + "/nope.key",
	}, app.Options{}); err == nil {
		t.Error("bad CA files accepted")
	}
	// A schema-disabled app still builds.
	a, err := app.New(context.Background(), app.Config{
		Store: sqlstore.SQLite, Memory: true, BaseURL: "https://x", Role: app.RoleMDM, EnrollAllowAny: true,
		DisableSchemaValidation: true,
	}, app.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("close = %v", err)
	}
}

func TestNewServiceConfigError(t *testing.T) {
	t.Parallel()
	// The store opens and the CA is generated, but an empty BaseURL makes
	// service.New fail, so New must close the store and return the error.
	_, err := app.New(context.Background(), app.Config{
		Store: sqlstore.SQLite, Memory: true, BaseURL: "", Role: app.RoleAll, EnrollAllowAny: true,
	}, app.Options{})
	if err == nil {
		t.Error("empty BaseURL accepted")
	}
}

func TestBuildCAFromValidFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	local, err := ca.NewSelfSigned(ca.SelfSignedOptions{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: local.Certificate().Raw})
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := app.New(context.Background(), app.Config{
		Store: sqlstore.SQLite, Memory: true, BaseURL: "https://x", Role: app.RoleAll, EnrollAllowAny: true,
		CACert: certPath, CAKey: keyPath,
	}, app.Options{})
	if err != nil {
		t.Fatalf("New from CA files: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	if a.CA.Certificate().Raw == nil {
		t.Error("loaded CA has no certificate")
	}
}
