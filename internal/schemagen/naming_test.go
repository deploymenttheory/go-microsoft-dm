package schemagen

import (
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
)

func TestNamingHelpers(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"DMClient": "dmclient", "ADMX_AppCompat": "admxappcompat", "6to4": "n6to4", "": "x", "___": "x"} {
		if got := packageName(in); got != want {
			t.Errorf("packageName(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"HWDevID": "HWDevID", "6to4_State": "N6to4State", "ADMX_MSS-legacy": "ADMXMSSLegacy", "Profile Name": "ProfileName", "": "X", "-": "X", "8128000B": "N8128000B"} {
		if got := identifier(in); got != want {
			t.Errorf("identifier(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"ProviderID": "providerID", "Type": "typeValue", "Profile Name": "profileName", "Map": "mapValue"} {
		if got := paramName(in); got != want {
			t.Errorf("paramName(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[csp.EnumValue]string{
		{Value: "0", Description: "Not allowed. Turns off scanning."}: "NotAllowedTurnsOff",
		{Value: "1", Description: "Allowed"}:                          "Allowed",
		{Value: "2", Description: ""}:                                 "Value2",
		{Value: "0x10", Description: "!!!"}:                           "Value0x10",
	} {
		if got := enumIdent(in); got != want {
			t.Errorf("enumIdent(%+v) = %q, want %q", in, got, want)
		}
	}
}

func TestNameNodesCollisions(t *testing.T) {
	t.Parallel()
	// Two dynamic siblings with a child of the same name collide on the
	// static join and are disambiguated with their titles; a third
	// identical collision gets a number.
	root := &csp.Node{Name: "R", URI: "./Vendor/MSFT/R", Format: csp.FormatNode, Children: []*csp.Node{
		{Dynamic: true, Title: "A", URI: "./Vendor/MSFT/R/{A}", Format: csp.FormatNode, Children: []*csp.Node{{Name: "X", URI: "./Vendor/MSFT/R/{A}/X", Format: csp.FormatChr}}},
		{Dynamic: true, Title: "B", URI: "./Vendor/MSFT/R/{B}", Format: csp.FormatNode, Children: []*csp.Node{{Name: "X", URI: "./Vendor/MSFT/R/{B}/X", Format: csp.FormatChr}}},
		{Name: "X", URI: "./Vendor/MSFT/R/X", Format: csp.FormatChr},
		{Name: "Tree", URI: "./Vendor/MSFT/R/Tree", Format: csp.FormatChr},
		{Name: "Type", URI: "./Vendor/MSFT/R/Type", Format: csp.FormatNode, Children: []*csp.Node{
			{Dynamic: true, Title: "Type", URI: "./Vendor/MSFT/R/Type/{Type}", Format: csp.FormatNode, Children: []*csp.Node{
				{Dynamic: true, Title: "Type", URI: "./Vendor/MSFT/R/Type/{Type}/{Type}", Format: csp.FormatChr},
			}},
		}},
	}}
	tr := &csp.Tree{Name: "R", Scope: csp.ScopeLegacy, Root: root}
	got := map[string]string{}
	var params map[string][]string
	params = map[string][]string{}
	for _, n := range nameNodes(tr, "") {
		got[n.node.URI] = n.scoped
		params[n.node.URI] = paramList(n)
	}
	want := map[string]string{
		"./Vendor/MSFT/R/{A}":                "A",
		"./Vendor/MSFT/R/{A}/X":              "XByA",
		"./Vendor/MSFT/R/{B}":                "B",
		"./Vendor/MSFT/R/{B}/X":              "XByB",
		"./Vendor/MSFT/R/X":                  "X",
		"./Vendor/MSFT/R/Tree":               "TreeNode",
		"./Vendor/MSFT/R/Type":               "Type",
		"./Vendor/MSFT/R/Type/{Type}":        "TypeTypeByType",
		"./Vendor/MSFT/R/Type/{Type}/{Type}": "TypeTypeByTypeType",
	}
	for uri, w := range want {
		if got[uri] != w {
			t.Errorf("%s: got %q, want %q (all: %v)", uri, got[uri], w, got)
		}
	}
	if p := params["./Vendor/MSFT/R/Type/{Type}/{Type}"]; len(p) != 2 || p[0] != "typeValue" || p[1] != "typeValue2" {
		t.Errorf("params %v", p)
	}
}
