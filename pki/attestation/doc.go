// Package attestation verifies the TPM attestation material MS-MDE2 carries in AdditionalContext.
//
// # Design
//
// Phase 11 of the implementation plan fills this package. It classifies
// enrollments as attested, unattested or unverifiable and exposes an
// admission hook; it does not decide policy. Until Phase 11 the package is
// empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 11: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package attestation
