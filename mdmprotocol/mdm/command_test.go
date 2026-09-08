package mdm

import (
	"errors"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/schema/registry"
)

func TestScopeOf(t *testing.T) {
	t.Parallel()
	cases := map[string]Scope{
		"./User/Vendor/MSFT/Policy": ScopeUser, "./user/x": ScopeUser, "./User": ScopeUser,
		"./Device/Vendor/MSFT/Policy": ScopeDevice, "./Vendor/MSFT/DMClient": ScopeDevice,
		"./DevDetail/SwV": ScopeDevice, "./UserData/x": ScopeDevice,
	}
	for uri, want := range cases {
		if got := ScopeOf(uri); got != want {
			t.Errorf("ScopeOf(%q) = %s, want %s", uri, got, want)
		}
	}
}

func TestBuilders(t *testing.T) {
	t.Parallel()
	add, err := NewAdd("./Vendor/MSFT/Policy/Config/A/B", "1", WithFormat(syncml.FormatInt), WithID("a"))
	if err != nil || add.ID != "a" || add.Scope != ScopeDevice {
		t.Fatalf("add = %+v, %v", add, err)
	}
	if it := itemsOf(add.Body)[0]; it.Target != "./Vendor/MSFT/Policy/Config/A/B" || it.Data.Value != "1" || it.Meta.Format != syncml.FormatInt {
		t.Errorf("add item = %+v", it)
	}
	repl, err := NewReplace("./User/Vendor/MSFT/Policy/Config/C", "2")
	if err != nil || repl.Scope != ScopeUser {
		t.Errorf("replace = %+v, %v", repl, err)
	}
	del, err := NewDelete("./Vendor/MSFT/WiFi/Profile/X")
	if err != nil || len(itemsOf(del.Body)) != 1 || itemsOf(del.Body)[0].Data != nil {
		t.Errorf("delete = %+v, %v", del, err)
	}
	get, err := NewGet([]string{"./DevDetail/SwV", "./DevDetail/LrgObj"})
	if err != nil || len(itemsOf(get.Body)) != 2 {
		t.Errorf("get = %+v, %v", get, err)
	}
	exec, err := NewExec("./Vendor/MSFT/DMClient/Provider/P/Unenroll", "")
	if err != nil || itemsOf(exec.Body)[0].Data != nil {
		t.Errorf("exec = %+v, %v", exec, err)
	}
	execData, err := NewExec("./x/Exec", "params")
	if err != nil || execData.Body.(*syncml.Exec).Items[0].Data.Value != "params" {
		t.Errorf("exec data = %+v", execData)
	}
	xml, err := NewReplace("./Vendor/MSFT/DeclaredConfiguration/X", "<Doc/>", WithXML(), WithType("application/xml"))
	if err != nil || itemsOf(xml.Body)[0].Data.XML != "<Doc/>" {
		t.Errorf("xml = %+v", xml)
	}
}

func TestBuilderRejects(t *testing.T) {
	t.Parallel()
	if _, err := NewAdd("/bad", "1"); !errors.Is(err, ErrCommand) {
		t.Errorf("bad locuri: %v", err)
	}
	if _, err := NewGet(nil); !errors.Is(err, ErrCommand) {
		t.Errorf("empty get: %v", err)
	}
	if _, err := NewGet([]string{"./Device/A", "./User/B"}); !errors.Is(err, ErrScope) {
		t.Errorf("mixed get scope: %v", err)
	}
}

