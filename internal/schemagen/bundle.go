package schemagen

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrBundleMissing reports a bundle or manifest that cannot be read.
	ErrBundleMissing = errors.New("schemagen: bundle missing")
	// ErrBundleMismatch reports a bundle that does not match its manifest entry.
	ErrBundleMismatch = errors.New("schemagen: bundle does not match manifest")
	// ErrManifestInvalid reports a manifest that cannot be used.
	ErrManifestInvalid = errors.New("schemagen: invalid manifest")
)

// Bundle is one pinned DDF drop. The fields are the facts a reviewer needs to
// tell one Microsoft drop from the next: the download URL, what the HTTP
// response said about it, and what the archive contains.
type Bundle struct {
	File         string `json:"file"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified"`
	ETag         string `json:"etag,omitempty"`
	TopFolder    string `json:"topFolder"`
	Files        int    `json:"files"`
}

// Manifest lists every DDF drop under third_party/ddf. Drops are added beside
// each other and never replaced, so two can be diffed.
type Manifest struct {
	Bundles []Bundle `json:"bundles"`
}

// LoadManifest reads and validates a manifest file.
func LoadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrBundleMissing, err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrManifestInvalid, err)
	}
	if len(m.Bundles) == 0 {
		return Manifest{}, fmt.Errorf("%w: no bundles listed in %s", ErrManifestInvalid, path)
	}
	for _, b := range m.Bundles {
		if b.File == "" || b.SHA256 == "" || b.Size <= 0 || b.TopFolder == "" || b.Files <= 0 {
			return Manifest{}, fmt.Errorf("%w: entry %q is incomplete", ErrManifestInvalid, b.File)
		}
		if filepath.Base(b.File) != b.File {
			return Manifest{}, fmt.Errorf(
				"%w: entry %q must be a bare file name",
				ErrManifestInvalid,
				b.File,
			)
		}
	}
	return m, nil
}

// VerifyBundles checks every manifest entry against the file beside it in dir.
func VerifyBundles(dir string, m Manifest) error {
	for _, b := range m.Bundles {
		if err := VerifyBundle(filepath.Join(dir, b.File), b); err != nil {
			return err
		}
	}
	return nil
}

// VerifyBundle checks one archive's size, SHA-256, single top-level folder and
// XML file count against its manifest entry. Every difference is reported as
// ErrBundleMismatch with the observed and expected values, because the whole
// point of the pin is to notice when Microsoft changes a drop in place.
func VerifyBundle(path string, b Bundle) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBundleMissing, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBundleMissing, err)
	}
	if info.Size() != b.Size {
		return fmt.Errorf(
			"%w: %s is %d bytes, manifest says %d",
			ErrBundleMismatch,
			b.File,
			info.Size(),
			b.Size,
		)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("%w: %w", ErrBundleMissing, err)
	}
	if sum := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(sum, b.SHA256) {
		return fmt.Errorf(
			"%w: %s sha256 is %s, manifest says %s",
			ErrBundleMismatch,
			b.File,
			sum,
			b.SHA256,
		)
	}
	top, files, err := inspect(path, info.Size())
	if err != nil {
		return err
	}
	if top != b.TopFolder {
		return fmt.Errorf(
			"%w: %s top folder is %q, manifest says %q",
			ErrBundleMismatch,
			b.File,
			top,
			b.TopFolder,
		)
	}
	if files != b.Files {
		return fmt.Errorf(
			"%w: %s has %d xml files, manifest says %d",
			ErrBundleMismatch,
			b.File,
			files,
			b.Files,
		)
	}
	return nil
}

// inspect returns the single top-level folder and the XML file count. A drop
// with no folder, more than one, or a non-XML member is a mismatch: every
// Microsoft drop so far has been one folder of .xml files and the parser in
// Phase 3 relies on that.
func inspect(path string, size int64) (string, int, error) {
	name := filepath.Base(path)
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", ErrBundleMissing, err)
	}
	defer f.Close()
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %s: %w", ErrBundleMismatch, name, err)
	}
	top := ""
	files := 0
	for _, e := range zr.File {
		first, rest, ok := strings.Cut(e.Name, "/")
		if !ok {
			return "", 0, fmt.Errorf(
				"%w: %s has a top-level file %q",
				ErrBundleMismatch,
				name,
				e.Name,
			)
		}
		folder := first + "/"
		switch {
		case top == "":
			top = folder
		case top != folder:
			return "", 0, fmt.Errorf(
				"%w: %s has more than one top folder (%q, %q)",
				ErrBundleMismatch,
				name,
				top,
				folder,
			)
		}
		if rest == "" || strings.HasSuffix(rest, "/") {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(rest), ".xml") {
			return "", 0, fmt.Errorf(
				"%w: %s contains a non-xml member %q",
				ErrBundleMismatch,
				name,
				e.Name,
			)
		}
		files++
	}
	if top == "" || files == 0 {
		return "", 0, fmt.Errorf("%w: %s contains no xml files", ErrBundleMismatch, name)
	}
	return top, files, nil
}
