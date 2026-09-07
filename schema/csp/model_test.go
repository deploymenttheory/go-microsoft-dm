package csp_test

import (
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
)

func TestModelHelpers(t *testing.T) {
	t.Parallel()
	root := &csp.Node{Name: "R", URI: "./Vendor/MSFT/R", Format: csp.FormatNode, Children: []*csp.Node{
		{Name: "A", URI: "./Vendor/MSFT/R/A", Format: csp.FormatChr, Access: []csp.Access{csp.AccessGet}},
		{Dynamic: true, Title: "Id", URI: "./Vendor/MSFT/R/{Id}", Format: csp.FormatNode, Children: []*csp.Node{
			{Name: "cs", URI: "./Vendor/MSFT/R/{Id}/cs", Format: csp.FormatInt, CaseSensitive: true},
		}},
	}}
	tr := &csp.Tree{Name: "R", Scope: csp.ScopeLegacy, Root: root}
	if len(tr.Nodes()) != 4 || root.Leaf() || !root.Children[0].Leaf() || !root.Children[0].Allows(csp.AccessGet) || root.Children[0].Allows(csp.AccessExec) {
		t.Fatal("helpers")
	}
	n := 0
	root.Walk(func(x *csp.Node) bool { n++; return x.Name != "A" })
	if n != 2 {
		t.Fatalf("walk stopped after %d", n)
	}
	n = 0
	root.Walk(func(x *csp.Node) bool { n++; return x.Title != "Id" })
	if n != 3 {
		t.Fatalf("walk stopped in child after %d", n)
	}
	if got := csp.Segments("./Vendor/MSFT/R/a%20b/?list=Struct"); len(got) != 5 || got[4] != "a b" {
		t.Fatalf("segments %v", got)
	}
	if got := csp.Segments("bad%zz"); len(got) != 1 || got[0] != "bad%zz" {
		t.Fatalf("segments with bad escape %v", got)
	}
	// Case-sensitive child only matches exactly; a case-insensitive one matches any case.
	if m, ok := tr.Lookup("./Vendor/MSFT/R/x/cs"); !ok || m.Node.Name != "cs" || m.Params["Id"] != "x" {
		t.Fatalf("cs exact: %+v %v", m, ok)
	}
	if _, ok := tr.Lookup("./Vendor/MSFT/R/x/CS"); ok {
		t.Fatal("case-sensitive node matched other case")
	}
	if m, ok := tr.Lookup("./vendor/msft/r/a"); !ok || m.Node.Name != "A" {
		t.Fatal("case-insensitive static match")
	}
	if m, ok := tr.Lookup("./Vendor/MSFT/R"); !ok || m.Node != root {
		t.Fatal("root match")
	}
	if got := root.Children[1].Children[0].Concrete(map[string]string{"Id": "a/b"}); got != "./Vendor/MSFT/R/a%2Fb/cs" {
		t.Fatalf("concrete %q", got)
	}
}
