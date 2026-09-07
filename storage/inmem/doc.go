// Package inmem is the in-memory implementation of the storage contracts.
//
// # Design
//
// Store keeps enrollments and certificates in maps under one mutex and
// returns copies, so callers never share memory with the store. It exists
// for tests, the simulator and single-process deployments and loses state
// on restart. Listing sorts on every call; the SQL backends in the server
// module index instead. It passes storagetest.RunAll, which is the
// definition of correct behaviour for every backend.
//
// # References
//
//   - Decision record 0010: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0010-storage-interfaces-and-contract-suite.md
package inmem
