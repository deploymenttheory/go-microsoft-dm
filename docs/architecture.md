# Architecture

The repository contains a reusable protocol library and a reference server. This guide describes
the implemented design and nothing that is still planned; the [implementation plan](implementation_plan.md)
holds what comes next and [design decisions](research/decisions/README.md) hold the contracts
and rationale.

## Modules and dependency direction

| Module | Contents today | Dependencies |
|---|---|---|
| `github.com/deploymenttheory/go-microsoft-dm` | Foundation packages, the DDF bundle verifier, placeholder packages for every later tier | OpenTelemetry API modules; exact versions in the root `go.mod` |
| `github.com/deploymenttheory/go-microsoft-dm/server` | Placeholder packages and the `dmserver` and `dmctl` stubs | The root module |

Both modules use Go 1.27. `go.work` joins them for local development and the server's `replace`
resolves the library from the checkout when it is built alone. Library code and tests cannot
import the server module. `internal/layout` loads both modules and checks the tier order,
populated tiers, directory-level cycles and the module boundary, in production and test imports.

| Tier | Paths | Responsibility | State |
|---|---|---|---|
| Foundation | `clock`, `paging`, `secrets`, `telemetry`, `state`, `ratelimit`, `testpki` | Injected time, cursor paging, redacting secrets, the OpenTelemetry seam, transactional expiring state, GCRA quotas, test PKI | Implemented (decision record 0004) |
| Schema | `schema/csp`, `schema/csp/<name>`, `schema/policy/<area>`, `schema/registry`, `schema/support`, `schema/validation`, `internal/schemagen`, `cmd/ddfgen` | Runtime model, generated node tables and URI constants for 57 CSPs and 261 Policy areas, the registry over all 400 trees, build applicability and command validation | Implemented (decision records 0006 and 0007) |
| Protocol | `mdmprotocol/{syncml,soap,wapprov,enroll,mdm,windc,event,dmhook}` | SyncML codec, SOAP types, provisioning document, enrollment, session engine, WinDC, events, hooks | `syncml` implemented (decision record 0005); the rest are placeholders (Phases 4, 5, 12) |
| PKI | `pki/{ca,wstep,xcep,scep,attestation,revocation}` | CA interface, CSR parsing, XCEP policy, SCEP, attestation, revocation | Placeholders (Phases 4, 9, 11) |
| Platform services | `msplatformservices/{wns,entra,graph}` | WNS push, Entra token validation, Graph | Placeholders (Phases 8, 10) |
| Storage | `storage`, `storage/inmem`, `storage/storagetest` | Domain contracts, in-memory backend, contract suite | Placeholders (Phase 4) |
| Client | `simulator` | A Windows MDM client in software | Placeholder (Phases 4, 5) |
| Server | `server/{sqlstore,service,httpapi,pushnotify,windcsync,adminauth,audit,eventsink}` | Persistence, orchestration, transport, administration | Placeholders (Phases 6, 8, 12, 15) |
| App | `server/cmd/{dmserver,dmctl}`, `server/internal/app`, `server/e2e` | Composition, CLI, scenarios | Stubs (Phase 6) |

## Pinned references

`third_party/ddf/DDFv2Feb2026.zip` is Microsoft's February 2026 DDF v2 bundle, checked in with a
manifest recording its URL, SHA-256, size, HTTP `Last-Modified`, ETag, top folder and file
count. `internal/schemagen` verifies the bundle against the manifest and generates the schema
tier from it; `cmd/ddfgen verify` and `make verify` check both, and
`TestPinnedBundleMatchesManifest` runs the pin check in the unit suite. Later drops are added
beside it, never in its place.

`third_party/specs/MANIFEST.json` records the OMA DM 1.2.1, SyncML Common 1.2.2, MS-MDE2,
MS-MDM, MS-XCEP, MS-WSTEP and WBXML documents with their SHA-256; `make specs` downloads and
verifies them. `make refs` clones the read-only reference implementations. Decision record 0002
records the revisions in force and when to re-check them.

## Foundation packages

`clock` injects time. `paging` bounds page sizes. `secrets` carries credentials that redact
themselves through every formatting path and resolves them from memory, the environment, a
rooted directory or a chain. `telemetry` accepts explicit OpenTelemetry providers, defaults to
no-ops, measures outbound HTTP with bounded attributes and never records a URL path or query
(a WNS channel URI is a credential; an OMA DM session URL carries the device identifier).
`state` gives atomic, expiring byte records for nonces, challenges and quotas. `ratelimit`
applies GCRA quotas over `state` with explicit proxy trust. `testpki` issues test identities,
TLS server certificates and certificate requests, including the PrintableString subject the
Windows enrollment client sends, which the standard library rejects.

## CSP schema

`cmd/ddfgen generate` parses the pinned DDF v2 bundle with `internal/schemagen` and writes one
package per configuration service provider under `schema/csp` and per Policy area under
`schema/policy`: a `csp.Tree` literal with every node's format, access, applicability, allowed
values, naming rule, dependencies and behaviour flags, a URI constant per static node and a
function per node below a dynamic segment, and a constant per ENUM and Flag value.
`schema/registry` joins the 400 trees and resolves any concrete URI to its node with the
dynamic segments captured. `schema/support` decides whether a node applies to a device build,
knowing that 24H2, 25H2 and 26H2 share a servicing branch and which DDF values are sentinels.
`schema/validation` checks a command's node, verb, format and value before it is queued.
`make verify` fails when regeneration would change a byte or drop a locked identifier;
`make ddf-diff` reviews the next Microsoft drop node by node.

## SyncML codec

`mdmprotocol/syncml` is the typed SyncML 1.2 codec for the OMA DM subset Windows uses: one Go
type per element, commands kept in document order behind a sealed interface, a DTD-ordered
writer that emits the 1.2 namespace (or 1.1 when answering a 1.1 client) and never CDATA, and
a token parser that accepts what Microsoft's samples and the real client send while refusing
the OMA commands Windows does not implement. `Validate` applies the session rules separately
from decoding, including the Atomic rules the client enforces with 500 and 507. The package
also carries the OMA DM Security MD5 digest, large-object chunking and reassembly, and the
"Next Message" and abort response shapes. It performs no I/O; the session engine that uses it
arrives in Phase 5.

## Checks

`make ci` runs lint, the DDF verification, both modules' tests with race detection, the fuzz
smoke run and the coverage gate (95% overall and per non-exempt package). The GitHub workflows
run the same targets on pull requests and on `main`. Release-please manages versions for both
modules from Conventional Commits.
