// Package ca defines the certificate authority interface and the in-memory and file-backed issuers.
//
// # Design
//
// Phase 4 of the implementation plan fills this package. Issuance sets
// NotBefore to now and never back-dates; validity and renewal windows are
// configuration. The interface is what pki/wstep and pki/scep call. Until
// Phase 4 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 4: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package ca
