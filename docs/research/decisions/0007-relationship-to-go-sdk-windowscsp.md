# 0007: Relationship to go-sdk-windowscsp

## Context

`deploymenttheory/go-sdk-windowscsp` (same organisation, MIT) already parses DDF v2 into typed
Go and generates a package per CSP with `Get`, `Add`, `Replace`, `Delete` and `Exec` methods,
URI functions, enum types, a SyncML fragment builder and an in-memory fake tree. The research
store recorded on 2026-09-06 that it is proof the approach works and a reference, and deferred
the dependency question to this phase.

## Decision

go-microsoft-dm does not depend on go-sdk-windowscsp and does not absorb its code. The
generator here is written against this project's runtime model and its needs: a registry for
server-side lookup, applicability resolution against a device build, command validation, and
a diff between drops. The SDK's generated packages are shaped for a client that calls a CSP
tree (a service type per CSP with typed accessors over a `client.Client`); this project's are
shaped for a server that validates and builds commands (constants, URI functions, node tables).

What was learned from the SDK and applied: parse the DDF elements in no namespace and the
MSFT extensions by namespace; represent dynamic segments as `{Title}` placeholders and derive
URI functions with one parameter per dynamic segment; keep the model's `Dynamic`, `Leaf` and
format helpers small; and lower-case the CSP name for the package directory. What was done
differently: the runtime model is hand-written and the data generated, rather than generating
accessors per node; ENUM values are constants rather than a Go type per node, because the
1,202 ENUM nodes would produce 1,202 types; every node keeps its full DDF facts (dependency
groups, changed allowed values, GP mapping, conflict resolution, deprecation) because the
validation and diff tools use them; and the SDK's fake CSP tree (`windowscsp/clienttest`) is
a reference for the Phase 5 simulator, not an import.

Revisit criteria: if the SDK grows a server-side registry and validation over the same DDF
model, or if this project needs a Windows-side client to drive `mdmlocalmanagement.dll` in
the guest harness (Phase 7), where the SDK's `client` package and the organisation's Win32
bindings are the natural fit. Either would be a new record.

## Rationale

Two generators over one input is a small duplication (the parser is under 400 lines) and it
buys independence of release cadence, module size and API shape. A library that embeds a
server-side schema should not pull a client SDK's dependencies. Keeping the SDK as a reference
still gives this project a second reading of every DDF quirk.

## Constraints

Bundle facts are compared against the SDK's parser when a new drop arrives; a disagreement is
investigated, not averaged. Neither project's package names are stable for the other.

## Verification

`go.mod` carries no `go-sdk-windowscsp` requirement; `internal/layout` and `go list -m all`
show none. Decision record 0003 lists the SDK among read-only references.

## References

- Decision record 0003 (reference projects and dependency policy)
- Decision record 0006 (schema generator over DDF v2)
- [Research store](../../research.md), section 4 ("Same-organisation building blocks") and section 10, open question 7
- <https://github.com/deploymenttheory/go-sdk-windowscsp>

Reference source identifiers and paths (relative to the named project):

- `deploymenttheory/go-sdk-windowscsp`, `internal/ddf/parse.go`, `internal/ddf/model.go`, `internal/codegen/generator.go`, `windowscsp/csp/dmclient/dmclient_uris.go`, `dmclient_crud.go`, `dmclient_enums.go`, `windowscsp/clienttest/`
