// Package httpapi serves the MS-MDE2 and MS-MDM endpoints over HTTP.
//
// # Design
//
// Handler mounts the enrollment handler (discovery, policy, enrollment) and
// the management handler on the paths the service configured, on one
// http.ServeMux. The enrollment handler already answers the discovery GET
// probe, sets Content-Length and never chunks; the management handler does
// the same for SyncML. This package adds only the routing and an optional
// access log; TLS and trusted-proxy certificate forwarding are the server
// binary's concern.
//
// # References
//
//   - Decision record 0015: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0015-reference-server-roles-and-configuration.md
package httpapi
