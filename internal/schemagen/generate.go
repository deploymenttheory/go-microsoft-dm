package schemagen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	// ErrGenerate wraps generator failures.
	ErrGenerate = errors.New("schemagen: generate")
	// ErrVerify reports that the checked-in output differs from regeneration.
	ErrVerify = errors.New("schemagen: verify")
)

// GeneratedFrom is schema/GENERATED_FROM.json: the provenance every generated
// file is stamped with.
type GeneratedFrom struct {
	Bundle       string `json:"bundle"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	LastModified string `json:"lastModified"`
	Files        int    `json:"files"`
	Trees        int    `json:"trees"`
	Nodes        int    `json:"nodes"`
	Generator    string `json:"generator"`
}

// Files maps a path relative to the schema directory to its content.
type Files map[string][]byte

// Run parses the pinned bundle named by the manifest entry and renders every
// generated file. The manifest's first bundle is the source; later drops are
// kept for ddfgen diff.
func Run(ddfDir string, m Manifest) (Files, error) {
	if len(m.Bundles) == 0 {
		return nil, fmt.Errorf("%w: manifest lists no bundle", ErrGenerate)
	}
	b := m.Bundles[0]
	zipPath := filepath.Join(ddfDir, b.File)
	if err := VerifyBundle(zipPath, b); err != nil {
		return nil, err
	}
	drop, err := ParseZip(zipPath)
	if err != nil {
		return nil, err
	}
	return render(drop, b)
}

func render(drop *Drop, b Bundle) (Files, error) {
	gf := &GeneratedFrom{
		Bundle:       b.File,
		URL:          b.URL,
		SHA256:       b.SHA256,
		LastModified: b.LastModified,
		Files:        drop.Files,
		Trees:        len(drop.Trees),
		Generator:    "ddfgen (cmd/ddfgen, internal/schemagen)",
	}
	for _, t := range drop.Trees {
		gf.Nodes += len(t.Nodes())
	}
	gs, err := groups(drop)
	if err != nil {
		return nil, err
	}
	files := Files{}
	var lock []string
	for _, g := range gs {
		out, err := emitGroup(g, gf, &lock)
		if err != nil {
			return nil, err
		}
		for k, v := range out {
			files[k] = v
		}
	}
	reg, err := emitRegistry(gs, gf)
	if err != nil {
		return nil, err
	}
	files["registry/registry.gen.go"] = reg
	// escape lives in every generated package as a tiny helper so the
	// generated code has no import beyond csp.
	sort.Strings(lock)
	files["EXPORTED_IDENTIFIERS.lock"] = []byte(strings.Join(lock, "\n") + "\n")
	js, err := json.MarshalIndent(gf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGenerate, err)
	}
	files["GENERATED_FROM.json"] = append(js, '\n')
	return files, nil
}

// generatedDirs are the subdirectories of schema the generator owns; stale
// files in them are removed on Write.
var generatedDirs = []string{"csp", "policy", "registry"}

// Write stores files under outDir and removes generated files that are no
// longer produced. Hand-written files (ALLOWED_REMOVALS.md, doc.go in
// hand-written packages) are untouched.
func Write(outDir string, files Files) error {
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("%w: %w", ErrGenerate, err)
	}
	root, err := os.OpenRoot(outDir)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrGenerate, err)
	}
	defer root.Close()
	written := map[string]bool{}
	for rel, data := range files {
		rel = filepath.FromSlash(rel)
		if dir := filepath.Dir(rel); dir != "." {
			if err := root.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("%w: %w", ErrGenerate, err)
			}
		}
		if err := root.WriteFile(rel, data, 0o600); err != nil {
			return fmt.Errorf("%w: %w", ErrGenerate, err)
		}
		written[rel] = true
	}
	for _, dir := range generatedDirs {
		if err := removeStale(root, dir, written); err != nil {
			return err
		}
	}
	return nil
}

// readDir lists a directory inside the root; os.Root has no ReadDir.
func readDir(root *os.Root, dir string) ([]os.DirEntry, error) {
	f, err := root.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGenerate, err)
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGenerate, err)
	}
	return entries, nil
}

func removeStale(root *os.Root, dir string, written map[string]bool) error {
	entries, err := readDir(root, dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrGenerate, err)
	}
	for _, e := range entries {
		rel := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if err := removeStale(root, rel, written); err != nil {
				return err
			}
			if rest, _ := readDir(root, rel); len(rest) == 0 {
				if err := root.Remove(rel); err != nil {
					return fmt.Errorf("%w: %w", ErrGenerate, err)
				}
			}
			continue
		}
		if strings.HasSuffix(e.Name(), ".gen.go") && !written[rel] {
			if err := root.Remove(rel); err != nil {
				return fmt.Errorf("%w: %w", ErrGenerate, err)
			}
		}
	}
	return nil
}

// Verify regenerates into memory and compares with outDir: every generated
// file must match byte for byte, and every identifier in the checked-in lock
// must still be generated unless ALLOWED_REMOVALS.md names it.
func Verify(ddfDir string, m Manifest, outDir string) error {
	files, err := Run(ddfDir, m)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(outDir)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrVerify, err)
	}
	defer root.Close()
	var problems []string
	onDisk, _ := root.ReadFile("EXPORTED_IDENTIFIERS.lock")
	merged, stale := mergeLock(onDisk, files["EXPORTED_IDENTIFIERS.lock"], allowedRemovals(root))
	for _, n := range stale {
		problems = append(
			problems,
			"EXPORTED_IDENTIFIERS.lock: "+n+" is no longer generated; add it to ALLOWED_REMOVALS.md to allow its removal",
		)
	}
	files["EXPORTED_IDENTIFIERS.lock"] = merged
	for rel, want := range files {
		got, err := root.ReadFile(filepath.FromSlash(rel))
		if err != nil {
			problems = append(problems, rel+": missing")
			continue
		}
		if !bytes.Equal(got, want) {
			problems = append(problems, rel+": differs from regenerated output")
		}
	}
	for _, dir := range generatedDirs {
		problems = append(problems, staleOnDisk(root, dir, files)...)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%w:\n  %s", ErrVerify, strings.Join(problems, "\n  "))
	}
	return nil
}

func staleOnDisk(root *os.Root, dir string, files Files) []string {
	var out []string
	entries, err := readDir(root, dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		rel := filepath.ToSlash(filepath.Join(dir, e.Name()))
		if e.IsDir() {
			out = append(out, staleOnDisk(root, rel, files)...)
			continue
		}
		if strings.HasSuffix(e.Name(), ".gen.go") {
			if _, ok := files[rel]; !ok {
				out = append(out, rel+": not generated any more")
			}
		}
	}
	return out
}

// mergeLock keeps every identifier the checked-in lock has, adds the newly
// generated ones, and reports those no longer generated and not allowed to go.
func mergeLock(onDisk, generated []byte, allowed map[string]bool) ([]byte, []string) {
	gen := map[string]bool{}
	for _, l := range strings.Split(strings.TrimSpace(string(generated)), "\n") {
		if l != "" {
			gen[l] = true
		}
	}
	keep := map[string]bool{}
	var stale []string
	for _, l := range strings.Split(strings.TrimSpace(string(onDisk)), "\n") {
		if l == "" {
			continue
		}
		if gen[l] || !allowed[l] {
			keep[l] = true
		}
		if !gen[l] && !allowed[l] {
			stale = append(stale, l)
		}
	}
	for l := range gen {
		keep[l] = true
	}
	out := make([]string, 0, len(keep))
	for l := range keep {
		out = append(out, l)
	}
	sort.Strings(out)
	sort.Strings(stale)
	return []byte(strings.Join(out, "\n") + "\n"), stale
}

// allowedRemovals reads the "- `identifier` reason" entries of ALLOWED_REMOVALS.md.
func allowedRemovals(root *os.Root) map[string]bool {
	out := map[string]bool{}
	data, err := root.ReadFile("ALLOWED_REMOVALS.md")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- `") {
			continue
		}
		rest := line[3:]
		if i := strings.IndexByte(rest, '`'); i > 0 {
			out[rest[:i]] = true
		}
	}
	return out
}
