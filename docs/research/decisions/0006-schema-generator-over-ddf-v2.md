# 0006: Schema generator over DDF v2

## Context

The Windows client's management tree is described only by Microsoft's DDF v2 bundle: 313 XML
files, one per configuration service provider or Policy area, published roughly twice a year
as a zip. Every later phase needs to know whether a URI exists, what format and values it
takes, which verbs it permits, whether it is scoped to a user, when it appeared in Windows and
whether writes to it must be atomic. Hand-maintaining that for 5,955 nodes is not an option,
and the "What's new in MDM" page that used to list per-version changes stopped at 22H2.

## Decision

`internal/schemagen` parses the pinned bundle and `cmd/ddfgen` renders the schema tier from
it; `make verify` regenerates into memory and fails on any byte of difference or any locked
exported identifier that disappeared without an entry in `schema/ALLOWED_REMOVALS.md`.

The parser reads the DDF elements in no namespace (the bundle never declares the tempuri.org
default namespace its XSD names, and a namespaced file is refused) and the `MSFT:` extensions
by namespace URI. It strips the UTF-8 BOM, ignores the DOCTYPE, requires `Path` on root nodes
and refuses it elsewhere, trims the `./Vendor/MSFT/` typo on SUPL's root, turns an empty
`NodeName` plus `DFTitle` into a `{Title}` segment, propagates `MSFT:Applicability` to
children that have none, splits `OsBuildVersion` and `EditionAllowList` into lists, and
refuses unknown `DFFormat` and `AllowedValues ValueType` values so a new drop that adds one
fails loudly. `MSFT:DependencyChangedAllowedValues` inside a dependency group is modelled as
the values that apply when the group's dependencies hold.

`schema/csp` is the runtime model: `Tree`, `Node` and the extension types, `Lookup` that
resolves a concrete URI segment by segment (case-insensitive unless a node says `CS`, dynamic
nodes matching any segment and capturing it, `?prop=` and `?list=` ignored, a legacy root
matching with or without `./Device`), `Registry` over every tree with longest-root-first
resolution so a Policy area beats the Policy CSP skeleton, and `Inherit`, which the generated
literals call to restore inherited applicability.

Generated output: one package per CSP under `schema/csp/<name>` and per Policy area under
`schema/policy/<area>` (package names are the lower-cased alphanumerics of the DDF name), each
with `tree.gen.go` (the full node table as a `csp.Tree` literal, `DeviceTree` and `UserTree`
when a file has both roots), `uris.gen.go` (a constant per static node and a function per
node under a dynamic segment, with `url.PathEscape` on each parameter), `values.gen.go` (a
constant per ENUM and Flag entry, named from the first four words of its description or its
value) and `doc.gen.go`; `schema/registry` imports every package and joins the trees;
`schema/GENERATED_FROM.json` records the bundle, its SHA-256, `Last-Modified`, file, tree and
node counts; `schema/EXPORTED_IDENTIFIERS.lock` lists every exported name. Identifiers join
the static segments below the root; nodes that collide are told apart by their dynamic
titles (`XByProfileName`), then by a number; the names the package itself defines (`Root`,
`Tree`, `Trees`, `escape` and their scoped forms) are reserved, so `RootCATrustedCertificates/Root`
becomes `DeviceRootNode`.

`schema/support` resolves applicability: `OsBuildVersion` entries are classified as released,
Insider (25000, 26000, 25145, 25965), Server (20348, 25398) or unreleased (99.9.99999,
99.9.9999, 88.8.88888); the two nodes stamped `11.0.` are read as `10.0.`; 24H2, 25H2 and
26H2 share the 26100 branch and 22H2 and 23H2 the 22621 branch, so a node introduced in
26100.3613 applies to a 26300.9278 device; `SupportedOn` returns the decision, the entry that
decided it and a one-sentence reason, and reports deprecation against `OsBuildDeprecated`.

`schema/validation` checks a command before it is queued: the node exists, the verb is in
its `AccessType`, interior nodes carry no value, the value parses for its format, ENUM, Flag
(bitwise), Range (with the `[a-b],[c]` alternatives the bundle writes) and RegEx values are
evaluated, delimited lists are split first, ADMX payloads are checked structurally against
the Learn grammar (`<enabled/>`, `<disabled/>`, `<data id value/>`), `UniqueName` patterns
are applied to captured dynamic segments, `AtomicRequired` on the node or an ancestor is
reported, and XSD, SDDL and JSON constraints are reported as present but not evaluated.

