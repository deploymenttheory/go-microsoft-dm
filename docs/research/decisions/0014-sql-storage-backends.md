# 0014: SQL storage backends

## Context

The reference server needs durable state so a device stays enrolled and its command history
survives a restart, and so Phase 7 can point a real Windows guest at something persistent. The
library defines the storage contracts (`storage.Store`, `storage.CredentialStore`,
`mdm.CommandQueue`) and an in-memory backend; Phase 6 adds SQL backends that pass the same
contract suites. Fleet's schema at scale named the traps the tables must avoid (research section
4 and 7): keying on the wrong identifier, storing whole SyncML envelopes, and no retention plan.

## Decision

`server/sqlstore` implements the durable contracts over `database/sql` for three dialects:
SQLite (the pure-Go `modernc.org/sqlite` driver, the default and the one every test exercises),
PostgreSQL (`pgx` stdlib) and MySQL (`go-sql-driver/mysql`), all pure Go so `make test` needs no
cgo or containers. A small `dialect` value captures the only three differences that matter: the
parameter placeholder (`?` for SQLite and MySQL, `$n` for PostgreSQL, rewritten by `rebind`), the
upsert clause (`ON CONFLICT` versus `ON DUPLICATE KEY`), and the column types for blobs and the
auto-increment id. Times are stored as Unix nanoseconds and booleans as 0/1 so every driver
round-trips them identically; certificate DER and the JSON-encoded AdditionalContext, command
body and result are stored verbatim (the body as its SyncML fragment, decoded with
`syncml.DecodeCommand`).

The tables are keyed as decision records 0009 and 0010 decided: `enrollments` on the certificate
serial with the thumbprint unique and `device_id` and `hwdevid` indexed (HWDevID non-unique, so
the same hardware can enrol many times); `certificates` on the serial; `mdm_credentials` on the
device id; `commands` on `(device_id, id)` with a `(device_id, seq)` index for the monotonic
delivery order, the result stored per command as JSON and never the raw envelope; `device_facts`
on the device id; and an append-only `events` log. `Prune` deletes terminal commands before a
cutoff, so retention is a first-class operation. Because `storage.Store` and `mdm.CommandQueue`
both define `Get` and `List` with different signatures, the queue is a separate `Queue` type over
the same connection, reached through `Store.Queue()`.

Basic credentials use the salted verifier from `mdm.HashBasicCredential` in `credential_hash`.
On `Open`, a transaction converts legacy Basic rows with a nonempty username or password,
then clears `basic_username` and `basic_password`. The old columns remain for schema
compatibility and current writes leave both empty. Reads return only the verifier. Migration
is idempotent, preserves MD5 credentials, and fails startup with rollback if conversion fails.
The all-empty legacy credential is not activated automatically.

Stop all old server instances before upgrading and allow one new instance to finish migration
before starting others. Do not roll back to code that expects plaintext Basic fields; restore
from a protected pre-upgrade backup or reprovision credentials if rollback is necessary.
Clearing live rows does not erase old database pages, WAL, snapshots or backups; handle those
under the deployment's existing secret-retention policy. Migration cost grows with the number
of Basic accounts because each verifier requires a password derivation. The reference server
provisions MD5, so its normal credential rows need no conversion.

OMA DM session state is not persisted. A session is a short-lived, single-node construct holding
engine-internal state (the send map, the reassembler, the once-only package-1 flag), so the
reference server keeps it in `mdm.MemorySessions`. Durable device facts are captured by the
service's hooks and written to `device_facts`, independent of session lifetime, so nothing a
device reported is lost when its session ends. Sharing sessions across server replicas would need
a library-level `Session` serializer and is deferred.

## Rationale

One `database/sql` implementation with a three-line dialect abstraction is far less code than
three hand-written backends and keeps the SQL in one place, and the shared contract suite proves
all three behave identically. Pure-Go drivers keep the default test run fast and container-free;
PostgreSQL and MySQL are exercised by `make test-storage` against real databases when a DSN is
set. Storing times as integers and results as JSON sidesteps the driver-specific time and
null-handling quirks that make cross-dialect SQL brittle. Keeping sessions in memory matches the
engine's interface separation (a `SessionStore` distinct from the durable stores) and avoids
serializing state that only the engine understands; persisting the durable facts through hooks
gives the operator everything they need without it.

## Constraints

The schema is applied idempotently at open (`CREATE TABLE IF NOT EXISTS`); a versioned migration
table is a later concern. SQLite is limited to one open connection (a file database is not safe
for concurrent writers); PostgreSQL and MySQL use the pool. The reference server is single-node;
multi-node session sharing is not implemented. `make test` covers the SQLite paths; the
PostgreSQL and MySQL dialect branches (placeholders, upsert, types) are covered by the
`rebind`/`upsert` unit tests and by `make test-storage` against live databases, so their driver
error branches are not counted by the aggregate coverage gate.

## Verification

`server/sqlstore` runs `storagetest.RunAll`, `RunQueueSuite` and `RunCredentialSuite` against a
temp-file SQLite database, plus unit tests for the dialect helpers (all three placeholders and
upserts), the device-facts and event log, the paging cursors, and a closed-database pass that
exercises the driver error paths across every method. `make test-storage` runs the same contract
suites against PostgreSQL and MySQL when `TEST_POSTGRES_DSN` or `TEST_MYSQL_DSN` is set. The
`server/e2e` scenarios run against SQLite and the in-memory role.

## References

- [server/sqlstore](../../../server/sqlstore)
- [Decision record 0010](0010-storage-interfaces-and-contract-suite.md), [decision record 0009](0009-wstep-ca-and-csr-handling.md)
- [Research store](../../research.md), section 4 (Storage schema patterns), section 7 (storage pitfalls)
- `modernc.org/sqlite`, `github.com/jackc/pgx`, `github.com/go-sql-driver/mysql`

Reference source identifiers and paths (relative to the named project):

- `fleetdm/fleet`, `server/datastore/mysql/microsoft_mdm.go` (the enrollment, command and response tables this schema simplifies)
- `deploymenttheory/go-apple-dm`, `server/sqlstore` (the dialect-abstraction and contract-suite pattern mirrored here)
