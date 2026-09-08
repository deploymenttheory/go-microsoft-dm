# 0016: dmctl structure

## Context

An operator needs to see what the reference server knows and to queue commands without writing
Go. Phase 6 provides `dmctl`. There is no admin API yet (adminauth is Phase 15), so the tool
works against the store directly.

## Decision

`dmctl` opens the same `DM_STORE`/`DM_DSN` as `dmserver` and operates on the store: `enrollments`
lists enrolled devices; `facts <deviceID>` shows the device's reported facts; `commands
<deviceID>` lists its queued commands with state and attempts; `results <deviceID>` shows each
command's status and result items; `events <deviceID>` prints the event log; and `queue <deviceID>
<file>` enqueues commands from a plain-text file, one per line, in an operator-friendly grammar
(`replace <uri> <value> [format]`, `add ...`, `get <uri>`, `delete <uri>`, `exec <uri> [value]`),
built through the same `mdm` command builders the engine uses, so the Atomic and scope rules and,
when enabled, schema validation apply. It refuses the `memory` store, which is ephemeral to a
running `dmserver` and not shareable.

## Rationale

Operating on the store directly is the least machinery for the phase and needs no authentication
model, which is a later decision. Reusing the `mdm` builders means a command typed at the CLI is
validated exactly as one queued in code, so an invalid batch cannot be created. The line grammar
maps one-to-one onto the builders and keeps the common case (a policy Replace) a single line.

## Constraints

`dmctl` shares the database with a running server, so it is a local-operator tool, not a remote
client; a remote admin API arrives with adminauth in Phase 15. It cannot target the in-memory
store. It performs no authorization; anyone who can read `DM_DSN` can queue commands.

## Verification

`dmctl` builds and vets; its command parser and store operations are exercised through the shared
`sqlstore` and `mdm` packages, which have their own suites. The binary is coverage-exempt as
wiring.

## References

- [server/cmd/dmctl](../../../server/cmd/dmctl)
- [Decision record 0012](0012-command-queue-results-and-retention.md), [decision record 0015](0015-reference-server-roles-and-configuration.md)
