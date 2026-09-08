// Package app composes the reference server from DM_* configuration.
//
// # Design
//
// Config is read from the environment (Load) or set directly. New opens the
// SQL store (sqlite, postgres, mysql, or an ephemeral in-memory sqlite for the
// "memory" role), builds or loads the enrollment CA, composes the service and
// returns an App holding the HTTP handler and a Close. The command binaries
// (dmserver, dmctl) are thin wrappers over this package.
//
// It performs no listening or TLS itself; dmserver owns the socket.
//
// # References
//
//   - Decision record 0015: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0015-reference-server-roles-and-configuration.md
package app
