// Package mdm runs the OMA DM management session: packages 1 to 4, authentication, the command queue contract and the response builder.
//
// # Design
//
// Phase 5 of the implementation plan fills this package. The engine
// classifies requests, authenticates with mutual TLS or syncml:auth-md5,
// gates user-scoped commands on LoginStatus, and stores per-command results
// rather than raw envelopes. Persistence is behind storage contracts. Until
// Phase 5 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 5: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - Microsoft MS-MDM: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mdm/33769a92-ac31-47ef-ae7b-dc8501f7104f
package mdm
