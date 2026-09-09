package mdm_test

import (
	"context"
	"crypto/x509"
	"errors"
	"strconv"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
)

// driver runs a scripted session against a Service, tracking MsgIDs and the
// MD5 nonce so tests can exercise the multi-message engine paths.
type driver struct {
	t         *testing.T
	svc       *mdm.Service
	clientMsg int
	nonce     []byte
	user      string
	pass      string
	tree      map[string]string
	asm       syncml.Assembler
}

func newDriver(t *testing.T, svc *mdm.Service) *driver {
	return &driver{t: t, svc: svc, tree: map[string]string{"./DevDetail/SwV": "10.0.26100.1"}}
}

func (d *driver) send(m *syncml.Message) *syncml.Message {
	d.t.Helper()
	body, err := syncml.Encode(m, syncml.EncodeOptions{})
	if err != nil {
		d.t.Fatal(err)
	}
	out, err := d.svc.Handle(context.Background(), verifiedTransport(), body)
	if err != nil {
		d.t.Fatalf("Handle: %v", err)
	}
	if out == nil {
		return nil
	}
	resp, err := syncml.Decode(out, syncml.DecodeOptions{})
	if err != nil {
		d.t.Fatal(err)
	}
	return resp
}

func (d *driver) header(sessionID string) syncml.Header {
	h := syncml.Header{
		VerDTD: syncml.VerDTD, VerProto: syncml.VerProto, SessionID: sessionID, MsgID: strconv.Itoa(d.clientMsg),
		Target: syncml.Location{LocURI: "https://mdm.example.test/svc"}, Source: syncml.Location{LocURI: dev},
	}
	if len(d.nonce) > 0 && d.pass != "" {
		h.Cred = syncml.NewMD5Cred(syncml.MD5Digest(d.user, d.pass, d.nonce))
	}
	return h
}

// pkgOne builds package 1 with optional extra alerts.
func (d *driver) pkgOne(sessionID, loginStatus string, extra ...syncml.Command) *syncml.Message {
	d.clientMsg = 1
	m := &syncml.Message{Header: d.header(sessionID)}
	ids := &syncml.CmdIDs{}
	m.Body.Commands = append(m.Body.Commands, &syncml.Alert{CmdID: ids.Next(), Data: syncml.AlertClientInitiated.Wire()})
	if loginStatus != "" {
		m.Body.Commands = append(m.Body.Commands, &syncml.Alert{CmdID: ids.Next(), Data: syncml.AlertClientEvent.Wire(),
			Items: []syncml.Item{{Meta: &syncml.Meta{Type: syncml.AlertTypeLoginStatus, Format: syncml.FormatChr}, Data: &syncml.Data{Value: loginStatus}}}})
	}
	m.Body.Commands = append(m.Body.Commands, &syncml.Replace{CmdID: ids.Next(),
		Items: []syncml.Item{{Source: "./DevInfo/DevId", Data: &syncml.Data{Value: dev}}, {Source: "./DevInfo/Man", Data: &syncml.Data{Value: "VMware, Inc."}}}})
	m.Body.Commands = append(m.Body.Commands, extra...)
	m.Body.Final = true
	return m
}

// respond answers a server message with statuses and Get results, applying
// commands to the local tree; it returns the client's reply, or nil when the
// server's Final message needs no response.
func (d *driver) respond(resp *syncml.Message) *syncml.Message {
	d.clientMsg++
	reply := &syncml.Message{Header: d.header(resp.Header.SessionID)}
	ids := &syncml.CmdIDs{}
	reply.Body.Commands = append(reply.Body.Commands, syncml.StatusForHeader(resp, ids.Next(), syncml.StatusOK))
	emit := resp.Body.Final
	needsResponse := false
	for _, c := range resp.Body.Commands {
		if _, ok := c.(*syncml.Status); ok {
			continue
		}
		needsResponse = true
		d.answerCommand(resp, c, reply, ids, emit)
	}
	if !resp.Body.Final {
		reply.Body.Commands = []syncml.Command{syncml.StatusForHeader(resp, "1", syncml.StatusOK),
			&syncml.Alert{CmdID: "2", Data: syncml.AlertNextMessage.Wire()}}
		return reply
	}
	if !needsResponse {
		return nil
	}
	reply.Body.Final = true
	return reply
}

