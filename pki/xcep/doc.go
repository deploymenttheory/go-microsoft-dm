// Package xcep builds MS-XCEP GetPolicies responses.
//
// # Design
//
// Phase 4 of the implementation plan fills this package. Windows consumes
// only minimalKeyLength, the hash OID and crypto providers, so that is what
// the builder models. Until Phase 4 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 4: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - Microsoft MS-XCEP: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-xcep/08ec4475-32c2-457d-8c27-5a176660a210
package xcep
