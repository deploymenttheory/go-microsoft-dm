# 0001: Library-first architecture with a generated schema core and WinDC as an extension of MDM

## Context

Applications need reusable Windows device management protocol components (MS-MDE2 enrollment,
OMA DM over SyncML, the CSP schema, WNS push, Windows declared configuration) and an example of
how to assemble them into a server. The protocols are published by Microsoft and the Open
Mobile Alliance under their own terms, which the project must be able to implement.

## Decision

The root Go module provides protocol handling, generated schema types, certificate services,
Microsoft service clients, storage contracts, in-memory implementations and a simulator. The
`server` module depends on the library and supplies SQL backends, orchestration, HTTP adapters
and administrative tools. Library code and tests do not import the server module.

Packages sit in ordered tiers and import only their own tier or lower: foundation (`clock`,
`paging`, `secrets`, `telemetry`, `state`, `ratelimit`, `testpki`), schema (`schema/...`),
protocol (`mdmprotocol/...`), PKI (`pki/...`), platform services (`msplatformservices/...`),
storage (`storage`, `storage/inmem`, `storage/storagetest`), client (`simulator`), server
(`server/...`) and app (`cmd/ddfgen`, `server/cmd`, `server/internal/app`, `server/e2e`,
`internal/schemagen`). Directory-level cycles are rejected. There are no upward exceptions.

Windows declared configuration is an extension of the MDM enrollment, not a separate product:
the primary MDM enrollment provisions the linked enrollment, the same MS-MDE2 flow enrolls it,
and the same SyncML session engine carries its documents and alerts. `mdmprotocol/windc` holds
the document model and the `server/windcsync` engine sits beside `server/service`, the way
declarative management sits inside the MDM enrollment in go-apple-dm.

Implementation proceeds under the copyright grant in Microsoft's Open Specifications
intellectual-property notice, which permits implementing the documented technologies and
redistributing, with or without modification, the schemas, IDLs and code samples in the
documents. Patent coverage is recorded, not assumed: as of 2026-09-07 neither the Open
Specification Promise (revised 2023-02-24) nor the Community Promise (revised 2023-01-31)
lists MS-MDE2, MS-MDM, MS-XCEP or MS-WSTEP; the Open Specification Promise does list the
standards they profile (SOAP, WSDL, WS-Addressing, WS-Trust, WS-Security including the
UsernameToken and X.509 Certificate Token profiles, WS-Federation). The remaining Microsoft
patent programmes are the only per-document lookup, through the biannual Patent and Program
Map. The OMA documents permit implementation and restrict reproduction of the documents
themselves, so `make specs` downloads them and nothing under `third_party/specs/` except the
manifest is committed.

## Rationale

Separate modules let consumers use protocol packages without the server's database drivers
and authorization dependencies. Generated types keep protocol modeling tied to the pinned DDF
bundle, which is the only machine-readable CSP description Microsoft publishes. Shared storage
contract suites define backend behavior once. Treating WinDC as an extension keeps one
enrollment identity, one session engine and one storage model, which every open Windows MDM
server that ignored WinDC would have had to retrofit.

Every open implementation of these protocols (Fleet, local-mdm, MDMatador under MIT; Mattrax
source-available) proceeds on the same copyright grant; the patent position is the same one
they carry, made explicit here so it is re-checked rather than forgotten.

## Constraints

The reference server has no management UI, inventory product or fleet policy system. Windows
10 and non-desktop editions, generic telecom OMA DM, Intune-internal services and MAM-only
enrollment are out of scope (research store, section 10). Real-client behavior is proven on a
Windows 11 guest through guestweave (Phase 7 of the implementation plan); vendor endorsement
key chains need physical hardware. The patent status of the four Microsoft documents is
re-checked at every pinned-reference re-check trigger in decision record 0002.

## Verification

`internal/layout` loads both modules and checks tier edges, populated tiers, directory-level
cycles and that no library package imports the server module in production or test code.
The DDF pin is checked by `make verify`. Later phases add the protocol conformance,
simulator and guest tests the plan names.

## References

- [internal/layout](../../../internal/layout)
- [go.mod](../../../go.mod), [server/go.mod](../../../server/go.mod), [go.work](../../../go.work)
- [Implementation plan](../../implementation_plan.md), repository layout target and Phase 12
- [Research store](../../research.md), sections 9 and 10
- Microsoft Open Specifications IP notice (on every document landing page, for example
  <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692>)
- Open Specification Promise: <https://learn.microsoft.com/en-us/openspecs/dev_center/ms-devcentlp/1c24c7c8-28b0-4ce1-a47d-95fe1ff504bc>
- Microsoft Community Promise: <https://learn.microsoft.com/en-us/openspecs/dev_center/ms-devcentlp/8b8d1b7a-a10a-4667-9558-6d9c43adf60d>
- Patent promises and the Patent and Program Map: <https://learn.microsoft.com/en-us/openspecs/dev_center/ms-devcentlp/13571077-e344-4e6f-a477-369894979798>

Reference source identifiers and paths (relative to the named project):

- `deploymenttheory/go-apple-dm`, `docs/research/decisions/0001-architecture.md`, `0039-ddm-is-an-extension-of-mdm.md`, `0044-repository-layout.md`, `internal/layout/`
- `fleetdm/fleet`, `server/mdm/microsoft/`, `server/service/microsoft_mdm.go` (single-module server with the Windows code beside the Apple code)
- `Malcolm/local-mdm`, `internal/platform/windows/` (single-module server, closest architectural sibling)
- `mattrax/Mattrax` (`rust` branch), `crates/ms-mde`, `crates/ms-mdm`, `crates/ms-ddf` (one crate per protocol, the decomposition mirrored by `mdmprotocol/`)
