package syncml_test

import (
	"errors"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func valid() *syncml.Message {
	return &syncml.Message{
		Namespace: syncml.NamespaceSyncML12,
		Header: syncml.Header{VerDTD: "1.2", VerProto: "DM/1.2", SessionID: "1", MsgID: "1",
			Target: syncml.Location{LocURI: "dev"}, Source: syncml.Location{LocURI: "https://s"}},
		Body: syncml.Body{Commands: []syncml.Command{
			&syncml.Get{CmdID: "1", Items: []syncml.Item{{Target: "./DevDetail/SwV"}}},
		}, Final: true},
	}
}

func TestValidateAcceptsAValidMessage(t *testing.T) {
	t.Parallel()
	if err := syncml.Validate(valid()); err != nil {
		t.Fatal(err)
	}
	m := valid()
	m.Namespace = syncml.NamespaceSyncML11
	if err := syncml.Validate(m); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRules(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		mutate func(*syncml.Message)
		want   error
	}{
		"no namespace":    {func(m *syncml.Message) { m.Namespace = "" }, syncml.ErrVersion},
		"VerDTD 1.1":      {func(m *syncml.Message) { m.Header.VerDTD = "1.1" }, syncml.ErrVersion},
		"VerProto DM/1.1": {func(m *syncml.Message) { m.Header.VerProto = "DM/1.1" }, syncml.ErrVersion},
		"empty SessionID": {func(m *syncml.Message) { m.Header.SessionID = "" }, syncml.ErrSessionID},
		"MsgID zero":      {func(m *syncml.Message) { m.Header.MsgID = "0" }, syncml.ErrMsgID},
		"MsgID text":      {func(m *syncml.Message) { m.Header.MsgID = "one" }, syncml.ErrMsgID},
		"no Target":       {func(m *syncml.Message) { m.Header.Target.LocURI = "" }, syncml.ErrRouting},
		"no Source":       {func(m *syncml.Message) { m.Header.Source.LocURI = "" }, syncml.ErrRouting},
		"empty body":      {func(m *syncml.Message) { m.Body.Commands = nil }, syncml.ErrEmptyBody},
		"CmdID missing":   {func(m *syncml.Message) { m.Body.Commands[0].(*syncml.Get).CmdID = "" }, syncml.ErrCmdID},
		"CmdID zero":      {func(m *syncml.Message) { m.Body.Commands[0].(*syncml.Get).CmdID = "0" }, syncml.ErrCmdID},
		"duplicate CmdID": {func(m *syncml.Message) {
			m.Body.Commands = append(m.Body.Commands, &syncml.Delete{CmdID: "1", Items: []syncml.Item{{Target: "./x"}}})
		}, syncml.ErrDuplicateCmdID},
		"duplicate CmdID nested": {func(m *syncml.Message) {
			m.Body.Commands = append(m.Body.Commands, &syncml.Atomic{CmdID: "2", Commands: []syncml.Command{&syncml.Add{CmdID: "1", Items: []syncml.Item{{Target: "./x"}}}}})
		}, syncml.ErrDuplicateCmdID},
		"Get without Item": {func(m *syncml.Message) { m.Body.Commands[0].(*syncml.Get).Items = nil }, syncml.ErrItems},
		"Exec with two Items": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Exec{CmdID: "1", Items: []syncml.Item{{Target: "./a"}, {Target: "./b"}}}
		}, syncml.ErrItems},
		"Exec without Item":    {func(m *syncml.Message) { m.Body.Commands[0] = &syncml.Exec{CmdID: "1"} }, syncml.ErrItems},
		"Item without LocURI":  {func(m *syncml.Message) { m.Body.Commands[0].(*syncml.Get).Items[0].Target = "" }, syncml.ErrLocURI},
		"LocURI leading slash": {func(m *syncml.Message) { m.Body.Commands[0].(*syncml.Get).Items[0].Target = "/Vendor/MSFT/x" }, syncml.ErrLocURI},
		"LocURI empty segment": {func(m *syncml.Message) { m.Body.Commands[0].(*syncml.Get).Items[0].Target = "./Vendor//MSFT" }, syncml.ErrLocURI},
		"LocURI star segment":  {func(m *syncml.Message) { m.Body.Commands[0].(*syncml.Get).Items[0].Target = "./Vendor/*/x" }, syncml.ErrLocURI},
		"Data conflict": {func(m *syncml.Message) {
			m.Body.Commands[0].(*syncml.Get).Items[0].Data = &syncml.Data{Value: "a", XML: "<b/>"}
		}, syncml.ErrItems},
		"unknown format": {func(m *syncml.Message) {
			m.Body.Commands[0].(*syncml.Get).Items[0].Meta = &syncml.Meta{Format: "string"}
		}, syncml.ErrFormat},
		"unknown alert code": {func(m *syncml.Message) { m.Body.Commands[0] = &syncml.Alert{CmdID: "1", Data: "1100"} }, syncml.ErrAlertCode},
		"alert code text":    {func(m *syncml.Message) { m.Body.Commands[0] = &syncml.Alert{CmdID: "1", Data: "abort"} }, syncml.ErrAlertCode},
		"1224 without Item":  {func(m *syncml.Message) { m.Body.Commands[0] = &syncml.Alert{CmdID: "1", Data: "1224"} }, syncml.ErrItems},
		"1226 Item without Type": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Alert{CmdID: "1", Data: "1226", Items: []syncml.Item{{Meta: &syncml.Meta{Format: "int"}, Data: &syncml.Data{Value: "1"}}}}
		}, syncml.ErrItems},
		"Status without MsgRef": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Status{CmdID: "1", CmdRef: "0", Cmd: "SyncHdr", Data: syncml.Data{Value: "200"}}
		}, syncml.ErrStatusRef},
		"Status without CmdRef": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Status{CmdID: "1", MsgRef: "1", Cmd: "Get", Data: syncml.Data{Value: "200"}}
		}, syncml.ErrStatusRef},
		"Status unknown Cmd": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Status{CmdID: "1", MsgRef: "1", CmdRef: "2", Cmd: "Copy", Data: syncml.Data{Value: "200"}}
		}, syncml.ErrStatusRef},
		"Status CmdRef 0 not SyncHdr": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Status{CmdID: "1", MsgRef: "1", CmdRef: "0", Cmd: "Get", Data: syncml.Data{Value: "200"}}
		}, syncml.ErrStatusRef},
		"Status bad code": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Status{CmdID: "1", MsgRef: "1", CmdRef: "2", Cmd: "Get", Data: syncml.Data{Value: "OK"}}
		}, syncml.ErrStatusCode},
		"Status Chal bad type": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Status{CmdID: "1", MsgRef: "1", CmdRef: "0", Cmd: "SyncHdr", Data: syncml.Data{Value: "401"}, Chal: &syncml.Chal{Meta: syncml.Meta{Type: "syncml:auth-hmac"}}}
		}, syncml.ErrAuth},
		"Status Cred bad type": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Status{CmdID: "1", MsgRef: "1", CmdRef: "0", Cmd: "SyncHdr", Data: syncml.Data{Value: "200"}, Cred: &syncml.Cred{Meta: syncml.Meta{Type: "x"}, Data: "y"}}
		}, syncml.ErrAuth},
		"Results without CmdRef": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Results{CmdID: "1", Items: []syncml.Item{{Source: "./a", Data: &syncml.Data{Value: "1"}}}}
		}, syncml.ErrStatusRef},
		"Results bad MsgRef": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Results{CmdID: "1", MsgRef: "x", CmdRef: "2", Items: []syncml.Item{{Source: "./a"}}}
		}, syncml.ErrStatusRef},
		"Results without Item": {func(m *syncml.Message) { m.Body.Commands[0] = &syncml.Results{CmdID: "1", CmdRef: "2"} }, syncml.ErrItems},
		"Results bad format": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Results{CmdID: "1", CmdRef: "2", Meta: &syncml.Meta{Format: "nope"}, Items: []syncml.Item{{Source: "./a"}}}
		}, syncml.ErrFormat},
		"Cred bad type":          {func(m *syncml.Message) { m.Header.Cred = &syncml.Cred{Meta: syncml.Meta{Type: "x"}, Data: "y"} }, syncml.ErrAuth},
		"Cred no data":           {func(m *syncml.Message) { m.Header.Cred = &syncml.Cred{Meta: syncml.Meta{Type: syncml.AuthMD5}} }, syncml.ErrAuth},
		"header Meta bad format": {func(m *syncml.Message) { m.Header.Meta = &syncml.Meta{Format: "weird"} }, syncml.ErrFormat},
		"nested Atomic": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Atomic{CmdID: "1", Commands: []syncml.Command{&syncml.Atomic{CmdID: "2", Commands: []syncml.Command{&syncml.Add{CmdID: "3", Items: []syncml.Item{{Target: "./x"}}}}}}}
		}, syncml.ErrAtomicNested},
		"Get inside Atomic": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Atomic{CmdID: "1", Commands: []syncml.Command{&syncml.Get{CmdID: "2", Items: []syncml.Item{{Target: "./x"}}}}}
		}, syncml.ErrAtomicGet},
		"Get inside Sequence inside Atomic": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Atomic{CmdID: "1", Commands: []syncml.Command{&syncml.Sequence{CmdID: "2", Commands: []syncml.Command{&syncml.Get{CmdID: "3", Items: []syncml.Item{{Target: "./x"}}}}}}}
		}, syncml.ErrAtomicGet},
		"Add then Replace in Atomic": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Atomic{CmdID: "1", Commands: []syncml.Command{
				&syncml.Add{CmdID: "2", Items: []syncml.Item{{Target: "./x"}}},
				&syncml.Replace{CmdID: "3", Items: []syncml.Item{{Target: "./x", Data: &syncml.Data{Value: "1"}}}},
			}}
		}, syncml.ErrAtomicAddThenReplace},
		"empty Atomic":   {func(m *syncml.Message) { m.Body.Commands[0] = &syncml.Atomic{CmdID: "1"} }, syncml.ErrItems},
		"empty Sequence": {func(m *syncml.Message) { m.Body.Commands[0] = &syncml.Sequence{CmdID: "1"} }, syncml.ErrItems},
		"Add bad format": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Add{CmdID: "1", Meta: &syncml.Meta{Format: "nope"}, Items: []syncml.Item{{Target: "./x"}}}
		}, syncml.ErrFormat},
		"Replace bad format": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Replace{CmdID: "1", Meta: &syncml.Meta{Format: "nope"}, Items: []syncml.Item{{Target: "./x"}}}
		}, syncml.ErrFormat},
		"Delete without Item": {func(m *syncml.Message) { m.Body.Commands[0] = &syncml.Delete{CmdID: "1"} }, syncml.ErrItems},
		"alert item bad format": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Alert{CmdID: "1", Data: "1224", Items: []syncml.Item{{Meta: &syncml.Meta{Type: "t", Format: "nope"}}}}
		}, syncml.ErrFormat},
		"status item bad format": {func(m *syncml.Message) {
			m.Body.Commands[0] = &syncml.Status{CmdID: "1", MsgRef: "1", CmdRef: "0", Cmd: "SyncHdr", Data: syncml.Data{Value: "200"}, Items: []syncml.Item{{Meta: &syncml.Meta{Format: "nope"}}}}
		}, syncml.ErrFormat},
	}
	for name, tc := range cases {
		m := valid()
		tc.mutate(m)
		err := syncml.Validate(m)
		if !errors.Is(err, tc.want) || !errors.Is(err, syncml.ErrInvalid) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
		var ve *syncml.ValidationError
		if !errors.As(err, &ve) || ve.Path == "" || ve.Error() == "" {
			t.Errorf("%s: no ValidationError with a path: %v", name, err)
		}
	}
}

