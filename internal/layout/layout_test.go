package layout_test

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/internal/layout"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func load(t *testing.T) *layout.Graph {
	t.Helper()
	g, err := layout.LoadRepo(repoRoot(t))
	if err != nil {
		t.Fatalf("load import graph: %v", err)
	}
	return g
}

// The tiers of decision record 0001, in ascending order. A package may
// import its own tier and every tier below it, and nothing above.
const (
	tierFoundation = iota // no domain knowledge: clock, paging, secrets, telemetry, state, ratelimit, testpki
	tierSchema            // generated CSP types and the validation and support helpers over them
	tierProtocol          // mdmprotocol: wire formats and protocol state machines, no I/O
	tierPKI               // pki: identity issuance and verification
	tierServices          // msplatformservices: outbound clients to Microsoft services
	tierStorage           // storage: contracts and the in-memory backend, no drivers
	tierClient            // simulator: a Windows MDM client, in software
	tierServer            // server: persistence, service layer, transport
	tierApp               // composition: cmd, internal/app, e2e, the generator
)

var tierNames = map[int]string{
	tierFoundation: "foundation", tierSchema: "schema", tierProtocol: "mdmprotocol",
	tierPKI: "pki", tierServices: "msplatformservices", tierStorage: "storage",
	tierClient: "simulator", tierServer: "server", tierApp: "app",
}

// tierOf maps a package to its tier by path, which is the point of the
// layout: the import path states the tier, so this function is a reading of
// the tree rather than a list to maintain.
func tierOf(pkg string) int {
	switch {
	case strings.HasPrefix(pkg, "cmd/"), strings.HasPrefix(pkg, "server/cmd/"),
		strings.HasPrefix(pkg, "server/e2e"), pkg == "server/internal/app",
		pkg == "internal/schemagen":
		return tierApp
	case strings.HasPrefix(pkg, "server/"):
		return tierServer
	case pkg == "simulator":
		return tierClient
	case pkg == "storage", strings.HasPrefix(pkg, "storage/"):
		return tierStorage
	case strings.HasPrefix(pkg, "msplatformservices/"):
		return tierServices
	case strings.HasPrefix(pkg, "pki/"):
		return tierPKI
	case strings.HasPrefix(pkg, "mdmprotocol/"):
		return tierProtocol
	case strings.HasPrefix(pkg, "schema/"):
		return tierSchema
	default:
		return tierFoundation
	}
}

// isScaffolding reports whether pkg exists to serve tests rather than to run
// in a program. A fake or a contract suite follows the package it exercises
// and often needs a backend to seed, so tier rules say nothing about them;
// these are the packages scripts/coverage-exempt.txt exempts, for the same
// reason. internal/layout is test tooling and imports nothing in the tree.
func isScaffolding(pkg string) bool {
	return strings.HasSuffix(path.Base(pkg), "test") ||
		pkg == "testpki" ||
		pkg == "internal/layout"
}

// knownTierExceptions records the exact permitted upward import edges. There
// are none. The test rejects additions to or unrecorded removals from this set.
var knownTierExceptions = map[string][]string{}

// TestTiersOnlyImportDownwards is the boundary decision record 0001 claims,
// and the reason the tree is arranged this way at all. A caller who wants a
// SyncML type must not acquire a database driver with it.
//
// golangci-lint cannot enforce this: the workflow runs it with
// --issues-exit-code=0 and only-new-issues, so a depguard rule would report
// a violation without failing the build. Tests fail the build.
func TestTiersOnlyImportDownwards(t *testing.T) {
	t.Parallel()
	g := load(t)
	for _, pkg := range g.Packages() {
		if isScaffolding(pkg) {
			continue
		}
		from := tierOf(pkg)
		var up []string
		for _, dep := range g.Imports[pkg] {
			if to := tierOf(dep); to > from && !isScaffolding(dep) {
				up = append(up, dep)
			}
		}
		want := knownTierExceptions[pkg]
		if fmt.Sprint(up) == fmt.Sprint(want) {
			continue
		}
		for _, dep := range up {
			if !slices.Contains(want, dep) {
				t.Errorf("%s (%s) imports %s (%s): a package may not import a higher tier",
					pkg, tierNames[from], dep, tierNames[tierOf(dep)])
			}
		}
		for _, dep := range want {
			if !slices.Contains(up, dep) {
				t.Errorf("%s no longer imports %s: delete it from knownTierExceptions", pkg, dep)
			}
		}
	}
}

// TestEveryTierIsPopulated guards the reading above. tierOf falls through to
// foundation, so a renamed or mistyped tier directory would silently demote
// every package under it and make TestTiersOnlyImportDownwards vacuous.
func TestEveryTierIsPopulated(t *testing.T) {
	t.Parallel()
	g := load(t)
	count := map[int]int{}
	for _, pkg := range g.Packages() {
		count[tierOf(pkg)]++
	}
	for tier, name := range tierNames {
		if count[tier] == 0 {
			t.Errorf("tier %s has no packages: has a tier directory been renamed?", name)
		}
	}
}

// TestNoUnitCycles proves every directory can be assigned to one tier. A
// directory in a strongly connected component cannot: part of it would sit
// above another part.
func TestNoUnitCycles(t *testing.T) {
	t.Parallel()
	g := load(t)
	for _, c := range layout.Cycles(g.UnitGraph()) {
		t.Errorf("directory-level cycle, so these cannot be assigned to tiers: %s", strings.Join(c, " -> "))
	}
}

// TestLibraryNeverImportsServer is the module boundary: the library is what
// consumers embed, and a test-only import would still drag the server's
// drivers into `go test ./...` for every consumer that vendors the tree.
func TestLibraryNeverImportsServer(t *testing.T) {
	t.Parallel()
	g := load(t)
	for _, pkg := range g.Packages() {
		if strings.HasPrefix(pkg, "server/") {
			continue
		}
		for _, dep := range g.Imports[pkg] {
			if strings.HasPrefix(dep, "server/") {
				t.Errorf("%s imports %s: the library must not import the server module", pkg, dep)
			}
		}
		for _, dep := range g.TestImports[pkg] {
			if strings.HasPrefix(dep, "server/") {
				t.Errorf("%s imports %s in its tests: the library must not import the server module, even in tests", pkg, dep)
			}
		}
	}
	if _, ok := g.Imports["server/internal/app"]; !ok {
		t.Fatal("server module was not loaded, so the boundary was not checked")
	}
}
