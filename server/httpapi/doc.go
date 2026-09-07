// Package httpapi exposes the SOAP and SyncML endpoints over HTTP.
//
// # Design
//
// Phase 6 of the implementation plan fills this package: the discovery,
// policy, enrollment and management endpoints, TLS termination or
// trusted-proxy certificate forwarding, and Content-Length on every
// enrollment response. Until Phase 6 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 6: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
package httpapi
