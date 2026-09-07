// Package storage defines the domain storage contracts the library and server implement.
//
// # Design
//
// Two contracts arrive with enrollment. EnrollmentStore keeps one record per
// issued identity: an Enrollment is keyed by the serial of the certificate
// the device authenticates with, indexed by the MDE2 DeviceID (the current
// enrollment for a device) and by HWDevID (every enrollment a piece of
// hardware ever made). A device that enrolls again gets a new record and its
// previous active record becomes superseded; nothing is overwritten, so the
// history of a device survives re-enrollment. CertificateStore keeps every
// certificate the authority issued with its thumbprint and revocation flag,
// so the session engine can resolve a client certificate and the revocation
// package can publish what it must.
//
// HWDevID is never a key. It is a hardware identifier the client reports and
// the same hardware may legitimately enroll many times (a wipe, a new user).
// Recorder, the enroll.Recorder over these contracts, writes the certificate
// and the enrollment and, when the HWDevID already belongs to a different
// DeviceID, reports a Conflict to the configured sink rather than refusing
// or silently replacing (research pitfall "Duplicate HWDevID").
//
// The package defines contracts and the shared error values; storage/inmem
// implements them in memory and storage/storagetest holds the suite every
// backend must pass. Command queues, results and device facts arrive with
// the session engine in Phase 5.
//
// # References
//
//   - Decision record 0010: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0010-storage-interfaces-and-contract-suite.md
//   - Decision record 0009: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0009-wstep-ca-and-csr-handling.md
package storage
