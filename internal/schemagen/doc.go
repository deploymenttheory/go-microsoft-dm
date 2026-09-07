// Package schemagen reads the pinned DDF v2 bundle and, from Phase 3, generates the schema tier.
//
// # Design
//
// In Phase 0 the package verifies the checked-in bundle against its manifest
// so make verify has something to check. Phase 3 adds the DDF parser and the
// emitter. It never runs at library runtime; cmd/ddfgen is its only caller.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Decision record 0002: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0002-pinned-references.md
//   - Implementation plan, Phase 3: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package schemagen
