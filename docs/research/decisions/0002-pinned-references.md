# 0002: Pinned references and re-check triggers

## Context

Windows 11 version 26H2 is the compliance target. It ships as an enablement package on the
shared 24H2 servicing branch, so nothing in it adds MDM, CSP or WinDC surface beyond build
26100.x on the current cumulative update, and the "What's new in MDM" page stopped at 22H2.
Every normative detail therefore has to come from a specific revision of a specific document,
and the project has to notice when a document moves.

## Decision

The revisions in force are:

| Reference | Pinned revision | Role |
|---|---|---|
| MS-MDE2 | v19.0, 2026-08-11 | Enrollment wire protocol; supersedes every Learn enrollment page |
| MS-MDM | v15.0, 2024-04-23 | Windows subset of OMA DM; management wire protocol |
| MS-XCEP | v18.0, 2026-03-09 | Enrollment policy (GetPolicies), profiled by MS-MDE2 3.3 |
| MS-WSTEP | v15.0, 2024-04-23 | RST and RSTRC, profiled by MS-MDE2 3.4 |
| OMA DM 1.2.1 enabler | V1_2_1-20080617-A | Protocol, RepPro, Security, StdObj, TND, Bootstrap; the ERELD is the pin manifest |
| OMA SyncML Common 1.2.2 | V1_2_2-20090724-A | RepPro (DTD order, status codes, WBXML tokens), MetaInfo, HTTP binding |
| OMA DM w7 characteristic | ac_w7_dm V1_0_1 | Bootstrap parameters, with the Microsoft extensions in MS-MDE2 2.2.9.5 |
| DDF v2 bundle | DDFv2Feb2026.zip, Last-Modified 2026-02-19 | The only machine-readable CSP description; schema source |
| Learn client-management pages | ms.date 2025-08-04 bulk commit unless noted | Windows behavior the specifications omit |
| Windows 11 release information | ms.date 2026-08-27 | Build to KB to date mapping for `OsBuildVersion` values |

Two manifests make the pins checkable. `third_party/specs/MANIFEST.json` records the URL,
revision, date and SHA-256 of every document; `make specs` downloads and verifies them and
records a hash only when `SPECS_RECORD=1` is set. `third_party/ddf/MANIFEST.json` records the
URL, SHA-256, size, HTTP `Last-Modified`, ETag, top folder and file count of every DDF drop;
the drops themselves are committed beside each other and never replaced, and `make verify`
checks them through `internal/schemagen`.

Re-check triggers: 26H2 general availability; the next DDF drop (historically February, July
and September); any change on the MS-MDE2 landing page (four revisions in the twelve months
to 2026-08); a new "What's new in MDM" table on Learn; and, for the patent position in
decision record 0001, each of the above. A re-check that changes a pin updates the research
store with the date, this record, and the manifests.

## Rationale

The "newest wins" rule of the research store only works when the newest is known. Pinning by
hash rather than by URL catches the silent in-place republication Microsoft has used since
January 2024 instead of errata. Keeping every DDF drop under version control lets the next
drop be diffed, which is now the only per-version delta source for CSPs.

## Constraints

The Microsoft PDFs and OMA documents are downloaded on demand and not committed. The DDF
bundle is committed because it is small, is the schema source and has no reproduction
restriction. The Learn pages are not pinned by hash; their `ms.date` and `gitcommit` front
matter are recorded in the research store instead. `MS-MDE2` revisions arrive without a
change narrative that extracts cleanly, so a revision bump is read in full.

## Verification

`make specs` succeeds with the recorded hashes (run 2026-09-07). `make verify` and
`internal/schemagen`'s `TestPinnedBundleMatchesManifest` check the DDF pin; the bundle facts
match the research store's inspection (727,650 bytes, `DDFDrop012026/`, 313 XML files).

## References

- [third_party/specs/MANIFEST.json](../../../third_party/specs/MANIFEST.json)
- [third_party/ddf/MANIFEST.json](../../../third_party/ddf/MANIFEST.json)
- [internal/schemagen/bundle.go](../../../internal/schemagen/bundle.go)
- [scripts/specs.sh](../../../scripts/specs.sh), [scripts/ddf.sh](../../../scripts/ddf.sh)
- [Research store](../../research.md), sections 0, 1 and 9
- Implementation plan, "Target and pinned references"