func TestAtomicRules(t *testing.T) {
	t.Parallel()
	a, _ := NewReplace("./Vendor/MSFT/Policy/Config/A", "1")
	b, _ := NewReplace("./Vendor/MSFT/Policy/Config/B", "2")
	ok, err := NewAtomic([]*Command{a, b})
	if err != nil || len(ok.Body.(*syncml.Atomic).Commands) != 2 {
		t.Fatalf("atomic = %+v, %v", ok, err)
	}
	// Nested Atomic.
	if _, err := NewAtomic([]*Command{ok}); !errors.Is(err, ErrCommand) {
		t.Errorf("nested atomic: %v", err)
	}
	// Get inside Atomic.
	g, _ := NewGet([]string{"./DevDetail/SwV"})
	if _, err := NewAtomic([]*Command{g}); !errors.Is(err, ErrCommand) {
		t.Errorf("get in atomic: %v", err)
	}
	// Get inside Atomic through a Sequence.
	seq, _ := NewSequence([]*Command{g})
	if _, err := NewAtomic([]*Command{seq}); !errors.Is(err, ErrCommand) {
		t.Errorf("get in atomic via sequence: %v", err)
	}
	// Add then Replace on the same node.
	addA, _ := NewAdd("./Vendor/MSFT/Policy/Config/A", "1")
	replA, _ := NewReplace("./Vendor/MSFT/Policy/Config/A", "2")
	if _, err := NewAtomic([]*Command{addA, replA}); !errors.Is(err, ErrCommand) {
		t.Errorf("add then replace: %v", err)
	}
	// Mixed scope.
	userCmd, _ := NewReplace("./User/Vendor/MSFT/Policy/Config/U", "1")
	if _, err := NewAtomic([]*Command{a, userCmd}); !errors.Is(err, ErrScope) {
		t.Errorf("mixed scope atomic: %v", err)
	}
	if _, err := NewAtomic(nil); !errors.Is(err, ErrCommand) {
		t.Errorf("empty atomic: %v", err)
	}
	if _, err := NewAtomic([]*Command{nil}); !errors.Is(err, ErrCommand) {
		t.Errorf("nil member: %v", err)
	}
}

func TestSequenceRules(t *testing.T) {
	t.Parallel()
	a, _ := NewReplace("./Vendor/MSFT/Policy/Config/A", "1")
	g, _ := NewGet([]string{"./DevDetail/SwV"})
	// A Sequence may hold a Get.
	if _, err := NewSequence([]*Command{a, g}); err != nil {
		t.Errorf("sequence with get: %v", err)
	}
	inner, _ := NewSequence([]*Command{a})
	if _, err := NewSequence([]*Command{inner}); !errors.Is(err, ErrCommand) {
		t.Errorf("nested sequence: %v", err)
	}
	if _, err := NewSequence(nil); !errors.Is(err, ErrCommand) {
		t.Errorf("empty sequence: %v", err)
	}
	userCmd, _ := NewReplace("./User/A/B", "1")
	if _, err := NewSequence([]*Command{a, userCmd}); !errors.Is(err, ErrScope) {
		t.Errorf("mixed sequence: %v", err)
	}
}

func TestValidateAndLeaves(t *testing.T) {
	t.Parallel()
	if err := Validate(nil); !errors.Is(err, ErrCommand) {
		t.Errorf("nil: %v", err)
	}
	if err := Validate(&Command{Body: &syncml.Status{}}); !errors.Is(err, ErrCommand) {
		t.Errorf("status is not a server command: %v", err)
	}
	if err := Validate(&Command{Scope: ScopeDevice, Body: &syncml.Exec{Items: []syncml.Item{{Target: "./a"}, {Target: "./b"}}}}); !errors.Is(err, ErrCommand) {
		t.Errorf("exec two items: %v", err)
	}
	if err := Validate(&Command{Scope: ScopeDevice, Body: &syncml.Get{}}); !errors.Is(err, ErrCommand) {
		t.Errorf("get without items: %v", err)
	}
	// Scope mismatch between the declared scope and the URI.
	if err := Validate(&Command{Scope: ScopeDevice, Body: &syncml.Replace{Items: []syncml.Item{{Target: "./User/A"}}}}); !errors.Is(err, ErrScope) {
		t.Errorf("scope mismatch: %v", err)
	}
	a, _ := NewReplace("./A", "1")
	b, _ := NewReplace("./B", "2")
	atomic, _ := NewAtomic([]*Command{a, b})
	if got := Leaves(atomic.Body); len(got) != 2 {
		t.Errorf("leaves = %d", len(got))
	}
}

func TestCheckAgainstSchema(t *testing.T) {
	t.Parallel()
	reg := registry.Registry()
	// A real, writable node with a valid value.
	ok, err := NewReplace("./Vendor/MSFT/Policy/Config/Camera/AllowCamera", "0", WithFormat(syncml.FormatInt))
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(reg, ok); err != nil {
		t.Errorf("valid command rejected: %v", err)
	}
	// A nil registry skips schema validation but still checks structure.
	if err := Check(nil, ok); err != nil {
		t.Errorf("nil registry: %v", err)
	}
	// An unknown node is rejected by the schema.
	bad, _ := NewReplace("./Vendor/MSFT/Policy/Config/Nope/DoesNotExist", "1", WithFormat(syncml.FormatInt))
	if err := Check(reg, bad); err == nil || !strings.Contains(err.Error(), "mdm: invalid command") {
		t.Errorf("unknown node accepted: %v", err)
	}
}
