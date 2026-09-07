// Package graph calls the two Microsoft Graph operations a third-party MDM needs.
//
// # Design
//
// Phase 10 of the implementation plan fills this package: device compliance
// PATCH and the mobilityManagementPolicy setup helper. The approved-MDM-app
// constraint is recorded, not worked around. Until Phase 10 the package is
// empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 10: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package graph
