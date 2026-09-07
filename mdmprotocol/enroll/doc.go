// Package enroll implements MS-MDE2 discovery, XCEP policy and WSTEP enrollment messages and handlers.
//
// # Design
//
// Phase 4 of the implementation plan fills this package with the OnPremise
// authentication policy; Phase 9 adds ROBO renewal; Phase 10 adds the
// Federated and Certificate policies. Handlers are transport-neutral and take
// a CA from pki/ca. Until Phase 4 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 4: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - Microsoft MS-MDE2: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692
package enroll
