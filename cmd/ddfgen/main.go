// Command ddfgen verifies, generates from and diffs the pinned DDF v2 bundle.
//
// # Design
//
// The command is wiring only: bundle checks live in internal/schemagen so they
// are testable without a process boundary. In Phase 0 only verify is
// implemented; generate and diff arrive with Phase 3 of the implementation plan.
//
// # References
//
//   - Decision record 0002: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0002-pinned-references.md
//   - Implementation plan, Phase 3: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deploymenttheory/go-microsoft-dm/internal/schemagen"
)

const usage = `usage: ddfgen <verify|generate|diff> [flags]

  verify    check third_party/ddf against its MANIFEST.json
  generate  regenerate the schema tier (Phase 3)
  diff      compare two bundles (Phase 3)
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "verify":
		dir := filepath.Join("third_party", "ddf")
		if len(args) > 1 {
			dir = args[1]
		}
		m, err := schemagen.LoadManifest(filepath.Join(dir, "MANIFEST.json"))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := schemagen.VerifyBundles(dir, m); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, b := range m.Bundles {
			fmt.Fprintf(stdout, "ddfgen: %s ok (%d files, sha256 %s)\n", b.File, b.Files, b.SHA256[:12])
		}
		return 0
	case "generate", "diff":
		fmt.Fprintf(stderr, "ddfgen: %s is not implemented until Phase 3 of the implementation plan\n", args[0])
		return 2
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}
