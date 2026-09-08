// Package pushnotify tracks enrollment-bound channels and requests WNS wakes.
//
// # Design
//
// A queue observer persists successful channel/status reads. Service records
// WNS outcomes, suppresses dead or aged channels, and offers explicit
// missed-check-in checks. Callers schedule checks and request wakes; no hidden
// worker changes polling or treats WNS acceptance as device acknowledgement.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 8: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package pushnotify
