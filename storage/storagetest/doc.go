// Package storagetest is the contract suite every storage backend must pass.
//
// # Design
//
// RunAll takes a factory that returns a fresh store and runs the enrollment
// and certificate suites against it, then a concurrency suite. The suite is
// the specification of the contracts in package storage: keying by serial,
// lookup by DeviceID and thumbprint, supersession on re-enrollment,
// HWDevID history, state transitions, revocation, paging and the error
// values. SQL backends in the server module run the same suite.
//
// Failing wraps a store and returns a configured error from any named
// method, so callers can test how they handle a backend failure.
//
// # References
//
//   - Decision record 0010: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0010-storage-interfaces-and-contract-suite.md
package storagetest
