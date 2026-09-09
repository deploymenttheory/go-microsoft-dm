package simulator_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
	"github.com/deploymenttheory/go-microsoft-dm/schema/registry"
	"github.com/deploymenttheory/go-microsoft-dm/simulator"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
	"github.com/deploymenttheory/go-microsoft-dm/testpki"
)

var t0 = time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

const deviceID = "F717C0F0-5F68-4AC3-A341-01B2544219DF"

// harness is an in-memory management server behind an httptest server.
type harness struct {
	srv     *httptest.Server
	store   *inmem.Store
	queue   *inmem.Queue
	hooks   *recordHooks
	fakeClk *clock.Fake
	errs    []error
	ca      *testpki.CA
	handler *mdm.Handler
	svc     *mdm.Service
}

type recordHooks struct {
	mdm.NopHooks
	packageOne []mdm.Facts
	generic    []mdm.Event
	events     []mdm.Event
	unenrolled []string
}

func (h *recordHooks) PackageOne(_ context.Context, _, _ string, f mdm.Facts) error {
	h.packageOne = append(h.packageOne, f)
	return nil
}
func (h *recordHooks) GenericAlert(_ context.Context, e mdm.Event) error {
	h.generic = append(h.generic, e)
	return nil
}
func (h *recordHooks) ClientEvent(_ context.Context, e mdm.Event) error {
	h.events = append(h.events, e)
	return nil
}
func (h *recordHooks) Unenrolled(_ context.Context, d string) error {
	h.unenrolled = append(h.unenrolled, d)
	return nil
}

func newHarness(t *testing.T, useTLS bool, auth mdm.AuthType) *harness {
	t.Helper()
	return newHarnessClientAuth(t, useTLS, auth, tls.VerifyClientCertIfGiven)
}

