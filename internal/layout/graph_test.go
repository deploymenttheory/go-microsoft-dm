package layout_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/internal/layout"
)

func TestLoadReportsADirectoryThatIsNotAModule(t *testing.T) {
	t.Parallel()
	if _, err := layout.Load(t.TempDir()); !errors.Is(err, layout.ErrGoList) {
		t.Fatalf("err = %v, want ErrGoList", err)
	}
}

func TestLoadReportsAModuleWithNoPackages(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/empty\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := layout.Load(dir); !errors.Is(err, layout.ErrGoList) {
		t.Fatalf("err = %v, want ErrGoList", err)
	}
}

func TestLoadRepoSeparatesProductionAndTestEdges(t *testing.T) {
	t.Parallel()
	g, err := layout.LoadRepo(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if g.Module != "github.com/deploymenttheory/go-microsoft-dm" {
		t.Fatalf("module = %q", g.Module)
	}
	// cmd/ddfgen imports internal/schemagen in production code only.
	if got := g.Imports["cmd/ddfgen"]; fmt.Sprint(got) != "[internal/schemagen]" {
		t.Errorf("cmd/ddfgen imports = %v", got)
	}
	// internal/layout's tests import the package itself; its production code
	// imports nothing in the repository.
	if got := g.Imports["internal/layout"]; len(got) != 0 {
		t.Errorf("internal/layout production imports = %v", got)
	}
	if got := g.TestImports["internal/layout"]; fmt.Sprint(got) != "[internal/layout]" {
		t.Errorf("internal/layout test imports = %v", got)
	}
	// Server packages are keyed under server/.
	if _, ok := g.Imports["server/cmd/dmserver"]; !ok {
		t.Errorf("server module packages missing: %v", g.Packages())
	}
}

func TestLoadRepoWithoutAServerModule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/lib\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "a.go"), []byte("package sub\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	g, err := layout.LoadRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(g.Packages()) != "[sub]" || g.Module != "example.com/lib" {
		t.Errorf("packages = %v module = %q", g.Packages(), g.Module)
	}
}

func TestCyclesFindsAComponent(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		graph map[string][]string
		want  string
	}{
		"two units pointing at each other": {map[string][]string{"a": {"b"}, "b": {"a"}}, "[[a b]]"},
		"a longer ring":                    {map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}}, "[[a b c]]"},
		"two independent rings":            {map[string][]string{"a": {"b"}, "b": {"a"}, "x": {"y"}, "y": {"x"}}, "[[a b] [x y]]"},
		"a chain is not a cycle":           {map[string][]string{"a": {"b"}, "b": {"c"}, "c": {}}, "[]"},
		"a diamond is not a cycle":         {map[string][]string{"a": {"b", "c"}, "b": {"d"}, "c": {"d"}, "d": {}}, "[]"},
		"empty":                            {map[string][]string{}, "[]"},
	}
	for name, tc := range cases {
		if got := fmt.Sprint(layout.Cycles(tc.graph)); got != tc.want {
			t.Errorf("%s: got %s, want %s", name, got, tc.want)
		}
	}
}

func TestUnitNamespacesAreTwoDeep(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"clock":                    "clock",
		"internal/layout":          "internal/layout",
		"schema/support":           "schema/support",
		"mdmprotocol/syncml":       "mdmprotocol/syncml",
		"mdmprotocol/syncml/wbxml": "mdmprotocol/syncml",
		"server/sqlstore/sqlite":   "server/sqlstore",
		"storage/inmem":            "storage/inmem",
		"simulator/fake":           "simulator",
	}
	for pkg, want := range cases {
		if got := layout.Unit(pkg); got != want {
			t.Errorf("Unit(%q) = %q, want %q", pkg, got, want)
		}
	}
}

func TestReachesIsTransitive(t *testing.T) {
	t.Parallel()
	g := &layout.Graph{Imports: map[string][]string{"a": {"b"}, "b": {"c"}, "c": {}, "d": {"a"}}}
	if got := fmt.Sprint(g.Reaches("a")); got != "[b c]" {
		t.Errorf("Reaches(a) = %s", got)
	}
	if got := g.Reaches("c"); len(got) != 0 {
		t.Errorf("Reaches(c) = %v", got)
	}
}

func TestUnitGraphDropsSelfEdges(t *testing.T) {
	t.Parallel()
	g := &layout.Graph{Imports: map[string][]string{
		"mdmprotocol/syncml":       {"mdmprotocol/syncml/wbxml", "clock"},
		"mdmprotocol/syncml/wbxml": {},
		"clock":                    {},
	}}
	got := g.UnitGraph()
	if fmt.Sprint(got["mdmprotocol/syncml"]) != "[clock]" {
		t.Errorf("unit graph = %v", got)
	}
	if fmt.Sprint(g.Packages()) != "[clock mdmprotocol/syncml mdmprotocol/syncml/wbxml]" {
		t.Errorf("packages = %v", g.Packages())
	}
}

func TestLoadRepoReportsABrokenServerModule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/lib\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "a.go"), []byte("package sub\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A server directory with a go.mod but no packages is a module the tier
	// tests would silently skip if LoadRepo swallowed the error.
	if err := os.MkdirAll(filepath.Join(dir, "server"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server", "go.mod"), []byte("module example.com/lib/server\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := layout.LoadRepo(dir); !errors.Is(err, layout.ErrGoList) {
		t.Fatalf("err = %v, want ErrGoList", err)
	}
	if _, err := layout.LoadRepo(filepath.Join(dir, "missing")); !errors.Is(err, layout.ErrGoList) {
		t.Fatalf("missing root: err = %v, want ErrGoList", err)
	}
}
