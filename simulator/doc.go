// Package simulator is a Windows MDM client in software for tests and end-to-end scenarios.
//
// # Design
//
// Phase 4 of the implementation plan adds enrollment; Phase 5 adds the
// package 1 to 4 session client with a fake CSP tree. It is the unit-test
// client; a real Windows guest on guestweave is the conformance client (Phase
// 7). Until Phase 4 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phases 4 and 5: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package simulator
