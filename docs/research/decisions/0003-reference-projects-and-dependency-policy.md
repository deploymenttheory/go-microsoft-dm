# 0003: Reference projects and dependency policy

## Context

Every open Windows MDM server hand-rolls SOAP and SyncML, two of them vendor Go's x509
request parser, one patches GOROOT, and their issue trackers record the operational mistakes
the research store catalogues in section 7. They are the best available evidence of how the
Windows client behaves and the worst available basis for a dependency.

## Decision

The repositories `scripts/refs.sh` clones under `third_party/refs/` are read-only references:
Fleet, local-mdm, mjoliver/Windows-MDM, marcosd4h/mdm and MDMatador, oscartbeaumont/windows_mdm,
Mattrax and the PhenixH fork, dejiblue/windows-mdm, NachoMDM, the WSO2 and Entgra plugins,
SyncMLViewer, TameMyCerts.WSTEP, mattrax/xml, go-activesync, go-wns, pytune, AADInternals,
csp-builder, smallstep/pkcs7, smallstep/scep, go-attestation, Imitune and kapytein/syncml.
None is a code dependency. Licence positions: mjoliver/Windows-MDM, NachoMDM and
apexaegis carry no licence (all rights reserved; read for protocol knowledge only); Imitune
and kapytein/syncml are GPL-3.0 (never vendored); Mattrax's current branch is source-available,
not open source; the PhenixH fork is AGPL-3.0; Fleet is MIT with a proprietary `ee/` tree.
`third_party/refs/` is gitignored and carries a `go.mod` so no clone is ever part of `./...`.

Candidate exceptions, each requiring a decision record before its first import:
`github.com/smallstep/pkcs7` for the PKCS#7 renewal token and detached `MS-Signature`
(Phase 4 record); `github.com/smallstep/scep` for `ClientCertificateInstall/SCEP` (Phase 9
record; preferred over `micromdm/scep`). To be evaluated when their phase arrives:
`google/go-attestation` and `go-tpm` (Phase 11); `russellhaering/goxmldsig` (Phase 10);
`mattrax/xml` (Phase 2, adopted only if a failing test proves Go 1.27's encoder cannot
produce the SOAP namespace shape). OpenTelemetry API modules are accepted for the telemetry
seam (decision record 0004). SQL drivers live in the server module only.

`deploymenttheory/go-sdk-windowscsp` is a reference, not a dependency (user decision
2026-09-06). It proves that DDF v2 can drive generated Go; whether this project depends on it,
absorbs it or stays independent is decided in the Phase 3 record (0007) once the schema tier
has a shape. `deploymenttheory/go-sdk-appleservices` is not used, matching go-apple-dm's rule.

Before implementing a feature, read at least two references for it (the research store names
them and their file paths), record what they do and where they fail in the feature's decision
record, and add a failing-path test for the failure.

## Rationale

The references are valuable for the client behaviors they encode and the mistakes they
document; neither transfers through an import. A dependency on a product's server package
would pull its storage, licensing and release cadence into a library. The small, single-purpose
smallstep packages are the only candidates whose scope matches the need.

## Constraints

The GPL and unlicensed references must never be copied, only read. `make refs` needs network
access and clones shallowly; `third_party/refs/COMMITS.txt` records the commit each reference
was read at, and decision records cite those commits. `make refs-activity` lists references
pushed in the last thirty days so the research store's status column can be kept current.

## Verification

`internal/layout` and `go list -m all` show no reference module in either `go.mod`.
`TestLibraryNeverImportsServer` and the tier tests guard the module boundary. The refs
`go.mod` keeps `go vet ./...` and `go test ./...` out of the clones.

## References

- [scripts/refs.sh](../../../scripts/refs.sh), [scripts/refs-activity.sh](../../../scripts/refs-activity.sh)
- [third_party/README.md](../../../third_party/README.md)
- [Research store](../../research.md), sections 3, 4, 5, 8 and 9
- Implementation plan, "Dependency policy"

Reference source identifiers and paths (relative to the named project):

- `fleetdm/fleet`, `server/mdm/microsoft/wstep_csr.go` (vendored x509 parser; `isPrintable` admits `!` and `0x00`), `server/mdm/microsoft/wstep.go`, `server/service/microsoft_mdm.go`
- `oscartbeaumont/windows_mdm`, `patch/patch.go` (GOROOT patch), `mdm_manage.go` (regex SyncML parsing)
- `Malcolm/local-mdm`, `internal/platform/windows/wns.go` (the one working WNS client)
- `mattrax/Mattrax` (`rust` branch), `crates/ms-mdm/src/` (typed model to follow)
- `deploymenttheory/go-sdk-windowscsp`, `internal/ddf/parse.go`, `internal/codegen/generator.go`
