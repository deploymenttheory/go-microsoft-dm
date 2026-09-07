// Package inmem is the in-memory implementation of the storage contracts.
//
// # Design
//
// Phase 4 of the implementation plan fills this package. It exists for tests,
// the simulator and single-process deployments; it loses state on restart.
// Until Phase 4 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 4: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package inmem