func (d *driver) answerCommand(resp *syncml.Message, c syncml.Command, reply *syncml.Message, ids *syncml.CmdIDs, emit bool) {
	switch b := c.(type) {
	case *syncml.Status:
		return
	case *syncml.Get:
		if !emit {
			return
		}
		for _, it := range b.Items {
			if v, ok := d.tree[it.Target]; ok {
				reply.Body.Commands = append(reply.Body.Commands, &syncml.Results{CmdID: ids.Next(), MsgRef: resp.Header.MsgID, CmdRef: b.CmdID, Cmd: syncml.CmdGet,
					Items: []syncml.Item{{Source: it.Target, Meta: &syncml.Meta{Format: syncml.FormatChr}, Data: &syncml.Data{Value: v}}}})
			}
		}
		d.ackCmd(resp, c, reply, ids, syncml.StatusOK)
	case *syncml.Replace:
		for _, it := range b.Items {
			if full, done, err := d.asm.Add(it); err == nil && done && full.Target != "" {
				d.tree[full.Target] = full.Data.Text()
			}
		}
		if emit {
			d.ackCmd(resp, c, reply, ids, syncml.StatusOK)
		}
	case *syncml.Atomic:
		for _, m := range b.Commands {
			d.answerCommand(resp, m, reply, ids, emit)
		}
		if emit {
			d.ackCmd(resp, c, reply, ids, syncml.StatusOK)
		}
	default:
		if emit {
			d.ackCmd(resp, c, reply, ids, syncml.StatusOK)
		}
	}
}

func (d *driver) ackCmd(resp *syncml.Message, c syncml.Command, reply *syncml.Message, ids *syncml.CmdIDs, code syncml.StatusCode) {
	reply.Body.Commands = append(reply.Body.Commands, &syncml.Status{CmdID: ids.Next(), MsgRef: resp.Header.MsgID, CmdRef: c.ID(), Cmd: c.Name(), Data: syncml.Data{Value: code.Wire()}})
}

// run drives a whole session to completion, applying continuation requests.
func (d *driver) run(first *syncml.Message) {
	resp := d.send(first)
	for n := 0; resp != nil; n++ {
		if n > 40 {
			d.t.Fatal("session did not terminate")
		}
		reply := d.respond(resp)
		if reply == nil {
			return
		}
		resp = d.send(reply)
	}
}

