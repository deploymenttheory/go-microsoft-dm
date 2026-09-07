package schemagen_test

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/internal/schemagen"
)

// writeZip creates an archive of the named members (folders end in "/") and
// returns its path and a matching manifest entry.
func writeZip(t *testing.T, dir, name string, members []string) (string, schemagen.Bundle) {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	files := 0
	for _, m := range members {
		w, err := zw.Create(m)
		if err != nil {
			t.Fatal(err)
		}
		if m[len(m)-1] != '/' {
			if _, err := w.Write([]byte("<MgmtTree/>")); err != nil {
				t.Fatal(err)
			}
			files++
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, schemagen.Bundle{
		File: name, URL: "https://example.invalid/" + name, SHA256: sha256hex(raw),
		Size: int64(len(raw)), LastModified: "2026-02-19T18:04:42Z", TopFolder: "DDFDrop/", Files: files,
	}
}

func sha256hex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func TestVerifyBundleAcceptsAMatchingArchive(t *testing.T) {
	t.Parallel()
	path, b := writeZip(t, t.TempDir(), "a.zip", []string{"DDFDrop/", "DDFDrop/DMClient.xml", "DDFDrop/Policy.xml"})
	if err := schemagen.VerifyBundle(path, b); err != nil {
		t.Fatal(err)
	}
	if b.Files != 2 {
		t.Fatalf("files = %d", b.Files)
	}
}

func TestVerifyBundleRejectsEveryMismatch(t *testing.T) {
	t.Parallel()
	path, good := writeZip(t, t.TempDir(), "a.zip", []string{"DDFDrop/", "DDFDrop/DMClient.xml"})
	cases := map[string]func(schemagen.Bundle) schemagen.Bundle{
		"size":       func(b schemagen.Bundle) schemagen.Bundle { b.Size++; return b },
		"sha256":     func(b schemagen.Bundle) schemagen.Bundle { b.SHA256 = "00" + b.SHA256[2:]; return b },
		"upper hash": func(b schemagen.Bundle) schemagen.Bundle { b.SHA256 = "ABCDEF" + b.SHA256[6:]; return b },
		"top folder": func(b schemagen.Bundle) schemagen.Bundle { b.TopFolder = "Other/"; return b },
		"file count": func(b schemagen.Bundle) schemagen.Bundle { b.Files = 5; return b },
	}
	for name, mutate := range cases {
		if err := schemagen.VerifyBundle(path, mutate(good)); !errors.Is(err, schemagen.ErrBundleMismatch) {
			t.Errorf("%s: err = %v, want ErrBundleMismatch", name, err)
		}
	}
}

func TestVerifyBundleRejectsMalformedArchives(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := map[string][]string{
		"top-level file":   {"DMClient.xml"},
		"two top folders":  {"A/x.xml", "B/y.xml"},
		"non-xml member":   {"DDFDrop/readme.txt"},
		"empty archive":    {},
		"only a directory": {"DDFDrop/"},
	}
	for name, members := range cases {
		path, b := writeZip(t, dir, name+".zip", members)
		if err := schemagen.VerifyBundle(path, b); !errors.Is(err, schemagen.ErrBundleMismatch) {
			t.Errorf("%s: err = %v, want ErrBundleMismatch", name, err)
		}
	}
	notZip := filepath.Join(dir, "not.zip")
	if err := os.WriteFile(notZip, []byte("plain text"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := schemagen.Bundle{File: "not.zip", Size: 10, SHA256: sha256hex([]byte("plain text")), TopFolder: "x/", Files: 1}
	if err := schemagen.VerifyBundle(notZip, b); !errors.Is(err, schemagen.ErrBundleMismatch) {
		t.Errorf("not a zip: err = %v", err)
	}
	if err := schemagen.VerifyBundle(filepath.Join(dir, "absent.zip"), b); !errors.Is(err, schemagen.ErrBundleMissing) {
		t.Errorf("absent: err = %v", err)
	}
}

func TestLoadManifestAndVerifyBundles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, b := writeZip(t, dir, "a.zip", []string{"DDFDrop/", "DDFDrop/DMClient.xml"})
	write := func(name string, raw []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	marshal := func(m schemagen.Manifest) []byte {
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	good := write("MANIFEST.json", marshal(schemagen.Manifest{Bundles: []schemagen.Bundle{b}}))
	m, err := schemagen.LoadManifest(good)
	if err != nil {
		t.Fatal(err)
	}
	if err := schemagen.VerifyBundles(dir, m); err != nil {
		t.Fatal(err)
	}
	if err := schemagen.VerifyBundles(t.TempDir(), m); !errors.Is(err, schemagen.ErrBundleMissing) {
		t.Errorf("bundle elsewhere: err = %v", err)
	}
	if _, err := schemagen.LoadManifest(filepath.Join(dir, "nope.json")); !errors.Is(err, schemagen.ErrBundleMissing) {
		t.Errorf("missing manifest: err = %v", err)
	}
	pathy := b
	pathy.File = "../a.zip"
	bad := map[string][]byte{
		"not json":   []byte("{"),
		"no bundles": marshal(schemagen.Manifest{}),
		"incomplete": marshal(schemagen.Manifest{Bundles: []schemagen.Bundle{{File: "a.zip"}}}),
		"with path":  marshal(schemagen.Manifest{Bundles: []schemagen.Bundle{pathy}}),
	}
	for name, raw := range bad {
		if _, err := schemagen.LoadManifest(write(name+".json", raw)); !errors.Is(err, schemagen.ErrManifestInvalid) {
			t.Errorf("%s: err = %v, want ErrManifestInvalid", name, err)
		}
	}
}

// TestPinnedBundleMatchesManifest is the check make verify runs, from inside
// the test suite so a stale manifest fails CI even before the Makefile runs.
func TestPinnedBundleMatchesManifest(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "..", "third_party", "ddf")
	m, err := schemagen.LoadManifest(filepath.Join(dir, "MANIFEST.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := schemagen.VerifyBundles(dir, m); err != nil {
		t.Fatal(err)
	}
}
