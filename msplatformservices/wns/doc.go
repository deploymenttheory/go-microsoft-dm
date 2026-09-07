// Package wns sends raw WNS push notifications to enrolled devices.
//
// # Design
//
// Phase 8 of the implementation plan fills this package: two token sources
// (Partner Center and Entra), a raw-notification sender with the MDM headers,
// and response classification. Poll policy lives in the provisioning
// document, not here. Until Phase 8 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 8: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - Microsoft: https://learn.microsoft.com/en-us/windows/client-management/push-notification-windows-mdm
package wns
