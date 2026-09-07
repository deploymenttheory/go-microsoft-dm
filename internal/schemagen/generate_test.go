package schemagen_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/internal/schemagen"
)

const demoDDF = `<?xml version="1.0" encoding="UTF-8"?>
<MgmtTree xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM">
  <VerDTD>1.2</VerDTD>
  <Node>
    <NodeName>Demo</NodeName>
    <Path>./Device/Vendor/MSFT</Path>
    <DFProperties><AccessType><Get /></AccessType><DFFormat><node /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><MIME /></DFType>
      <MSFT:Applicability><MSFT:OsBuildVersion>10.0.22000</MSFT:OsBuildVersion><MSFT:CspVersion>1.0</MSFT:CspVersion></MSFT:Applicability>
    </DFProperties>
    <Node>
      <NodeName>Root</NodeName>
      <DFProperties><AccessType><Get /><Replace /></AccessType><Description>Collides with the reserved root constant.</Description><DFFormat><chr /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><MIME>text/plain</MIME></DFType></DFProperties>
    </Node>
    <Node>
      <NodeName>Mode</NodeName>
      <DFProperties><AccessType><Get /><Replace /></AccessType><Description>Picks a mode. Second sentence.</Description><DFFormat><int /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><MIME>text/plain</MIME></DFType>
        <MSFT:AllowedValues ValueType="ENUM">
          <MSFT:Enum><MSFT:Value>0</MSFT:Value><MSFT:ValueDescription>Off. Nothing happens.</MSFT:ValueDescription></MSFT:Enum>
          <MSFT:Enum><MSFT:Value>1</MSFT:Value><MSFT:ValueDescription>Off. Nothing happens.</MSFT:ValueDescription></MSFT:Enum>
          <MSFT:Enum><MSFT:Value>2</MSFT:Value><MSFT:ValueDescription></MSFT:ValueDescription></MSFT:Enum>
        </MSFT:AllowedValues>
      </DFProperties>
    </Node>
    <Node>
      <NodeName>Profiles</NodeName>
      <DFProperties><AccessType><Get /></AccessType><DFFormat><node /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><DDFName /></DFType></DFProperties>
      <Node>
        <NodeName></NodeName>
        <DFProperties><AccessType><Add /><Delete /><Get /></AccessType><DFFormat><node /></DFFormat><Occurrence><ZeroOrMore /></Occurrence><Scope><Dynamic /></Scope><DFTitle>Profile Name</DFTitle><DFType><DDFName /></DFType>
          <MSFT:DynamicNodeNaming><MSFT:ServerGeneratedUniqueIdentifier /></MSFT:DynamicNodeNaming>
        </DFProperties>
        <Node>
          <NodeName>Type</NodeName>
          <DFProperties><AccessType><Get /></AccessType><DFFormat><chr /></DFFormat><Occurrence><One /></Occurrence><Scope><Dynamic /></Scope><DFType><MIME>text/plain</MIME></DFType></DFProperties>
        </Node>
      </Node>
      <Node>
        <NodeName>Type</NodeName>
        <DFProperties><AccessType><Get /></AccessType><DFFormat><chr /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><MIME>text/plain</MIME></DFType></DFProperties>
      </Node>
    </Node>
  </Node>
  <Node>
    <NodeName>Demo</NodeName>
    <Path>./User/Vendor/MSFT</Path>
    <DFProperties><AccessType><Get /></AccessType><DFFormat><node /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><MIME /></DFType></DFProperties>
    <Node>
      <NodeName>Mode</NodeName>
      <DFProperties><AccessType><Get /><Replace /></AccessType><DFFormat><int /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><MIME>text/plain</MIME></DFType></DFProperties>
    </Node>
  </Node>
</MgmtTree>`

const areaDDF = `<?xml version="1.0" encoding="UTF-8"?>
<MgmtTree xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM">
  <VerDTD>1.2</VerDTD>
  <Node>
    <NodeName>6to4</NodeName>
    <Path>./Device/Vendor/MSFT/Policy/Config</Path>
    <DFProperties><AccessType><Get /></AccessType><DFFormat><node /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><MIME>com.microsoft/11/MDM/Policy</MIME></DFType></DFProperties>
    <Node>
      <NodeName>6to4_State</NodeName>
      <DFProperties><AccessType><Add /><Delete /><Get /><Replace /></AccessType><DFFormat><chr /></DFFormat><Occurrence><ZeroOrOne /></Occurrence><Scope><Dynamic /></Scope><DFType><MIME>text/plain</MIME></DFType>
        <MSFT:AllowedValues ValueType="ADMX"><MSFT:AdmxBacked Area="tcpip~AT~Network~TCPIP~IPv6Transition" Name="6to4_State" File="tcpip.admx" /></MSFT:AllowedValues>
        <MSFT:ConflictResolution>LastWrite</MSFT:ConflictResolution>
      </DFProperties>
    </Node>
  </Node>
</MgmtTree>`

