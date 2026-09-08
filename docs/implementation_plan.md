# go-microsoft-dm implementation plan

Plan date: 2026-09-07. Derived from [docs/research.md](research.md) (research date 2026-09-06),
read in full and followed in section order. Every phase cites the research sections it is built
from, the decision records it must produce, the pitfalls from research section 7 it must prove
are avoided, and the open questions from research section 10 it settles or advances.

This document is the working plan. It is expected to change. When a phase completes, update its
status row and move durable design content into a decision record; do not leave design decisions
living only here. When the research store gains a newer source, the "newest wins" rule applies
to this plan too: fix the plan, cite the date.

## How to use this document

1. Read research sections 0, 9 and 10 first. They carry the target, the conclusions and the open
   questions that shape every phase.
2. Find the first phase whose status is not `done`. Read its inputs (the research sections it
   cites) before its deliverables. The research entries hold normative detail (element orders,
   status codes, header names, node paths) that this plan deliberately does not repeat in full.
3. Before writing code for a feature, read at least two reference implementations for that feature
   (research section 3 names them and their file paths), then write or amend the decision record
   the phase calls for. This rule carries over from go-apple-dm unchanged (research section 9,
   "Rules that carry over").
4. Every exported function gets a failure-path test. Every pitfall row a phase claims to address
   gets a test whose name cites the row.
5. Keep phase boundaries. A phase is done when its exit criteria pass, not when its code exists.

## Status

| Phase | Title | Status | Notes |
|---|---|---|---|
| 0 | Repository bootstrap and governance | done (2026-09-07) | Module path is `go-microsoft-dm`, matching the repository |
| 1 | Foundation packages | done (2026-09-07) | `testpki.WindowsCSR` reproduces the Fleet-documented `!` subject |
| 2 | SyncML wire model | done (2026-09-07) | `mattrax/xml` not needed; hand-written writer over `encoding/xml` tokens |
| 3 | CSP schema generated from DDF v2 | done (2026-09-07) | Census corrections recorded in 0006; SDK stays a reference (0007) |
| 4 | Enrollment: MS-MDE2 with XCEP and WSTEP | done (2026-09-07) | OnPremise only; PKCS#7 and Renew refused with `NotEligibleToRenew` until Phase 9; records 0008 to 0010 |
| 5 | Management session: MS-MDM over SyncML | done (2026-09-07) | Packages 1 to 4, MD5 and mTLS auth, scope gating, chunking, unenroll; records 0011 to 0013 |
| 6 | Reference server, SQL storage and simulator end-to-end | not started | |
| 7 | Real-client conformance on guestweave | not started | Settles open questions 1, 2, 4, 5, 6, 9 |
| 8 | WNS push and the poll schedule | not started | |
| 9 | Certificate lifecycle: ROBO renewal, SCEP, PFX | not started | |
| 10 | Entra identity paths, Terms of Use, Graph | not started | |
| 11 | Enrollment attestation | not started | |
| 12 | Windows declared configuration (WinDC) | not started | The differentiator; no prior art |
| 13 | CSP operation library: policy, apps, inventory, lifecycle | not started | |
| 14 | WBXML encoding | not started | Deferred by design |
| 15 | Operations: telemetry, audit, admin API, security documentation | not started | |

Status values: `not started`, `in progress`, `blocked (reason)`, `done (date)`.

## Target and pinned references

From research section 0 and section 9 ("What the 26H2 target means in practice"). Windows 11
version 26H2 is build 26300.x, delivered as an enablement package on the shared 24H2 (26100)
servicing branch. Nothing in the fetched sources adds MDM, CSP or WinDC surface in 26H2. Compliance
with 26H2 therefore means compliance with build 26100.x on the current LCU, plus a re-check when
the facts below move.

| Reference | Pinned revision | Research entry | Role |
|---|---|---|---|
| MS-MDE2 | v19.0, 2026-08-11 | 1.3 | Enrollment wire protocol; supersedes every Learn enrollment page |
| MS-MDM | v15.0, 2024-04-23 | 1.3 | Windows subset of OMA DM; management wire protocol |
| MS-XCEP | v18.0, 2026-03-09 | 1.3 | Enrollment policy (GetPolicies), profiled by MDE2 3.3 |
| MS-WSTEP | v15.0, 2024-04-23 | 1.3 | RST/RSTRC, profiled by MDE2 3.4 |
| OMA DM 1.2.1 enabler | V1_2_1-20080617-A | 1.1 | Protocol, RepPro, Security, StdObj, TND, Bootstrap; the ERELD is the pin manifest |
| OMA SyncML Common 1.2.2 | V1_2_2-20090724-A | 1.2 | RepPro (DTD order, status codes, WBXML tokens), MetaInfo, HTTP binding |
| OMA DM w7 characteristic | ac_w7_dm V1_0_1 | 1.1 | Bootstrap parameters, with the Microsoft extensions in MDE2 2.2.9.5 |
| DDF v2 bundle | DDFv2Feb2026.zip, Last-Modified 2026-02-19 | 1.5 | The only machine-readable CSP description; schema source |
| Learn client-management pages | ms.date 2025-08-04 bulk commit unless noted | 1.4, 1.6, 1.7 | Windows-specific behaviour the specs omit |
| Windows 11 release information | ms.date 2026-08-27 | 0 | Build-to-KB-to-date mapping for `OsBuildVersion` values |

Re-check triggers (research section 9): 26H2 general availability; the next DDF drop (historically
February, July and September); any change on the MS-MDE2 landing page (four revisions in the last
twelve months); a new "What's new in MDM" table on Learn (stalled at 22H2 since 2025-08-04).

Phase 0 records these pins in `docs/research/decisions/0002-pinned-references.md` and in a
machine-readable manifest so `make verify` can check the DDF pin.

## Repository layout target

From research section 9 ("Rules that carry over") and go-apple-dm decision 0044. The tier order is
the same as go-apple-dm's; two tier directories are renamed for the platform.

| Tier | Directories | Responsibility | May import |
|---|---|---|---|
| foundation | `clock`, `paging`, `secrets`, `telemetry`, `state`, `ratelimit`, `testpki` | No domain knowledge | foundation |
| schema | `schema/...`, generated by `cmd/ddfgen` from `internal/schemagen` | CSP tree types, node catalogs, allowed values, applicability, validation | foundation |
| protocol | `mdmprotocol/{syncml,soap,wapprov,enroll,mdm,windc,event,dmhook}` | Wire formats and protocol state machines, no I/O | schema and below |
| pki | `pki/{ca,wstep,xcep,scep,attestation,revocation}` | Identity issuance and verification | protocol and below |
| platform services | `msplatformservices/{wns,entra,graph}` | Outbound clients to Microsoft services | pki and below |
| storage | `storage/...`, `storage/inmem`, `storage/storagetest` | Domain contracts, in-memory backend, contract suite | services and below |
| client | `simulator` | A Windows MDM client in software | storage and below |
| server | `server/{sqlstore,service,httpapi,pushnotify,windcsync,adminauth,audit,eventsink}` | Persistence, orchestration, transport (own module) | client and below |
| app | `server/cmd/{dmserver,dmctl}`, `server/internal/app`, `server/e2e`, `cmd/ddfgen`, `internal/schemagen` | Composition, CLI, generator, scenarios | everything |

Rules enforced by `internal/layout` (copied in shape from go-apple-dm, tier names changed):
a package imports its own tier or lower; the library never imports the server module, including
in tests; directory-level cycles are rejected; every tier must be populated; explicit upward
exceptions are listed in the test, and there are none at the start.

Read-only references live under `third_party/refs/` (gitignored, populated by `make refs`) and are
never imported. Specifications and the DDF bundle live under `third_party/specs/` and
`third_party/ddf/` (see Phase 0).

## Dependency policy

From research sections 4 and 9. Decide each in a decision record before the first import.

| Candidate | Purpose | Position |
|---|---|---|
| `github.com/smallstep/pkcs7` | PKCS#7 for ROBO renewal tokens and detached `MS-Signature` | Accepted candidate exception (Phase 4 record) |
| `github.com/smallstep/scep` | SCEP server for `ClientCertificateInstall/SCEP` | Accepted candidate exception (Phase 9 record); prefer over `micromdm/scep` |
| `github.com/google/go-attestation`, `go-tpm` | AIK claim verification | Evaluate in Phase 11 |
| `github.com/mattrax/xml` | Fork of `encoding/xml` for SOAP namespace prefixes | Evaluate in Phase 2 against Go 1.27's encoder; adopt only if a failing test proves the need |
| `github.com/russellhaering/goxmldsig` | XML-DSIG for the Certificate auth policy RST | Evaluate in Phase 10 |
| OpenTelemetry API modules | Telemetry seam | Same shape as go-apple-dm decision 0040 (Phase 1) |
| SQLite, PostgreSQL, MySQL drivers | Server module only | Phase 6 |
| Fleet, Mattrax, MDMatador, local-mdm, oscartbeaumont, NachoMDM and forks | Read-only references | Never a dependency; `mjoliver/Windows-MDM`, `NachoMDM` and `apexaegis` have no licence and are read for protocol knowledge only |
| `kapytein/syncml`, `hotnops/Imitune` | GPL | Read only, never vendored |
| `deploymenttheory/go-sdk-windowscsp` | DDF v2 parser and CSP SDK | Reference only (user decision 2026-09-06); dependency question deferred to the Phase 3 record |
| `deploymenttheory/go-sdk-appleservices` | n/a | Not used, matching go-apple-dm's rule |

