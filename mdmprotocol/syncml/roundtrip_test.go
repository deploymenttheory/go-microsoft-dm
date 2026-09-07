package syncml_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestFixturesRoundTrip decodes every message fixture, encodes it, decodes
// the result and requires the two structures to be identical. It also
// requires Encode to be idempotent over its own output.
func TestFixturesRoundTrip(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".xml") {
			continue
		}
		raw := fixture(t, name)
		if !bytes.Contains(raw, []byte("<SyncML")) {
			continue // command fragments are covered by TestFragmentsRoundTrip
		}
		n++
		first, err := syncml.Unmarshal(raw)
		if err != nil {
			t.Errorf("%s: decode: %v", name, err)
			continue
		}
		if first.Namespace == "" {
			// A fragment without xmlns is written with the 1.2 namespace.
			first.Namespace = syncml.NamespaceSyncML12
		}
		for _, opts := range []syncml.EncodeOptions{{}, {Indent: "  "}, {Indent: "\t", XMLDeclaration: true}} {
			out, err := syncml.Encode(first, opts)
			if err != nil {
				t.Errorf("%s: encode: %v", name, err)
				continue
			}
			second, err := syncml.Unmarshal(out)
			if err != nil {
				t.Errorf("%s: decode after encode: %v\n%s", name, err, out)
				continue
			}
			if !reflect.DeepEqual(first, second) {
				t.Errorf("%s: round trip changed the message\nfirst:  %+v\nsecond: %+v\n%s", name, first, second, out)
			}
			again, err := syncml.Encode(second, opts)
			if err != nil || !bytes.Equal(out, again) {
				t.Errorf("%s: encode is not idempotent (%v)", name, err)
			}
			if bytes.Contains(out, []byte("CDATA")) {
				t.Errorf("%s: encoder produced CDATA", name)
			}
		}
	}
	if n < 10 {
		t.Fatalf("only %d message fixtures found", n)
	}
}