// writeDrop builds a zip with the given files under one top folder and
// returns its path and manifest entry.
func writeDrop(t *testing.T, dir, name string, files map[string]string) (string, schemagen.Bundle) {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for n, c := range files {
		w, err := zw.Create("Drop/" + n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	raw, _ := os.ReadFile(p)
	sum := sha256.Sum256(raw)
	return p, schemagen.Bundle{File: name, URL: "https://example.invalid/" + name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(raw)), LastModified: "2026-02-19T18:04:42Z", TopFolder: "Drop/", Files: len(files)}
}

func TestGenerateWriteVerify(t *testing.T) {
	t.Parallel()
	ddf := t.TempDir()
	_, b := writeDrop(t, ddf, "demo.zip", map[string]string{"Demo.xml": demoDDF, "6to4_AreaDDF.xml": areaDDF})
	m := schemagen.Manifest{Bundles: []schemagen.Bundle{b}}

	files, err := schemagen.Run(ddf, m)
	if err != nil {
		t.Fatal(err)
	}
	again, err := schemagen.Run(ddf, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(again) {
		t.Fatal("non-deterministic file set")
	}
	for k, v := range files {
		if !bytes.Equal(v, again[k]) {
			t.Fatalf("%s differs between runs", k)
		}
	}
	for _, want := range []string{"csp/demo/doc.gen.go", "csp/demo/tree.gen.go", "csp/demo/uris.gen.go", "csp/demo/values.gen.go", "policy/n6to4/uris.gen.go", "registry/registry.gen.go", "EXPORTED_IDENTIFIERS.lock", "GENERATED_FROM.json"} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s in %v", want, keys(files))
		}
	}
	if _, ok := files["policy/n6to4/values.gen.go"]; ok {
		t.Error("values file emitted for a package without enums")
	}
	uris := string(files["csp/demo/uris.gen.go"])
	for _, want := range []string{
		`const DeviceRoot = "./Device/Vendor/MSFT/Demo"`,
		`const UserRoot = "./User/Vendor/MSFT/Demo"`,
		`const DeviceRootNode = "./Device/Vendor/MSFT/Demo/Root"`,
		`const DeviceMode = "./Device/Vendor/MSFT/Demo/Mode"`,
		`// Picks a mode.`,
		`const UserMode = "./User/Vendor/MSFT/Demo/Mode"`,
		`func DeviceProfilesProfileName(profileName string) string {`,
		`return "./Device/Vendor/MSFT/Demo/Profiles/" + escape(profileName)`,
		`func DeviceProfilesTypeByProfileName(profileName string) string {`,
		`const DeviceProfilesType = "./Device/Vendor/MSFT/Demo/Profiles/Type"`,
		`func escape(s string) string { return url.PathEscape(s) }`,
		`import "net/url"`,
	} {
		if !strings.Contains(uris, want) {
			t.Errorf("uris.gen.go lacks %q\n%s", want, uris)
		}
	}
	// gofmt aligns the "=" of a const block, so compare with spaces squeezed.
	values := strings.Join(strings.Fields(string(files["csp/demo/values.gen.go"])), " ")
	for _, want := range []string{`DeviceModeOffNothingHappens = "0"`, `DeviceModeOffNothingHappens2 = "1"`, `DeviceModeValue2 = "2"`} {
		if !strings.Contains(values, want) {
			t.Errorf("values.gen.go lacks %q\n%s", want, values)
		}
	}
	lock := string(files["EXPORTED_IDENTIFIERS.lock"])
	for _, want := range []string{"csp/demo/DeviceRoot\n", "csp/demo/DeviceProfilesTypeByProfileName\n", "csp/demo/DeviceModeValue2\n", "policy/n6to4/N6to4State\n", "csp/demo/Trees\n"} {
		if !strings.Contains(lock, want) {
			t.Errorf("lock lacks %q", want)
		}
	}
	if !strings.Contains(string(files["policy/n6to4/uris.gen.go"]), `const N6to4State = "./Device/Vendor/MSFT/Policy/Config/6to4/6to4_State"`) {
		t.Errorf("policy uris\n%s", files["policy/n6to4/uris.gen.go"])
	}
	if !strings.Contains(string(files["GENERATED_FROM.json"]), `"nodes": 11`) {
		t.Errorf("provenance\n%s", files["GENERATED_FROM.json"])
	}

	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "stale.gen.go"), []byte("package schema\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(out, "csp", "old"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "csp", "old", "tree.gen.go"), []byte("package old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "csp", "old", "keep_test.go"), []byte("package old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := schemagen.Write(out, files); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "csp", "old", "tree.gen.go")); !errors.Is(err, os.ErrNotExist) {
		t.Error("stale generated file survived")
	}
	if _, err := os.Stat(filepath.Join(out, "csp", "old", "keep_test.go")); err != nil {
		t.Error("hand-written file removed")
	}
	if err := schemagen.Verify(ddf, m, out); err != nil {
		t.Fatalf("verify after write: %v", err)
	}

	// Tampering with a generated file, dropping an identifier from the
	// output, and an unexplained lock entry all fail verification.
	tree := filepath.Join(out, "csp", "demo", "tree.gen.go")
	orig, _ := os.ReadFile(tree)
	if err := os.WriteFile(tree, append(orig, []byte("// edited\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := schemagen.Verify(ddf, m, out); !errors.Is(err, schemagen.ErrVerify) || !strings.Contains(err.Error(), "csp/demo/tree.gen.go: differs") {
		t.Fatalf("tampered file: %v", err)
	}
	if err := os.WriteFile(tree, orig, 0o600); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(out, "EXPORTED_IDENTIFIERS.lock")
	if err := os.WriteFile(lockPath, []byte(lock+"csp/demo/Gone\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := schemagen.Verify(ddf, m, out); !errors.Is(err, schemagen.ErrVerify) || !strings.Contains(err.Error(), "csp/demo/Gone is no longer generated") {
		t.Fatalf("stale lock entry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "ALLOWED_REMOVALS.md"), []byte("# Allowed removals\n\n- `csp/demo/Gone` removed on purpose\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := schemagen.Verify(ddf, m, out); !errors.Is(err, schemagen.ErrVerify) || !strings.Contains(err.Error(), "EXPORTED_IDENTIFIERS.lock: differs") {
		t.Fatalf("allowed removal still needs the lock rewritten: %v", err)
	}
	if err := schemagen.Write(out, files); err != nil {
		t.Fatal(err)
	}
	if err := schemagen.Verify(ddf, m, out); err != nil {
		t.Fatalf("after regenerate with allowed removal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "csp", "demo", "extra.gen.go"), []byte("package demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := schemagen.Verify(ddf, m, out); !errors.Is(err, schemagen.ErrVerify) || !strings.Contains(err.Error(), "extra.gen.go: not generated any more") {
		t.Fatalf("extra generated file: %v", err)
	}
	if err := os.Remove(filepath.Join(out, "registry", "registry.gen.go")); err != nil {
		t.Fatal(err)
	}
	if err := schemagen.Verify(ddf, m, out); !errors.Is(err, schemagen.ErrVerify) || !strings.Contains(err.Error(), "registry/registry.gen.go: missing") {
		t.Fatalf("missing file: %v", err)
	}
	if err := schemagen.Verify(ddf, m, filepath.Join(out, "nope")); !errors.Is(err, schemagen.ErrVerify) {
		t.Fatalf("missing out dir: %v", err)
	}
}

func keys(f schemagen.Files) []string {
	var out []string
	for k := range f {
		out = append(out, k)
	}
	return out
}

func TestRunFailures(t *testing.T) {
	t.Parallel()
	ddf := t.TempDir()
	if _, err := schemagen.Run(ddf, schemagen.Manifest{}); !errors.Is(err, schemagen.ErrGenerate) {
		t.Fatalf("empty manifest: %v", err)
	}
	_, b := writeDrop(t, ddf, "demo.zip", map[string]string{"Demo.xml": demoDDF})
	bad := b
	bad.SHA256 = "00" + bad.SHA256[2:]
	if _, err := schemagen.Run(ddf, schemagen.Manifest{Bundles: []schemagen.Bundle{bad}}); !errors.Is(err, schemagen.ErrBundleMismatch) {
		t.Fatalf("bad pin: %v", err)
	}
	_, broken := writeDrop(t, ddf, "broken.zip", map[string]string{"Broken.xml": "<MgmtTree><Node><NodeName>X</NodeName></Node></MgmtTree>"})
	if _, err := schemagen.Run(ddf, schemagen.Manifest{Bundles: []schemagen.Bundle{broken}}); !errors.Is(err, schemagen.ErrDDF) {
		t.Fatalf("broken ddf: %v", err)
	}
	// Two files whose names map to one package name is a generator error.
	_, clash := writeDrop(t, ddf, "clash.zip", map[string]string{
		"Demo.xml":  demoDDF,
		"De_mo.xml": strings.ReplaceAll(demoDDF, "<NodeName>Demo</NodeName>", "<NodeName>De_mo</NodeName>"),
	})
	if _, err := schemagen.Run(ddf, schemagen.Manifest{Bundles: []schemagen.Bundle{clash}}); !errors.Is(err, schemagen.ErrGenerate) {
		t.Fatalf("package clash: %v", err)
	}
	if err := schemagen.Write(filepath.Join(ddf, "file-not-dir", "x"), schemagen.Files{"a/b.gen.go": []byte("x")}); err == nil {
		// MkdirAll succeeds; ensure a write into a path under a file fails.
		if err := os.WriteFile(filepath.Join(ddf, "blocker"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := schemagen.Write(filepath.Join(ddf, "blocker"), schemagen.Files{"a.gen.go": []byte("x")}); !errors.Is(err, schemagen.ErrGenerate) {
			t.Fatalf("write under a file: %v", err)
		}
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldZip, _ := writeDrop(t, dir, "old.zip", map[string]string{"Demo.xml": demoDDF, "Gone_AreaDDF.xml": strings.ReplaceAll(areaDDF, "6to4", "Gone")})
	newer := strings.ReplaceAll(demoDDF, "<MSFT:OsBuildVersion>10.0.22000</MSFT:OsBuildVersion>", "<MSFT:OsBuildVersion>10.0.26100</MSFT:OsBuildVersion>")
	newer = strings.Replace(newer, "<NodeName>Root</NodeName>", "<NodeName>Rooted</NodeName>", 1)
	newer = strings.Replace(newer, "<MSFT:Value>2</MSFT:Value><MSFT:ValueDescription></MSFT:ValueDescription>", "<MSFT:Value>3</MSFT:Value><MSFT:ValueDescription>Three</MSFT:ValueDescription>", 1)
	newZip, _ := writeDrop(t, dir, "new.zip", map[string]string{"Demo.xml": newer, "6to4_AreaDDF.xml": areaDDF})
	o, err := schemagen.ParseZip(oldZip)
	if err != nil {
		t.Fatal(err)
	}
	n, err := schemagen.ParseZip(newZip)
	if err != nil {
		t.Fatal(err)
	}
	r := schemagen.Diff(o, n)
	s := r.String()
	for _, want := range []string{
		"files added: 1, removed: 1",
		"+ file 6to4_AreaDDF.xml",
		"- file Gone_AreaDDF.xml",
		"+ ./Device/Vendor/MSFT/Demo/Rooted",
		"- ./Device/Vendor/MSFT/Demo/Root",
		"~ ./Device/Vendor/MSFT/Demo: applicability 10.0.22000 csp 1.0 -> 10.0.26100 csp 1.0",
		"~ ./Device/Vendor/MSFT/Demo/Mode: applicability 10.0.22000 csp 1.0 -> 10.0.26100 csp 1.0; allowed values",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("report lacks %q\n%s", want, s)
		}
	}
	if strings.Contains(s, "~ ./User/Vendor/MSFT/Demo/Mode") {
		t.Error("unchanged node reported")
	}
	if same := schemagen.Diff(o, o); len(same.Added)+len(same.Removed)+len(same.Changed)+len(same.FilesAdded)+len(same.FilesRemoved) != 0 {
		t.Errorf("self diff %+v", same)
	}
}

// TestCheckedInSchemaMatchesPinnedBundle is make verify from inside the test
// suite: the committed schema/ must be exactly what the pinned bundle renders.
// It also exercises every emitter branch the synthetic drop does not reach.
func TestCheckedInSchemaMatchesPinnedBundle(t *testing.T) {
	t.Parallel()
	ddf := filepath.Join("..", "..", "third_party", "ddf")
	m, err := schemagen.LoadManifest(filepath.Join(ddf, "MANIFEST.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := schemagen.Verify(ddf, m, filepath.Join("..", "..", "schema")); err != nil {
		t.Fatalf("schema/ is stale; run make generate: %v", err)
	}
}

func TestWriteRemovesEmptyGeneratedDirectoriesAndDiffFields(t *testing.T) {
	t.Parallel()
	ddf := t.TempDir()
	legacy := `<?xml version="1.0" encoding="UTF-8"?>
<MgmtTree xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM">
  <VerDTD>1.2</VerDTD>
  <Node>
    <NodeName>Old</NodeName>
    <Path>./Vendor/MSFT</Path>
    <DFProperties><AccessType><Get /></AccessType><DFFormat><node /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><MIME /></DFType></DFProperties>
    <Node>
      <NodeName>Long</NodeName>
      <DFProperties><AccessType><Get /><Exec /></AccessType><Description>` + strings.Repeat("word ", 60) + `</Description><DFFormat><null /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><MIME>text/plain</MIME></DFType><DefaultValue>x</DefaultValue><CaseSense><CS /></CaseSense>
        <MSFT:GpMapping GpEnglishName="Old" GpAreaPath="Old~AT~System" />
        <MSFT:ConflictResolution>LastWrite</MSFT:ConflictResolution><MSFT:ReplaceBehavior>Append</MSFT:ReplaceBehavior><MSFT:RebootBehavior>Automatic</MSFT:RebootBehavior><MSFT:AtomicRequired /><MSFT:Deprecated />
      </DFProperties>
    </Node>
  </Node>
  <Node>
    <NodeName>Old</NodeName>
    <Path>./Device/Vendor/MSFT</Path>
    <DFProperties><AccessType><Get /></AccessType><DFFormat><node /></DFFormat><Occurrence><One /></Occurrence><Scope><Permanent /></Scope><DFType><MIME /></DFType></DFProperties>
  </Node>
</MgmtTree>`
	_, b := writeDrop(t, ddf, "old.zip", map[string]string{"Old.xml": legacy, "Demo.xml": demoDDF})
	m := schemagen.Manifest{Bundles: []schemagen.Bundle{b}}
	files, err := schemagen.Run(ddf, m)
	if err != nil {
		t.Fatal(err)
	}
	uris := string(files["csp/old/uris.gen.go"])
	if !strings.Contains(uris, "const LegacyRoot = \"./Vendor/MSFT/Old\"") || !strings.Contains(uris, "const DeviceRoot = \"./Device/Vendor/MSFT/Old\"") || !strings.Contains(uris, "word word word") || !strings.Contains(uris, "...") {
		t.Fatalf("legacy package\n%s", uris)
	}
	// gofmt aligns literal fields; compare with spaces squeezed.
	tree := strings.Join(strings.Fields(string(files["csp/old/tree.gen.go"])), " ")
	for _, want := range []string{"CaseSensitive: true", "AtomicRequired: true", `RebootBehavior: "Automatic"`, `ReplaceBehavior: "Append"`, `GpMapping: &csp.GpMapping{`, `Deprecated: &csp.Deprecated{OsBuildDeprecated: ""}`, `Default: "x"`, `MIME: "text/plain"`} {
		if !strings.Contains(tree, want) {
			t.Errorf("tree lacks %s", want)
		}
	}
	out := t.TempDir()
	if err := os.MkdirAll(filepath.Join(out, "policy", "gone", "deeper"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "policy", "gone", "deeper", "x.gen.go"), []byte("package x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := schemagen.Write(out, files); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "policy", "gone")); !errors.Is(err, os.ErrNotExist) {
		t.Error("emptied generated directory not removed")
	}
	if err := schemagen.Verify(ddf, m, out); err != nil {
		t.Fatal(err)
	}

	// Diff reports every compared field.
	changed := strings.Replace(legacy, "<DFFormat><null /></DFFormat>", "<DFFormat><chr /></DFFormat>", 1)
	changed = strings.Replace(changed, "<AccessType><Get /><Exec /></AccessType>", "<AccessType><Get /></AccessType>", 1)
	changed = strings.Replace(changed, "<DefaultValue>x</DefaultValue>", "<DefaultValue>y</DefaultValue>", 1)
	changed = strings.Replace(changed, "<MSFT:Deprecated />", "", 1)
	changed = strings.Replace(changed, "<Description>word ", "<Description>other ", 1)
	newZip, _ := writeDrop(t, ddf, "new.zip", map[string]string{"Old.xml": changed})
	o, _ := schemagen.ParseZip(filepath.Join(ddf, "old.zip"))
	n, err := schemagen.ParseZip(newZip)
	if err != nil {
		t.Fatal(err)
	}
	s := schemagen.Diff(o, n).String()
	for _, want := range []string{"format null -> chr", "access [Exec Get] -> [Get]", `default "x" -> "y"`, "deprecated true -> false", "description", "- file Demo.xml"} {
		if !strings.Contains(s, want) {
			t.Errorf("diff lacks %q\n%s", want, s)
		}
	}
}
