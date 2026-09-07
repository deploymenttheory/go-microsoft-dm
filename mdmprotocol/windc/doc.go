// Package windc models Windows declared configuration: discovery JSON, documents, results and the 1224 alert payload.
//
// # Design
//
// Phase 12 of the implementation plan fills this package. It carries the
// document model and state vocabulary; the desired-state engine lives in
// server/windcsync. Until Phase 12 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 12: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - Microsoft: https://learn.microsoft.com/en-us/windows/client-management/declared-configuration
package windc
