// Package secrets provides named credential sources and a value type that
// redacts formatted output.
//
// # Design
//
// Secret redacts fmt, text and JSON serialization; Bytes provides explicit
// access to the value. Static, Env, Dir and Chain providers support injected
// values, normalized environment names, bounded rooted files and fallback. Dir
// uses os.Root so names cannot escape its configured directory.
//
// WNS client secrets, Entra client secrets, OMA DM MD5 credentials and PFX
// passwords all flow through this type in later phases. Redaction reduces
// accidental disclosure through formatting; it does not protect values after
// explicit extraction or encrypt memory. Sealing for persisted values arrives
// with the server module (Phase 15 of the implementation plan).
//
// # References
//
//   - Decision record 0004: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0004-go-baseline-and-foundation-contracts.md
//   - Microsoft, WNS credentials for MDM: https://learn.microsoft.com/en-us/windows/client-management/push-notification-windows-mdm
//   - OMA DM Security 1.2.1 (credentials): https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Security-V1_2_1-20080617-A.pdf
package secrets
