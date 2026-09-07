package schemagen_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/internal/schemagen"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
)

const pinnedBundle = "../../third_party/ddf/DDFv2Feb2026.zip"

func loadDrop(t *testing.T) *schemagen.Drop {
	t.Helper()
	d, err := schemagen.ParseZip(filepath.FromSlash(pinnedBundle))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestPinnedBundleCensus pins the numbers the research store's bundle
// inspection reports (docs/research.md 1.5) so a wrong parse is caught by
// counts. Where the parser's count differs from the research store the
// number here is the parser's (the store counted the
// DependencyChangedAllowedValues elements inside dependency groups as
// AllowedValues, and one named node carries a UniqueName rule); the
// differences are recorded in decision record 0006.
func TestPinnedBundleCensus(t *testing.T) {
	t.Parallel()
	c := schemagen.TakeCensus(loadDrop(t))
	want := map[string]int{
		"Files": 313, "Trees": 400, "StandaloneCSPs": 52, "PolicyAreas": 261, "DualScopeFiles": 87,
		"Nodes": 5955, "Leaves": 5146, "DynamicNodes": 132,
		"Applicability": 4120, "AdmxBacked": 2516, "GpMappings": 907, "Lists": 197,
		"Dependencies": 125, "Deprecated": 31, "AtomicRequired": 21, "RebootBehavior": 18,
	}
	got := map[string]int{
		"Files": c.Files, "Trees": c.Trees, "StandaloneCSPs": c.StandaloneCSPs, "PolicyAreas": c.PolicyAreas, "DualScopeFiles": c.DualScopeFiles,
		"Nodes": c.Nodes, "Leaves": c.Leaves, "DynamicNodes": c.DynamicNodes,
		"Applicability": c.Applicability, "AdmxBacked": c.AdmxBacked, "GpMappings": c.GpMappings, "Lists": c.Lists,
		"Dependencies": c.Dependencies, "Deprecated": c.Deprecated, "AtomicRequired": c.AtomicRequired, "RebootBehavior": c.RebootBehavior,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %d, want %d", k, got[k], w)
		}
	}
	formats := map[csp.Format]int{csp.FormatChr: 3467, csp.FormatInt: 1379, csp.FormatNode: 809, csp.FormatBool: 218, csp.FormatB64: 28, csp.FormatNull: 28, csp.FormatXML: 21, csp.FormatBin: 3, csp.FormatTime: 2}
	for f, w := range formats {
		if c.Formats[f] != w {
			t.Errorf("format %s = %d, want %d", f, c.Formats[f], w)
		}
	}
	if len(c.Formats) != len(formats) {
		t.Errorf("formats seen %v", c.Formats)
	}
	valueTypes := map[csp.ValueType]int{csp.ValueTypeADMX: 2516, csp.ValueTypeEnum: 1202, csp.ValueTypeNone: 487, csp.ValueTypeRange: 220, csp.ValueTypeRegEx: 33, csp.ValueTypeXSD: 18, csp.ValueTypeFlag: 11, csp.ValueTypeSDDL: 1, csp.ValueTypeJSON: 1}
	for v, w := range valueTypes {
		if c.ValueTypes[v] != w {
			t.Errorf("value type %s = %d, want %d", v, c.ValueTypes[v], w)
		}
	}
	naming := map[csp.NamingKind]int{csp.NamingUniqueName: 64, csp.NamingClientInventory: 33, csp.NamingServerGeneratedUniqueIdentifier: 29, csp.NamingUnspecified: 6}
	for k, w := range naming {
		if c.NamingKinds[k] != w {
			t.Errorf("naming %q = %d, want %d", k, c.NamingKinds[k], w)
		}
	}
	t.Logf("census: %+v", c)
}

