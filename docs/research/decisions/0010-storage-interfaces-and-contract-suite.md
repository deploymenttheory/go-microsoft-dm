# 0010: Storage interfaces and the contract suite

## Context

Enrollment produces state the rest of the system reads: which certificate belongs to which
device, what the device said about itself, and what the authority has issued and revoked.
The session engine (Phase 5) must resolve a client certificate to an enrollment, the renewal
and revocation packages (Phase 9) must list certificates, and the server (Phase 6) must
implement all of it over SQL without re-deciding the semantics. Windows adds one trap: the
`HWDevID` a client reports is a hardware identifier, and the same hardware legitimately
enrolls more than once (a wipe, a second user, a re-image), so a store keyed on it would
either refuse legitimate enrollments or silently overwrite them (research pitfall "Duplicate
HWDevID").

## Decision

`storage` defines the contracts, shared errors (`ErrNotFound`, `ErrConflict`, `ErrInvalid`)
and value types; `storage/inmem` implements them in memory; `storage/storagetest.RunAll` is
the suite every backend must pass, and `storagetest.Failing` wraps a store to inject errors
by method name for callers' failure tests.

`EnrollmentStore` keeps one `Enrollment` per issued identity. The key is the certificate
serial (decimal string); the thumbprint (SHA-1 uppercase hex) is a unique secondary key; the
MDE2 `DeviceID` indexes the current enrollment; `HWDevID` indexes history. `Create` stores in
`StateActive` and, in the same operation, marks the device's previous active enrollment
`StateSuperseded`; `Get` returns the active enrollment for a DeviceID; `GetBySerial` and
`GetByThumbprint` return any state; `ListByHWDevID` returns every enrollment the hardware ever
made, oldest first; `SetState` moves between `active`, `superseded` and `unenrolled` and keeps
the DeviceID index consistent (reactivating supersedes whatever is current); `TouchLastSeen`
keeps the latest time; `List` filters by state, type, UPN and DeviceID and pages with an
offset cursor ordered by `EnrolledAt` then serial. Records carry the full typed
`AdditionalContext`, the UPN and the enrollment type, and are returned as copies.

`CertificateStore` keeps every issued `Certificate` (serial, thumbprint, subject, DeviceID,
validity, DER, revocation flag and time) with `PutCertificate` (conflict on serial or
thumbprint), `Certificate`, `CertificateByThumbprint`, `Revoke` (idempotent, first time kept)
and `ListCertificates` filtered by DeviceID, revocation and expiry. `Store` is both.

`storage.Recorder` implements `enroll.Recorder`: it writes the certificate, looks up the
`HWDevID` history, creates the enrollment, and when the hardware was previously enrolled
under a different DeviceID hands a `Conflict` (the identifier, the earlier enrollments and
the new one) to the configured `ConflictSink`. The enrollment is recorded either way; the
sink's error, if any, aborts. `storage.Unenroll` sets the state and revokes in one call.

## Rationale

Keying on the certificate makes the session engine's lookup a single read by the identity
the client actually presents, and makes a re-enrollment a new record instead of a mutation,
so the history of a device survives and a superseded certificate can still be recognised and
told apart. HWDevID as an index rather than a key is the whole answer to the pitfall: the
conflict is visible to an operator through the sink, the client is not turned away, and the
policy of what to do about it stays outside the library. Contracts plus a suite plus an
in-memory backend is the pattern go-apple-dm proved: the SQL backends in Phase 6 run the
same suite and cannot drift.

## Constraints

Command queues, results, device facts and the user channel are Phase 5 additions to these
contracts. The offset cursor of the in-memory backend is an implementation detail; SQL
backends may use keyset cursors as long as the suite's ordering holds. Certificates are
stored as issued and never re-parsed by the store. Nothing here decides retention.

## Verification

`storage/storagetest` covers creation and every lookup, copy semantics in both directions,
`ErrNotFound` and `ErrInvalid` for each method, serial and thumbprint conflicts, supersession
on re-enrollment, HWDevID history, every state transition including reactivation and
unenrolling a superseded record, last-seen monotonicity, list filters and paging past the end,
certificate storage, idempotent revocation, certificate filters and paging, and concurrent
creation from 32 goroutines. `storage/inmem` runs the suite. `storage` tests drive the
Recorder through a first enrollment, a re-enrollment of the same device (no conflict), a
different device on the same hardware (recorded and reported), no HWDevID, no sink, every
store failure through `Failing`, a failing sink, and `Unenroll`. `simulator` enrolls two
devices with one HWDevID against the in-memory server and sees the conflict event.

## References

- [storage](../../../storage), [storage/inmem](../../../storage/inmem), [storage/storagetest](../../../storage/storagetest)
- [Research store](../../research.md), section 7 (pitfall "Duplicate HWDevID"), section 1.8 (AdditionalContext catalogue)
- Microsoft, [MS-MDE2] 3.4.4.1.1.1 (DeviceID, HWDevID): <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692>

Reference source identifiers and paths (relative to the named project):

- `deploymenttheory/go-apple-dm`, `storage/storage.go` (contracts and shared errors), `storage/storagetest/{suite,failing}.go`, `storage/inmem/inmem.go`
- `fleetdm/fleet`, `server/datastore/mysql/microsoft_mdm.go` (enrollments keyed on `mdm_device_id` with `host_uuid`; the HWDevID join this record avoids)
