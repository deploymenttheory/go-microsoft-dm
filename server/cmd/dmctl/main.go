// Command dmctl is the reference operator CLI for go-microsoft-dm.
//
// # Design
//
// The binary is wiring only; behaviour lives in server/internal/app and the
// packages it composes. Phase 6 of the implementation plan fills it. Until then
// it exits with status 2 so nothing mistakes the placeholder for a server.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 6: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "dmctl: not implemented until Phase 6 of the implementation plan")
	os.Exit(2)
}
