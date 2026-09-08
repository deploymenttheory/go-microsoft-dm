//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/simulator"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

const deviceID = "F717C0F0-5F68-4AC3-A341-01B2544219DF"

// enrollDevice runs the MS-MDE2 enrollment against the harness and returns the
// enrollment result and a configured management client.
func enrollDevice(t *testing.T, h *harness) (*simulator.Enrollment, *simulator.Device) {
	t.Helper()
	res, err := simulator.Enroll(context.Background(), simulator.Client{
		HTTP: h.client(), DiscoveryURL: h.srv.URL + enroll.DiscoveryPath,
		Email: "e2e@contoso.com", Password: "pw", DeviceID: deviceID,
		HWDevID: strings.Repeat("A", 64), WindowsSubject: true,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	dev := &simulator.Device{
		HTTP: h.client(), ManagementURL: res.Account.Address, DeviceID: deviceID,
		Username: res.Account.ServerAuth.Name, Password: res.Account.ServerAuth.Secret,
		LoginStatus: syncml.LoginStatusUser,
	}
	return res, dev
}

func TestEnrollAndFirstSession(t *testing.T) {
	h := newHarness(t)
	res, dev := enrollDevice(t, h)
	if res.Certificate == nil || res.Account.ProviderID != "e2e" {
		t.Fatalf("enrollment result = %+v", res.Account)
	}
	if len(res.Account.ServerAuth.Nonce) != 16 || len(res.Account.ClientAuth.Nonce) != 16 ||
		bytes.Equal(res.Account.ServerAuth.Nonce, res.Account.ClientAuth.Nonce) {
		t.Fatal("simulation server must provision independent initial digest nonces for native Windows enrollment")
	}
	// The enrollment is persisted.
	stored, err := h.app.Store.Get(context.Background(), deviceID)
	if err != nil || stored.Serial != res.Certificate.SerialNumber.String() {
		t.Fatalf("stored enrollment = %+v, %v", stored, err)
	}
	// The first management session runs and records device facts.
	tr, err := dev.RunSession(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Challenged != 1 || !tr.Ended {
		t.Errorf("first session transcript = %+v", tr)
	}
	facts, err := h.app.Store.Facts(context.Background(), deviceID)
	if err != nil || facts.DevInfo["Man"] == "" {
		t.Errorf("facts = %+v, %v", facts, err)
	}
}

// Exercise the same read-only round trip used during native desktop validation.
func TestEnrollmentReadOnlyQuery(t *testing.T) {
	h := newHarness(t)
	_, dev := enrollDevice(t, h)
	ctx := context.Background()
	dev.Tree = map[string]string{"./DevInfo/Man": "Desktop validation manufacturer"}
	cmd, err := mdm.NewGet([]string{"./DevInfo/Man"})
	if err != nil {
		t.Fatal(err)
	}
	queued, err := h.app.Store.Queue().Enqueue(ctx, deviceID, cmd, timeNow())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dev.RunSession(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	result, err := h.app.Store.Queue().Get(ctx, deviceID, queued.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != mdm.StateAcknowledged || result.Result == nil || result.Result.Status != syncml.StatusOK ||
		len(result.Result.Items) != 1 || result.Result.Items[0].Data.Text() != dev.Tree["./DevInfo/Man"] {
		t.Fatalf("read-only query was not successfully answered: %+v", result)
	}
}

func TestReenrollmentRefreshesDigestCredentials(t *testing.T) {
	h := newHarness(t)
	first, dev := enrollDevice(t, h)
	ctx := context.Background()
	dev.Generics = []simulator.GenericAlert{{Type: syncml.AlertTypeUnenrollmentUserRequest, Format: syncml.FormatInt, Data: "1"}}
	if _, err := dev.RunSession(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	serial := first.Certificate.SerialNumber.String()
	previous, err := h.app.Store.GetBySerial(ctx, serial)
	if err != nil || previous.State != storage.StateUnenrolled {
		t.Fatalf("first enrollment was not removed: %v", err)
	}
	cert, err := h.app.Store.Certificate(ctx, serial)
	if err != nil || !cert.Revoked {
		t.Fatalf("first certificate was not revoked: %v", err)
	}
	second, next := enrollDevice(t, h)
	for _, pair := range [][2]simulator.Credential{
		{first.Account.ServerAuth, second.Account.ServerAuth},
		{first.Account.ClientAuth, second.Account.ClientAuth},
	} {
		if pair[0].Secret == pair[1].Secret || len(pair[1].Nonce) != 16 || bytes.Equal(pair[0].Nonce, pair[1].Nonce) {
			t.Fatal("reenrollment must generate fresh secrets and initial digest nonces")
		}
	}
	if first.Certificate.SerialNumber.Cmp(second.Certificate.SerialNumber) == 0 {
		t.Fatal("reenrollment reused the previous certificate")
	}
	active, err := h.app.Store.Get(ctx, deviceID)
	if err != nil || active.State != storage.StateActive || active.Serial != second.Certificate.SerialNumber.String() {
		t.Fatalf("second enrollment is not active: %v", err)
	}
	// A new device object begins again at session 1, as after native unenrollment.
	// A dropped final exchange can leave an authenticated old session behind.
	if err := h.app.Service.Management.Config().Sessions.PutSession(ctx, &mdm.Session{
		Key: mdm.SessionKey(deviceID, "1"), DeviceID: deviceID, SessionID: "1",
		EnrollmentKey: serial, ClientMsgID: 9, ServerMsgID: 9,
		Authenticated: true, LastSeen: timeNow(),
		Sent: map[string]string{}, Children: map[string]string{},
	}); err != nil {
		t.Fatal(err)
	}
	tr, err := next.RunSession(ctx, "1")
	if err != nil || !tr.Ended || tr.Challenged != 1 {
		t.Fatalf("reenrolled device could not authenticate: transcript=%+v, err=%v", tr, err)
	}
}

func TestQueueRebootAndPolicy(t *testing.T) {
	h := newHarness(t)
	_, dev := enrollDevice(t, h)
	ctx := context.Background()
	q := h.app.Store.Queue()

	reboot, err := mdm.NewExec("./Device/Vendor/MSFT/Reboot/RebootNow", "")
	if err != nil {
		t.Fatal(err)
	}
	rebootQ, err := q.Enqueue(ctx, deviceID, reboot, timeNow())
	if err != nil {
		t.Fatal(err)
	}
	policy, err := mdm.NewReplace("./Vendor/MSFT/Policy/Config/Camera/AllowCamera", "0", mdm.WithFormat(syncml.FormatInt))
	if err != nil {
		t.Fatal(err)
	}
	policyQ, err := q.Enqueue(ctx, deviceID, policy, timeNow())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dev.RunSession(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{rebootQ.ID, policyQ.ID} {
		c, err := q.Get(ctx, deviceID, id)
		if err != nil || c.State != mdm.StateAcknowledged {
			t.Errorf("command %s state = %v, %v", id, c.State, err)
		}
	}
	if dev.Tree["./Vendor/MSFT/Policy/Config/Camera/AllowCamera"] != "0" {
		t.Error("policy not applied on the device")
	}
}

func TestChunkedGet(t *testing.T) {
	h := newHarness(t)
	_, dev := enrollDevice(t, h)
	ctx := context.Background()
	big := strings.Repeat("Z", 5000)
	dev.Tree = map[string]string{"./Vendor/MSFT/DiagnosticLog/Big": big, "./DevDetail/SwV": "10.0.26100.1", "./DevDetail/LrgObj": "true"}
	dev.UploadChunkSize = 1000
	get, err := mdm.NewGet([]string{"./Vendor/MSFT/DiagnosticLog/Big"})
	if err != nil {
		t.Fatal(err)
	}
	getQ, err := h.app.Store.Queue().Enqueue(ctx, deviceID, get, timeNow())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dev.RunSession(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	c, err := h.app.Store.Queue().Get(ctx, deviceID, getQ.ID)
	if err != nil || c.Result == nil || len(c.Result.Items) != 1 || c.Result.Items[0].Data.Text() != big {
		t.Errorf("chunked get result = %+v, %v", c.Result, err)
	}
}

func TestUnenroll(t *testing.T) {
	h := newHarness(t)
	_, dev := enrollDevice(t, h)
	ctx := context.Background()
	dev.Generics = []simulator.GenericAlert{{Type: syncml.AlertTypeUnenrollmentUserRequest, Format: syncml.FormatInt, Data: "1"}}
	if _, err := dev.RunSession(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	e, err := h.app.Store.GetBySerial(ctx, mustSerial(t, h))
	if err != nil {
		t.Fatal(err)
	}
	if e.State != storage.StateUnenrolled {
		t.Errorf("enrollment state after unenroll = %s", e.State)
	}
	cert, err := h.app.Store.Certificate(ctx, e.Serial)
	if err != nil || !cert.Revoked {
		t.Errorf("certificate revoked = %v, %v", cert.Revoked, err)
	}
}

func mustSerial(t *testing.T, h *harness) string {
	t.Helper()
	// The device may already be unenrolled; look it up by HWDevID history.
	list, err := h.app.Store.ListByHWDevID(context.Background(), strings.Repeat("A", 64))
	if err != nil || len(list) == 0 {
		t.Fatalf("hwdevid history = %v, %v", list, err)
	}
	return list[len(list)-1].Serial
}