func TestValidateAllowsWindowsShapes(t *testing.T) {
	t.Parallel()
	m := valid()
	m.Body.Commands = []syncml.Command{
		&syncml.Atomic{CmdID: "1", Commands: []syncml.Command{
			&syncml.Add{CmdID: "2", Items: []syncml.Item{{Target: "./Device/Vendor/MSFT/EnterpriseDesktopAppManagement/MSI/%7BX%7D/DownloadInstall"}}},
			&syncml.Exec{CmdID: "3", Items: []syncml.Item{{Target: "./Device/Vendor/MSFT/EnterpriseDesktopAppManagement/MSI/%7BX%7D/DownloadInstall", Meta: &syncml.Meta{Format: "xml"}, Data: &syncml.Data{XML: "<MsiInstallJob/>"}}}},
			&syncml.Replace{CmdID: "4", Items: []syncml.Item{{Target: "./Device/Vendor/MSFT/Policy/Config/A/B", Data: &syncml.Data{Value: "1"}}}},
			&syncml.Delete{CmdID: "5", Items: []syncml.Item{{Target: "./Device/Vendor/MSFT/Policy/Config/A/C"}}},
		}},
		&syncml.Get{CmdID: "6", Items: []syncml.Item{{Target: "./Device/Vendor/MSFT/EnterpriseDesktopAppManagement?prop=Type"}}},
		&syncml.Sequence{CmdID: "7", Commands: []syncml.Command{&syncml.Get{CmdID: "8", Items: []syncml.Item{{Target: "./Vendor/MSFT/DeviceStatus"}}}}},
		&syncml.Status{CmdID: "9", MsgRef: "1", CmdRef: "0", Cmd: "SyncHdr", Data: syncml.Data{Value: "212"}, Chal: syncml.NewMD5Chal([]byte("n"))},
		&syncml.Results{CmdID: "10", CmdRef: "6", Cmd: "Get", Items: []syncml.Item{{Source: "./a", Meta: &syncml.Meta{Format: "node"}, Data: &syncml.Data{Value: "MSI/UpgradeCode"}}}},
	}
	if err := syncml.Validate(m); err != nil {
		t.Fatal(err)
	}
}

