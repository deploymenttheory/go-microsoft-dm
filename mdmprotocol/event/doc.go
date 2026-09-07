// Package event is the typed event bus every protocol domain publishes to.
//
// # Design
//
// Phase 5 of the implementation plan fills this package. It depends only on
// the protocol core so every domain can publish; projections that know every
// domain live in server/eventsink. Until Phase 5 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 5: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package event
