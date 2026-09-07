package mdm

import (
	"context"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func TestNopHooks(t *testing.T) {
	t.Parallel()
	var h NopHooks
	if h.PackageOne(context.Background(), "d", "s", Facts{}) != nil ||
		h.GenericAlert(context.Background(), Event{}) != nil ||
		h.ClientEvent(context.Background(), Event{}) != nil ||
		h.Unenrolled(context.Background(), "d") != nil {
		t.Error("NopHooks returned an error")
	}
}

func TestStateTerminal(t *testing.T) {
	t.Parallel()
	for _, s := range []State{StateAcknowledged, StateFailed, StateCancelled} {
		if !s.Terminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	for _, s := range []State{StatePending, StateSent} {
		if s.Terminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}
}

func TestCloneAndAssign(t *testing.T) {
	t.Parallel()
	items := []syncml.Item{{Target: "./A", Meta: &syncml.Meta{Format: "int"}, Data: &syncml.Data{Value: "1"}}}
	cmds := []syncml.Command{
		&syncml.Add{Items: items}, &syncml.Replace{Items: items}, &syncml.Delete{Items: items},
		&syncml.Get{Items: items}, &syncml.Exec{Items: items},
		&syncml.Atomic{Commands: []syncml.Command{&syncml.Replace{Items: items}}},
		&syncml.Sequence{Commands: []syncml.Command{&syncml.Get{Items: items}}},
	}
	for _, c := range cmds {
		clone := cloneCommand(c)
		assignID(clone, "99")
		if clone.ID() != "99" {
			t.Errorf("%s: assignID = %q", c.Name(), clone.ID())
		}
		// The clone's items are independent.
		if leaves := itemsOf(clone); len(leaves) > 0 {
			leaves[0].Target = "changed"
			if itemsOf(c)[0].Target == "changed" {
				t.Errorf("%s: clone shares items", c.Name())
			}
			setItems(clone, items)
		}
	}
	// A Status is returned unchanged by cloneCommand.
	if cloneCommand(&syncml.Status{CmdID: "1"}).ID() != "1" {
		t.Error("clone of status")
	}
}

func TestLeavesAndItoa(t *testing.T) {
	t.Parallel()
	seq := &syncml.Sequence{Commands: []syncml.Command{&syncml.Get{CmdID: "1"}, &syncml.Replace{CmdID: "2"}}}
	if got := Leaves(seq); len(got) != 2 {
		t.Errorf("sequence leaves = %d", len(got))
	}
	if got := Leaves(&syncml.Add{CmdID: "1"}); len(got) != 1 {
		t.Errorf("add leaves = %d", len(got))
	}
	for n, want := range map[int]string{0: "0", 5: "5", -7: "-7", 1000: "1000"} {
		if got := itoa(n); got != want {
			t.Errorf("itoa(%d) = %q", n, got)
		}
	}
	if firstNonEmpty("", "", "x") != "x" || firstNonEmpty("") != "" {
		t.Error("firstNonEmpty")
	}
}

func TestScopeAllows(t *testing.T) {
	t.Parallel()
	s := &Service{}
	// AVD device session: only device commands.
	sess := &Session{Facts: Facts{SyncType: "device", LoginStatus: syncml.LoginStatusUser}}
	if !s.scopeAllows(sess, ScopeDevice) || s.scopeAllows(sess, ScopeUser) {
		t.Error("device session scope")
	}
	// AVD user session: only user commands.
	sess = &Session{Facts: Facts{SyncType: syncml.LoginStatusUser, LoginStatus: syncml.LoginStatusUser}}
	if s.scopeAllows(sess, ScopeDevice) || !s.scopeAllows(sess, ScopeUser) {
		t.Error("user session scope")
	}
	// Mixed session, no user: user commands held.
	sess = &Session{Facts: Facts{LoginStatus: syncml.LoginStatusNone}}
	if !s.scopeAllows(sess, ScopeDevice) || s.scopeAllows(sess, ScopeUser) {
		t.Error("mixed no-user scope")
	}
}

func TestVerifyCredential(t *testing.T) {
	t.Parallel()
	nonce := []byte{1, 2, 3, 4}
	hash := syncml.CredentialHash("u", "p")
	md5ID := &Identity{AuthType: AuthDigest, CredentialHash: hash}
	good := syncml.NewMD5Cred(syncml.MD5DigestFromHash(hash, nonce))
	if !verifyCredential(md5ID, good, nonce) {
		t.Error("valid MD5 rejected")
	}
	if verifyCredential(md5ID, nil, nonce) {
		t.Error("nil cred accepted")
	}
	if verifyCredential(md5ID, good, nil) {
		t.Error("no nonce accepted")
	}
	if verifyCredential(md5ID, syncml.NewBasicCred("u", "p"), nonce) {
		t.Error("wrong cred type accepted")
	}
	basicID := &Identity{AuthType: AuthBasic, BasicUsername: "u", BasicPassword: "p"}
	if !verifyCredential(basicID, syncml.NewBasicCred("u", "p"), nil) {
		t.Error("valid basic rejected")
	}
	if verifyCredential(basicID, syncml.NewBasicCred("u", "wrong"), nil) {
		t.Error("wrong basic accepted")
	}
	if verifyCredential(basicID, &syncml.Cred{Meta: syncml.Meta{Type: syncml.AuthBasic}, Data: "not base64!"}, nil) {
		t.Error("garbage basic accepted")
	}
	if verifyCredential(&Identity{AuthType: AuthCertificate}, good, nonce) {
		t.Error("certificate identity verified a credential")
	}
}

func TestOnlyHeaderStatusAndStatusFor(t *testing.T) {
	t.Parallel()
	resp := &syncml.Message{}
	resp.Body.Commands = []syncml.Command{&syncml.Status{CmdRef: "0"}}
	if !onlyHeaderStatus(resp) {
		t.Error("header-only not detected")
	}
	resp.Body.Commands = append(resp.Body.Commands, &syncml.Get{})
	if onlyHeaderStatus(resp) {
		t.Error("non-header status")
	}
}