func TestCheckLocURIAndChildNodes(t *testing.T) {
	t.Parallel()
	for uri, ok := range map[string]bool{
		"./Device/Vendor/MSFT/Policy": true,
		"./DevInfo/DevId":             true,
		"./Vendor/MSFT/X?prop=Type":   true,
		"./Vendor/MSFT/X?list=Struct": true,
		"./Vendor/MSFT/DiagnosticLog/EtwLog/Channels/Microsoft-Client-Licensing-Platform%2FAdmin": true,
		"":                 false,
		"/Vendor/MSFT":     false,
		"./Vendor//MSFT":   false,
		"./Vendor/*":       false,
		"./Vendor/MSFT/":   false,
		"./Vendor/a.b/*/c": false,
	} {
		if err := syncml.CheckLocURI(uri); (err == nil) != ok {
			t.Errorf("CheckLocURI(%q) = %v, want ok=%v", uri, err, ok)
		}
	}
	if got := syncml.ChildNodes("MSI/UpgradeCode/%7B1803A630%7D"); len(got) != 3 || got[2] != "{1803A630}" {
		t.Fatalf("ChildNodes = %v", got)
	}
	if got := syncml.ChildNodes("  "); got != nil {
		t.Fatalf("ChildNodes(blank) = %v", got)
	}
	if got := syncml.ChildNodes("a//b/"); len(got) != 2 {
		t.Fatalf("ChildNodes(a//b/) = %v", got)
	}
}

