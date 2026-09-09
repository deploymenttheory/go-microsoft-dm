package mdm_test

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
)

const dev = "DEVICE-01"

// fakeAuth trusts the device unconditionally, so tests can focus on session
// and message rules.
type fakeAuth struct{ authType mdm.AuthType }

func (f fakeAuth) Lookup(_ context.Context, deviceID string) (*mdm.Identity, error) {
	if deviceID != dev {
		return nil, mdm.ErrUnenrolled
	}
	return &mdm.Identity{DeviceID: dev, EnrollmentKey: "1", AuthType: f.authType}, nil
}

func (f fakeAuth) TrustCertificate(context.Context, *mdm.Identity, [][]*x509.Certificate) bool {
	return f.authType == mdm.AuthCertificate
}

func newService(t testing.TB) *mdm.Service {
	t.Helper()
	svc, err := mdm.New(mdm.Config{
		ServerURL: "https://mdm.example.test/svc", Auth: fakeAuth{authType: mdm.AuthCertificate},
		Queue: inmem.NewQueue(), Clock: clock.NewFake(t0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

var t0 = timeDate()

func packageOne(sessionID, msgID string) string {
	return fmt.Sprintf(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>`+
		`<SessionID>%s</SessionID><MsgID>%s</MsgID><Target><LocURI>https://mdm.example.test/svc</LocURI></Target>`+
		`<Source><LocURI>%s</LocURI></Source></SyncHdr><SyncBody><Alert><CmdID>1</CmdID><Data>1201</Data></Alert>`+
		`<Replace><CmdID>2</CmdID><Item><Source><LocURI>./DevInfo/DevId</LocURI></Source><Data>%s</Data></Item></Replace>`+
		`<Final/></SyncBody></SyncML>`, sessionID, msgID, dev, dev)
}

func TestHandleNewConfig(t *testing.T) {
	t.Parallel()
	if _, err := mdm.New(mdm.Config{}); !errors.Is(err, mdm.ErrConfig) {
		t.Errorf("no url: %v", err)
	}
	if _, err := mdm.New(mdm.Config{ServerURL: "x"}); !errors.Is(err, mdm.ErrConfig) {
		t.Errorf("no auth/queue: %v", err)
	}
	svc := newService(t)
	cfg := svc.Config()
	if cfg.Sessions == nil || cfg.Nonce == nil || cfg.MaxRequestSize == 0 || cfg.NonceTTL == 0 || cfg.MaxMessageBytes == 0 {
		t.Errorf("defaults not filled: %+v", cfg)
	}
}

func TestHandleFirstSessionSucceeds(t *testing.T) {
	t.Parallel()
	svc := newService(t)
	out, err := svc.Handle(context.Background(), verifiedTransport(), []byte(packageOne("1", "1")))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := syncml.Decode(out, syncml.DecodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Header.MsgID != "1" || resp.Header.SessionID != "1" {
		t.Errorf("response header = %+v", resp.Header)
	}
	// The SyncHdr status is 200.
	if len(resp.Body.Statuses()) == 0 || resp.Body.Statuses()[0].Code() != syncml.StatusOK {
		t.Errorf("no ok status: %+v", resp.Body.Statuses())
	}
}

func TestHandleRejects(t *testing.T) {
	t.Parallel()
	svc := newService(t)
	ctx := context.Background()
	tr := verifiedTransport()

	// New session must open with MsgID 1.
	if _, err := svc.Handle(ctx, tr, []byte(packageOne("1", "2"))); !errors.Is(err, syncml.ErrInvalid) {
		t.Errorf("msgid 2 on new session: %v", err)
	}
	// First message must be package 1 (has the Alert and DevInfo).
	noAlert := strings.Replace(packageOne("1", "1"), `<Alert><CmdID>1</CmdID><Data>1201</Data></Alert>`, "", 1)
	if _, err := svc.Handle(ctx, tr, []byte(noAlert)); !errors.Is(err, syncml.ErrInvalid) {
		t.Errorf("no alert in package 1: %v", err)
	}
	// A LocURI starting with "/" is rejected by validation.
	badURI := strings.Replace(packageOne("1", "1"), "./DevInfo/DevId", "/DevInfo/DevId", 1)
	if _, err := svc.Handle(ctx, tr, []byte(badURI)); !errors.Is(err, syncml.ErrInvalid) {
		t.Errorf("bad locuri: %v", err)
	}
	// An unenrolled device.
	other := strings.ReplaceAll(packageOne("1", "1"), dev, "UNKNOWN")
	if _, err := svc.Handle(ctx, tr, []byte(other)); !errors.Is(err, mdm.ErrUnenrolled) {
		t.Errorf("unenrolled: %v", err)
	}
	// Malformed XML.
	if _, err := svc.Handle(ctx, tr, []byte("<SyncML")); !errors.Is(err, syncml.ErrSyntax) {
		t.Errorf("malformed: %v", err)
	}
	// Oversize.
	svc2, _ := mdm.New(mdm.Config{ServerURL: "https://x/svc", Auth: fakeAuth{}, Queue: inmem.NewQueue(), MaxRequestSize: 32, Clock: clock.NewFake(t0)})
	if _, err := svc2.Handle(ctx, tr, []byte(packageOne("1", "1"))); !errors.Is(err, syncml.ErrTooLarge) {
		t.Errorf("oversize: %v", err)
	}
}

func TestHandleMidSessionSequence(t *testing.T) {
	t.Parallel()
	svc := newService(t)
	ctx := context.Background()
	tr := verifiedTransport()
	if _, err := svc.Handle(ctx, tr, []byte(packageOne("1", "1"))); err != nil {
		t.Fatal(err)
	}
	// A second message on the same session must carry MsgID 2, not 3.
	msg3 := fmt.Sprintf(`<SyncML xmlns="SYNCML:SYNCML1.2"><SyncHdr><VerDTD>1.2</VerDTD><VerProto>DM/1.2</VerProto>`+
		`<SessionID>1</SessionID><MsgID>3</MsgID><Target><LocURI>https://mdm.example.test/svc</LocURI></Target>`+
		`<Source><LocURI>%s</LocURI></Source></SyncHdr><SyncBody><Status><CmdID>1</CmdID><MsgRef>1</MsgRef><CmdRef>0</CmdRef><Cmd>SyncHdr</Cmd><Data>200</Data></Status><Final/></SyncBody></SyncML>`, dev)
	if _, err := svc.Handle(ctx, tr, []byte(msg3)); !errors.Is(err, syncml.ErrInvalid) {
		t.Errorf("wrong msgid sequence: %v", err)
	}
	// A different SessionID mid-flight is a new session opened at MsgID 2.
	if _, err := svc.Handle(ctx, tr, []byte(packageOne("2", "2"))); !errors.Is(err, syncml.ErrInvalid) {
		t.Errorf("session id change: %v", err)
	}
}

func timeDate() (t timeT) { return t0Value }

// verifiedTransport supplies trusted transport evidence to tests of session
// behavior. TLS verification itself is exercised with real simulator handshakes.
func verifiedTransport() *mdm.Transport {
	leaf := &x509.Certificate{Raw: []byte("verified test leaf")}
	return &mdm.Transport{Certificates: []*x509.Certificate{leaf}, VerifiedChains: [][]*x509.Certificate{{leaf}}}
}