func TestParseDDFFailures(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   string
		want error
	}{
		"not xml":                     {"<MgmtTree>", schemagen.ErrDDF},
		"wrong root":                  {`<Tree/>`, schemagen.ErrDDF},
		"namespaced ddf":              {`<MgmtTree xmlns="http://tempuri.org/DM_DDF-V1_2"><Node><NodeName>X</NodeName><Path>.</Path><DFProperties><DFFormat><node/></DFFormat></DFProperties></Node></MgmtTree>`, schemagen.ErrDDFNamespace},
		"no root node":                {`<MgmtTree><VerDTD>1.2</VerDTD></MgmtTree>`, schemagen.ErrDDF},
		"missing Path":                {`<MgmtTree><Node><NodeName>X</NodeName><DFProperties><DFFormat><node/></DFFormat></DFProperties></Node></MgmtTree>`, schemagen.ErrDDF},
		"empty Path":                  {`<MgmtTree><Node><NodeName>X</NodeName><Path> </Path><DFProperties><DFFormat><node/></DFFormat></DFProperties></Node></MgmtTree>`, schemagen.ErrDDF},
		"unknown format":              {`<MgmtTree><Node><NodeName>X</NodeName><Path>.</Path><DFProperties><DFFormat><string/></DFFormat></DFProperties></Node></MgmtTree>`, schemagen.ErrDDF},
		"missing format":              {`<MgmtTree><Node><NodeName>X</NodeName><Path>.</Path><DFProperties></DFProperties></Node></MgmtTree>`, schemagen.ErrDDF},
		"unknown ValueType":           {`<MgmtTree xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM"><Node><NodeName>X</NodeName><Path>.</Path><DFProperties><DFFormat><chr/></DFFormat><MSFT:AllowedValues ValueType="Glob"/></DFProperties></Node></MgmtTree>`, schemagen.ErrDDF},
		"dynamic child without title": {`<MgmtTree><Node><NodeName>X</NodeName><Path>.</Path><DFProperties><DFFormat><node/></DFFormat></DFProperties><Node><NodeName></NodeName><DFProperties><DFFormat><node/></DFFormat></DFProperties></Node></Node></MgmtTree>`, schemagen.ErrDDF},
		"duplicate children":          {`<MgmtTree><Node><NodeName>X</NodeName><Path>.</Path><DFProperties><DFFormat><node/></DFFormat></DFProperties><Node><NodeName>a</NodeName><DFProperties><DFFormat><chr/></DFFormat></DFProperties></Node><Node><NodeName>A</NodeName><DFProperties><DFFormat><chr/></DFFormat></DFProperties></Node></Node></MgmtTree>`, schemagen.ErrDDF},
		"empty DynamicNodeNaming":     {`<MgmtTree xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM"><Node><NodeName>X</NodeName><Path>.</Path><DFProperties><DFFormat><node/></DFFormat></DFProperties><Node><NodeName></NodeName><DFProperties><DFFormat><node/></DFFormat><DFTitle>T</DFTitle><MSFT:DynamicNodeNaming/></DFProperties></Node></Node></MgmtTree>`, schemagen.ErrDDF},
		"Path on a child":             {`<MgmtTree><Node><NodeName>X</NodeName><Path>.</Path><DFProperties><DFFormat><node/></DFFormat></DFProperties><Node><NodeName>a</NodeName><Path>./Other</Path><DFProperties><DFFormat><chr/></DFFormat></DFProperties></Node></Node></MgmtTree>`, schemagen.ErrDDF},
	}
	for name, tc := range cases {
		_, err := schemagen.ParseDDF(name+".xml", []byte(tc.in))
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if _, err := schemagen.ParseZip(filepath.Join(t.TempDir(), "missing.zip")); !errors.Is(err, schemagen.ErrDDF) {
		t.Errorf("missing zip: %v", err)
	}
}