func TestFragmentsRoundTrip(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"learn-oma-dm-loginstatus-alert.xml", "learn-edam-alert-1224-win32csp.xml"} {
		c, err := syncml.DecodeCommand(fixture(t, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		a, ok := c.(*syncml.Alert)
		if !ok || a.Code() != syncml.AlertClientEvent || len(a.Items) != 1 || a.Items[0].Meta == nil {
			t.Fatalf("%s: decoded %+v", name, c)
		}
		out, err := syncml.EncodeCommand(c, syncml.EncodeOptions{Indent: " "})
		if err != nil {
			t.Fatal(err)
		}
		back, err := syncml.DecodeCommand(out)
		if err != nil || !reflect.DeepEqual(c, back) {
			t.Fatalf("%s: fragment round trip: %v\n%s", name, err, out)
		}
	}
}

// TestFleetPackageOne reads the facts the session engine needs from the
// package 1 Fleet's test client sends.
func TestFleetPackageOne(t *testing.T) {
	t.Parallel()
	m, err := syncml.Unmarshal(fixture(t, "fleet-mdmtest-package1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := syncml.Validate(m); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if m.Namespace != syncml.NamespaceSyncML12 || m.Header.VerDTD != "1.2" || m.Header.VerProto != "DM/1.2" ||
		m.Header.SessionID != "0" || m.Header.MsgID != "1" || !m.Body.Final {
		t.Fatalf("header %+v final %v", m.Header, m.Body.Final)
	}
	if !syncml.IsPackageOne(m) || !m.Body.HasAlert(syncml.AlertClientInitiated) {
		t.Fatal("not recognised as package 1")
	}
	if got := syncml.LoginStatus(m); got != syncml.LoginStatusUser {
		t.Fatalf("LoginStatus = %q", got)
	}
	info := syncml.DevInfo(m)
	if info["DevId"] != "F717C0F0-5F68-4AC3-A341-01B2544219DF" || info["Man"] != "VMware, Inc." || info["Lang"] != "en-US" || len(info) != 5 {
		t.Fatalf("DevInfo = %v", info)
	}
	if got := len(m.Body.Commands); got != 3 || m.Body.Commands[0].ID() != "2" || m.Body.Commands[2].Name() != syncml.CmdReplace {
		t.Fatalf("commands = %d, order lost", got)
	}
}

func TestUnenrollAlertAndNamespaces(t *testing.T) {
	t.Parallel()
	m, err := syncml.Unmarshal(fixture(t, "fleet-mdmtest-unenroll-1226.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := syncml.Validate(m); err != nil {
		t.Fatal(err)
	}
	a := m.Body.Alerts()[0]
	if a.Code() != syncml.AlertGeneric || a.Items[0].Meta.Type != syncml.AlertTypeUnenrollmentUserRequest || a.Items[0].Meta.Format != syncml.FormatInt || a.Items[0].Data.Value != "1" {
		t.Fatalf("alert %+v", a)
	}

	// The WinDC fixture is 1.1 and carries markup in the alert Data.
	w, err := syncml.Unmarshal(fixture(t, "learn-windc-1224-declaredconfigurationdocuments.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if w.Namespace != syncml.NamespaceSyncML11 {
		t.Fatalf("namespace %q", w.Namespace)
	}
	if err := syncml.Validate(w); err != nil {
		t.Fatalf("1.1 must validate: %v", err)
	}
	item := w.Body.Alerts()[1].Items[0]
	if item.Meta.Type != syncml.AlertTypeDeclaredConfigurationDocuments || !strings.HasPrefix(item.Data.XML, "<DeclaredConfigurations") || item.Data.Value != "" {
		t.Fatalf("windc item %+v", item)
	}
	out, err := syncml.Marshal(w)
	if err != nil || !bytes.Contains(out, []byte(`<SyncML xmlns="SYNCML:SYNCML1.1">`)) || !bytes.Contains(out, []byte(`state="60"/>`)) {
		t.Fatalf("1.1 not preserved or markup mangled: %v\n%s", err, out)
	}
}

func TestLearnSamplesTolerated(t *testing.T) {
	t.Parallel()
	// Markup inside Results/Data and an empty SyncHdr.
	r, err := syncml.Unmarshal(fixture(t, "learn-diagnosticlog-results-collection.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Namespace != "" {
		t.Fatalf("namespace %q", r.Namespace)
	}
	res := r.Body.Commands[2].(*syncml.Results)
	if res.CmdRef != "1" || len(res.Items) != 1 || !strings.Contains(res.Items[0].Data.XML, `<Command HRESULT="-2147024637">`) {
		t.Fatalf("results %+v", res)
	}
	if err := syncml.Validate(r); err == nil {
		t.Fatal("a fragment without a header must not validate")
	}

	// Type without the metinf namespace and a LocURI padded with newlines.
	p, err := syncml.Unmarshal(fixture(t, "learn-diagnosticlog-replace-type-no-ns.xml"))
	if err != nil {
		t.Fatal(err)
	}
	it := p.Body.Commands[0].(*syncml.Replace).Items[0]
	if it.Target != "./Vendor/MSFT/DiagnosticLog/EtwLog/Collectors/DeviceManagement/Providers/3da494e4-0fe2-415C-b895-fb5265c5c83b/Keywords" || it.Meta.Type != "text/plain" || it.Meta.Format != "chr" {
		t.Fatalf("item %+v", it)
	}

	// A utf-16 declaration over UTF-8 bytes, self-closing SyncHdr and Final with a space.
	u, err := syncml.Unmarshal(fixture(t, "learn-edam-response-utf16-decl.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !u.Body.Final || len(u.Body.Statuses()) != 2 || u.Body.Statuses()[0].Cmd != syncml.CmdSyncHdr || u.Body.Statuses()[1].Code() != syncml.StatusOK {
		t.Fatalf("statuses %+v", u.Body.Statuses())
	}

	// The MsiInstallJob document inside Exec/Data survives verbatim.
	e, err := syncml.Unmarshal(fixture(t, "learn-edam-add-exec-msiinstalljob.xml"))
	if err != nil {
		t.Fatal(err)
	}
	ex := e.Body.Commands[1].(*syncml.Exec)
	if ex.CmdID != "67890" || !strings.HasPrefix(ex.Items[0].Data.XML, `<MsiInstallJob id="{9BD4F7CD-880A-40B5-B74C-1BEECB51E596}">`) || !strings.HasSuffix(ex.Items[0].Data.XML, "</MsiInstallJob>") {
		t.Fatalf("exec %+v", ex.Items[0].Data)
	}
	if ex.Items[0].Meta.Format != syncml.FormatXML {
		t.Fatalf("format %q", ex.Items[0].Meta.Format)
	}
}

func TestMD5CredHeaderFixture(t *testing.T) {
	t.Parallel()
	m, err := syncml.Unmarshal(fixture(t, "oma-security-5.3.1-md5-cred.xml"))
	if err != nil {
		t.Fatal(err)
	}
	h := m.Header
	if h.Source.LocName != "Bruce1" || h.Cred == nil || h.Cred.Meta.Type != syncml.AuthMD5 || h.Cred.Meta.Format != "b64" || h.Cred.Data != "18EA3F" || h.Meta == nil || h.Meta.MaxMsgSize != 5000 {
		t.Fatalf("header %+v cred %+v meta %+v", h, h.Cred, h.Meta)
	}
	if err := syncml.Validate(m); err != nil {
		t.Fatal(err)
	}
}

func TestAtomicAndStatusFixtures(t *testing.T) {
	t.Parallel()
	a, err := syncml.Unmarshal(fixture(t, "msmdm-3.1.5.1.3-atomic.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := syncml.Validate(a); err != nil {
		t.Fatal(err)
	}
	at := a.Body.Commands[0].(*syncml.Atomic)
	if at.CmdID != "10" || len(at.Commands) != 2 || at.Commands[1].ID() != "9" {
		t.Fatalf("atomic %+v", at)
	}
	var seen []string
	a.Body.Walk(func(c syncml.Command, depth int) bool {
		seen = append(seen, c.Name()+"@"+string(rune('0'+depth)))
		return true
	})
	if strings.Join(seen, ",") != "Atomic@0,Replace@1,Replace@1" {
		t.Fatalf("walk %v", seen)
	}

	s, err := syncml.Unmarshal(fixture(t, "msmdm-3.1.5.2-status-results.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := syncml.Validate(s); err != nil {
		t.Fatal(err)
	}
	st := s.Body.Statuses()
	if len(st) != 2 || st[0].CmdRef != "0" || st[1].Cmd != syncml.CmdGet || st[1].Code() != syncml.StatusOK {
		t.Fatalf("statuses %+v", st)
	}
	res := s.Body.Commands[2].(*syncml.Results)
	if res.Cmd != syncml.CmdGet || res.Items[0].Source != "./DevDetail/SwV" || res.Items[0].Data.Value != "10.0.26100.9278" {
		t.Fatalf("results %+v", res)
	}
	var target *syncml.ValidationError
	if err := syncml.Validate(&syncml.Message{}); !errors.As(err, &target) {
		t.Fatalf("empty message: %v", err)
	}
}
