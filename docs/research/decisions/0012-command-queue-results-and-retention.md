# 0012: Command queue, results and retention

## Context

The session engine delivers commands a caller queued and files what the device answered.
Fleet's experience at scale named the traps: ordering the queue by a second-resolution
timestamp delivered same-second commands in random order; storing the full SyncML response
envelope per session was the top database writer at ten thousand hosts; and no retention plan
meant the tables grew without bound. The queue contract has to make those mistakes impossible
rather than merely avoidable.

## Decision

`mdmprotocol/mdm.CommandQueue` persists commands per device and is implemented by the storage
tier (`storage/inmem.Queue` now, SQL in Phase 6); `storage/storagetest.RunQueueSuite` is the
contract every backend passes. A `Command` is typed (Add, Replace, Delete, Get, Exec, Atomic,
Sequence built by the package's builders, which refuse what the Windows client refuses) and
scoped device or user by its LocURIs. `Enqueue` assigns a monotonic per-device sequence and a
stable id, refuses a duplicate id, and stores the command `pending`. `Deliverable` returns
pending and still-unacknowledged sent commands in sequence order, never in wall-clock order.
`MarkSent` records a delivery; `StoreResult` files the device's answer and moves the command to
`acknowledged` or `failed` by the Status class.

A `Result` is per command, never the raw envelope: the Status code, the `msft:originalerror`
HRESULT on a failure, the Results items of a successful Get (reassembled when chunked), and the
per-member statuses of an Atomic or Sequence as `ChildResult` entries. The raw message is not
stored at all by the contract; a deployment that wants it keeps a bounded, opt-in debug ring
above the queue. `Prune` deletes terminal commands completed before a cutoff, so retention is a
first-class operation rather than an afterthought, and `Cancel` withdraws non-terminal commands.

The engine feeds the queue; it never invents commands beyond the one-time first-session reads.
The desired-state discipline the research store calls for (never re-send unchanged
configuration each session) is the caller's to apply through this contract: the queue delivers
what was enqueued and reports what was acknowledged, which is the information a caller needs to
diff desired against acknowledged state.

## Rationale

A monotonic sequence assigned at enqueue is the direct fix for the same-second ordering bug and
gives a total, stable delivery order. Per-command results with typed children are what a
console actually reads and are a fraction of the size of the envelopes, which is the fix for the
top-writer bug; keeping the raw message only behind an opt-in ring bounds that cost. Making
`Prune` part of the contract means every backend has a retention story from the first commit.
A contract suite plus an in-memory backend is the pattern the storage tier already uses, so the
SQL backend cannot drift from the semantics the engine relies on.

## Constraints

The in-memory queue keeps everything in memory and loses it on restart; it is for tests, the
simulator and single-process use. The sequence is per device, not global. A command's result is
overwritten if the same command id is somehow answered twice; ids are unique per device by
construction. The debug ring of raw messages is not part of this decision; it is a server
concern (Phase 6).

## Verification

`storage/storagetest.RunQueueSuite` covers enqueue with an assigned sequence and id, duplicate
ids, deliverable ordering, sent then acknowledged and failed transitions, not-found on unknown
commands and devices, list filters (state, internal, scope) with paging, cancel of non-terminal
commands, prune of terminal commands before a cutoff, and concurrent enqueue from many
goroutines with no lost or duplicated sequence. `storage/inmem` runs the suite. The engine tests
in `mdmprotocol/mdm` exercise the queue through whole sessions: a Get result filed, an Atomic's
children recorded, and delivery split across messages by a byte budget.

## References

- [mdmprotocol/mdm](../../../mdmprotocol/mdm) (`queue.go`, `command.go`, `results.go`), [storage/inmem](../../../storage/inmem) (`queue.go`), [storage/storagetest](../../../storage/storagetest) (`queue.go`)
- [Research store](../../research.md), section 7 (pitfalls "Aggressive polling", "Storing the full SyncML response", "Same-second commands")
- Fleet issues #43773, #44188 and #49616 (the three pitfalls this contract designs against)
- Microsoft, [MS-MDM] 2.2.6.1 (Status) and 2.2.7.8 (Results): <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mdm/33769a92-ac31-47ef-ae7b-dc8501f7104f>
