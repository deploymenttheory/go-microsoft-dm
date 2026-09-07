# third_party

Pinned upstream material. Nothing here is imported by Go code; the schema tier
is generated from `ddf/` by `cmd/ddfgen`, and everything else is read by people.

| Directory | Committed | Populated by | Contents |
|---|---|---|---|
| `ddf/` | yes | `make ddf DDF_URL=...` adds a drop; `make verify` checks the pins | Microsoft's DDF v2 bundles, one zip per drop, never replaced, with `MANIFEST.json` recording URL, SHA-256, size, HTTP `Last-Modified`, ETag, top folder and file count. |
| `specs/` | manifest only | `make specs` (`SPECS_RECORD=1` to record new hashes) | The OMA DM 1.2.1 and SyncML Common 1.2.2 documents, the four Microsoft Open Specifications (MS-MDE2, MS-MDM, MS-XCEP, MS-WSTEP) and the W3C WBXML note, verified against `MANIFEST.json`. Downloads are gitignored because the documents carry their publishers' own use terms. |
| `refs/` | no | `make refs` | Shallow clones of the reference implementations listed in `scripts/refs.sh`, with `COMMITS.txt` recording the commit each was read at. Read-only: never a dependency, never copied. See decision record 0003. |

Which revision of each document is in force, and when to re-check, is recorded in
[decision record 0002](../docs/research/decisions/0002-pinned-references.md).
