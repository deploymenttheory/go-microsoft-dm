# 0005: SyncML message model and XML policy

## Context

Every message between a Windows client and the server is a SyncML 1.2 document carrying the
OMA DM subset MS-MDM defines. The session engine (Phase 5), the command builders (Phase 13)
and the declared-configuration engine (Phase 12) all need one typed model that preserves
command order, survives what the real client sends, and produces what the real client
accepts. The reference implementations show the failure modes: regex extraction of SessionID
and MsgID (oscartbeaumont), one slice per command type that loses document order (Fleet's
`SyncBody`), and CDATA that only works because it is stripped before sending (Fleet's guide).

## Decision

`mdmprotocol/syncml` holds one Go type per element: `Message{Namespace, Header, Body}`,
`Header` in DTD order (VerDTD, VerProto, SessionID, MsgID, Target, Source, RespURI?, NoResp?,
Cred?, Meta?), `Body{Commands []Command, Final, MSFTNamespace}` where `Command` is sealed and
implemented by `Add`, `Alert`, `Atomic`, `Delete`, `Exec`, `Get`, `Replace`, `Results`,
`Sequence` and `Status`; `Item{Target, Source, Meta, Data, MoreData}`; `Data{Value, XML,
OriginalError}`; `Meta{Format, Type, Mark, Size, NextNonce, MaxMsgSize, MaxObjSize}`; `Cred`
and `Chal`. Alert and status codes, alert `Type` strings, formats, marks and authentication
types are constants with their exact wire spelling.

Encoding is a hand-written writer over `encoding/xml`'s text escaper, not struct tags: it
emits elements in the order the SyncML RepPro DTD gives (with MS-MDM's `Cmd` inside
`Results`), `xmlns="SYNCML:SYNCML1.2"` on the root, `xmlns="syncml:metinf"` on every Meta
child, `<Final/>` and `<NoResp/>` as empty elements, `xmlns:msft` on SyncBody when asked,
`msft:originalerror` on Data, entity-escaped character data and never CDATA. `Data.XML` is
written verbatim for the payloads MS-MDM 2.2.5.1 allows as markup. A message decoded from a
`SYNCML:SYNCML1.1` document is re-encoded as 1.1 so a client is answered in the namespace it
used; everything else is 1.2.

Decoding is a recursive-descent parser over `encoding/xml` tokens. It is strict about
structure: only DTD elements at each position, `Final` last, `Copy`, `Map`, `Move`, `Put`,
`Search` and `Sync` refused with `ErrUnsupportedCommand`, `TargetParent` and `SourceParent`
refused, `Status` and `Results` refused inside `Atomic` and `Sequence`, a size limit
(8 MiB by default) enforced before parsing. It is lenient about lexical detail Microsoft's own
samples and the real client exhibit: either namespace or none on the root, Meta children
with or without the metinf namespace, whitespace around identifiers and LocURIs, CDATA
normalised to text, unknown Meta children skipped, a `utf-16` declaration on UTF-8 bytes
accepted, and real UTF-16 transcoded before parsing. Markup inside `Data` is captured from
the source bytes verbatim.

`Validate` is separate from `Decode` so fixtures and fragments can be round-tripped without
a header, and so the session engine reports every broken rule at once: versions, SessionID,
numeric MsgID, routing, non-empty body, CmdID present, not "0" and unique across nested
commands, item counts (Exec exactly one), LocURI and node-name rules from the Learn OMA DM
page, known alert codes with Items carrying Meta/Type for 1224 and 1226, Status references
and numeric codes, known formats, and the Atomic rules the client enforces with 500 and 507:
no nested Atomic, no Get inside Atomic (also through a Sequence), no Add then Replace on one
node.

Authentication helpers implement OMA DM Security 5.3.2 exactly: `MD5Digest` is
`B64(H(B64(H(username:password)):nonce))` with the nonce as raw bytes; `NewMD5Chal` puts the
nonce on the wire in base64; `VerifyMD5` compares in constant time against a stored
credential hash. Large-object helpers split an Item on rune boundaries with `MoreData` and
`Size`, and `Assembler` reassembles chunks with a size bound and a typed error when another
Item interrupts. Response helpers build the header for a reply, `Status` for the SyncHdr and
for a command, the "Next Message" shape (Status plus Alert 1222, no Final) and the abort
shape (Alert 1223), and read package 1 (`IsPackageOne`, `DevInfo`, `LoginStatus`).

`mattrax/xml` is not adopted. The evaluation in Phase 1's scratch probe and this package's
tests shows Go 1.27's `encoding/xml` decodes both namespaces and emits the metinf namespace
correctly; the only shape it cannot produce with struct tags is the self-closing `<Final/>`,
which a hand-written writer produces anyway. WBXML remains Phase 14.

## Rationale

An ordered command slice is the only representation that lets a server answer Status elements
in request order, which MS-MDM 2.2.6.1 requires. A hand-written writer costs about the same as
the struct tags plus custom marshallers the ordered slice would need, and it makes the exact
bytes a matter of tests rather than of encoder behaviour. Separating lexical leniency from
semantic validation means the fixtures from Learn, which omit headers and namespaces, exercise
the codec while the engine still refuses what the protocol forbids. Preserving markup inside
Data verbatim is what the MSI install job, the DiagnosticLog collection and every WinDC
document need; escaping it would corrupt the payload the client parses.

## Constraints

A decoded fragment without a root namespace is written as 1.2. Unknown Meta children are
dropped, not preserved, so a client extension in Meta does not survive a round trip. The
encoder writes empty required header elements for an incomplete header rather than failing;
Validate is the gate. Whether the WinDC client requires 1.1 is open question 5 and is settled
on a real guest in Phases 7 and 12; the per-message namespace field is the switch. The
MaxMsgSize the Windows client honours is unpublished; the 8 MiB decode limit is a
denial-of-service bound, not a protocol value. The encoder's compact form is what is sent;
indentation exists for logs and fixtures only.

## Verification

`mdmprotocol/syncml` tests: every fixture in `testdata/` (Learn OMA DM alert, Fleet's package 1
and unenroll, MS-MDM 3.1.5 examples, OMA Security 5.3.1 header, DiagnosticLog and
EnterpriseDesktopAppManagement samples, the WinDC 1224 alert) round-trips through decode,
encode and decode with identical structures and idempotent output in compact and indented
forms; `TestEncodeCanonicalShape` pins the exact bytes; `TestDecodeRejections` covers every
refused shape; `TestCDATAIsNormalised` pins the Learn known issue; `TestValidateRules` covers
each rule with a `ValidationError` path; `TestMD5DigestKnownAnswers` matches the OMA DM
Protocol 9.4.2 worked example (`Zz6EivR3yeaaENcRN6lpAQ==`); `FuzzDecode` runs in
`make fuzz-smoke`. Coverage is above the 95% gate.

## References

- [mdmprotocol/syncml](../../../mdmprotocol/syncml)
- [mdmprotocol/syncml/testdata/README.md](../../../mdmprotocol/syncml/testdata/README.md)
- [Research store](../../research.md), sections 1.1, 1.2, 1.3 (MS-MDM), 1.4 (OMA DM protocol support, Known issues), 2 and 7
- OMA SyncML RepPro DTD 1.2: <https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/OMA-SUP-DTD_SyncML_RepPro-V1_2-20070221-A.txt>
- OMA SyncML MetaInfo DTD 1.2: <https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/OMA-SUP-DTD_SyncML_MetaInfo-V1_2-20070221-A.txt>
- OMA DM Protocol 1.2.1 sections 6, 7, 8.3, 8.7, 9: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Protocol-V1_2_1-20080617-A.pdf>
- OMA DM Security 1.2.1 section 5.3: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Security-V1_2_1-20080617-A.pdf>
- MS-MDM v15.0 sections 1.3.1, 2.1, 2.2: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mdm/33769a92-ac31-47ef-ae7b-dc8501f7104f>
- Learn, OMA DM protocol support (2025-08-04): <https://learn.microsoft.com/en-us/windows/client-management/oma-dm-protocol-support>
- Learn, Known issues in MDM (2025-08-04): <https://learn.microsoft.com/en-us/windows/client-management/mdm-known-issues>

Reference source identifiers and paths (relative to the named project):

- `fleetdm/fleet`, `server/fleet/microsoft_mdm.go:1010-1305` (`SyncML`, `SyncBody` as one slice per verb, `CmdItem`, `RawXmlData` innerxml), `:1306-1410` (`IsValidHeader`, `IsValidBody`), `server/mdm/microsoft/syncml/syncml.go` (status and alert constants), `pkg/mdm/mdmtest/windows.go:128-209` (package 1), `:372-395` (MD5 credential)
- `mattrax/Mattrax` (`rust` branch), `crates/ms-mdm/src/sync_body.rs` (ordered enum of children), `alert.rs` (Item optional against the spec), `status.rs`, `results.rs` (Cmd inside Results), `session_id.rs` (4-byte bound), `data.rs` (`msft:originalerror`)
- `oscartbeaumont/windows_mdm`, `mdm_manage.go` (regex extraction of SessionID and MsgID; the failure this package's ordered model avoids)
- `Malcolm/local-mdm`, `internal/platform/windows/syncml.go` (typed builder without a parser)

## Desktop conformance

Native build 26200.9278 accepts server namespace 1.1 with protocol version 1.2 and continues sending namespace 1.2. CmdIDs start at 2 and DevInfo DmV is 1.3. See [native evidence](../../testing/windows-host-conformance.md); namespace acceptance does not establish WinDC compatibility.
