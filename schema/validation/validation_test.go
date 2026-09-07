package validation_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
	"github.com/deploymenttheory/go-microsoft-dm/schema/validation"
)

func str(s string) *string { return &s }

func fixture() *csp.Registry {
	rw := []csp.Access{csp.AccessAdd, csp.AccessDelete, csp.AccessGet, csp.AccessReplace}
	root := "./Device/Vendor/MSFT/Demo"
	leafs := []*csp.Node{
		{Name: "Int", URI: root + "/Int", Format: csp.FormatInt, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeRange, Value: "[0-10],[100]"}},
		{Name: "Enum", URI: root + "/Enum", Format: csp.FormatInt, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeEnum, Enum: []csp.EnumValue{{Value: "0"}, {Value: "1"}}}},
		{Name: "Flags", URI: root + "/Flags", Format: csp.FormatInt, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeFlag, Enum: []csp.EnumValue{{Value: "4"}, {Value: "8"}, {Value: "x"}}}},
		{Name: "Guid", URI: root + "/Guid", Format: csp.FormatChr, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeRegEx, Value: `[0-9A-Fa-f]{8}\-[0-9A-Fa-f]{4}`}},
		{Name: "BadRe", URI: root + "/BadRe", Format: csp.FormatChr, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeRegEx, Value: `[z-a`}},
		{Name: "List", URI: root + "/List", Format: csp.FormatChr, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeEnum, Enum: []csp.EnumValue{{Value: "a"}, {Value: "b"}}, ListDelimiter: "0xF000"}},
		{Name: "SemiList", URI: root + "/SemiList", Format: csp.FormatChr, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeRange, Value: "[1-3]", ListDelimiter: ";"}},
		{Name: "Admx", URI: root + "/Admx", Format: csp.FormatChr, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeADMX, ADMX: &csp.ADMX{Area: "A", Name: "N", File: "F"}}},
		{Name: "Xsd", URI: root + "/Xsd", Format: csp.FormatXML, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeXSD, Value: "<xs:schema/>"}},
		{Name: "Json", URI: root + "/Json", Format: csp.FormatChr, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeJSON}},
		{Name: "Sddl", URI: root + "/Sddl", Format: csp.FormatChr, Access: rw, AllowedValues: &csp.AllowedValues{Type: csp.ValueTypeSDDL}},
		{Name: "Bool", URI: root + "/Bool", Format: csp.FormatBool, Access: rw},
		{Name: "B64", URI: root + "/B64", Format: csp.FormatB64, Access: rw},
		{Name: "Float", URI: root + "/Float", Format: csp.FormatFloat, Access: rw},
		{Name: "Plain", URI: root + "/Plain", Format: csp.FormatChr, Access: []csp.Access{csp.AccessGet}},
		{Name: "Run", URI: root + "/Run", Format: csp.FormatNull, Access: []csp.Access{csp.AccessExec}},
		{Name: "RunChr", URI: root + "/RunChr", Format: csp.FormatChr, Access: []csp.Access{csp.AccessExec, csp.AccessGet}},
	}
	dyn := &csp.Node{Dynamic: true, Title: "ID", URI: root + "/Apps/{ID}", Format: csp.FormatNode, Access: rw, AtomicRequired: true,
		Naming:   csp.Naming{Kind: csp.NamingUniqueName, Pattern: `\{[0-9A-F]{4}\}`},
		Children: []*csp.Node{{Name: "Install", URI: root + "/Apps/{ID}/Install", Format: csp.FormatXML, Access: rw}}}
	apps := &csp.Node{Name: "Apps", URI: root + "/Apps", Format: csp.FormatNode, Access: []csp.Access{csp.AccessGet}, Children: []*csp.Node{dyn}}
	tree := &csp.Tree{Name: "Demo", Scope: csp.ScopeDevice, Root: &csp.Node{Name: "Demo", URI: root, Format: csp.FormatNode, Access: []csp.Access{csp.AccessGet}, Children: append(leafs, apps)}}
	userTree := &csp.Tree{Name: "Demo", Scope: csp.ScopeUser, Root: &csp.Node{Name: "Demo", URI: "./User/Vendor/MSFT/Demo", Format: csp.FormatNode, Access: []csp.Access{csp.AccessGet}, Children: []*csp.Node{
		{Name: "Int", URI: "./User/Vendor/MSFT/Demo/Int", Format: csp.FormatInt, Access: rw},
	}}}
	return csp.NewRegistry(tree, userTree)
}

