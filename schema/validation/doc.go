// Package validation validates OMA-URIs, scopes, formats and values against the generated CSP schema.
//
// # Design
//
// Phase 3 of the implementation plan fills this package. Validation covers
// URI existence, scope prefix, leaf format, allowed values, access type per
// verb and AtomicRequired. It does not queue or send commands; the session
// engine and the command builders call it. Until Phase 3 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 3: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package validation