func certService(t *testing.T, hooks mdm.Hooks) (*mdm.Service, *inmem.Queue) {
	q := inmem.NewQueue()
	svc, err := mdm.New(mdm.Config{
		ServerURL: "https://mdm.example.test/svc", Auth: fakeAuth{authType: mdm.AuthCertificate},
		Queue: q, Hooks: hooks, Clock: clock.NewFake(t0), ProviderID: "P", AllowBasic: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, q
}

func TestEngineMD5AndResults(t *testing.T) {
	t.Parallel()
	// fakeAuth returns AuthDigest but no CredentialHash, so verifyCredential
	// needs a hash: use a real credential via an authenticator with a hash.
	q := inmem.NewQueue()
	svc, err := mdm.New(mdm.Config{
		ServerURL: "https://mdm.example.test/svc",
		Auth:      hashAuth{hash: syncml.CredentialHash("u", "p")},
		Queue:     q, Clock: clock.NewFake(t0), ProviderID: "P",
	})
	if err != nil {
		t.Fatal(err)
	}
	repl, _ := mdm.NewReplace("./Vendor/MSFT/Policy/Config/A/B", "1", mdm.WithFormat(syncml.FormatInt))
	if _, err := q.Enqueue(context.Background(), dev, repl, t0); err != nil {
		t.Fatal(err)
	}
	get, _ := mdm.NewGet([]string{"./DevDetail/SwV"})
	getID, err := q.Enqueue(context.Background(), dev, get, t0)
	if err != nil {
		t.Fatal(err)
	}
	d := newDriver(t, svc)
	d.user, d.pass = "u", "p"

	// msg1: no credentials -> challenge.
	resp := d.send(d.pkgOne("1", syncml.LoginStatusUser))
	if len(resp.Body.Statuses()) == 0 || resp.Body.Statuses()[0].Code() != syncml.StatusAuthenticationRequired {
		t.Fatalf("expected 407, got %+v", resp.Body.Statuses())
	}
	chal := resp.Body.Statuses()[0].Chal
	if chal == nil {
		t.Fatal("no challenge")
	}
	nonce, err := chal.Nonce()
	if err != nil {
		t.Fatal(err)
	}
	d.nonce = nonce

	// msg2: package 1 with credentials at the next MsgID -> authenticated.
	m2 := d.pkgOne("1", syncml.LoginStatusUser)
	d.clientMsg = 2
	m2.Header = d.header("1")
	d.run(m2)
	acked, _ := q.Get(context.Background(), dev, getID.ID)
	if acked.State != mdm.StateAcknowledged || acked.Result == nil || len(acked.Result.Items) != 1 {
		t.Errorf("get result = %+v", acked.Result)
	}
}

// hashAuth carries an MD5 credential hash.
type hashAuth struct{ hash []byte }

func (a hashAuth) Lookup(_ context.Context, deviceID string) (*mdm.Identity, error) {
	if deviceID != dev {
		return nil, mdm.ErrUnenrolled
	}
	return &mdm.Identity{DeviceID: dev, EnrollmentKey: "1", AuthType: mdm.AuthDigest, CredentialHash: a.hash}, nil
}
func (a hashAuth) TrustCertificate(context.Context, *mdm.Identity, [][]*x509.Certificate) bool {
	return false
}

func TestEngineAtomicChildren(t *testing.T) {
	t.Parallel()
	svc, q := certService(t, nil)
	a, _ := mdm.NewReplace("./Vendor/MSFT/Policy/Config/A/One", "1", mdm.WithFormat(syncml.FormatInt))
	b, _ := mdm.NewReplace("./Vendor/MSFT/Policy/Config/A/Two", "2", mdm.WithFormat(syncml.FormatInt))
	atomic, _ := mdm.NewAtomic([]*mdm.Command{a, b})
	id, _ := q.Enqueue(context.Background(), dev, atomic, t0)
	d := newDriver(t, svc)
	d.run(d.pkgOne("1", syncml.LoginStatusUser))
	got, _ := q.Get(context.Background(), dev, id.ID)
	if got.State != mdm.StateAcknowledged || got.Result == nil || len(got.Result.Children) != 2 {
		t.Errorf("atomic result = %+v", got.Result)
	}
}

func TestEngineChunkedDelivery(t *testing.T) {
	t.Parallel()
	svc, q := certService(t, nil)
	big := ""
	for i := 0; i < 5000; i++ {
		big += "A"
	}
	repl, _ := mdm.NewReplace("./Vendor/MSFT/Policy/Config/Big", big, mdm.WithFormat(syncml.FormatChr))
	id, _ := q.Enqueue(context.Background(), dev, repl, t0)
	d := newDriver(t, svc)
	// Advertise a small MaxObjSize so the engine chunks.
	first := d.pkgOne("1", syncml.LoginStatusUser)
	first.Header.Meta = &syncml.Meta{MaxObjSize: 1000}
	d.run(first)
	got, _ := q.Get(context.Background(), dev, id.ID)
	if got.State != mdm.StateAcknowledged {
		t.Errorf("chunked replace state = %s", got.State)
	}
	if d.tree["./Vendor/MSFT/Policy/Config/Big"] != big {
		t.Errorf("reassembled %d bytes, want %d", len(d.tree["./Vendor/MSFT/Policy/Config/Big"]), len(big))
	}
}

func TestEngineUserScope(t *testing.T) {
	t.Parallel()
	svc, q := certService(t, nil)
	userCmd, _ := mdm.NewReplace("./User/Vendor/MSFT/Policy/Config/U", "1", mdm.WithFormat(syncml.FormatInt))
	uid, _ := q.Enqueue(context.Background(), dev, userCmd, t0)
	d := newDriver(t, svc)
	// No user signed in: the command is held.
	d.run(d.pkgOne("1", syncml.LoginStatusNone))
	held, _ := q.Get(context.Background(), dev, uid.ID)
	if held.State != mdm.StatePending {
		t.Errorf("user command delivered without a user: %s", held.State)
	}
	// A user signs in: the command is delivered.
	d2 := newDriver(t, svc)
	d2.run(d2.pkgOne("2", syncml.LoginStatusUser))
	done, _ := q.Get(context.Background(), dev, uid.ID)
	if done.State != mdm.StateAcknowledged {
		t.Errorf("user command state after sign-in = %s", done.State)
	}
}

type recHooks struct {
	mdm.NopHooks
	generic, events []mdm.Event
	unenrolled      []string
	facts           []mdm.Facts
}

func (h *recHooks) PackageOne(_ context.Context, _, _ string, f mdm.Facts) error {
	h.facts = append(h.facts, f)
	return nil
}
func (h *recHooks) GenericAlert(_ context.Context, e mdm.Event) error {
	h.generic = append(h.generic, e)
	return nil
}
func (h *recHooks) ClientEvent(_ context.Context, e mdm.Event) error {
	h.events = append(h.events, e)
	return nil
}
func (h *recHooks) Unenrolled(_ context.Context, d string) error {
	h.unenrolled = append(h.unenrolled, d)
	return nil
}

func TestEngineAlerts(t *testing.T) {
	t.Parallel()
	hooks := &recHooks{}
	svc, _ := certService(t, hooks)
	unenroll := &syncml.Alert{CmdID: "9", Data: syncml.AlertGeneric.Wire(), Items: []syncml.Item{
		{Meta: &syncml.Meta{Type: syncml.AlertTypeUnenrollmentUserRequest, Format: syncml.FormatInt}, Data: &syncml.Data{Value: "1"}}}}
	custom := &syncml.Alert{CmdID: "10", Data: syncml.AlertClientEvent.Wire(), Items: []syncml.Item{
		{Meta: &syncml.Meta{Type: "com.example/custom", Format: syncml.FormatChr}, Data: &syncml.Data{Value: "x"}}}}
	d := newDriver(t, svc)
	d.run(d.pkgOne("1", syncml.LoginStatusUser, custom, unenroll))
	if len(hooks.facts) != 1 || hooks.facts[0].LoginStatus != syncml.LoginStatusUser {
		t.Errorf("facts = %+v", hooks.facts)
	}
	if len(hooks.events) != 1 || hooks.events[0].Type != "com.example/custom" {
		t.Errorf("client events = %+v", hooks.events)
	}
	if len(hooks.generic) != 1 || len(hooks.unenrolled) != 1 || hooks.unenrolled[0] != dev {
		t.Errorf("generic=%+v unenrolled=%+v", hooks.generic, hooks.unenrolled)
	}
}

func TestEngineAbort(t *testing.T) {
	t.Parallel()
	svc, _ := certService(t, nil)
	abort := &syncml.Alert{CmdID: "9", Data: syncml.AlertSessionAbort.Wire()}
	d := newDriver(t, svc)
	resp := d.send(d.pkgOne("1", syncml.LoginStatusUser, abort))
	// The server acknowledges and the session ends.
	if resp != nil && !resp.Body.Final {
		t.Errorf("abort not final: %+v", resp)
	}
	if _, err := svc.Config().Sessions.GetSession(context.Background(), mdm.SessionKey(dev, "1")); err == nil {
		t.Error("session survived abort")
	}
}

func TestEngineBasicAuth(t *testing.T) {
	t.Parallel()
	q := inmem.NewQueue()
	svc, err := mdm.New(mdm.Config{
		ServerURL: "https://mdm.example.test/svc",
		Auth:      basicAuth{user: "u", pass: "p"}, Queue: q, Clock: clock.NewFake(t0), AllowBasic: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	d := newDriver(t, svc)
	// msg1 without a credential -> basic challenge.
	resp := d.send(d.pkgOne("1", syncml.LoginStatusUser))
	if resp.Body.Statuses()[0].Code() != syncml.StatusAuthenticationRequired {
		t.Fatalf("expected challenge, got %v", resp.Body.Statuses()[0].Code())
	}
	// msg2 with a basic credential.
	m2 := d.pkgOne("1", syncml.LoginStatusUser)
	d.clientMsg = 2
	m2.Header = d.header("1")
	m2.Header.Cred = syncml.NewBasicCred("u", "p")
	if _, err := svc.Handle(context.Background(), verifiedTransport(), mustEncode(t, m2)); err != nil {
		t.Fatalf("basic auth: %v", err)
	}
	// A wrong basic password is rejected.
	d3 := newDriver(t, svc)
	d3.send(d3.pkgOne("3", syncml.LoginStatusUser))
	m4 := d3.pkgOne("3", syncml.LoginStatusUser)
	d3.clientMsg = 2
	m4.Header = d3.header("3")
	m4.Header.Cred = syncml.NewBasicCred("u", "wrong")
	resp = mustHandle(t, svc, m4)
	if resp.Body.Statuses()[0].Code() != syncml.StatusInvalidCredentials {
		t.Errorf("wrong password code = %v", resp.Body.Statuses()[0].Code())
	}
}

type basicAuth struct{ user, pass string }

func (a basicAuth) Lookup(_ context.Context, deviceID string) (*mdm.Identity, error) {
	if deviceID != dev {
		return nil, mdm.ErrUnenrolled
	}
	hash, err := mdm.HashBasicCredential(a.user, a.pass)
	if err != nil {
		return nil, err
	}
	return &mdm.Identity{DeviceID: dev, EnrollmentKey: "1", AuthType: mdm.AuthBasic, CredentialHash: hash}, nil
}
func (a basicAuth) TrustCertificate(context.Context, *mdm.Identity, [][]*x509.Certificate) bool {
	return false
}

func mustEncode(t *testing.T, m *syncml.Message) []byte {
	t.Helper()
	b, err := syncml.Encode(m, syncml.EncodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustHandle(t *testing.T, svc *mdm.Service, m *syncml.Message) *syncml.Message {
	t.Helper()
	out, err := svc.Handle(context.Background(), verifiedTransport(), mustEncode(t, m))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := syncml.Decode(out, syncml.DecodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

type failHooks struct {
	mdm.NopHooks
	onPkg, onGeneric, onEvent, onUnenroll error
}

func (h failHooks) PackageOne(context.Context, string, string, mdm.Facts) error { return h.onPkg }
func (h failHooks) GenericAlert(context.Context, mdm.Event) error               { return h.onGeneric }
func (h failHooks) ClientEvent(context.Context, mdm.Event) error                { return h.onEvent }
func (h failHooks) Unenrolled(context.Context, string) error                    { return h.onUnenroll }

func TestEngineHookErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	custom := &syncml.Alert{CmdID: "8", Data: syncml.AlertClientEvent.Wire(), Items: []syncml.Item{
		{Meta: &syncml.Meta{Type: "com.example/x"}, Data: &syncml.Data{Value: "1"}}}}
	unenroll := &syncml.Alert{CmdID: "9", Data: syncml.AlertGeneric.Wire(), Items: []syncml.Item{
		{Meta: &syncml.Meta{Type: syncml.AlertTypeUnenrollmentUserRequest}, Data: &syncml.Data{Value: "1"}}}}
	cases := map[string]struct {
		hooks failHooks
		alert syncml.Command
	}{
		"package one":  {hooks: failHooks{onPkg: boom}},
		"client event": {hooks: failHooks{onEvent: boom}, alert: custom},
		"generic":      {hooks: failHooks{onGeneric: boom}, alert: unenroll},
		"unenrolled":   {hooks: failHooks{onUnenroll: boom}, alert: unenroll},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, _ := certService(t, tc.hooks)
			d := newDriver(t, svc)
			var extra []syncml.Command
			if tc.alert != nil {
				extra = append(extra, tc.alert)
			}
			body, _ := syncml.Encode(d.pkgOne("1", syncml.LoginStatusUser, extra...), syncml.EncodeOptions{})
			if _, err := svc.Handle(context.Background(), verifiedTransport(), body); !errors.Is(err, boom) {
				t.Errorf("%s: err = %v", name, err)
			}
		})
	}
}

func TestEngineSyncTypeAndPrep(t *testing.T) {
	t.Parallel()
	hooks := &recHooks{}
	svc, _ := certService(t, hooks)
	syncType := &syncml.Alert{CmdID: "8", Data: syncml.AlertClientEvent.Wire(), Items: []syncml.Item{
		{Meta: &syncml.Meta{Type: syncml.AlertTypeSyncType}, Data: &syncml.Data{Value: "device"}}}}
	prep := &syncml.Alert{CmdID: "9", Data: syncml.AlertClientEvent.Wire(), Items: []syncml.Item{
		{Meta: &syncml.Meta{Type: syncml.AlertTypeDevicePrepSync}, Data: &syncml.Data{Value: "PendingProvisioning"}}}}
	noEnd := &syncml.Alert{CmdID: "10", Data: syncml.AlertNoEndOfData.Wire()}
	d := newDriver(t, svc)
	d.run(d.pkgOne("1", syncml.LoginStatusUser, syncType, prep, noEnd))
	if len(hooks.facts) != 1 || hooks.facts[0].SyncType != "device" || hooks.facts[0].DevicePrepSync != "PendingProvisioning" {
		t.Errorf("facts = %+v", hooks.facts)
	}
	// SyncType and DevicePrepSync are consumed as facts, not delivered as
	// client events.
	if len(hooks.events) != 0 {
		t.Errorf("consumed events delivered: %+v", hooks.events)
	}
}

func TestEngineDeliveryBudget(t *testing.T) {
	t.Parallel()
	q := inmem.NewQueue()
	svc, err := mdm.New(mdm.Config{
		ServerURL: "https://mdm.example.test/svc", Auth: fakeAuth{authType: mdm.AuthCertificate},
		Queue: q, Clock: clock.NewFake(t0), MaxMessageBytes: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		c, _ := mdm.NewReplace("./Vendor/MSFT/Policy/Config/N"+strconv.Itoa(i), "value", mdm.WithFormat(syncml.FormatChr))
		if _, err := q.Enqueue(context.Background(), dev, c, t0); err != nil {
			t.Fatal(err)
		}
	}
	d := newDriver(t, svc)
	d.run(d.pkgOne("1", syncml.LoginStatusUser))
	// Every command was eventually delivered and acknowledged across
	// messages despite the tiny per-message budget.
	list, _ := q.List(context.Background(), dev, mdm.Query{}, pagingAll())
	acked := 0
	for _, c := range list.Items {
		if c.State == mdm.StateAcknowledged && !c.Internal {
			acked++
		}
	}
	if acked != 5 {
		t.Errorf("acknowledged %d of 5", acked)
	}
}

func pagingAll() paging.Page { return paging.Page{Limit: 1000} }