`ddfgen diff old.zip new.zip` lists files and nodes added and removed and nodes whose
format, access, applicability, allowed values, default, deprecation or description changed,
which is how the next drop is reviewed.

## Rationale

Generating from the pinned zip, verified by hash, keeps the schema tier reproducible and
makes each Microsoft drop a reviewable diff, which the research store identifies as the only
remaining per-version change source. One package per CSP keeps a consumer that needs DMClient
from linking 5,955 nodes; the registry exists for the server, which needs them all. Rendering
trees as literals rather than embedding the XML means no parser runs at startup and the data
is inspectable in source; restoring inheritance at init keeps the literals a third of the
size they would be with every ancestor's applicability copied down. A hand-written runtime
model with generated data, rather than generated types per CSP, keeps validation and support
logic in two testable packages.

## Constraints

The bundle census the parser produces differs from the research store's inspection in three
counts: 2,516 ADMX, 1,202 ENUM and 220 Range allowed values (the store's grep also counted
`DependencyChangedAllowedValues`), and 64 `UniqueName` naming rules on dynamic nodes (one
more sits on the named node `RootCATrustedCertificates/.../EncodedCertificate`). 87 files
carry both a User and a Device root, not 109. Six dynamic nodes carry no naming rule.
ADMX-backed values cannot be validated beyond the payload grammar without the ADMX files,
which the bundle does not ship; a deployment that wants full validation needs
`C:\Windows\PolicyDefinitions`. Edition allow lists are membership tests over hex ids that
Microsoft does not document. The `DeclaredConfiguration` DDF lacks `RefreshInterval`,
`Host/BulkTemplate` and the User root the protocol pages describe (open question 6); the
conformance test asserts their absence so a drop that adds them is noticed. Generated
packages are exempt from the per-package coverage gate; the registry conformance test
resolves every generated node URI, dynamic segments included, back to its own node.
Regenerating 1,057 files takes about a second; `go build ./schema/...` about ten.

## Verification

`internal/schemagen` tests pin the bundle census (313 files, 400 trees, 52 CSP files, 261 area
files, 87 dual-scope files, 5,955 nodes, 5,146 leaves, 132 dynamic nodes, the format and
extension censuses), cover every parser refusal, prove generation is deterministic, and
exercise Write, Verify (tampered file, stale lock entry, allowed removal, extra and missing
generated files) and Diff. `schema/csp`, `schema/support` and `schema/validation` tests cover
lookup, branches, sentinels, deprecation and every validation rule. `schema/registry`'s
conformance test checks the counts, resolves all 5,955 node URIs, and names the DMClient,
DevDetail, DevInfo, DMAcc, DeclaredConfiguration, Policy, Reboot, RemoteWipe and
EnterpriseDesktopAppManagement nodes the later phases use. `make verify` passes on the
checked-in output.

## References

- [internal/schemagen](../../../internal/schemagen), [cmd/ddfgen](../../../cmd/ddfgen)
- [schema/csp](../../../schema/csp), [schema/support](../../../schema/support), [schema/validation](../../../schema/validation), [schema/registry](../../../schema/registry)
- [schema/GENERATED_FROM.json](../../../schema/GENERATED_FROM.json), [schema/ALLOWED_REMOVALS.md](../../../schema/ALLOWED_REMOVALS.md)
- [Research store](../../research.md), section 1.5 (DDF bundle inspection) and section 0 (build lineage)
- Microsoft, DDF files and the DDF v2 XSDs: <https://learn.microsoft.com/en-us/windows/client-management/mdm/configuration-service-provider-ddf>
- Microsoft, Understanding ADMX policies: <https://learn.microsoft.com/en-us/windows/client-management/understanding-admx-backed-policies>
- OMA DM Tree and Description 1.2.1: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_TND-V1_2_1-20080617-A.pdf>

Reference source identifiers and paths (relative to the named project):

- `deploymenttheory/go-apple-dm`, `internal/schemagen/generate.go` (`Run`, `Write`, `Verify`, `mergeLock`), `cmd/admgen/main.go`, `schema/ALLOWED_REMOVALS.md`
- `deploymenttheory/go-sdk-windowscsp`, `internal/ddf/parse.go`, `internal/ddf/model.go`, `internal/codegen/generator.go`, `windowscsp/csp/dmclient/` (per-CSP packages with URI functions and enum types; see decision record 0007)
- `mattrax/Mattrax` (`rust` branch), `crates/ms-ddf/src/ddf_v2.rs`, `msft.rs`
- `mjoliver/Windows-MDM`, `cmd/ddf-compiler` (DDF compiled to a policy catalog)
