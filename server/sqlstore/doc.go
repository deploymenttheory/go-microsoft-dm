// Package sqlstore persists the durable MDM state in a SQL database.
//
// # Design
//
// One Store over database/sql implements the library's durable contracts:
// storage.Store (enrollments and issued certificates), storage.CredentialStore
// (the OMA DM session credential per device), and mdm.CommandQueue (the
// per-device command queue with per-command results). It also keeps a device
// facts table and an append-only event log the service writes through.
//
// Three dialects are supported: SQLite (the pure-Go modernc.org/sqlite driver,
// the default and the one the tests exercise everywhere), PostgreSQL and
// MySQL. A small Dialect abstraction covers the three differences that matter:
// the parameter placeholder, the upsert clause and the column types. Times are
// stored as Unix nanoseconds and booleans as 0/1 so every driver round-trips
// them identically. Certificate DER and JSON blobs are stored verbatim.
//
// OMA DM session state is not persisted here: a session is a short-lived,
// single-node construct holding engine-internal state, so the reference server
// keeps it in mdm.MemorySessions. Multi-node session sharing is a later
// concern (decision record 0014). Durable device facts are captured by the
// service's hooks and written to the facts table, independent of session
// lifetime.
//
// # References
//
//   - Decision record 0014: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0014-sql-storage-backends.md
//   - storage contracts: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/storage/storage.go
package sqlstore