func TestCheckAcceptsValidCommands(t *testing.T) {
	t.Parallel()
	reg := fixture()
	ok := []validation.Command{
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Int", Format: "int", Value: str("7")},
		{Verb: validation.Replace, URI: "./Vendor/MSFT/Demo/Int", Value: str("100")},
		{Verb: validation.Add, URI: "./Device/Vendor/MSFT/Demo/Enum", Value: str("1")},
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Flags", Value: str("12")},
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Guid", Value: str("DEADBEEF-1234")},
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/List", Value: str("ab")},
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/SemiList", Value: str("1;3")},
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(`<enabled/><data id="x" value="1"/>`)},
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(`<Disabled/>`)},
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Xsd", Format: "xml", Value: str(`<a><b/></a>`)},
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Bool", Value: str("True")},
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/B64", Value: str("aGk=")},
		{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Float", Value: str("1.5")},
		{Verb: validation.Get, URI: "./Device/Vendor/MSFT/Demo/Plain"},
		{Verb: validation.Get, URI: "./Device/Vendor/MSFT/Demo?prop=Type"},
		{Verb: validation.Exec, URI: "./Device/Vendor/MSFT/Demo/Run"},
		{Verb: validation.Exec, URI: "./Device/Vendor/MSFT/Demo/RunChr", Value: str("go")},
		{Verb: validation.Add, URI: "./Device/Vendor/MSFT/Demo/Apps/%7BBEEF%7D", Format: "node"},
		{Verb: validation.Delete, URI: "./Device/Vendor/MSFT/Demo/Apps/%7BBEEF%7D"},
		{Verb: validation.Replace, URI: "./User/Vendor/MSFT/Demo/Int", Value: str("1")},
		{Verb: validation.Replace, URI: "./user/vendor/msft/demo/int", Value: str("1")},
	}
	for _, c := range ok {
		r := validation.Check(reg, c)
		if err := r.Err(); err != nil {
			t.Errorf("%s %s: %v", c.Verb, c.URI, err)
		}
		if r.Node == nil || r.Tree == nil {
			t.Errorf("%s: node not resolved", c.URI)
		}
	}
	r := validation.Check(reg, validation.Command{Verb: validation.Add, URI: "./Device/Vendor/MSFT/Demo/Apps/%7BBEEF%7D/Install", Value: str("<x/>")})
	if err := r.Err(); err != nil || !r.AtomicRequired || r.Params["ID"] != "{BEEF}" {
		t.Fatalf("atomic child: %v %+v", err, r)
	}
	if r := validation.Check(reg, validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Int", Value: str("1")}); r.AtomicRequired {
		t.Fatal("Int is not atomic-required")
	}
	for _, uri := range []string{"Xsd", "Json", "Sddl"} {
		r := validation.Check(reg, validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/" + uri, Value: str("<a/>")})
		if len(r.Unevaluated) != 1 {
			t.Errorf("%s: unevaluated %v", uri, r.Unevaluated)
		}
	}
	r = validation.Check(reg, validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/BadRe", Value: str("x")})
	if r.Err() != nil || len(r.Unevaluated) != 1 || !strings.Contains(r.Unevaluated[0], "does not compile") {
		t.Fatalf("bad regex must be reported, not fail: %v %v", r.Err(), r.Unevaluated)
	}
}

func TestCheckRejections(t *testing.T) {
	t.Parallel()
	reg := fixture()
	cases := map[string]struct {
		c    validation.Command
		want error
	}{
		"unknown node":             {validation.Command{Verb: validation.Get, URI: "./Device/Vendor/MSFT/Nope"}, validation.ErrUnknownNode},
		"unknown child":            {validation.Command{Verb: validation.Get, URI: "./Device/Vendor/MSFT/Demo/Missing"}, validation.ErrUnknownNode},
		"verb not permitted":       {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Plain", Value: str("x")}, validation.ErrVerb},
		"exec on a leaf":           {validation.Command{Verb: validation.Exec, URI: "./Device/Vendor/MSFT/Demo/Int"}, validation.ErrVerb},
		"user tree via device":     {validation.Command{Verb: validation.Get, URI: "./Device/Vendor/MSFT/Demo/Apps/x/Install"}, validation.ErrDynamicName},
		"interior with value":      {validation.Command{Verb: validation.Add, URI: "./Device/Vendor/MSFT/Demo/Apps/%7BBEEF%7D", Value: str("x")}, validation.ErrInteriorValue},
		"interior wrong format":    {validation.Command{Verb: validation.Add, URI: "./Device/Vendor/MSFT/Demo/Apps/%7BBEEF%7D", Format: "chr"}, validation.ErrFormat},
		"format mismatch":          {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Int", Format: "chr", Value: str("1")}, validation.ErrFormat},
		"missing value":            {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Int"}, validation.ErrFormat},
		"int not integer":          {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Int", Value: str("seven")}, validation.ErrFormat},
		"range low":                {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Int", Value: str("-1")}, validation.ErrValue},
		"range gap":                {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Int", Value: str("50")}, validation.ErrValue},
		"enum":                     {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Enum", Value: str("2")}, validation.ErrValue},
		"flag bits":                {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Flags", Value: str("3")}, validation.ErrValue},
		"flag not int":             {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Flags", Value: str("x")}, validation.ErrFormat},
		"regex":                    {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Guid", Value: str("nope")}, validation.ErrValue},
		"list member":              {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/List", Value: str("ac")}, validation.ErrValue},
		"semi list member":         {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/SemiList", Value: str("1;9")}, validation.ErrValue},
		"admx no state":            {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(`<data id="x" value="1"/>`)}, validation.ErrValue},
		"admx two states":          {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(`<enabled/><disabled/>`)}, validation.ErrValue},
		"admx data after disabled": {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(`<disabled/><data id="x" value="1"/>`)}, validation.ErrValue},
		"admx data without id":     {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(`<enabled/><data value="1"/>`)}, validation.ErrValue},
		"admx nested":              {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(`<enabled><data id="x" value="1"/></enabled>`)}, validation.ErrValue},
		"admx text":                {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(`<enabled/>text`)}, validation.ErrValue},
		"admx unknown element":     {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(`<enabled/><item/>`)}, validation.ErrValue},
		"admx malformed":           {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(`<enabled`)}, validation.ErrValue},
		"admx empty":               {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Admx", Value: str(``)}, validation.ErrValue},
		"xml malformed":            {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Xsd", Value: str(`<a>`)}, validation.ErrFormat},
		"bool":                     {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Bool", Value: str("yes")}, validation.ErrFormat},
		"b64":                      {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/B64", Value: str("***")}, validation.ErrFormat},
		"float":                    {validation.Command{Verb: validation.Replace, URI: "./Device/Vendor/MSFT/Demo/Float", Value: str("x")}, validation.ErrFormat},
		"null with value":          {validation.Command{Verb: validation.Exec, URI: "./Device/Vendor/MSFT/Demo/Run", Value: str("x")}, validation.ErrFormat},
		"dynamic name pattern":     {validation.Command{Verb: validation.Add, URI: "./Device/Vendor/MSFT/Demo/Apps/plain"}, validation.ErrDynamicName},
	}
	for name, tc := range cases {
		r := validation.Check(reg, tc.c)
		err := r.Err()
		if !errors.Is(err, tc.want) || !errors.Is(err, validation.ErrInvalid) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
		var p *validation.Problem
		if !errors.As(err, &p) || p.Error() == "" {
			t.Errorf("%s: no Problem in %v", name, err)
		}
	}
}