Before the first commit of protocol code, read the Microsoft Open Specification Promise terms
(research section 10, "Not covered") and record the conclusion in the Phase 0 architecture record.

## Cross-cutting conventions

- Language: Go 1.27 in both modules; `go.work` for local development.
- Naming: `DM_` prefixes configuration; `MDM` names the protocol (go-apple-dm decision 0043).
  Windows protocol identifiers (`w7` parameter names, CSP node names, alert `Type` strings, SOAP
  action URIs) are preserved exactly and exposed as constants (the closed-vocabulary rule of
  go-apple-dm decision 0041).
- XML: emit `SYNCML:SYNCML1.2`, `VerDTD` 1.2, `VerProto` DM/1.2; parse `SYNCML:SYNCML1.1` too.
  Marshal in DTD order. Never emit CDATA; entity-escape `<Data>`. (Research 1.2 DTD entry, 1.4
  OMA-DM page, section 7 "SyncML 1.1 vs 1.2" row.)
- Identity: enrollments are keyed on the issued certificate and the MDE2 `DeviceID`, never on
  `HWDevID` or `DevInfo/DevId` (research 1.4 DevInfo entry, section 7 duplicate-`HWDevID` row).
- Persistence: per-command results, not raw envelopes; queues ordered by a monotonic sequence;
  retention and pruning designed with the table (section 7, Fleet #44188, #49616, #43897).
- Tests: `make test` with race detection on both modules; storage contract suites; simulator
  scenarios; bounded fuzz on every decoder; 95% coverage overall and per non-exempt package.
- Documentation: `doc.go` per package; decision records use the template; `docs/architecture.md`
  describes implemented behaviour only.
- Commits: Conventional Commits; release-please owns versions (already configured in
  `.github/workflows/release-please.yml`).

---

## Phase 0: Repository bootstrap and governance

Goal: turn the template into a Go workspace with the same working agreements as go-apple-dm, and
pin every reference the later phases depend on.

Inputs: research Purpose, section 0, section 1 (all pins), section 3 (clone list), section 9
("Rules that carry over"), section 10 ("Not covered": Open Specification Promise).

Deliverables:

1. `go.mod` (`module github.com/deploymenttheory/go-microsoft-dm`, `go 1.27.0`), `server/go.mod`
   (`.../go-microsoft-dm/server` with a `replace` to `..`), `go.work` using both.
2. `Makefile` with the go-apple-dm target set adapted: `help`, `tools`, `generate`, `verify`,
   `lint`, `test`, `test-storage`, `test-conformance`, `test-e2e`, `test-conformance-guest`
   (new; Phase 7), `fuzz-smoke`, `fuzz`, `coverage`, `vuln`, `refs`, `refs-activity`, `specs`,
   `ddf`, `ci`, `clean`. `generate` and `verify` operate on the checked-in DDF bundle instead of a
   submodule.
3. `scripts/refs.sh` with the clone list from research section 3, 4 and 5 (Go first):
   `fleetdm/fleet`, `Malcolm/local-mdm`, `mjoliver/Windows-MDM`, `marcosd4h/mdm`,
   `marcosd4h/MDMatador`, `oscartbeaumont/windows_mdm`, `mattrax/Mattrax`, `PhenixH/Mattrax`,
   `dejiblue/windows-mdm`, `AmberWolfCyber/NachoMDM`, `wso2/carbon-device-mgt-plugins`,
   `entgra/device-mgt-plugins`, `deploymenttheory/go-sdk-windowscsp`,
   `deploymenttheory/go-bindings-win32`, `deploymenttheory/go-bindings-winrt`,
   `okieselbach/SyncMLViewer`, `Sleepw4lker/TameMyCerts.WSTEP`, `mattrax/xml`,
   `remdev/go-activesync`, `oniestel/go-wns`, `secureworks/pytune`, `Gerenios/AADInternals`,
   `getprimo/csp-builder`, `smallstep/pkcs7`, `smallstep/scep`, `google/go-attestation`,
   `hotnops/Imitune` (GPL, read only), `kapytein/syncml` (GPL, read only). Writes
   `third_party/refs/COMMITS.txt`. `scripts/refs-activity.sh` reports last-push dates for the
   research store's status column.
4. `scripts/specs.sh` (`make specs`): downloads the OMA PDFs and DTD text files listed in research
   1.1 and 1.2, the four Microsoft Open Specification PDFs (MDE2, MDM, XCEP, WSTEP) and the W3C
   WBXML NOTE into `third_party/specs/` (gitignored) and verifies each against
   `third_party/specs/MANIFEST.json` (URL, expected SHA-256, revision, date). Record the SHA-256
   values when first fetched.
5. `third_party/ddf/DDFv2Feb2026.zip` checked in (727,650 bytes; small enough and it is the schema
   source, matching research section 9's advice to keep every drop under version control) with
   `third_party/ddf/MANIFEST.json` (URL, HTTP Last-Modified 2026-02-19, SHA-256, top folder
   `DDFDrop012026/`, file count 313). Later drops are added beside it, never replacing it, so they
   can be diffed.
6. `internal/layout` with the tier table above and tests `TestTiersOnlyImportDownwards`,
   `TestEveryTierIsPopulated`, `TestNoUnitCycles`, `TestLibraryNeverImportsServer`.
7. `CLAUDE.md` in the shape of go-apple-dm's: modules, generated files rule, read-only references,
   dependency policy, design and comment rules, checks. `CONTRIBUTING.md`, `README.md`,
   `SECURITY.md`, `docs/README.md` rewritten from the template boilerplate.
8. `docs/research/decisions/TEMPLATE.md` (copy of go-apple-dm's), `docs/research/decisions/README.md`
   (index), and the first records:
   - `0001-architecture.md`: library-first, two modules, tier order, the WinDC-extends-MDM stance
     (mirrors go-apple-dm 0001 and 0039), the Open Specification Promise conclusion.
   - `0002-pinned-references.md`: the pinned-reference table above, the re-check triggers, and the
     manifests that make the pins checkable.
   - `0003-reference-projects-and-dependency-policy.md`: read-only references, licence positions,
     the candidate exceptions, the go-sdk-windowscsp position.
9. `.golangci.yml` reviewed for the two-module layout (it exists already; keep `new-from-merge-base`
   behaviour) and `.github/workflows/go-lint.yml` adjusted to run in both modules. Add a `go-test`
   workflow that runs `make verify test`.
10. `.gitignore` entries for `third_party/refs/`, `third_party/specs/*.pdf`, `cover/`.

Decision records: 0001, 0002, 0003.

Verification: `go build ./...` in both modules succeeds with placeholder `doc.go` files in each
tier directory so `TestEveryTierIsPopulated` passes; `make refs`, `make specs` and `make verify`
run; the lint workflow passes on a PR.

Exit criteria: a clean checkout runs `make ci` (with empty test sets where a phase has not started)
and every later phase has a directory to land in.

## Phase 1: Foundation packages

Goal: the domain-free packages every tier above uses.

Inputs: go-apple-dm's foundation tier as the contract reference; research section 7 rows on
retention, ordering and rate concerns that foundation types must make expressible.

Deliverables:

- `clock`: injectable time source; the deterministic test clock.
- `paging`: cursor and page types shared by storage and admin APIs, without domain imports.
- `secrets`: provider interface and redaction helpers for logs (WNS secrets, MD5 credentials,
  Entra client secrets, PFX passwords all flow through here later).
- `telemetry`: the OpenTelemetry seam the consumer owns (same design as go-apple-dm 0040): tracer
  and meter providers injected; no exporter in the library.
- `state`: bounded state helpers (nonce stores, challenge tables) with explicit capacity.
- `ratelimit`: per-peer and aggregate buckets, disabled by default.
- `testpki`: test-only CA, leaf and client-certificate helpers used by every later phase's tests,
  including a helper that produces a CSR with the non-printable subject characters Windows sends
  (research section 4, x509 pitfall), so the Phase 4 parser is tested from day one.

Decision records: `0004-go-baseline-and-foundation-contracts.md` (Go 1.27, JSON and XML policy at
the language level, telemetry seam ownership).

Verification: unit tests with failure paths; `internal/layout` shows all seven in the foundation
tier with no upward imports.

## Phase 2: SyncML wire model

Goal: a typed, DTD-ordered SyncML 1.2 codec for the DM subset Windows uses, with no I/O, that
round-trips real client messages byte-for-byte in meaning.

Inputs: research 1.1 (Protocol 1.2.1 sections 6, 7, 8.3 to 8.7, 9; RepPro 1.2.1 alert table),
1.2 (SyncML RepPro 1.2.2 sections 6 and 10, the RepPro DTD, MetaInfo 1.2.2), 1.3 MS-MDM 2.2.x
(namespace, SyncHdr, common elements, Data/Item/Meta, Status, the command list, Results shape),
1.4 OMA-DM protocol support page (Windows command semantics and status table), section 2 rows
"SyncML session", "Alert codes", "Status codes", "Add ... Atomic", "Results", "Final", "Chunking",
"SyncML namespace", section 4 "XML encoding pitfalls", section 7 rows "SyncML 1.1 vs 1.2" and
"Large objects".

Package: `mdmprotocol/syncml`.

Design:

- One Go type per element in DTD order: `SyncML{SyncHdr, SyncBody}`, `SyncHdr{VerDTD, VerProto,
  SessionID, MsgID, Target, Source, RespURI?, NoResp?, Cred?, Meta?}`, `SyncBody{Commands...,
  Final?}` where the command list is an ordered slice of a sealed interface implemented by
  `Add`, `Alert`, `Atomic`, `Delete`, `Exec`, `Get`, `Replace`, `Results`, `Sequence`, `Status`.
  `Copy`, `Map`, `Move`, `Put`, `Search`, `Sync` are recognised on decode and rejected with a typed
  error (Windows does not use them; MS-MDM 2.2.7).
- `Item{Target?, Source?, Meta?, Data?, MoreData?}`; `Meta` carries `Format`, `Type`, `Mark`,
  `Size`, `NextNonce`, `MaxMsgSize`, `MaxObjSize` with the `syncml:metinf` namespace on each child,
  the way the client emits them.
- `Data` is `*string` with char-data encoding: absent for `Get`, present and entity-escaped for
  `Replace`/`Add`; CDATA is never produced and is normalised on decode.
- `Cred` and `Chal` types for `syncml:auth-basic` and `syncml:auth-md5`; the MD5 digest helper
  implements the OMA DM Security rule that the nonce is base64 on the wire and raw bytes in the
  hash (research 1.1 Security entry; MS-MDM 1.3.1).
- Constants packages: `syncml.Alert*` (1200, 1201, 1222, 1223, 1224, 1225, 1226), `syncml.Status*`
  with the Windows meanings from the Learn table (200, 202, 212, 213, 214, 215, 216, 400, 401, 403,
  404, 405, 406, 407, 412, 413, 415, 416, 418, 420, 424, 425, 500, 507, 508, 516), the Windows alert
  `Type` strings (`com.microsoft/MDM/LoginStatus`, `com.microsoft.mdm.synctype`,
  `com.microsoft/MDM/DevicePrepSync`, `com.microsoft/MDM/AADUserToken`,
  `Reversed-Domain-Name:com.microsoft.mdm.win32csp_install`,
  `com.microsoft.mdm.declaredconfigurationdocuments`), MIME types, namespace strings.
- Namespace handling: emit `xmlns="SYNCML:SYNCML1.2"` on the root; accept `SYNCML:SYNCML1.1`.
  Evaluate Go 1.27 `encoding/xml` against the exact output shape; only if a test fails adopt
  `mattrax/xml` (Phase 2 record).
- Validation helpers used by the session engine later: every command has a `CmdID`; `LocURI`
  never starts with `/`; node-name rules from the Learn OMA-DM page; `Final` at most once and last.
- Large-object primitives: split an `Item` by `MaxObjSize`/`MaxMsgSize` into chunks with
  `MoreData`, reassemble chunks, and the 213 and 1225 semantics as pure functions.
- Message-level helpers: `NextMsgID`, the "Next Message" response shape (Alert 1222 plus Status for
  SyncHdr, no other commands, no Final), session-abort (1223).

Fixtures: the Learn OMA-DM page samples, the Learn DiagnosticLog and EnterpriseDesktopAppManagement
samples (both namespaces), Fleet's `mdmtest/windows.go` package-1 output, and the WinDC alert
sample from research 1.6. Store under `mdmprotocol/syncml/testdata/`. Phase 7 adds SyncMLViewer
captures from a real 26100 client.

Decision record: `0005-syncml-message-model-and-xml-policy.md` (typed model, DTD order, namespace
policy, CDATA policy, the `mattrax/xml` evaluation result, the Mattrax Rust crate as the design
reference, the oscartbeaumont regex approach as the failure to avoid).

Verification: round-trip tests on every fixture; property tests that marshal then unmarshal
preserves command order and `CmdID`s; `FuzzDecode` in `make fuzz-smoke`; failure tests for
unknown commands, missing `CmdID`, leading-slash `LocURI`, CDATA input, `SYNCML:SYNCML1.1` input,
`Final` in the wrong place, oversized messages.

Pitfalls addressed: "SyncML 1.1 vs 1.2" (whole row), "Large objects and MaxMsgSize" (primitives),
regex parsing and CDATA from section 9 "What the references get wrong".

## Phase 3: CSP schema generated from DDF v2

Goal: a deterministic generator from the pinned DDF bundle into a `schema/` tier that later phases
use to validate OMA-URIs, formats, scopes, allowed values and applicability, mirroring go-apple-dm's
`admgen` role.

Inputs: research 1.5 (DDF page, XSD facts, Policy DDF, CSP index, Policy CSP, ADMX pages, and above
all the "DDF bundle inspection" entry), 1.4 CSP entries (DMClient, DMAcc, DevDetail, DevInfo,
DeviceStatus, CertificateStore, ClientCertificateInstall, RootCATrustedCertificates, RemoteWipe,
Reboot, EnrollmentStatusTracking, DeviceManageability, the two app-management CSPs), 1.6
DeclaredConfiguration DDF entry, section 4 "Same-organisation building blocks", section 9 bullet
on DDF as schema, open question 7.

Packages: `internal/schemagen` (parser and emitter), `cmd/ddfgen` (`generate`, `verify`, `diff`),
`schema/` outputs.

Design:

- Parser facts the bundle inspection requires: UTF-8 BOM; DOCTYPE `-//OMA//DTD-DM-DDF 1.2//EN`;
  DDF elements are in no namespace (do not validate against the `tempuri.org` XSD); `MSFT:` prefix
  bound to `http://schemas.microsoft.com/MobileDevice/DM`; `Path` only on root nodes; dynamic nodes
  have an empty `NodeName` plus `DFTitle` and a `MSFT:DynamicNodeNaming` kind (`UniqueName`,
  `ClientInventory`, `ServerGeneratedUniqueIdentifier`); `MSFT:Applicability` inherits to children;
  `OsBuildVersion` is a comma-separated list whose first entry is the major release, with sentinels
  `99.9.99999`, `99.9.9999`, `88.8.88888`, Insider and Server builds, and the `11.0.` quirk on two
  nodes; `CspVersion` values 1.0 to 11.0; `EditionAllowList` hex IDs; nine `AllowedValues`
  `ValueType`s (ADMX, ENUM, None, Range, RegEx, XSD, Flag, SDDL, JSON); `AdmxBacked` with
  `Area`, `Name`, `File`; `DFFormat` values chr, int, node, bool, b64, null, xml, bin, time;
  legacy root paths without `./Device` or `./User` (18 CSPs) and the `./Vendor/MSFT//SUPL` typo;
  109 files with both a User and a Device root; `Deprecated@OsBuildDeprecated`; `AtomicRequired`;
  `RebootBehavior`; `ConflictResolution`; `ReplaceBehavior`; `DependencyBehavior`; `GpMapping`.
- Model: `schema/csp` (a `Tree` per CSP with `Node{URI, Scope, Format, Access, Applicability,
  AllowedValues, Dynamic, Deprecated, AtomicRequired, ...}`), one generated package per standalone
  CSP under `schema/csp/<name>` with typed OMA-URI constants and allowed-value constants, and
  `schema/policy/<area>` for the 261 Policy areas with the ADMX link metadata.
- Applicability resolution: `schema/support` resolves an `OsBuildVersion` list against a device's
  reported `SwV` (from `DevDetail`) using the build lineage in research section 0 (24H2, 25H2 and
  26H2 share 26100.x applicability once the LCU matches), treating sentinels as "not in a released
  build", and exposing `SupportedOn(build) (bool, reason)`.
- Validation (`schema/validation`): URI exists, scope matches the URI prefix, format matches the
  leaf, value within allowed values (ENUM, Range, RegEx evaluated; ADMX payload grammar checked
  structurally per research 1.5 "Understanding ADMX policies"; XSD, SDDL and JSON recorded but not
  evaluated in this phase), access type permits the verb, `AtomicRequired` honoured.
- Provenance: `schema/GENERATED_FROM.json` (bundle URL, SHA-256, Last-Modified, file count,
  node counts), `schema/EXPORTED_IDENTIFIERS.lock`, `schema/ALLOWED_REMOVALS.md`; `make verify`
  regenerates into a temporary directory and diffs.
- `ddfgen diff <old.zip> <new.zip>` prints added, removed and changed nodes with applicability
  changes, so the next DDF drop can be reviewed the way research section 0 and 9 describe (the
  Sept 2025 to Feb 2026 delta in the bundle inspection is the first test case).
- Do not ship ADMX files; record that `ValueType="ADMX"` nodes need `C:\Windows\PolicyDefinitions`
  for full value validation.

Decision records: `0006-schema-generator-over-ddf-v2.md` (the caveat list, determinism, what is
and is not validated), `0007-relationship-to-go-sdk-windowscsp.md` (reference only; revisit
criteria; what was learned from its parser and codegen without copying it).

Verification: `make generate` then `make verify` is a no-op; census tests assert the counts in the
bundle inspection (313 files, 52 standalone CSPs, 261 area files, 5,955 nodes, 5,146 leaves,
format census, extension census) so a wrong parse is caught by numbers; conformance tests that
the generated `DMClient`, `DevDetail`, `DevInfo`, `DMAcc`, `DeclaredConfiguration` packages expose
the nodes research 1.4 and 1.6 enumerate; failure tests for BOM-less input, a namespaced DDF, a
missing `Path`, an unknown `DFFormat`, an unknown `ValueType`.

Pitfalls addressed: the WinDC validation row in section 7 ("every CSP URI must exist in the DDF")
gains its data source here; open question 7 is answered in record 0007.

## Phase 4: Enrollment: MS-MDE2 with XCEP and WSTEP

Goal: enroll a Windows 11 device with the on-premises auth policy end to end, issuing the MDM
client certificate from a pluggable CA and returning a correct `wap-provisioningdoc`.

Inputs: research 1.3 (MDE2 v19.0 section map, faults, RequestVersion and EnrollmentVersion
rules; XCEP; WSTEP and its erratum; MDE v1 only for the WSDL), 1.4 (Mobile device enrollment,
MDM enrollment of Windows devices, Federated and On-premises and Certificate pages, Certificate
renewal for the `My/WSTEP/Renew` nodes set at enrollment, DMClient CSP, DMAcc CSP, w7 APPLICATION
CSP, CertificateStore CSP, RootCATrustedCertificates CSP), 1.1 Bootstrap and w7 characteristic,
section 2 rows Discovery GET, Discovery POST, AuthPolicy, EnrollmentPolicy, EnrollmentService,
DMClient bootstrap, section 3 (Fleet `microsoft_mdm.go`, `wstep.go`, `wstep_csr.go`; local-mdm
`enrollment.go`; Mattrax `ms-mde` crate; TameMyCerts.WSTEP models; WSO2 `win10-wap-provisioning.xml`),
section 4 (SOAP finding; x509 CSR pitfall; PKCS#10 vs PKCS#7), section 7 rows "Enrollment error
codes", "Client certificate back-dated", "Duplicate HWDevID", "Transport".

Packages: `mdmprotocol/soap` (envelope, WS-Addressing, WS-Security header types, faults),
`mdmprotocol/enroll` (discovery, policy, enrollment message types and handlers),
`mdmprotocol/wapprov` (typed `wap-provisioningdoc` builder: `characteristic`/`parm` model with the
CertificateStore, APPLICATION w7, DMClient and RootCATrustedCertificates characteristics from MDE2
2.2.9.x), `pki/ca` (CA interface, in-memory and file-backed issuers), `pki/wstep` (CSR parsing,
issuance policy), `pki/xcep` (policy response builder), `storage` contracts for enrollments and
issued certificates, `simulator` enrollment client.

Design:

- Discovery: `GET /EnrollmentServer/Discovery.svc` returns 200 with an empty body; `POST` parses
  `DiscoveryRequest` (EmailAddress, OSEdition, DeviceType, RequestVersion 1.0 to 9.0,
  ApplicationVersion, AuthPolicies) and answers with `AuthPolicy` (this phase: `OnPremise`;
  `Federated` and `Certificate` in Phase 10), `EnrollmentVersion` (configurable; default the
  highest the server implements, 3.0 in this phase, raised in Phase 11), `EnrollmentPolicyServiceUrl`,
  `EnrollmentServiceUrl`, and the newer `DeviceAssociationMaaUrl` and `GatewayService` fields
  parsed but unused. The response is never chunked: set `Content-Length`. Policy and enrollment
  URLs share a host name. The hostname convention `enterpriseenrollment.<domain>` is documented,
  not enforced.
- XCEP `GetPolicies`: optional; when enabled returns `minimalKeyLength` 2048 and the hash OID; the
  WS-Security `UsernameToken` header is validated for OnPremise.
- WSTEP `RequestSecurityToken`: `TokenType` DeviceEnrollmentToken, `RequestType` Issue,
  `BinarySecurityToken` `ValueType` PKCS10; the `AdditionalContext` items are parsed into a typed
  struct covering every item MDE2 lists (research 1.8 catalogue), stored with the enrollment and
  exposed to policy hooks; `EnrollmentType` Full vs Device decides the certificate store in the
  provisioning doc (`My/User` vs `My/System`).
- CSR parsing: a CSR parser tolerant of Windows's non-printable subject characters, implemented
  as a narrow relaxation of `PrintableString` handling with a test from `testpki` (Phase 1), not a
  1,585-line vendored parser and not a GOROOT patch (both catalogued in research section 4 and 9).
  PKCS#7 input is recognised and rejected in this phase with a typed error (Phase 9 accepts it).
- Issuance: `NotBefore = now` (never back-dated), validity and renewal window configurable,
  SHA-1 thumbprint uppercase hex for `CertificateStore` paths; certificate serial and DeviceID
  are the enrollment key; a repeated `HWDevID` from `AdditionalContext` is surfaced as a conflict
  event, not silently re-enrolled.
- Provisioning doc: CertificateStore (root, at most one intermediate, client cert with
  `PrivateKeyContainer`), `My/WSTEP/Renew` (`RenewPeriod` 40 to 60 days, `RetryInterval` 4 to 5
  days, `ROBOSupport` true, `ServerURL` reserved for Phase 9), APPLICATION w7 (`APPID`,
  `PROVIDER-ID`, `NAME`, `ADDR`, `ROLE` not set by the server, `DEFAULTENCODING` XML,
  `BACKCOMPATRETRYDISABLED`, `CONNRETRYFREQ`, `INITIALBACKOFFTIME`, `MAXBACKOFFTIME`,
  `SSLCLIENTCERTSEARCHCRITERIA` with U+F000 separators, `APPAUTH` CLIENT/DIGEST and APPSRV/BASIC
  or DIGEST as MDE2 2.2.9.5 requires), DMClient `Provider/<PROVIDER-ID>` with `UPN`,
  `EntDeviceName`, `EntDMID`, `Poll/*` (Microsoft defaults, not Fleet's 1-minute poll), and the
  `ProviderID` equal to `PROVIDER-ID`. All parameter names uppercase and case-sensitive.
- Faults: every `s:` and `a:` subcode from MDE2 2.2.10 and the `deviceenrollmentserviceerror`
  body (`DeviceCapReached` through `InvalidEnrollmentData`, plus `CustomServerError` 80180032) as
  typed errors the handler maps to the exact SOAP fault.
- Best-practice rule from the Learn enrollment page: no hard-coded checks on `User-Agent`, fixed
  URIs or value formats.
- Storage contracts (`storage`): `EnrollmentStore` (create, get by ID and by certificate, update
  state, list), `CertificateStore` (issued serials, thumbprints, revocation flag), the contract
  suite in `storage/storagetest`, the in-memory backend.
- Simulator: `simulator.Enroll` drives Discovery GET and POST, optional GetPolicies, RST, then
  parses the provisioning doc into an in-memory DMAcc and DMClient state ready for Phase 5.

Decision records: `0008-enrollment-protocol-and-provisioning-document.md` (MDE2 v19.0 as
authority, versions advertised, endpoint paths, fault mapping, what the two Learn pages get
older), `0009-wstep-ca-and-csr-handling.md` (CA interface, CSR tolerance, issuance policy, keying,
HWDevID conflict), `0010-storage-interfaces-and-contract-suite.md`.

Verification: handler tests against the Learn on-premises page samples and MDE2 section 4
examples; simulator enrollment against the in-memory server in the library test suite; failure
tests for each fault subcode, an unsupported `RequestVersion`, a chunked response guard, a
PKCS#7 token, a CSR with an invalid signature, duplicate `HWDevID`; contract suite runs against
the in-memory store.

Pitfalls addressed: "Enrollment error codes", "Client certificate back-dated", "Duplicate
HWDevID", "Transport" (enrollment half), and the GOROOT patch and vendored parser items in
section 9.

## Phase 5: Management session: MS-MDM over SyncML

Goal: run OMA DM packages 1 to 4 with a Windows client, authenticate it, deliver queued commands,
collect statuses and results, honour scope and chunking, and never do the destructive things the
references did.

Inputs: research 1.1 Protocol 1.2.1 (sections 6 to 9), 1.3 MS-MDM (1.3.1 auth, 2.1 transport
notes 1 to 9, 2.2.x, 3.1.5.x, 3.1.7 ACL and UserAgentOrigin, 3.2.5.1.x Azure details), 1.4 OMA-DM
protocol support page (every paragraph), Known issues page, DMClient CSP (Poll, SyncApplicationVersion,
MaxSyncApplicationVersion, MultipleSession, EnableOmaDmKeepAliveMessage, RequireMessageSigning),
DevInfo, DevDetail (`LrgObj`, `URI/*`), section 2 rows "SyncML session", "Status codes",
"Add ... Atomic", "Results", "Final", "Chunking", "User vs device scope", section 3 (Fleet
`isTrustedRequest`, `createResponseSyncML`, command queue; local-mdm `management.go`; MDMatador
`command_manager.go`; Mattrax `ms-mdm`), section 7 rows "Aggressive polling", "SyncML engine that
only accepts Add and Replace", "Storing the full SyncML response", "Same-second commands",
"User-scope commands before sign-in", "Large objects", "Transport", "./User provisioning".

Packages: `mdmprotocol/mdm` (session state machine, request classification, response builder,
authentication, scope gate), `storage` additions (command queue, results, session state, device
facts), `simulator` DM client.

Design:

- Transport parsing: `?mode=Maintenance|Machine&Platform=...`; headers `User-Agent` (logged, not
  checked), `Authorization: Bearer` (parsed; validated in Phase 10), `DeviceToken`, `MS-Signature`
  (verified when `RequireMessageSigning` is enabled; PKCS#7 detached, Phase 9 dependency),
  `MDM-GenericAlert`, `client-request-id` (EntDMID); content type XML in this phase with the
  WBXML branch returning 415 until Phase 14.
- Authentication: mutual TLS certificate matched to the enrollment; otherwise application-layer
  `syncml:auth-md5` challenge via `Chal` with a fresh `NextNonce` per session (bounded nonce store
  from Phase 1 `state`), `syncml:auth-basic` accepted only when configured; Status 212 to the
  `Cred` once authenticated for the session; server MD5 credentials to the client when the
  provisioning doc set CLIENT/DIGEST.
- Session state: `SessionID` opaque and constant (decimal integer, or 2 bytes once
  `SyncApplicationVersion` 2.0 is set); server `MsgID` starts at 1 and increments; `Final` on every
  server message when the package is complete; 1222 handling in both directions; 1223 abort;
  1224 client events and 1226 generic alerts routed to typed handlers; package 1 must carry Alert
  1200 or 1201 and the `DevInfo` Replace, otherwise 400.
- Device facts on package 1: `DevInfo` items stored per session; `LoginStatus` (`user`, `others`,
  `none`), `synctype` and `DevicePrepSync` alerts stored on the enrollment; on first session a
  `Get` on `./DevDetail` (for `SwV`, `LrgObj`, `URI/MaxSegLen`), `DeviceManageability/Capabilities/CSPVersions`
  and `DMClient/.../Push/ChannelURI` (Phase 8 consumes it) is queued automatically.
- Command queue contract: commands are typed (`Add`, `Replace`, `Get`, `Delete`, `Exec`, `Atomic`,
  `Sequence`), validated against `schema/validation` before queueing, ordered by a monotonic
  per-enrollment sequence, scoped `device` or `user`; user-scoped commands are held until
  `LoginStatus` reports `user`; a batch never mixes scopes inside one `Atomic`; the builder rejects
  nested `Atomic`, `Get` inside `Atomic`, and Add-then-Replace on one node inside `Atomic`.
- Response builder: all deliverable commands for the enrollment in one message subject to
  `MaxMsgSize` and chunking (large objects split with `MoreData`, statuses 213 expected); results
  stored per command (`Status`, `Results` items), never the raw envelope, with a bounded opt-in
  debug ring of raw messages.
- Desired-state discipline: the queue is fed by callers; the library offers a "diff desired vs
  acknowledged" helper so a server never re-sends unchanged configuration each session.
- Unenroll: `Exec` on `DMClient/Provider/{ID}/Unenroll` and `./Device/Vendor/MSFT/DMClient/Unenroll`;
  the 1226 alert from a user-initiated unenroll ends the enrollment; the Entra-joined and
  add-work-account limitations from the known-issues page are documented on the API.
- AVD and multi-session (MS-MDM 3.2.5.1.x): message shapes parsed, `SyncType` mismatch answered
  with 405; the multi-user session policy itself is out of scope and recorded as such.
- Simulator: package 1 to 4 client with a fake CSP tree (research 4: go-sdk-windowscsp's
  `clienttest` shows the shape), MD5 challenge handling, chunked upload, 1224/1226 emission,
  configurable `LoginStatus`, the option to send `SYNCML:SYNCML1.1`.

Decision records: `0011-management-session-engine.md` (state machine, authentication order, package
rules, what MS-MDM's "2.1 or later" typo means), `0012-command-queue-results-and-retention.md`
(monotonic ordering, per-command results, raw-message policy, pruning), `0013-scope-and-user-channel.md`
(LoginStatus gating, Atomic rules, Entra-joined `./User` limitation).

Verification: simulator scenarios in the library for first session, MD5 challenge, mTLS, chunked
`Get` result, user-scope hold and release, Atomic rejection cases, unenroll, 1222 continuation,
abort; failure tests for missing Alert in package 1, wrong `MsgID` sequence, `SessionID` change
mid-session, `LocURI` starting with `/`, oversize message, WBXML content type; `FuzzSession` on
the request classifier.

Pitfalls addressed: every row named in the inputs; the "Get inside Atomic" and CDATA known issues.

## Phase 6: Reference server, SQL storage and simulator end-to-end

Goal: an installable `dmserver` that enrolls and manages simulated devices with persistent state,
plus `dmctl` to operate it, so Phase 7 can point a real Windows guest at something.

Inputs: research section 4 "Storage schema patterns (Fleet)", section 7 storage rows, go-apple-dm
decisions 0012, 0025, 0034, 0035, 0043, 0044 as the shape to mirror.

Packages: `server/sqlstore/{sqlite,postgres,mysql}` with migration sets, `server/service`
(enrollment authorization, command delivery, hooks, events), `server/httpapi` (the SOAP and SyncML
endpoints, TLS termination or trusted-proxy certificate forwarding), `server/internal/app`
(composition from `DM_*` environment configuration), `server/cmd/dmserver`, `server/cmd/dmctl`
(enroll listing, command queueing from files, status and results inspection), `server/e2e`.

Design:

- Tables informed by Fleet's schema but keyed as Phase 4 decided: enrollments (by certificate
  serial and DeviceID; `HWDevID` indexed but non-unique), issued certificates, sessions, command
  queue (monotonic sequence), command results, device facts (DevInfo, DevDetail, CSPVersions,
  LoginStatus, ChannelURI), events, audit (Phase 15 extends).
- Endpoint paths (all configurable, defaults): `/EnrollmentServer/Discovery.svc`,
  `/EnrollmentServer/Policy.svc`, `/EnrollmentServer/Enrollment.svc`, `/ManagementServer/MDM.svc`.
  Enrollment responses always carry `Content-Length`.
- Roles: `all`, `mdm`, and later `windc` (Phase 12); `DM_STORE` selects sqlite, postgres, mysql
  or memory.
- `make testdb-up` and `make test-storage` run the contract suite from Phase 4 against PostgreSQL
  and MySQL in Docker; SQLite runs everywhere.
- `server/e2e` scenarios use the simulator: enroll, first session, queue a `Reboot/RebootNow`
  `Exec`, queue a Policy `Replace`, chunked `Get` of a large node, unenroll; each on `E2E_STORE`
  sqlite, postgres, inmem.

Decision records: `0014-sql-storage-backends.md`, `0015-reference-server-roles-and-configuration.md`,
`0016-dmctl-structure.md`.

Verification: `make test-storage`, `make test-e2e`, coverage gate active for the server module;
a Dockerfile builds `dmserver`.

## Phase 7: Real-client conformance on guestweave

Goal: enroll and manage a real Windows 11 guest (26100.x on the current LCU, and a 26300.x
Release Preview guest), capture what the real client sends, turn captures into fixtures, and close
the open questions that only a real client can answer.

Inputs: research section 5 (SyncMLViewer, `mdmlocalmanagement.dll`, Fleet's test client, MDM
diagnostics, the guestweave entry in full), section 0 (media for 26H2 via the ReleasePreview
ring), section 10 open questions 1, 2, 4, 5, 6 and 9, section 9 "Test environments are solved
in-house".

Deliverables:

- `docs/testing/guest-conformance.md`: how to create the guest on either host (`weave create win
  --from-windows pro-25h2` on Windows; `weave create --from-windows 11 <name>` on macOS; the
  ReleasePreview ring through go-sdk-winmediafoundry for 26300.x), snapshot the clean state, install
  SyncMLViewer, point discovery at `dmserver` (deep link `ms-device-enrollment:?mode=mdm&servername=...`
  or the Settings "Enroll only in device management" path with a hosts-file entry for
  `enterpriseenrollment.<domain>`), enroll with OnPremise credentials, capture, revert.
- `server/e2e/guest` (build tag `guest`): a harness that drives `weave serve`'s HTTP API to revert
  the snapshot, start the guest, run the in-guest agent to trigger `deviceenroller.exe /o {GUID}
  /c` or the scheduled task, and collect `mdmdiagnosticstool.exe` output and SyncMLViewer exports
  from the guest. `make test-conformance-guest` runs it when `WEAVE_URL` and `WEAVE_TOKEN` are
  set; otherwise it is skipped.
- Fixtures: sanitised SyncMLViewer captures of package 1 (both `LoginStatus` states), a chunked
  upload, a `DevDetail` result, the 1226 unenroll alert, checked into `mdmprotocol/syncml/testdata/captures/`
  with the build number in the file name.
- Findings recorded in the research store and the relevant decision records:
  - Question 1: capture against a server advertising `EnrollmentVersion` 3.0, 5.0 and 9.0; record
    which `AdditionalContext` items appear.
  - Question 2: set `Poll/NumberOfFirstRetries=0` and observe whether the client polls forever.
  - Question 5: send `SYNCML:SYNCML1.1` and `1.2` and record acceptance (WinDC half in Phase 12).
  - Question 6: `Get` on `ManagementServiceConfiguration/RefreshInterval`, `Host/BulkTemplate`, and
    a `./User/Vendor/MSFT/DeclaredConfiguration` node on a 26100 guest (needs Phase 12's linked
    enrollment for a full answer; the bare `Get` after primary enrollment already tells whether
    the nodes exist).
  - Question 9: dump `DeviceManageability/Capabilities/CSPVersions` on 26100 and 26300 guests and
    diff; run `ddfgen diff` if a new bundle appears.
  - Question 4 is advanced when Phase 8 exists; keep the guest harness ready to log Event 4603.
- The attestation items (question 1) are produced by the vTPM guest and are stored raw for
  Phase 11.

Decision record: `0017-conformance-testing-with-guestweave.md` (what the guest proves, what only
physical hardware proves: vendor EK chains, Autopilot and OOBE flows that need retail media).

Verification: the harness passes on at least one host; captures diff cleanly against the
simulator's output for the same scenario, and every difference is either fixed in Phases 2 to 5 or
recorded as a known client behaviour in a decision record.

Exit criteria: a real 26100.x guest enrolls, completes a session with a `Reboot/RebootNow` `Exec`
acknowledged, and unenrolls, driven from `make test-conformance-guest`.

## Phase 8: WNS push and the poll schedule

Goal: wake a device on demand without an agent, and make the polling schedule sane.

Inputs: research 1.7 (all six entries), 1.4 DMClient CSP `Push/*` and `Poll/*`, Known issues
("re-read ChannelURI every session"), section 6 Rudy Ooms push-to-session chain and PushLaunch
posts, section 7 rows "WNS push-initiated session on 24H2 and 25H2", "dmwappushservice",
"Aggressive polling", "After the device renews its WNS channel", section 10 questions 3 and 4,
section 3 (local-mdm `wns.go`; Fleet's built-but-disabled client), section 4 `oniestel/go-wns`.

Packages: `msplatformservices/wns`, `server/pushnotify`.

Design:

- Token source interface with two implementations: legacy Partner Center Package SID plus secret at
  `https://login.live.com/accesstoken.srf` (scope `notify.windows.com`; the documented MDM path),
  and Entra client credentials at `login.microsoftonline.com` with scope `https://wns.windows.com/.default`
  (the Windows App SDK model), selected by configuration so question 3 is a config change when
  Microsoft moves.
- Send: `POST <ChannelURI>` with `Authorization: Bearer`, `Content-Type: application/octet-stream`,
  `X-WNS-Type: wns/raw`, `X-WNS-Cache-Policy: cache`, optional `X-WNS-TTL`, `MS-CV`; a non-empty
  body (raw notifications must not be empty); host must be under `notify.windows.com`.
- Response classification (mirrors go-apple-dm 0042): 200 with `X-WNS-Status` and
  `X-WNS-DeviceConnectionStatus` recorded; 401 refresh token once; 403 credential mismatch (fatal
  config); 404 and 410 channel dead (mark, wait for poll); 406 throttled with `Retry-After`; 413
  payload; 5xx retry with backoff.
- Provisioning: the server sets `DMClient/Provider/{ID}/Push/PFN` (from configuration), reads
  `Push/Status` and `Push/ChannelURI` every session (Phase 5 already queues the `Get`); channel
  age is tracked against the 30-day expiry and the 15-day renewal.
- Poll policy: Microsoft's default schedule in the provisioning doc; a "has not checked in"
  server-side alert (the client logs nothing when `dmwappushservice` is missing); push is never
  the only path.
- Event 4603 (question 4): the guest harness from Phase 7 sends a push and records whether a
  session arrives; the finding goes in the research store and the WNS decision record.

Decision records: `0018-wns-push-and-poll-policy.md`.

Verification: an httptest WNS with every response code; the token source tests; a simulator that
"receives" a push by triggering a session; the guest harness sending a real push when WNS
credentials are configured (skipped otherwise); failure tests for empty body, non-Microsoft host,
expired token, dead channel.

## Phase 9: Certificate lifecycle: ROBO renewal, SCEP, PFX

Goal: keep enrolled devices enrolled past certificate expiry and deliver further certificates.

Inputs: research 1.4 Certificate renewal, CertificateStore CSP (`My/WSTEP/Renew/*`, `RenewNow`),
ClientCertificateInstall CSP, RootCATrustedCertificates CSP, 1.3 MDE2 3.5 and 3.6 and the WSTEP
erratum, section 2 row "Cert renewal (ROBO)", section 6 Oliver Kieselbach SCEP deep dive, section
7 rows "Certificate renewal edge cases", "Client certificate back-dated", and the Fleet #32919 note
on one-shot SCEP certs, section 4 PKCS#7 handling and SCEP library choice.

Packages: `pki/wstep` (Renew), `pki/scep` (server side for `ClientCertificateInstall/SCEP`),
`pki/revocation` (optional CRL and OCSP as in go-apple-dm), `mdmprotocol/enroll` (Renew RST),
`schema`-driven command builders for `ClientCertificateInstall` and `RootCATrustedCertificates`.

Design:

- ROBO: a dedicated renewal URL (`My/WSTEP/Renew/ServerURL`) that requires client TLS with the
  existing MDM certificate, never redirects, accepts `RequestType` Renew with a `#PKCS7`
  `BinarySecurityToken` (signature by the old certificate, signing time inside the window, same
  issuer, same requester), issues with `NotBefore = now`, returns a provisioning doc with the new
  certificate, optional root, and `APPLICATION/PROVIDER-ID`; `EntDMID` must be present.
  Configuration for TLS termination at the server or a trusted proxy forwarding the client cert.
- Certificate recovery (MDE2 3.6) parsed and recorded; behaviour deferred to Phase 11 alongside
  `Recovery/*`.
- SCEP: `smallstep/scep`-based endpoint with one-time challenges; a certificate lifecycle model
  (issued, renew window, expired, removed) so `ClientCertificateInstall/SCEP/{UniqueID}` profiles
  can be renewed and cleaned up; `PFXCertInstall` with password encryption types.
- The known dual-store install bug is a documented constraint on the builder.

Decision records: `0019-certificate-renewal-and-recovery.md`, `0020-scep-and-certificate-delivery.md`
(the `smallstep/scep` exception).

Verification: simulator ROBO renewal happy path and each rejection (bad signature, outside window,
different requester, expired cert, HTTP redirect refused); SCEP enrollment through the simulator;
guest harness renewal with a short `RenewPeriod` when available.

## Phase 10: Entra identity paths, Terms of Use, Graph

Goal: be a registrable third-party MDM in an Entra tenant: federated enrollment through the web
authentication broker, token validation, the Terms-of-Use step, device and user token handling in
sessions, and compliance reporting where Microsoft permits it.

Inputs: research 1.8 (every entry), 1.4 Federated authentication page, Certificate authentication
page, GPO auto-enroll page, MDM enrollment of Windows devices (deep link), 1.9 Bulk enrollment
(ppkg targets), MS-MDM 2.1 notes 3 and 9 and 3.2.5.1.x, section 7 "Enrollment error codes"
(auto-enroll codes), section 10 question 8, section 3 Fleet's setup guide and `mde_auth.go`.

Packages: `msplatformservices/entra` (token validation, discovery documents, key cache),
`msplatformservices/graph` (compliance PATCH, `mobilityManagementPolicy` setup helper),
`mdmprotocol/enroll` (Federated and Certificate auth policies), `server/httpapi` (Terms-of-Use
and STS endpoints).

Design:

- Federated: discovery returns `AuthPolicy Federated` with `AuthenticationServiceUrl`; the server
  hosts the passive page for `?appru=<ms-app://...>&login_hint=<UPN>` that completes an OIDC login
  (pluggable identity provider, as go-apple-dm's ADE web view) and returns the HTML form posting
  `wresult` to the `appru`; GetPolicies and RST carry `DeviceEnrollmentUserToken` binary tokens
  that the server validates. For Entra join there is no discovery step: the discovery URL is
  provisioned in Entra and the token arrives opaquely.
- Terms of Use endpoint: `redirect_uri`, `client-request-id`, `api-version`, `mode=azureadjoin`;
  bearer token claims ObjectID, UPN, TID, Resource; return `IsAccepted=true&OpaqueBlob=...`;
  rendered in an iframe on Windows 11.
- Token validation: v1.0 and v2.0 tokens; after 2026-07-01 new apps get v2.0, so `aud` accepts
  only the app ID; tenant allow-list; key cache with refresh.
- Certificate auth policy: `X509v3` binary token plus XML-DSIG over the RST (`goxmldsig`
  evaluation), needed for WinDC on Entra-registered devices (Phase 12).
- Sessions: `Authorization: Bearer` and `DeviceToken` header handling; `AADUserToken` alert; the
  device-session versus user-session model from MS-MDM 3.2.5.1.1 recorded.
- Enrollment-type mapping: Entra joined gives `EnrollmentType Device` with the certificate in
  `My/System`; add-work-account gives `Full` with `My/User`; hybrid via GPO user credential;
  auto-enroll error codes (0x8018002b, 0x80180014, 0x8018000a, 0x80180026, 0x8007064c) documented
  with their causes.
- Graph: `PATCH /devices/{id}` with `isManaged` and `isCompliant`, gated behind the "approved MDM
  app" constraint (question 8 stays open; the constraint is recorded); a `dmctl entra setup`
  helper that creates the `mobilityManagementPolicy` (beta) with `discoveryUrl`, `termsOfUseUrl`,
  `complianceUrl`.
- Bulk provisioning packages: document go-microsoft-dm as a valid `OnPremise` and `Certificate` target;
  a `dmctl ppkg` builder is optional (local-mdm shows the format).
- The Entra "Disable MDM enrollment when adding work or school account" toggle and the Conditional
  Access partner limits (Windows not supported) are recorded as deployment constraints.

Decision records: `0021-entra-integration-and-federated-enrollment.md`, `0022-token-validation.md`,
`0023-compliance-reporting-constraints.md`.

Verification: httptest identity provider and Entra discovery documents; simulator federated
enrollment; token tests for v1, v2, wrong `aud`, wrong tenant, expired; Terms-of-Use round trip;
a tenant-backed manual test recorded in `docs/testing/`.

## Phase 11: Enrollment attestation

Goal: verify the TPM-backed attestation material a Windows 11 device offers at enrollment and over
DM, and let policy act on it, without any Intune dependency.

Inputs: research 1.8 (MDE2 RST catalogue; attestation entry with what is wire-exposed versus
Intune-gated), 1.4 DeviceStatus CSP (`CertAttestation/MDMClientCertAttestation`), DMClient CSP
`Recovery/*`, 1.3 MDE2 4.2.3 Azure Attestation and `DeviceAssociationMaaUrl`, section 4 TPM
libraries, section 6 the call4cloud attestation series, section 7 "0x80180032", section 9 bullet
on attestation, section 5 guestweave (vTPM produces the claims; only vendor EK chains need
hardware), question 1.

Packages: `pki/attestation`.

Design:

- Advertise `EnrollmentVersion` 5.0 or higher (configurable up to 9.0) so `AIKAttestationClaim`,
  `AIKPub`, `AIKCert`, `AadAIKAttestationClaim`, `AADPub`, `EmmDeviceId`, `AzureAttestationBlob`
  (6.0), `AttestationStatus` and `AttestationStatusHResult` (7.0), `AIKAlgorithm` (9.0) arrive;
  parse each into typed values with the raw bytes retained.
- Verify the AIK claim (an `NCryptCreateClaim` blob with the MDM private key as subject and the
  AIK as authority) against the CSR's public key; verify the AIK certificate chain against a
  configured trust set (Microsoft AIK CA roots for physical devices; the emulator's provisioned EK
  for guestweave guests); classify results (attested, unattested, unverifiable) and expose an
  admission hook so a deployment can refuse unattested enrollments with `CustomServerError` or
  accept and record.
- `DeviceStatus/CertAttestation/MDMClientCertAttestation` `Get` after enrollment, parsed and stored.
- `Recovery/InitiateRecovery` and `AllowRecovery` as queueable commands with the renewal path from
  Phase 9.
- `go-attestation` evaluated for the claim and certificate parsing; the decision record states
  what is implemented directly.

Decision records: `0024-enrollment-attestation.md` (mirrors go-apple-dm 0032).

Verification: fixtures from the Phase 7 vTPM guest at 5.0, 7.0 and 9.0; unit tests for chain
failures, key mismatch, missing items at lower versions; simulator producing synthetic claims
from `testpki`.

## Phase 12: Windows declared configuration (WinDC)

Goal: the first open-source server-side implementation of WinDC: linked enrollment, document
delivery, result and state tracking, drift and abandonment semantics, resource-access and
extensibility scenarios, and bulk templates.

Inputs: research 1.6 (all fourteen entries; the CSP page, DDF and the five protocol pages, with
the recorded contradictions), 1.4 DMClient `LinkedEnrollment/*` and `ConfigRefresh/*`, section 2
rows "WinDC dual enrollment", "WinDC documents", section 6 the MMP-C and dcsvc posts (for
behaviour only, Intune-gated), section 7 "WinDC common errors" row, section 9 "No open source
project implements WinDC", questions 5 and 6, section 5 SyncMLViewer MMP-C support.

Packages: `mdmprotocol/windc` (JSON discovery types, document model, result parsing, state
enum, the 1224 alert payload), `server/windcsync` (desired-state engine and delivery), the
`windc` server role.

Design:

- Linked enrollment: the primary MDM queues `Replace` on
  `./Device/Vendor/MSFT/DMClient/Provider/<ID>/LinkedEnrollment/DiscoveryEndpoint` and `Exec` on
  `.../LinkedEnrollment/Enroll`, then reads `EnrollStatus` (0 to 8) and `LastError`. The WinDC
  discovery endpoint accepts the JSON request (`enrollmentType` Device or User, `osVersion`
  required, optional `upn`, `tenantId`, `emmDeviceId`, `userDomain`; `MS-CV` and
  `client-request-id` headers) and answers with `EnrollmentServiceUrl`,
  `EnrollmentPolicyServiceUrl`, `AuthenticationServiceUrl`, `AuthPolicy` (Federated for joined,
  Certificate for registered; both from Phase 10), optional `EnrollmentVersion`,
  `ManagementResource`, `TouUrl`, and the `UPNRequired` error and `Retry-After` behaviours. The
  linked enrollment then runs the Phase 4 MDE2 flow against those URLs and gets its own session
  endpoint; the two enrollments are linked in storage.
- Document model: `<DeclaredConfiguration schema="1.0" context="Device|User" id="{GUID}"
  checksum="..." osdefinedscenario="...">` with bodies for `MSFTWiredNetwork`, `MSFTResource`,
  `MSFTVPN`, `MSFTWifi`, `MSFTInventory`, `MSFTClientCertificateInstall` (the `<CSP name><URI
  path type>` shape), `MSFTExtensibilityMIProviderConfig` and `MSFTExtensibilityMIProviderInventory`
  (the `<DSC namespace className><Key><Value>` shape), and the undocumented `MSFTPolicies` name
  recorded but not modelled. `checksum` is a server-controlled version string; the library uses a
  content hash by default.
- Delivery: `Replace` on `Host/Complete/Documents/{DocID}/Document` (and `Host/Inventory/...` for
  inventory scenarios); validation before queueing: `DocID` in the `LocURI` equals the `id`
  attribute; `context` matches the `./Device` or `./User` prefix; every CSP URI exists in the DDF
  (Phase 3); User scope needs an Entra user signed in.
- State: the 1224 alert with `Type com.microsoft.mdm.declaredconfigurationdocuments` carrying
  `<DeclaredConfigurations>` is parsed on every message into per-document `state`, `checksum`,
  `result_checksum`; `Host/Complete/Results/{DocID}/Document` is fetched when a permanent state
  (60 to 82) arrives and parsed into per-URI `status` and `state`; the `DCCSPURIState` values are
  constants.
- Lifecycle: abandon (`Properties/Abandoned` = 1) and unabandon; delete of the document (settings
  persist); resource ownership transfer (legacy MDM writes to the same resource fail with
  `0x86000031`) recorded as a conflict rule the server enforces on its own command queue;
  `ManagementServiceConfiguration/ConflictResolution` and the possibly absent `RefreshInterval`
  (question 6) probed, not assumed.
- Bulk template: `Host/BulkTemplate/Documents/{DocID}/Document` with `@#var#` placeholders and
  `<ReflectedProperties>`, `BulkVariables/Value` with `<InstanceBlob>`, results at
  `BulkTemplate/Results`; behind a capability flag until question 6 is settled on a real guest.
- Engine (`server/windcsync`): desired documents per enrollment, diffing against acknowledged
  state, push (Phase 8) on change, drift reports; mirrors go-apple-dm's `ddmsync`.
- Namespace (question 5): the engine emits `SYNCML:SYNCML1.2` by default with a per-enrollment
  switch to 1.1; the Phase 7 harness settles which the WinDC client requires.

Decision records: `0025-windc-linked-enrollment.md`, `0026-windc-document-model-and-state.md`,
`0027-windc-desired-state-engine.md` (mirrors go-apple-dm 0020 and 0022), each stating which Learn
page wins where the CSP page, DDF and protocol pages disagree.

Verification: simulator WinDC client (linked enrollment, alert emission, result documents, state
transitions, abandon); fixtures from the Learn samples; the guest harness enrolling a linked
enrollment against `dmserver` on a 26100 guest and applying an `MSFTWifi` or `MSFTVPN` document;
failure tests for each error class in the section 7 WinDC row (scope mismatch, DocID mismatch,
URI typo).

## Phase 13: CSP operation library: policy, apps, inventory, lifecycle

Goal: typed, schema-validated builders and result parsers for the operations a management product
needs, so consumers do not hand-write SyncML.

Inputs: research 1.4 (Policy CSP, ADMX pages, EnterpriseDesktopAppManagement, EnterpriseModernAppManagement,
Win32AppInventory, DeviceStatus, DevDetail, DevInfo, DeviceManageability, RemoteWipe, Reboot,
EnrollmentStatusTracking, Collect MDM logs and DiagnosticLog), 1.5 ADMX ingestion, 1.9 ESP and
FirstSyncStatus, section 3 marcosd4h `sample_syncml_commands/`, section 5 authoring helpers,
section 7 rows on Exec-only nodes and destructive reapply.

Packages: `mdmprotocol/mdm/ops` (or per-area subpackages) on top of `schema/`.

Design:

- Policy CSP: `Policy/Config/{Area}/{Policy}` `Replace` and `Delete` with allowed-value checks;
  ADMX-backed payload encoder (`<enabled/>`, `<disabled/>`, `<data id value/>` with the element
  type rules and `&#xF000;` separators; normalise casing); `ADMXInstall` ingestion builder with the
  derived area name `{AppName}~{SettingType}~{Category~Path}`; `Policy/Result` reads.
- Apps: MSI install via `EnterpriseDesktopAppManagement` (Add then Exec with `MsiInstallJob`,
  `AtomicRequired`, SHA-256 validation, per-user vs per-machine context) with the async 1224
  `win32csp_install` alert parsed into a completion; modern apps via `EnterpriseModernAppManagement`
  (`StoreInstall`, `HostedInstall`, inventory, licences); `Win32AppInventory` reads.
- Inventory: `DevInfo`, `DevDetail`, `DeviceStatus` (all subtrees, Windows 11 note on `OSPlatform`),
  `DeviceManageability/Capabilities/CSPVersions` parsed into typed facts.
- Lifecycle: `RemoteWipe` `Exec` variants, `Reboot` now and schedules, `DiagnosticLog` collection,
  `Unenroll`, `DMClient` polling and `ConfigRefresh` settings, `FirstSyncStatus` population
  (`ExpectedPolicies`, `ExpectedMSIAppPackages`, `ExpectedModernAppPackages`, `ExpectedSCEPCerts`,
  `IsSyncDone`, `WasDeviceSuccessfullyProvisioned`) to feed the built-in OOBE progress page.
- Desired-state helpers so unchanged policy is not re-sent (the Audit Policy CSP reapply pitfall).

Decision records: `0028-csp-operations-library.md`.

Verification: builders validated against the schema for every node they touch; simulator scenarios
per area; guest-harness spot checks for `Reboot`, one Policy area, one ADMX-backed policy, one MSI.

## Phase 14: WBXML encoding

Goal: honour `DEFAULTENCODING=application/vnd.syncml.dm+wbxml` if a deployment selects it.

Inputs: research 1.2 (RepPro 1.2.2 section 8 code pages and tokens, MetaInfo code page 01, the
W3C WBXML NOTE), 1.1 RepPro 1.2.1 5.2, section 4 WBXML codecs (`remdev/go-activesync` as the
modern pure-Go reference), section 9 ("No open source Go server implements WBXML"), section 10
(WAP-192 not fetched).

Packages: `mdmprotocol/syncml/wbxml`.

Design: a token-table codec for code pages 00 (SyncML) and 01 (MetInf) with PUBLICID 0x1201,
accepting WBXML 1.1 to 1.3 on input; the Phase 5 415 branch becomes a content negotiation; the
provisioning doc gains the option. Fetch WAP-192 and pin it before starting.

Decision record: `0029-wbxml.md`.

Verification: round-trip every XML fixture through WBXML; fuzz the decoder; the simulator gains a
WBXML mode; the guest harness enrolls with WBXML selected.

## Phase 15: Operations: telemetry, audit, admin API, security documentation

Goal: what a deployment needs around the protocol: observability, audit, administration, secrets
at rest, and the threat model.

Inputs: go-apple-dm decisions 0011, 0013, 0034, 0037, 0038, 0040; research section 7 operational
rows (missing `dmwappushservice`, retention, proxy TLS bridging), section 10 "Not covered".

Deliverables: OpenTelemetry spans and metrics on the enrollment, session, push and WinDC paths;
event sinks with default-deny redaction; persisted audit trail; admin API with principals and
policy; secrets at rest for WNS secrets, Entra client secrets, MD5 credentials and PFX passwords;
`docs/security/threat-model.md`; `docs/operations/` guides (enrollment security, proxy and TLS
termination, WNS credentials, Entra registration); `docs/architecture.md` describing the
implemented system; diagrams.

Decision records: `0030-telemetry-seam.md`, `0031-event-sinks-and-audit.md`,
`0032-admin-api-and-authorization.md`, `0033-secrets-at-rest.md`.

Verification: coverage gate; a release through release-please; documentation link check.

---

## Open questions map

From research section 10. Each question is owned by a phase; the answer goes into the research
store (dated) and the named decision record.

| # | Question | Owning phase | How it is settled |
|---|---|---|---|
| 1 | Which `EnrollmentVersion` to advertise | 4 (default 3.0), 7 (captures at 5.0 and 9.0), 11 (raise) | Guest captures of `AdditionalContext` per version |
| 2 | Does `Poll/NumberOfFirstRetries = 0` mean infinite | 7 | Guest observation over 24 hours |
| 3 | Partner Center versus Entra WNS credentials | 8 | Both token sources implemented; configuration selects |
| 4 | Event 4603 on 24H2 and 25H2 | 8, with the Phase 7 harness | Push a guest, record whether the session runs |
| 5 | `SYNCML:SYNCML1.1` in WinDC | 7 (management), 12 (WinDC) | Send both, record acceptance |
| 6 | `RefreshInterval`, `BulkTemplate`, User-scope nodes on 26100 | 7 (probe), 12 (behaviour) | `Get` on each node on a real guest |
| 7 | Relationship to go-sdk-windowscsp | 3 | Decision record 0007; reference only |
| 8 | "Approved MDM app" for Graph `isCompliant` | 10 | Recorded as a constraint; feature gated |
| 9 | Anything new for MDM in 26H2 | 7 | `CSPVersions` and DDF diff between 26100 and 26300 guests |

## Pitfall coverage map

From research section 7. Each row must have at least one named test by the time its phase is done.

| Pitfall row | Phase | Test or mechanism |
|---|---|---|
| WNS 4603 on 24H2 and 25H2 | 8 | Non-empty body, cache policy, poll fallback, server-side arrival check |
| `dmwappushservice` missing | 8, 15 | "Has not checked in" alert; prerequisite documented |
| Aggressive polling | 4, 13 | Microsoft default schedule in the provisioning doc; desired-state diff helper |
| Add and Replace only | 5, 13 | All verbs and both scopes from the first session engine |
| Raw envelope storage | 5, 6 | Per-command results; bounded opt-in debug ring |
| Same-second ordering | 5, 6 | Monotonic sequence column; ordering test |
| Back-dated certificate, no ROBO | 4, 9 | `NotBefore = now`; renewal endpoint with client TLS |
| Duplicate `HWDevID` | 4 | Keyed on certificate and DeviceID; conflict event |
| User-scope before sign-in | 5 | `LoginStatus` hold; single-scope Atomic rule |
| WinDC common errors | 12 | DocID, context and URI validation before queueing |
| Enrollment error codes | 4, 10 | Exact fault subcode mapping; auto-enroll codes documented |
| `0x80180032` attestation | 11 | Admission hook on attestation result |
| Large objects and `MaxMsgSize` | 2, 5 | Chunk primitives; `LrgObj` and `MaxSegLen` read before chunking |
| Renewal edge cases | 9 | Client TLS, no redirect, PKCS#7 checks, proxy guidance |
| SyncML 1.1 vs 1.2 and Atomic rules | 2, 5 | Emit 1.2 parse both; DTD order; no CDATA; builder rejects invalid Atomics |
| Transport | 4, 5 | `Content-Length` on enrollment; headers parsed and logged, not enforced |
| ChannelURI renewal | 5, 8 | `Get` on `Push/ChannelURI` every session; 404 and 410 handling |
| `./User` on Entra-joined; unenroll limits | 5, 10 | `LoginStatus` gate; wipe or Entra-side removal documented |

## Decision record index to be created

Numbers are reserved here so phases can cross-reference before the files exist.

| Number | Title | Phase |
|---|---|---|
| 0001 | Library-first architecture with a generated schema core and WinDC as an extension of MDM | 0 |
| 0002 | Pinned references and re-check triggers | 0 |
| 0003 | Reference projects and dependency policy | 0 |
| 0004 | Go 1.27 baseline and foundation contracts | 1 |
| 0005 | SyncML message model and XML policy | 2 |
| 0006 | Schema generator over DDF v2 | 3 |
| 0007 | Relationship to go-sdk-windowscsp | 3 |
| 0008 | Enrollment protocol and the provisioning document | 4 |
| 0009 | WSTEP CA and CSR handling | 4 |
| 0010 | Storage interfaces and the contract suite | 4 |
| 0011 | Management session engine | 5 |
| 0012 | Command queue, results and retention | 5 |
| 0013 | Scope and the user channel | 5 |
| 0014 | SQL storage backends | 6 |
| 0015 | Reference server roles and configuration | 6 |
| 0016 | `dmctl` structure | 6 |
| 0017 | Conformance testing with guestweave | 7 |
| 0018 | WNS push and poll policy | 8 |
| 0019 | Certificate renewal and recovery | 9 |
| 0020 | SCEP and certificate delivery | 9 |
| 0021 | Entra integration and federated enrollment | 10 |
| 0022 | Token validation | 10 |
| 0023 | Compliance reporting constraints | 10 |
| 0024 | Enrollment attestation | 11 |
| 0025 | WinDC linked enrollment | 12 |
| 0026 | WinDC document model and state | 12 |
| 0027 | WinDC desired-state engine | 12 |
| 0028 | CSP operations library | 13 |
| 0029 | WBXML | 14 |
| 0030 | Telemetry seam | 15 |
| 0031 | Event sinks and audit | 15 |
| 0032 | Admin API and authorization | 15 |
| 0033 | Secrets at rest | 15 |

## Out of scope, restated

From research Purpose and section 10 "Not covered": Windows 10 and non-desktop editions; generic
telecom and IoT OMA-DM; Windows 365 and AVD multi-session policy beyond parsing; Intune-internal
services (MMP-C hosting, the EPM agent, device inventory agent, IC3); WIP and MAM-only enrollment;
the wider Graph Intune API; Autopilot v1 registration, self-deploying, pre-provisioning, device
preparation v2 and device association (all Intune-gated today; go-microsoft-dm participates only as the
Entra auto-enrollment target, recorded in Phase 10). Watch items: Autopilot device preparation's
promised third-party support; any Microsoft statement on WNS credentials for DMClient.

## Working agreements for agents picking this up

- Start from the Status table. Do not start a phase whose predecessors are not `done` unless the
  user says so; Phases 8 to 11 and 13 to 15 can run in parallel after Phase 7, and Phase 12 needs
  Phase 10's Certificate auth policy for Entra-registered devices only.
- The research store is the source of normative detail. When this plan and the research store
  disagree, the research store wins and this plan is fixed.
- Every phase produces: code with `doc.go`, tests including failure paths and the pitfall tests
  named above, its decision records, and an update to `docs/architecture.md` describing what is
  now implemented.
- Never import the server module from the library; never import `third_party/refs`; never
  hand-edit generated files; regenerate with `make generate` and check with `make verify`.
- Record every real-client observation in the research store with the build number and date.
