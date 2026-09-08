// Package wns sends raw WNS push notifications to enrolled devices.
//
// # Design
//
// Token sources support Partner Center and Entra. The raw sender validates
// Microsoft channel URLs, forbids redirects, refreshes a rejected token once,
// and offers bounded retries for throttling and server failures. WNS acceptance
// does not prove device check-in. Poll policy belongs to provisioning and the
// reference server; no background workers or device state live in this package.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 8: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - Microsoft: https://learn.microsoft.com/en-us/windows/client-management/push-notification-windows-mdm
package wns
