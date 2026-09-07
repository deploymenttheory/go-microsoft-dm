// Package storage defines the domain storage contracts the library and server implement.
//
// # Design
//
// Phase 4 of the implementation plan adds the enrollment and certificate
// contracts; Phase 5 adds the command queue, results and device facts.
// Enrollments are keyed on the issued certificate and the MDE2 DeviceID,
// never HWDevID. Until Phase 4 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 4: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package storage
