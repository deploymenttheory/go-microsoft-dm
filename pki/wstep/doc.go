// Package wstep parses enrollment CSRs and applies issuance policy for MS-WSTEP.
//
// # Design
//
// Phase 4 of the implementation plan fills this package, including a narrow
// PrintableString relaxation for the subjects Windows sends instead of a
// vendored parser. Phase 9 adds the PKCS#7 renewal token. Until Phase 4 the
// package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 4: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - Microsoft MS-WSTEP: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-wstep/4766a85d-0d18-4fa1-a51f-e5cb98b752ea
package wstep