func TestParseDDFShapes(t *testing.T) {
	t.Parallel()
	in := "\xEF\xBB\xBF" + `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE MgmtTree PUBLIC " -//OMA//DTD-DM-DDF 1.2//EN" "http://www.openmobilealliance.org/tech/DTD/DM_DDF-V1_2.dtd" [<?oma-dm-ddf-ver supported-versions="1.2"?>]>
<MgmtTree xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM">
  <VerDTD>1.2</VerDTD>
  <MSFT:Diagnostics></MSFT:Diagnostics>
  <Node>
    <NodeName>Demo</NodeName>
    <Path>./Vendor/MSFT/</Path>
    <DFProperties>
      <AccessType><Get /></AccessType>
      <DFFormat><node /></DFFormat>
      <Occurrence><One /></Occurrence>
      <Scope><Permanent /></Scope>
      <DFType><MIME>com.microsoft/11/MDM/Policy</MIME></DFType>
      <MSFT:Applicability>
        <MSFT:OsBuildVersion>10.0.22000, 10.0.19041.1202</MSFT:OsBuildVersion>
        <MSFT:CspVersion>9.9</MSFT:CspVersion>
        <MSFT:EditionAllowList>0x4;0x1B;</MSFT:EditionAllowList>
        <MSFT:RequiresAzureAd />
      </MSFT:Applicability>
    </DFProperties>
    <Node>
      <NodeName></NodeName>
      <DFProperties>
        <AccessType><Add /><Delete /><Get /></AccessType>
        <DFFormat><node /></DFFormat>
        <Occurrence><ZeroOrMore /></Occurrence>
        <Scope><Dynamic /></Scope>
        <DFTitle>ProfileName</DFTitle>
        <DFType><DDFName /></DFType>
        <CaseSense><CS /></CaseSense>
        <MSFT:DynamicNodeNaming><MSFT:UniqueName>[A-Za-z]+</MSFT:UniqueName></MSFT:DynamicNodeNaming>
        <MSFT:AtomicRequired />
        <MSFT:Deprecated OsBuildDeprecated="10.0.22000" />
      </DFProperties>
      <Node>
        <NodeName>Mode</NodeName>
        <DFProperties>
          <AccessType><Get /><Replace /><Exec /></AccessType>
          <Description> Picks a mode. </Description>
          <DFFormat><int /></DFFormat>
          <Occurrence><ZeroOrOne /></Occurrence>
          <Scope><Dynamic /></Scope>
          <DFType><MIME>text/plain</MIME></DFType>
          <DefaultValue>1</DefaultValue>
          <MSFT:AllowedValues ValueType="ENUM">
            <MSFT:Enum><MSFT:Value>0</MSFT:Value><MSFT:ValueDescription>Off</MSFT:ValueDescription></MSFT:Enum>
            <MSFT:Enum><MSFT:Value>1</MSFT:Value><MSFT:ValueDescription>On</MSFT:ValueDescription></MSFT:Enum>
            <MSFT:List Delimiter=";" />
          </MSFT:AllowedValues>
          <MSFT:GpMapping GpEnglishName="Demo_Mode" GpAreaPath="Demo~AT~System" GpElement="Demo_Mode_Enum" />
          <MSFT:ConflictResolution>LastWrite</MSFT:ConflictResolution>
          <MSFT:ReplaceBehavior>Append</MSFT:ReplaceBehavior>
          <MSFT:RebootBehavior>Automatic</MSFT:RebootBehavior>
          <MSFT:DependencyBehavior>
            <MSFT:DependencyGroup FriendlyId="NeedsOther">
              <MSFT:DependencyChangedAllowedValues ValueType="Range"><MSFT:Value>[0-1]</MSFT:Value></MSFT:DependencyChangedAllowedValues>
              <MSFT:Dependency Type="DependsOn">
                <MSFT:DependencyUri>Vendor/MSFT/Demo/Other</MSFT:DependencyUri>
                <MSFT:DependencyAllowedValue ValueType="Range"><MSFT:Value>[0]</MSFT:Value></MSFT:DependencyAllowedValue>
              </MSFT:Dependency>
            </MSFT:DependencyGroup>
          </MSFT:DependencyBehavior>
        </DFProperties>
      </Node>
      <Node>
        <NodeName>Policy</NodeName>
        <DFProperties>
          <AccessType><Add /><Replace /></AccessType>
          <DFFormat><chr /></DFFormat>
          <Occurrence><One /></Occurrence>
          <Scope><Dynamic /></Scope>
          <DFType><MIME>text/plain</MIME></DFType>
          <MSFT:AllowedValues ValueType="ADMX">
            <MSFT:AdmxBacked Area="Demo~AT~System" Name="Demo_Policy" File="Demo.admx" />
          </MSFT:AllowedValues>
        </DFProperties>
      </Node>
    </Node>
  </Node>
</MgmtTree>`
	trees, err := schemagen.ParseDDF("Demo.xml", []byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(trees) != 1 {
		t.Fatalf("trees = %d", len(trees))
	}
	tr := trees[0]
	if tr.Name != "Demo" || tr.File != "Demo.xml" || tr.Scope != csp.ScopeLegacy || tr.PolicyArea || tr.Root.URI != "./Vendor/MSFT/Demo" {
		t.Fatalf("tree %+v root %s", tr, tr.Root.URI)
	}
	root := tr.Root
	if root.MIME != "com.microsoft/11/MDM/Policy" || !root.Permanent || root.Occurrence != "One" || !root.OwnApplicability ||
		len(root.Applicability.OsBuildVersions) != 2 || root.Applicability.OsBuildVersions[1] != "10.0.19041.1202" ||
		root.Applicability.CspVersion != "9.9" || len(root.Applicability.EditionAllowList) != 2 || !root.Applicability.RequiresAzureAd {
		t.Fatalf("root %+v applicability %+v", root, root.Applicability)
	}
	dyn := root.Children[0]
	if !dyn.Dynamic || dyn.Title != "ProfileName" || dyn.URI != "./Vendor/MSFT/Demo/{ProfileName}" || dyn.Naming.Kind != csp.NamingUniqueName || dyn.Naming.Pattern != "[A-Za-z]+" ||
		!dyn.CaseSensitive || !dyn.AtomicRequired || dyn.Deprecated == nil || dyn.Deprecated.OsBuildDeprecated != "10.0.22000" || dyn.MIME != "" ||
		dyn.Applicability != root.Applicability || dyn.OwnApplicability || dyn.Permanent {
		t.Fatalf("dynamic %+v", dyn)
	}
	mode := dyn.Children[0]
	if mode.URI != "./Vendor/MSFT/Demo/{ProfileName}/Mode" || mode.Format != csp.FormatInt || !mode.Leaf() || mode.Description != "Picks a mode." || mode.Default != "1" ||
		len(mode.Access) != 3 || mode.Access[0] != csp.AccessExec || mode.Access[2] != csp.AccessReplace || !mode.Allows(csp.AccessGet) || mode.Allows(csp.AccessAdd) {
		t.Fatalf("mode %+v", mode)
	}
	av := mode.AllowedValues
	if av.Type != csp.ValueTypeEnum || len(av.Enum) != 2 || av.Enum[1].Value != "1" || av.Enum[1].Description != "On" || av.ListDelimiter != ";" {
		t.Fatalf("allowed %+v", av)
	}
	if mode.GpMapping == nil || mode.GpMapping.Element != "Demo_Mode_Enum" || mode.ConflictResolution != "LastWrite" || mode.ReplaceBehavior != "Append" || mode.RebootBehavior != "Automatic" {
		t.Fatalf("mode behaviours %+v", mode)
	}
	if len(mode.Dependencies) != 1 || mode.Dependencies[0].FriendlyID != "NeedsOther" || mode.Dependencies[0].ChangedAllowedValues == nil || mode.Dependencies[0].ChangedAllowedValues.Value != "[0-1]" || mode.Dependencies[0].Dependencies[0].URI != "Vendor/MSFT/Demo/Other" || mode.Dependencies[0].Dependencies[0].ValueType != csp.ValueTypeRange || mode.Dependencies[0].Dependencies[0].Value != "[0]" {
		t.Fatalf("dependencies %+v", mode.Dependencies)
	}
	pol := dyn.Children[1]
	if pol.AllowedValues.Type != csp.ValueTypeADMX || pol.AllowedValues.ADMX == nil || pol.AllowedValues.ADMX.File != "Demo.admx" {
		t.Fatalf("admx %+v", pol.AllowedValues)
	}
	if n := len(tr.Nodes()); n != 4 {
		t.Fatalf("nodes = %d", n)
	}
	// Lookups: case-insensitive static segments, dynamic capture, query
	// stripping, the legacy-root ./Device alias, and misses.
	m, ok := tr.Lookup("./Device/vendor/msft/demo/Work%20Wifi/mode?prop=Type")
	if !ok || m.Node != mode || m.Params["ProfileName"] != "Work Wifi" {
		t.Fatalf("lookup %+v %v", m, ok)
	}
	if _, ok := tr.Lookup("./Vendor/MSFT/Demo/x/Nope"); ok {
		t.Fatal("missing child matched")
	}
	if _, ok := tr.Lookup("./Vendor/MSFT/Other"); ok {
		t.Fatal("other root matched")
	}
	if _, ok := tr.Lookup("./Vendor"); ok {
		t.Fatal("short uri matched")
	}
	if got := mode.Concrete(map[string]string{"ProfileName": "Work Wifi"}); got != "./Vendor/MSFT/Demo/Work%20Wifi/Mode" {
		t.Fatalf("concrete %q", got)
	}
	if got := mode.Concrete(nil); got != mode.URI {
		t.Fatalf("concrete without params %q", got)
	}
}
