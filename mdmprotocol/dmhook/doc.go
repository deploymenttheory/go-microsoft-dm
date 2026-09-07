// Package dmhook defines the hook points a deployment wraps around enrollment and session operations.
//
// # Design
//
// Phase 5 of the implementation plan fills this package. Hooks observe and
// can veto; they do not own persistence or transport. Until Phase 5 the
// package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 5: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package dmhook
