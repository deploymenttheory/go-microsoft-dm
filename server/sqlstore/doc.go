// Package sqlstore holds the SQL storage backends and their migrations.
//
// # Design
//
// Phase 6 of the implementation plan fills this package with sqlite, postgres
// and mysql subpackages that pass storage/storagetest. Until Phase 6 the
// package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 6: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package sqlstore