func newHarnessClientAuth(t *testing.T, useTLS bool, auth mdm.AuthType, clientAuth tls.ClientAuthType) *harness {
	t.Helper()
	store := inmem.New()
	queue := inmem.NewQueue()
	hooks := &recordHooks{}
	ctx := context.Background()
	ca, err := testpki.NewCA("Sim CA")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, &storage.Enrollment{
		Serial: "1", Thumbprint: "TP-1", DeviceID: deviceID, EnrollmentType: enroll.EnrollmentTypeFull, EnrolledAt: t0,
	}); err != nil {
		t.Fatal(err)
	}
	if auth == mdm.AuthDigest {
		if err := store.PutMDMCredential(ctx, storage.MDMCredential{
			DeviceID: deviceID, AuthType: mdm.AuthDigest, CredentialHash: syncml.CredentialHash("dmclient", "s3cret"),
		}); err != nil {
			t.Fatal(err)
		}
	}
	clk := clock.NewFake(t0)
	svc, err := mdm.New(mdm.Config{
		ServerURL: "https://mdm.example.test/ManagementServer/MDM.svc",
		Auth:      &storage.MDMAuthenticator{Enrollments: store, Credentials: store},
		Queue:     queue, Hooks: hooks, Registry: registry.Registry(), Clock: clk, ProviderID: "SimServer",
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := mdm.NewHandler(svc)
	h := &harness{store: store, queue: queue, hooks: hooks, fakeClk: clk, ca: ca, svc: svc, handler: handler}
	handler.Log = func(_ string, err error) { h.errs = append(h.errs, err) }
	if useTLS {
		h.srv = httptest.NewUnstartedServer(handler)
		h.srv.TLS = &tls.Config{ClientAuth: clientAuth, ClientCAs: ca.Pool(), MinVersion: tls.VersionTLS12} //nolint:gosec // test server
		h.srv.StartTLS()
	} else {
		h.srv = httptest.NewServer(handler)
	}
	t.Cleanup(h.srv.Close)
	return h
}

// clientCert returns an http.Client that presents the device identity
// certificate to the mTLS harness.
func (h *harness) clientCert(t *testing.T) *http.Client {
	t.Helper()
	id, err := h.ca.IssueWithKey(deviceID, time.Now().Add(-time.Minute), mustKey(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.Create(context.Background(), &storage.Enrollment{
		Serial: id.Cert.SerialNumber.String(), Thumbprint: wapprov.Thumbprint(id.Cert.Raw), DeviceID: deviceID,
		EnrollmentType: enroll.EnrollmentTypeFull, EnrolledAt: t0,
	}); err != nil {
		t.Fatal(err)
	}
	certPEM, keyPEM, err := id.PEM()
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	base := h.srv.Client()
	tr := base.Transport.(*http.Transport).Clone()
	tr.TLSClientConfig.Certificates = []tls.Certificate{pair}
	return &http.Client{Transport: tr}
}

func (h *harness) device(auth mdm.AuthType) *simulator.Device {
	c := &simulator.Device{
		HTTP: h.srv.Client(), ManagementURL: h.srv.URL + "/ManagementServer/MDM.svc",
		DeviceID: deviceID, LoginStatus: syncml.LoginStatusUser,
	}
	if auth == mdm.AuthDigest {
		c.Username, c.Password = "dmclient", "s3cret"
	}
	return c
}

func enqueue(t *testing.T, h *harness, cmd *mdm.Command) string {
	t.Helper()
	qc, err := h.queue.Enqueue(context.Background(), deviceID, cmd, t0)
	if err != nil {
		t.Fatal(err)
	}
	return qc.ID
}

func mustKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestFirstSessionCertificate(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true, mdm.AuthCertificate)
	repl, err := mdm.NewReplace("./Vendor/MSFT/Policy/Config/Camera/AllowCamera", "0", mdm.WithFormat(syncml.FormatInt))
	if err != nil {
		t.Fatal(err)
	}
	replID := enqueue(t, h, repl)
	c := h.device(mdm.AuthCertificate)
	c.HTTP = h.clientCert(t)
	tr, err := c.RunSession(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	// A trusted client certificate authenticates the whole session; no
	// challenge is sent.
	if tr.Challenged != 0 {
		t.Errorf("challenged %d times with a client certificate", tr.Challenged)
	}
	if !tr.Ended {
		t.Error("session did not end cleanly")
	}
	got, _ := h.queue.Get(context.Background(), deviceID, replID)
	if got.State != mdm.StateAcknowledged {
		t.Errorf("replace state = %s", got.State)
	}
}

func TestFirstSessionMD5(t *testing.T) {
	t.Parallel()
	h := newHarness(t, false, mdm.AuthDigest)
	repl, err := mdm.NewReplace("./Vendor/MSFT/Policy/Config/Camera/AllowCamera", "0", mdm.WithFormat(syncml.FormatInt))
	if err != nil {
		t.Fatal(err)
	}
	replID := enqueue(t, h, repl)
	get, err := mdm.NewGet([]string{"./DevDetail/SwV"})
	if err != nil {
		t.Fatal(err)
	}
	getID := enqueue(t, h, get)
	c := h.device(mdm.AuthDigest)
	tr, err := c.RunSession(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Challenged != 1 {
		t.Errorf("challenged %d times, want 1", tr.Challenged)
	}
	if len(h.hooks.packageOne) != 1 || h.hooks.packageOne[0].LoginStatus != syncml.LoginStatusUser {
		t.Errorf("package one facts = %+v", h.hooks.packageOne)
	}
	if h.hooks.packageOne[0].DevInfo["Man"] != "VMware, Inc." {
		t.Errorf("devinfo = %+v", h.hooks.packageOne[0].DevInfo)
	}
	// The Replace and the two Gets (queued plus the auto first-session read)
	// were delivered and answered.
	replCmd, err := h.queue.Get(context.Background(), deviceID, replID)
	if err != nil {
		t.Fatal(err)
	}
	if replCmd.State != mdm.StateAcknowledged {
		t.Errorf("replace state = %s", replCmd.State)
	}
	getCmd, _ := h.queue.Get(context.Background(), deviceID, getID)
	if getCmd.State != mdm.StateAcknowledged || getCmd.Result == nil || len(getCmd.Result.Items) != 1 || getCmd.Result.Items[0].Data.Text() != "10.0.26100.1" {
		t.Errorf("get result = %+v", getCmd.Result)
	}
	if c.Tree["./Vendor/MSFT/Policy/Config/Camera/AllowCamera"] != "0" {
		t.Errorf("replace not applied on client: %v", c.Tree)
	}
	if !tr.Ended {
		t.Error("session did not end cleanly")
	}
}

func TestChunkedGetResult(t *testing.T) {
	t.Parallel()
	h := newHarness(t, false, mdm.AuthDigest)
	// The client returns a large value for a Get; the client uploads it in
	// chunks because the engine advertised no limit, so drive chunking from
	// the client side by making the value large and setting a small client
	// MaxObjSize is server-to-client. Instead, exercise server-to-client
	// chunking: enqueue a Replace with a large value and a small client
	// MaxObjSize so the engine splits it.
	big := strings.Repeat("A", 5000)
	repl, err := mdm.NewReplace("./Vendor/MSFT/Policy/Config/Big/Blob", big, mdm.WithFormat(syncml.FormatChr))
	if err != nil {
		t.Fatal(err)
	}
	replID := enqueue(t, h, repl)
	c := h.device(mdm.AuthDigest)
	c.MaxObjSize = 1000
	tr, err := c.RunSession(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if !tr.Ended {
		t.Fatal("session did not end")
	}
	got, _ := h.queue.Get(context.Background(), deviceID, replID)
	if got.State != mdm.StateAcknowledged {
		t.Errorf("chunked replace state = %s", got.State)
	}
	if c.Tree["./Vendor/MSFT/Policy/Config/Big/Blob"] != big {
		t.Errorf("client reassembled %d bytes, want %d", len(c.Tree["./Vendor/MSFT/Policy/Config/Big/Blob"]), len(big))
	}
	// The value crossed the wire in more than one message.
	if tr.Messages < 3 {
		t.Errorf("expected chunking across messages, got %d", tr.Messages)
	}
}

func TestUserScopeHeldUntilLogin(t *testing.T) {
	t.Parallel()
	h := newHarness(t, false, mdm.AuthDigest)
	userCmd, err := mdm.NewReplace("./User/Vendor/MSFT/Policy/Config/A/B", "1", mdm.WithFormat(syncml.FormatInt))
	if err != nil {
		t.Fatal(err)
	}
	userID := enqueue(t, h, userCmd)
	deviceCmd, err := mdm.NewReplace("./Device/Vendor/MSFT/Policy/Config/C/D", "1", mdm.WithFormat(syncml.FormatInt))
	if err != nil {
		t.Fatal(err)
	}
	deviceID2 := enqueue(t, h, deviceCmd)

	// First session: no user signed in. The device command runs; the user
	// command is held.
	noUser := h.device(mdm.AuthDigest)
	noUser.LoginStatus = syncml.LoginStatusNone
	if _, err := noUser.RunSession(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	dc, _ := h.queue.Get(context.Background(), deviceID, deviceID2)
	uc, _ := h.queue.Get(context.Background(), deviceID, userID)
	if dc.State != mdm.StateAcknowledged {
		t.Errorf("device command state = %s", dc.State)
	}
	if uc.State != mdm.StatePending {
		t.Errorf("user command delivered before sign-in: %s", uc.State)
	}

	// Second session: a user is signed in. The held command is delivered.
	withUser := h.device(mdm.AuthDigest)
	withUser.LoginStatus = syncml.LoginStatusUser
	if _, err := withUser.RunSession(context.Background(), "2"); err != nil {
		t.Fatal(err)
	}
	uc, _ = h.queue.Get(context.Background(), deviceID, userID)
	if uc.State != mdm.StateAcknowledged {
		t.Errorf("user command state after sign-in = %s", uc.State)
	}
}

func TestUnenrollAlertEndsEnrollment(t *testing.T) {
	t.Parallel()
	h := newHarness(t, false, mdm.AuthDigest)
	c := h.device(mdm.AuthDigest)
	c.Generics = []simulator.GenericAlert{{Type: syncml.AlertTypeUnenrollmentUserRequest, Format: syncml.FormatInt, Data: "1"}}
	if _, err := c.RunSession(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	if len(h.hooks.generic) != 1 || h.hooks.generic[0].Type != syncml.AlertTypeUnenrollmentUserRequest {
		t.Errorf("generic alerts = %+v", h.hooks.generic)
	}
	if len(h.hooks.unenrolled) != 1 || h.hooks.unenrolled[0] != deviceID {
		t.Errorf("unenrolled = %+v", h.hooks.unenrolled)
	}
}

func TestAtomicDelivery(t *testing.T) {
	t.Parallel()
	h := newHarness(t, false, mdm.AuthDigest)
	a, _ := mdm.NewReplace("./Vendor/MSFT/Policy/Config/A/One", "1", mdm.WithFormat(syncml.FormatInt))
	b, _ := mdm.NewReplace("./Vendor/MSFT/Policy/Config/A/Two", "2", mdm.WithFormat(syncml.FormatInt))
	atomic, err := mdm.NewAtomic([]*mdm.Command{a, b})
	if err != nil {
		t.Fatal(err)
	}
	atomicID := enqueue(t, h, atomic)
	c := h.device(mdm.AuthDigest)
	if _, err := c.RunSession(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	got, _ := h.queue.Get(context.Background(), deviceID, atomicID)
	if got.State != mdm.StateAcknowledged || got.Result == nil || len(got.Result.Children) != 2 {
		t.Errorf("atomic result = %+v", got.Result)
	}
	if c.Tree["./Vendor/MSFT/Policy/Config/A/One"] != "1" || c.Tree["./Vendor/MSFT/Policy/Config/A/Two"] != "2" {
		t.Errorf("atomic not applied: %v", c.Tree)
	}
}

func TestSessionTransportErrors(t *testing.T) {
	t.Parallel()
	// Unreachable server.
	c := &simulator.Device{HTTP: http.DefaultClient, ManagementURL: "https://127.0.0.1:1/svc", DeviceID: deviceID}
	if _, err := c.RunSession(context.Background(), "1"); err == nil {
		t.Error("unreachable server succeeded")
	}
	// Server that returns 500 with a body.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(bad.Close)
	c = &simulator.Device{HTTP: bad.Client(), ManagementURL: bad.URL + "/svc", DeviceID: deviceID}
	if _, err := c.RunSession(context.Background(), "1"); !errors.Is(err, simulator.ErrSession) {
		t.Errorf("500 = %v", err)
	}
	// Server that chunks.
	chunked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Transfer-Encoding", "chunked")
		_, _ = w.Write([]byte("<x/>"))
		w.(http.Flusher).Flush()
	}))
	t.Cleanup(chunked.Close)
	c = &simulator.Device{HTTP: chunked.Client(), ManagementURL: chunked.URL + "/svc", DeviceID: deviceID}
	if _, err := c.RunSession(context.Background(), "1"); err == nil {
		t.Error("chunked server accepted")
	}
}

func TestSessionDeleteAndExec(t *testing.T) {
	t.Parallel()
	h := newHarness(t, false, mdm.AuthDigest)
	c := h.device(mdm.AuthDigest)
	c.Tree = map[string]string{"./Vendor/MSFT/WiFi/Profile/X": "present"}
	del, _ := mdm.NewDelete("./Vendor/MSFT/WiFi/Profile/X")
	delID := enqueue(t, h, del)
	exec, _ := mdm.NewExec("./Vendor/MSFT/DMClient/Provider/SimServer/Unenroll", "")
	execID := enqueue(t, h, exec)
	get, _ := mdm.NewGet([]string{"./Vendor/MSFT/DoesNotExist"})
	getID := enqueue(t, h, get)
	if _, err := c.RunSession(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Tree["./Vendor/MSFT/WiFi/Profile/X"]; ok {
		t.Error("delete not applied")
	}
	if d, _ := h.queue.Get(context.Background(), deviceID, delID); d.State != mdm.StateAcknowledged {
		t.Errorf("delete state = %s", d.State)
	}
	if e, _ := h.queue.Get(context.Background(), deviceID, execID); e.State != mdm.StateAcknowledged {
		t.Errorf("exec state = %s", e.State)
	}
	// A Get of a node the client does not have returns 404 -> command failed.
	g, _ := h.queue.Get(context.Background(), deviceID, getID)
	if g.State != mdm.StateFailed {
		t.Errorf("missing-node get state = %s (result %+v)", g.State, g.Result)
	}
}

func TestSessionChunkedUpload(t *testing.T) {
	t.Parallel()
	h := newHarness(t, false, mdm.AuthDigest)
	c := h.device(mdm.AuthDigest)
	big := strings.Repeat("Z", 4096)
	c.Tree = map[string]string{"./Vendor/MSFT/DiagnosticLog/Big": big}
	get, _ := mdm.NewGet([]string{"./Vendor/MSFT/DiagnosticLog/Big"})
	getID := enqueue(t, h, get)
	c.UploadChunkSize = 1000
	tr, err := c.RunSession(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if !tr.Ended {
		t.Fatal("session did not end")
	}
	// The large value crossed the wire in several messages and the server
	// reassembled it.
	if tr.Messages < 4 {
		t.Errorf("expected chunked upload across messages, got %d", tr.Messages)
	}
	got, _ := h.queue.Get(context.Background(), deviceID, getID)
	if got.State != mdm.StateAcknowledged || got.Result == nil || len(got.Result.Items) != 1 {
		t.Fatalf("get result = %+v", got.Result)
	}
	if v := got.Result.Items[0].Data.Text(); v != big {
		t.Errorf("server reassembled %d bytes, want %d", len(v), len(big))
	}
}

func TestUnverifiedCertificateStillRequiresDigest(t *testing.T) {
	t.Parallel()
	h := newHarnessClientAuth(t, true, mdm.AuthDigest, tls.RequestClientCert)
	c := h.device(mdm.AuthDigest)
	c.HTTP = h.clientCert(t)
	tr, err := c.RunSession(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Challenged != 1 || !tr.Ended {
		t.Fatalf("digest fallback: challenged %d, ended %v", tr.Challenged, tr.Ended)
	}
}
