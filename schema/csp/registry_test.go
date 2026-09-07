package csp_test

import (
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
)

func tree(name, root string, scope csp.Scope, policy bool, children ...*csp.Node) *csp.Tree {
	return &csp.Tree{Name: name, Scope: scope, PolicyArea: policy, Root: &csp.Node{Name: name, URI: root, Format: csp.FormatNode, Children: children}}
}

func leaf(parent, name string) *csp.Node {
	return &csp.Node{Name: name, URI: parent + "/" + name, Format: csp.FormatChr, Access: []csp.Access{csp.AccessGet, csp.AccessReplace}}
}

func TestRegistryLookup(t *testing.T) {
	t.Parallel()
	policy := tree("Policy", "./Device/Vendor/MSFT/Policy", csp.ScopeDevice, false,
		&csp.Node{Name: "Config", URI: "./Device/Vendor/MSFT/Policy/Config", Format: csp.FormatNode, Children: []*csp.Node{
			{Dynamic: true, Title: "Area", URI: "./Device/Vendor/MSFT/Policy/Config/{Area}", Format: csp.FormatNode},
		}})
	bits := tree("BITS", "./Device/Vendor/MSFT/Policy/Config/BITS", csp.ScopeDevice, true, leaf("./Device/Vendor/MSFT/Policy/Config/BITS", "JobInactivityTimeout"))
	bitsUser := tree("BITS", "./User/Vendor/MSFT/Policy/Config/BITS", csp.ScopeUser, true, leaf("./User/Vendor/MSFT/Policy/Config/BITS", "JobInactivityTimeout"))
	legacy := tree("Firewall", "./Vendor/MSFT/Firewall", csp.ScopeLegacy, false, leaf("./Vendor/MSFT/Firewall", "MdmStore"))
	devinfo := tree("DevInfo", "./DevInfo", csp.ScopeLegacy, false, leaf("./DevInfo", "DevId"))
	r := csp.NewRegistry(policy, bits, bitsUser, legacy, devinfo)

	cases := map[string]struct {
		tree string
		uri  string
		ok   bool
	}{
		"./Device/Vendor/MSFT/Policy/Config/BITS/JobInactivityTimeout": {"BITS", "./Device/Vendor/MSFT/Policy/Config/BITS/JobInactivityTimeout", true},
		"./User/Vendor/MSFT/Policy/Config/BITS/JobInactivityTimeout":   {"BITS", "./User/Vendor/MSFT/Policy/Config/BITS/JobInactivityTimeout", true},
		"./Vendor/MSFT/Policy/Config/BITS/JobInactivityTimeout":        {"BITS", "./Device/Vendor/MSFT/Policy/Config/BITS/JobInactivityTimeout", true},
		"./Device/Vendor/MSFT/Policy/Config/Other":                     {"Policy", "./Device/Vendor/MSFT/Policy/Config/{Area}", true},
		"./Vendor/MSFT/Firewall/MdmStore":                              {"Firewall", "./Vendor/MSFT/Firewall/MdmStore", true},
		"./Device/Vendor/MSFT/Firewall/MdmStore":                       {"Firewall", "./Vendor/MSFT/Firewall/MdmStore", true},
		"./DevInfo/DevId":                                              {"DevInfo", "./DevInfo/DevId", true},
		"./devinfo/devid?prop=Type":                                    {"DevInfo", "./DevInfo/DevId", true},
		"./User/Vendor/MSFT/Firewall/MdmStore":                         {"", "", false},
		"./Device/Vendor/MSFT/Unknown/X":                               {"", "", false},
		"./Device":                                                     {"", "", false},
		".":                                                            {"", "", false},
		"":                                                             {"", "", false},
	}
	for in, tc := range cases {
		m, tr, ok := r.Lookup(in)
		if ok != tc.ok {
			t.Errorf("%q: ok = %v", in, ok)
			continue
		}
		if ok && (tr.Name != tc.tree || m.Node.URI != tc.uri) {
			t.Errorf("%q: tree %s node %s", in, tr.Name, m.Node.URI)
		}
	}
	if len(r.Trees()) != 5 {
		t.Fatal("trees")
	}
	if tr, ok := r.Tree("bits", csp.ScopeUser); !ok || tr != bitsUser {
		t.Fatal("Tree by scope")
	}
	if tr, ok := r.Tree("BITS", ""); !ok || tr != bits {
		t.Fatal("Tree any scope")
	}
	if _, ok := r.Tree("Nope", ""); ok {
		t.Fatal("unknown tree")
	}
}
