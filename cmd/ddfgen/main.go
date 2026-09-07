// Command ddfgen verifies, generates from and diffs the pinned DDF v2 bundle.
//
// # Design
//
// The command is wiring only: bundle checks, parsing, rendering and diffing
// live in internal/schemagen so they are testable without a process boundary.
// verify checks the bundle pin and that schema/ matches regeneration;
// generate rewrites schema/csp, schema/policy and schema/registry; diff
// compares two drops node by node.
//
// # References
//
//   - Decision record 0002: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0002-pinned-references.md
//   - Decision record 0006: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0006-schema-generator-over-ddf-v2.md
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/deploymenttheory/go-microsoft-dm/internal/schemagen"
)

const usage = `usage: ddfgen [-ddf dir] [-out dir] <verify|generate|diff old.zip new.zip>

  verify    check third_party/ddf against its MANIFEST.json and schema/ against regeneration
  generate  regenerate schema/csp, schema/policy and schema/registry from the pinned bundle
  diff      list nodes added, removed and changed between two DDF drops
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("ddfgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ddfDir := fs.String("ddf", filepath.Join("third_party", "ddf"), "directory holding the DDF drops and MANIFEST.json")
	outDir := fs.String("out", "schema", "schema output directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch fs.Arg(0) {
	case "verify":
		m, err := schemagen.LoadManifest(filepath.Join(*ddfDir, "MANIFEST.json"))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := schemagen.VerifyBundles(*ddfDir, m); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, b := range m.Bundles {
			fmt.Fprintf(stdout, "ddfgen: %s ok (%d files, sha256 %s)\n", b.File, b.Files, b.SHA256[:12])
		}
		if err := schemagen.Verify(*ddfDir, m, *outDir); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "ddfgen: %s matches regeneration\n", *outDir)
		return 0
	case "generate":
		m, err := schemagen.LoadManifest(filepath.Join(*ddfDir, "MANIFEST.json"))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		files, err := schemagen.Run(*ddfDir, m)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := schemagen.Write(*outDir, files); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "ddfgen: generated %d files into %s\n", len(files), *outDir)
		return 0
	case "diff":
		if fs.NArg() != 3 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		oldDrop, err := schemagen.ParseZip(fs.Arg(1))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		newDrop, err := schemagen.ParseZip(fs.Arg(2))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		report := schemagen.Diff(oldDrop, newDrop)
		fmt.Fprint(stdout, report.String())
		return 0
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}
