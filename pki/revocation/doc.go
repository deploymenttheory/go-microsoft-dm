// Package revocation offers optional CRL and OCSP publication for issued certificates.
//
// # Design
//
// Phase 9 of the implementation plan fills this package. It is optional in
// every deployment; the enrollment path does not require it. Until Phase 9
// the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 9: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package revocation
