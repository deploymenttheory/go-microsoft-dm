// Package service composes the enrollment and management engines over a store.
//
// # Design
//
// New builds an enroll.Service and an mdm.Service sharing one durable store
// (the SQL or in-memory backend) and a CA. It supplies the seams the engines
// leave open: an Authenticator for on-premise enrollment credentials, an
// Issuer from pki/wstep over pki/ca, a Provisioner whose CredentialSource
// mints the device's OMA DM account credentials and persists the APPSRV secret
// so the management session can verify it, a Recorder that stores the
// enrollment and logs an HWDevID conflict as an event, an mdm.Authenticator
// over the enrollment and credential stores, and Hooks that record device
// facts and route alerts and unenrollment to the store's event log.
//
// The package decides no protocol behaviour; it wires the library's engines to
// storage and configuration. It performs no HTTP itself (server/httpapi
// mounts the handlers) and holds no global state.
//
// # References
//
//   - Decision record 0015: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0015-reference-server-roles-and-configuration.md
package service