func TestCodes(t *testing.T) {
	t.Parallel()
	if syncml.StatusOK.String() != "OK" || !syncml.StatusOK.Success() || syncml.StatusAtomicFailed.Success() || syncml.StatusCode(299).String() != "299" || syncml.StatusCode(299).Known() {
		t.Fatal("status codes")
	}
	if syncml.AlertNextMessage.String() != "NEXT MESSAGE" || syncml.AlertCode(1300).String() != "1300" || syncml.AlertCode(1300).Known() || syncml.AlertGeneric.Wire() != "1226" || syncml.StatusOK.Wire() != "200" {
		t.Fatal("alert codes")
	}
	if _, err := syncml.ParseStatusCode("99"); !errors.Is(err, syncml.ErrSyntax) {
		t.Fatal("ParseStatusCode(99)")
	}
	if _, err := syncml.ParseAlertCode("12"); !errors.Is(err, syncml.ErrSyntax) {
		t.Fatal("ParseAlertCode(12)")
	}
	if (&syncml.Alert{Data: "x"}).Code() != 0 || (&syncml.Status{Data: syncml.Data{Value: "x"}}).Code() != 0 {
		t.Fatal("unparsable codes must be 0")
	}
	if !syncml.KnownFormat("chr") || syncml.KnownFormat("string") {
		t.Fatal("formats")
	}
	var d *syncml.Data
	if d.Text() != "" || (&syncml.Data{XML: "<a/>"}).Text() != "<a/>" || (&syncml.Data{Value: "v"}).Text() != "v" {
		t.Fatal("Data.Text")
	}
	it := syncml.Item{Source: "./s"}
	if it.LocURI() != "./s" {
		t.Fatal("LocURI falls back to Source")
	}
}
