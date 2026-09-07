// Package validation checks a command against the generated CSP schema
// before it is queued: the node exists, the verb is permitted, the scope
// matches, the format is right and the value is allowed.
//
// # Design
//
// Check takes a registry, a verb, a URI and an optional value and returns
// every rule the command breaks. ENUM, Flag, Range and RegEx allowed values
// are evaluated; ADMX payloads are checked structurally (enabled, disabled or
// data elements, as the Learn "Understanding ADMX policies" page defines
// them) because the ADMX files themselves are not in the bundle; XSD, SDDL
// and JSON constraints are reported as present but not evaluated. A node
// under an AtomicRequired ancestor is flagged so the caller wraps it.
//
// The package validates shape, not applicability: whether a node exists on
// the device's build is schema/support's question, and whether a user-scope
// node may be sent now is the session engine's.
//
// # References
//
//   - Decision record 0006: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0006-schema-generator-over-ddf-v2.md
//   - Microsoft, Understanding ADMX policies: https://learn.microsoft.com/en-us/windows/client-management/understanding-admx-backed-policies
//   - Microsoft, Policy CSP: https://learn.microsoft.com/en-us/windows/client-management/mdm/policy-configuration-service-provider
package validation
