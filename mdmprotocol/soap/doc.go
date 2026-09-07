// Package soap holds the SOAP envelope, WS-Addressing, WS-Security header and fault types MS-MDE2 uses.
//
// # Design
//
// Phase 4 of the implementation plan fills this package. It models the
// envelope shapes MS-MDE2 section 2 profiles and nothing beyond them; XCEP
// and WSTEP message bodies live in mdmprotocol/enroll. It performs no I/O.
// Until Phase 4 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 4: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - Microsoft MS-MDE2: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692
package soap
