package syncml_test

import (
	"errors"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func TestResponseHelpers(t *testing.T) {
	t.Parallel()
	req, err := syncml.Unmarshal(fixture(t, "fleet-mdmtest-package1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	resp := syncml.NewResponse(req, "https://mdm.example.test/ManagementServer/MDM.svc", "1")
	if resp.Header.SessionID != "0" || resp.Header.MsgID != "1" || resp.Header.Target.LocURI != req.Header.Source.LocURI || resp.Header.Source.LocURI != "https://mdm.example.test/ManagementServer/MDM.svc" || resp.Namespace != syncml.NamespaceSyncML12 {
		t.Fatalf("response header %+v", resp.Header)
	}
	if r := syncml.NewResponse(req, "", "1"); r.Header.Source.LocURI != req.Header.Target.LocURI {
		t.Fatal("default source")
	}
	var ids syncml.CmdIDs
	resp.Body.Commands = []syncml.Command{
		syncml.StatusForHeader(req, ids.Next(), syncml.StatusOK),
		syncml.StatusFor(req, req.Body.Commands[0], ids.Next(), syncml.StatusOK),
		&syncml.Get{CmdID: ids.Next(), Items: []syncml.Item{{Target: "./DevDetail/SwV"}}},
	}
	resp.Body.Final = true
	if err := syncml.Validate(resp); err != nil {
		t.Fatal(err)
	}
	st := resp.Body.Statuses()
	if st[0].CmdRef != "0" || st[0].Cmd != "SyncHdr" || st[0].MsgRef != "1" || st[1].CmdRef != "2" || st[1].Cmd != "Alert" || st[1].CmdID != "2" {
		t.Fatalf("statuses %+v %+v", st[0], st[1])
	}

	next := syncml.NewNextMessage(req, "https://s", "2")
	if err := syncml.Validate(next); err != nil {
		t.Fatal(err)
	}
	if next.Body.Final || len(next.Body.Commands) != 2 || !next.Body.HasAlert(syncml.AlertNextMessage) || !syncml.IsNextMessageRequest(next) {
		t.Fatalf("next message %+v", next.Body)
	}
	abort := syncml.NewAbort(req, "https://s", "2")
	if err := syncml.Validate(abort); err != nil {
		t.Fatal(err)
	}
	if !abort.Body.Final || !abort.Body.HasAlert(syncml.AlertSessionAbort) || syncml.IsNextMessageRequest(abort) {
		t.Fatalf("abort %+v", abort.Body)
	}
	if syncml.IsNextMessageRequest(req) || syncml.IsNextMessageRequest(&syncml.Message{}) {
		t.Fatal("package 1 is not a next-message request")
	}
	mixed := &syncml.Message{Body: syncml.Body{Commands: []syncml.Command{&syncml.Alert{CmdID: "1", Data: "1222"}, &syncml.Get{CmdID: "2"}}}}
	if syncml.IsNextMessageRequest(mixed) {
		t.Fatal("1222 with other commands is not a bare next-message request")
	}
}

func TestNextMsgIDAndPackageDetection(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"": "1", "1": "2", "41": "42"} {
		if got, err := syncml.NextMsgID(in); err != nil || got != want {
			t.Errorf("NextMsgID(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"0", "-1", "x"} {
		if _, err := syncml.NextMsgID(in); !errors.Is(err, syncml.ErrMsgID) {
			t.Errorf("NextMsgID(%q) accepted", in)
		}
	}
	noAlert := &syncml.Message{Body: syncml.Body{Commands: []syncml.Command{&syncml.Replace{CmdID: "1", Items: []syncml.Item{{Source: "./DevInfo/DevId", Data: &syncml.Data{Value: "d"}}}}}}}
	if syncml.IsPackageOne(noAlert) || syncml.DevInfo(noAlert)["DevId"] != "d" {
		t.Fatal("package 1 needs the alert; DevInfo is independent")
	}
	noDevInfo := &syncml.Message{Body: syncml.Body{Commands: []syncml.Command{&syncml.Alert{CmdID: "1", Data: "1201"}}}}
	if syncml.IsPackageOne(noDevInfo) || syncml.LoginStatus(noDevInfo) != "" || len(syncml.DevInfo(noDevInfo)) != 0 {
		t.Fatal("package 1 needs DevInfo")
	}
	server := &syncml.Message{Body: syncml.Body{Commands: []syncml.Command{&syncml.Alert{CmdID: "1", Data: "1200"}, &syncml.Replace{CmdID: "2", Items: []syncml.Item{{Source: "./DevInfo/Mod", Data: &syncml.Data{Value: "m"}}}}}}}
	if !syncml.IsPackageOne(server) {
		t.Fatal("1200 also opens a session")
	}
}
