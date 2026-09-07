// Package entra validates Entra tokens and fetches discovery documents for the federated enrollment paths.
//
// # Design
//
// Phase 10 of the implementation plan fills this package. It validates v1.0
// and v2.0 tokens against a tenant allow-list with a refreshing key cache.
// Until Phase 10 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 10: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package entra
