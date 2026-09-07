# Windows MDM (OMA-DM / MS-MDE2 / MS-MDM / WinDC): research store

Research date: 2026-09-06. Every URL in this document was fetched on that date and confirmed to
resolve. Dates are taken from the source itself: `ms.date` and `updated_at` in Microsoft Learn front
matter, the revision date on Microsoft Open Specifications pages, the date embedded in OMA PDF file
names, the post date for blogs, and the GitHub API `pushed_at` for repositories. Anything that could
not be verified is listed in section 10 rather than cited.

## Purpose

`go-oma-dm` will be a pure Go library and reference server for managing Windows 11 devices with the
protocols Windows ships: OMA Device Management 1.2.1 over SyncML (MS-MDM), Mobile Device Enrollment
Protocol Version 2 (MS-MDE2), the configuration service provider (CSP) tree described by DDF v2
files, Windows Notification Services (WNS) push, and the Windows declared configuration (WinDC)
desired-state protocol. The compliance target is Windows 11 version 26H2, the newest release.

This document is the prior-art map that every later design decision is founded on: where the
primary specifications are and which revision is in force, which open source projects implement
which parts of the protocol, which building blocks already exist in Go, what the community has
learned the hard way, and where the gaps are. It is a reference list, not a design document. It
follows the shape of `go-apple-dm/docs/research/reference_projects.md` so the two projects can be
read side by side.

Scope decisions (2026-09-06):

- Windows 11 only. Windows 10, Windows Phone, HoloLens, Surface Hub, and IoT editions are noted only
  where a document cannot be separated from them.
- Entra identity enrollment paths, WNS push, and the Autopilot and provisioning ecosystem are in
  scope. Features that only work with Intune are recorded and flagged `Intune-gated`.
- Generic OMA-DM for telecom or IoT devices (libdmclient, LwM2M, firmware update objects) is out of
  scope, except where an OMA document is the only source for a detail Windows relies on.
- Newest information wins. When two sources cover the same topic, the entry says which supersedes
  which.

## How to read this document

Each entry has the same shape:

```markdown
### Title
- Source: url · Publisher or author · Type · Date YYYY-MM-DD · Status
- Covers: one line on what is in it
- Notes: why it matters for go-oma-dm, what is newer or older than it, anything it contradicts
```

Repositories use `Repo: url · Language · License · Status (last push YYYY-MM) · Stars` and an
`Implements:` line drawn from the checklist in section 2.

Type is one of `spec`, `learn` (Microsoft Learn), `repo`, `blog`, `issue`, `qna` (Microsoft Q&A),
`download`, `talk`. Entries in sections 1.8, 1.9, 6, and 7 also carry `Third-party MDM: yes |
Intune-gated | unclear`.

Status legend:

| Status | Meaning |
|---|---|
| Active | Pushed or updated within the last 12 months |
| Maintenance | Pushed within 1 to 3 years, or the maintainer has declared it maintenance-only |
| Historical | No push in more than 3 years, archived on GitHub, or a spec that is final and no longer revised |
| Experimental | Author describes it as a proof of concept, playground, or work in progress |

Go projects are listed first in every section because this repository is Go. Projects in other
languages are included for protocol knowledge, not for code reuse.

---

## 0. Target: Windows 11 26H2

State as of 2026-09-06. Windows 11, version 26H2 is in the Release Preview Channel (build
26300.9278), announced 2026-08-27 with an ISO note added 2026-08-31. It is not yet generally
available: the Windows 11 release information page (ms.date 2026-08-27) lists only 26H1, 25H2, 24H2
and 23H2 in the General Availability Channel, and there is no `whats-new-windows-11-version-26h2`
page on Learn (HTTP 404). Microsoft's IT Pro blog "Get ready for Windows 11, version 26H2"
(2026-06-19, modified 2026-08-03) commits only to "later this calendar year".

Servicing branch, which decides CSP applicability. 26H2 is delivered as an enablement package
(eKB). The Insider blog states: "Windows 11, version 26H2 will be delivered as an enablement package
(eKB). This means that Windows 11, versions 24H2, 25H2, and 26H2 use a shared servicing branch."
The build lineage is 24H2 = 26100.x, 25H2 = 26200.x, 26H2 = 26300.x, all on the same monthly LCU
(2026-08 D, KB5120998, is 26100.9278 and 26200.9278; the 26H2 Release Preview build is 26300.9278,
the same revision). 24H2 was a full OS swap from 23H2 (22631) and is the last real platform break;
25H2 was an eKB over 24H2. Practically: any CSP node whose DDF `OsBuildVersion` is `10.0.26100[.rev]`
applies to 24H2, 25H2 and 26H2 alike once the device has the matching LCU. 26H2 adds no new CSP
surface that 24H2/25H2 devices on the same LCU would not also have; only features behind temporary
enterprise feature control get switched on by the eKB.

Separately, 26H1 (build 28000.x) is a hardware-scoped GA-channel release for new devices only ("not
offered as an in-place update from 24H2 or 25H2 on existing devices"; IoT Enterprise not supported)
and sits on a different servicing branch. The Feb 2026 DDF bundle contains no `28000` applicability
values and no `26300` values either.

Documented MDM, CSP and WinDC changes per version:

- 26H2: nothing Microsoft-documented yet on Learn or in the two blogs beyond commercial features
  (Windows settings backup, app-specific taskbar actions, File Explorer enhancements) and delivery
  via Autopatch, Intune, and WSUS.
- 25H2 (What's new, ms.date 2025-10-03): eKB over 24H2; policy items called out are
  `ApplicationManagement/RemoveDefaultMicrosoftStorePackages`, `Power/EnableEnergySaver`, taskbar
  `PinGeneration`; PowerShell 2.0 removed, WMIC uninstalled. No WinDC mention.
- 24H2 (What's new, ms.date 2026-07-15): full OS swap; checkpoint cumulative updates; LAPS CSP
  additions (`PasswordComplexity` passphrase, `PassphraseLength`, `PostAuthenticationActions`); App
  Control for Business; Personal Data Encryption for folders; AllJoyn management CSP retired; NTLMv1
  removed. No WinDC mention.
- The "What's new in MDM enrollment and management" page (ms.date 2025-08-04) has its newest table
  at Windows 11, version 22H2. There are no 23H2, 24H2, 25H2 or 26H2 tables. This is a documentation
  gap; the only per-version delta source now is the DDF `MSFT:Applicability/OsBuildVersion` values
  and the per-CSP "Applicable OS" tables.

Recent MDM-relevant servicing note. KB5101681 (2026-07-28, 26H1 preview) fixed "an issue affecting
newly provisioned or newly recovered Windows devices running versions of Windows released on or
after July 14, 2026 (KB5101649). After enrollment in a mobile device management (MDM) service such
as Microsoft Intune, affected devices could remain in an incorrect noncompliant state." The 25H2
release-health page (ms.date 2026-09-03) lists no open MDM, enrollment, or WinDC issues.

### Facts table

| Fact | Value | Source |
|---|---|---|
| 26H2 build | 26300.9278 | Windows Insider blog 2026-08-27 |
| 26H2 channel today | Release Preview; GA "later this calendar year" | Insider blog 2026-08-27; IT Pro blog 2026-06-19 |
| 26H2 delivery | Enablement package; shared servicing branch with 24H2 and 25H2 | Insider blog 2026-08-27; IT Pro blog |
| 25H2 latest build | 26200.9278 (2026-08 D, KB5120998, 2026-08-27); GA 2025-09-30 | Release information page (ms.date 2026-08-27) |
| 24H2 latest build | 26100.9278 (same KB); GA 2024-10-01; also LTSC 2024 | Release information page |
| 26H1 | 28000.2804 (2026-08 D, KB5120996); GA 2026-02-10; new devices only | Release information page |
| 23H2 | 22631.7517; Home/Pro end of updates reached; Ent/Edu until 2026-11-10 | Release information page |
| 24H2 vs 23H2 | Full OS swap, not eKB | What's new 24H2 (ms.date 2026-07-15) |
| 25H2 vs 24H2 | eKB | What's new 25H2 (ms.date 2025-10-03) |
| DDF bundle in force | DDFv2Feb2026.zip (Last-Modified 2026-02-19); highest real OsBuildVersion 10.0.26200.7019 | DDF page (updated 2026-02-20); bundle inspection below |
| Newest "What's new in MDM" table | 22H2 | new-in-windows-mdm-enrollment-management (ms.date 2025-08-04) |
| `whats-new-windows-11-version-26h2` | Does not exist (404) | learn.microsoft.com, checked 2026-09-06 |

#### Releasing Windows 11, version 26H2 to the Release Preview Channel
- Source: <https://blogs.windows.com/windows-insider/2026/08/27/releasing-windows-11-version-26h2-to-the-release-preview-channel/> · Microsoft Windows Insider Program · blog · 2026-08-27 (updated 2026-08-31 re ISOs) · Active
- Covers: 26H2 (build 26300.9278) to Release Preview; eKB delivery; shared servicing branch with 24H2 and 25H2; commercial feature list.
- Notes: The only primary source for the 26300 build number today. Supersedes the June IT Pro blog on timing detail. Nothing on MDM or CSPs.

#### Get ready for Windows 11, version 26H2 (Windows IT Pro Blog)
- Source: <https://techcommunity.microsoft.com/blog/windows-itpro-blog/get-ready-for-windows-11-version-26h2/4529367> · Microsoft · blog · 2026-06-19 (modified 2026-08-03) · Active
- Covers: "Supported devices get this feature update as a small enablement package instead of a full OS replacement"; available via Windows Autopatch, Microsoft Intune, WSUS.
- Notes: Body is JavaScript-rendered; extracted via curl. Confirms the shared-servicing model; no build number, no CSP or WinDC content.

#### Windows 11 release information
- Source: <https://learn.microsoft.com/en-us/windows/release-health/windows11-release-information> · Microsoft · learn · ms.date 2026-08-27, updated 2026-08-28 · Active
- Covers: Servicing-channel table (26H1, 25H2, 24H2, 23H2), LTSC 2024, and full monthly build, KB, and date history per version.
- Notes: Authoritative for the build to KB to date mapping that a DDF `OsBuildVersion` value like `10.0.26100.7019` must be resolved against (26100.7019 = 2025-10 D, KB5067036, 2025-10-28). 26H2 not yet listed.

#### What's new in Windows 11, version 25H2 for IT pros
- Source: <https://learn.microsoft.com/en-us/windows/whats-new/whats-new-windows-11-version-25h2> · Microsoft · learn · ms.date 2025-10-03, updated 2025-10-17 · Active
- Covers: eKB over 24H2; temporary enterprise feature control list; RemoveDefaultMicrosoftStorePackages, EnableEnergySaver, PinGeneration; PowerShell 2.0 and WMIC removal.
- Notes: No CSP-level changelog; no WinDC mention.

#### What's new in Windows 11, version 24H2 for IT pros
- Source: <https://learn.microsoft.com/en-us/windows/whats-new/whats-new-windows-11-version-24h2> · Microsoft · learn · ms.date 2026-07-15, updated 2026-07-21 · Active
- Covers: Full OS swap; checkpoint CUs; LAPS CSP additions; App Control for Business; AllJoyn CSP retired; SMB, LSA, NTLMv1 changes.
- Notes: Most recently maintained of the What's new pages. No WinDC mention.

#### What's new in MDM enrollment and management
- Source: <https://learn.microsoft.com/en-us/windows/client-management/new-in-windows-mdm-enrollment-management> · Microsoft · learn · ms.date 2025-08-04, updated 2025-08-04 · Maintenance
- Covers: Per-version tables of new and updated CSPs and Policy CSP nodes; newest is Windows 11 22H2 (PersonalDataEncryption CSP new; DeviceStatus/MDMClientCertAttestation; about 50 Policy nodes).
- Notes: Documentation gap: no 23H2, 24H2, 25H2 or 26H2 tables. The 2025-08-04 date matches a single bulk commit (`1493a5a72db8`) that touched many client-management pages, so it is a touch, not a content refresh.

#### What's new in Windows (hub)
- Source: <https://learn.microsoft.com/en-us/windows/whats-new/> · Microsoft · learn · ms.date 2025-09-30, updated 2025-10-17 · Active
- Covers: Links 25H2, 24H2, 23H2, 22H2, LTSC 2024.
- Notes: Confirms the absence of a 26H2 page.

#### July 28, 2026, KB5101681 (OS Build 28000.2608) Preview
- Source: <https://support.microsoft.com/en-us/servicing/os/windows-11/2026/07/july-28-2026-kb5101681-preview> · Microsoft · learn (support KB) · 2026-07-28 · Active
- Covers: Fix for MDM-enrolled devices stuck noncompliant after enrollment on builds released on or after 2026-07-14.
- Notes: Relevant regression class for any server that reads compliance or enrollment status; the same fix shipped for 24H2 and 25H2 in KB5101684 per the release-info table.

#### Windows 11, version 25H2 known issues and notifications
- Source: <https://learn.microsoft.com/en-us/windows/release-health/status-windows-11-25h2> · Microsoft · learn · ms.date 2026-09-03 · Active
- Covers: Open issues from KB5121003 and KB5120998 (desktop background, mouse, ARM Teams and Outlook, Defender notifications).
- Notes: No MDM or WinDC issues listed.

---

---

## 1. Primary specifications

### 1.1 OMA DM 1.2 and 1.2.1 enabler

Windows implements the OMA DM 1.2 protocol family. MS-MDM normatively cites the 1.2.1 documents
([OMA-DMP1.2.1], [OMA-DMRP1.2.1], [OMA-DMS1.2.1]), so 1.2.1 is the version of record here; the
user-supplied 1.2 PDF is the baseline the Learn OMA-DM page links. A full-text diff of the two
Protocol PDFs shows 1.2.1 is editorial only (reference renames, RFC 2119 capitalisation, a
heading fix, a sample locale fix). No new packages, alert codes, or status codes.

#### OMA DM release directory (index)
- Source: <https://www.openmobilealliance.org/release/DM/> · Open Mobile Alliance · spec (directory index) · Date 2020-08-18 (directory mtime; README.txt 2017-01-20) · Historical (content)
- Covers: Every DM enabler release directory: V1_1_2 (2003, 2004), V1_2 candidates (2005 to 2006), V1_2-20070209-A, V1_2_1-20080617-A, the V1_3 series (2009 to 2016), the V2_0 series (2010 to 2016), plus ETS, EVP, and cp. README.txt explains the `Vx_y-YYYYMMDD-{A|C|H}` naming (A approved, C candidate, H historic).
- Notes: Canonical root for resolving any OMA DM URL. DM 1.3 and 2.0 are newer but Windows does not implement them. The lowercase `/release/dm/` path also resolves; the canonical listing uses `/release/DM/`.

#### OMA DM V1.2 enabler directory (V1_2-20070209-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2-20070209-A/> · OMA · spec (directory listing) · Date 2007-02-09 · Historical (superseded by 1.2.1)
- Covers: 16 files: ERELD, ERP zip, RD, OMA-SUP-ac_w7_dm-V1_0, OMA-SUP-dtd_dm_ddf-V1_2 (the DDF DTD), the three MO DDF files (DevDetail, DevInfo, DMAcc), and the TS set: DM_Bootstrap, DM_Notification, DM_Protocol, DM_RepPro, DM_Security, DM_StdObj, DM_TND, DM_TNDS.
- Notes: This is the release the Learn "OMA DM protocol support" page links for the Protocol spec. Superseded by V1_2_1-20080617-A for Protocol, RepPro, Notification, Security, StdObj, TND, Bootstrap and ERELD; the DDF DTD, the three MO DDF files, RD and TNDS were not re-issued in 1.2.1.

#### OMA DM V1.2.1 enabler directory (V1_2_1-20080617-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/> · OMA · spec (directory listing) · Date 2008-06-17 · Maintenance (last 1.2.x approved release)
- Covers: OMA-ERELD-DM-V1_2_1, OMA-ERP-DM-V1_2_1 zip, OMA-RD-DM-V1_2-20070209-A, OMA-SUP-ac_w7_dm-V1_0_1-20080617-A.txt, OMA-SUP-dtd_dm_ddf-V1_2-20070209-A.dtd, the three MO DDF files (V1_2-20070209-A), OMA-TS-DM_Bootstrap, Notification, Protocol, RepPro, Security, StdObj, TND (all V1_2_1-20080617-A), and OMA-TS-DM_TNDS-V1_2-20070209-A.
- Notes: The set MS-MDM normatively cites and the Learn OMA-DM page points to for Security. Treat it as the primary OMA reference for go-oma-dm.

#### OMA Device Management Protocol V1.2 (OMA-TS-DM_Protocol-V1_2-20070209-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2-20070209-A/OMA-TS-DM_Protocol-V1_2-20070209-A.pdf> · OMA · spec · Date 2007-02-09 · Historical (superseded by 1.2.1)
- Covers: "OMA Device Management Protocol, Approved Version 1.2, 09 Feb 2007", 53 pages. Sections: 5 Node Addressing; 6 Multiple Messages in Package; 7 Large Object Handling; 8 Protocol Packages (8.1 Session Abort, 8.2 Package 0 notification, 8.3 Package 1 client init, 8.4 Package 2 server init, 8.5 Package 3 client response, 8.6 Package 4 further server ops, 8.7 Generic Alert); 9 Authentication (Basic and MD5 examples); 10 User Interaction commands (alert codes 1100 to 1104); 11 Protocol examples.
- Notes: Same section structure as 1.2.1. Prefer 1.2.1 and treat this as the 1.2 baseline only.

#### OMA Device Management Protocol V1.2.1 (OMA-TS-DM_Protocol-V1_2_1-20080617-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Protocol-V1_2_1-20080617-A.pdf> · OMA · spec · Date 2008-06-17 · Maintenance
- Covers: "OMA Device Management Protocol, Approved Version 1.2.1, 17 Jun 2008", 53 pages, identical TOC to 1.2. Key normative content: 6.2, the last message of a package MUST carry `Final`; the client MUST NOT send Final for packages 2 and 4 until the server sent Final for packages 1 and 3; a recipient asks for more messages with Alert 1222; a "Next Message" response is Alert 1222 (or 1223 abort) plus Status for SyncHdr, no other commands, no Final; the server MUST send Final in every message when possible. 7, Large Object: chunk with `<MoreData/>`, the recipient answers 213 "Chunked item accepted and buffered"; MaxObjSize may be in SyncHdr Meta and MUST be respected for the session; a missing MoreData terminus gives Alert 1225. 8.3 Package 1: VerDTD MUST be '1.2', VerProto MUST be 'DM/1.2', SessionID client-generated and constant for the session, MsgID unique, Target is the server, Source is the device, optional Cred; SyncBody MUST contain Alert 1200 or 1201, Replace with DevInfo items (./DevInfo/DevId, Man, Mod, DmV, Lang), MAY contain Client Event (1224) or Generic Alert (1226); Final on the last message. 8.7 Generic Alert 1226: Item with Meta Type (URN or Content-Type), Format, Mark; the server MUST answer 200/202 or 401/407/412/415/500. 9: Basic, MD5 challenge flow; the next nonce in Chal MUST be used on the next request; if the server challenges in package 2 the client MUST revert to package 1 and resend Alert and DevInfo with credentials.
- Notes: Windows uses packages 1 to 4 over HTTPS, Alerts 1200/1201/1222/1223/1224/1225/1226, Basic and MD5 application-layer auth, Large Object (MoreData, 213) since Windows 10, and session abort. Windows does not use package 0 over WAP Push or SMS for enterprise and does not implement the section 10 user-interaction alerts (not listed in MS-MDM 2.2.7.2).

#### OMA Device Management Representation Protocol V1.2.1 (OMA-TS-DM_RepPro-V1_2_1-20080617-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_RepPro-V1_2_1-20080617-A.pdf> · OMA · spec · Date 2008-06-17 · Maintenance
- Covers: 47 pages. 5.1 MIME usage: `application/vnd.syncml.dm+xml` and `application/vnd.syncml.dm+wbxml`; optional MIME parameters charset, verproto, verdtd. 5.2 WBXML usage: MUST accept any WBXML 1.x and use 1.1, 1.2 or 1.3. 6.1 common elements (VerDTD "1.2" MUST be in every message, root `<SyncML xmlns='SYNCML:SYNCML1.2'>`), 6.2 containers, 6.3 data description, 6.4 Meta-information, 6.5 Status, 6.6 commands. Alert code table: 1200 SERVER-INITIATED, 1201 CLIENT-INITIATED, 1202 to 1220 reserved, 1222 NEXT MESSAGE, 1223 SESSION ABORT, 1224 CLIENT EVENT, 1225 NO END OF DATA, 1226 GENERIC ALERT.
- Notes: MS-MDM 2.2.2 says the "MDM-specific SyncML xml message format is defined in [OMA-DMRP1.2.1]", this document. It is the source of the alert code numbers Windows uses and of the DM MIME types selected via `DEFAULTENCODING`. MS-MDM does not support `<Correlator>`.

#### OMA Device Management Notification Initiated Session V1.2.1 (OMA-TS-DM_Notification-V1_2_1-20080617-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Notification-V1_2_1-20080617-A.pdf> · OMA · spec · Date 2008-06-17 · Maintenance (not used by Windows enterprise MDM)
- Covers: Package 0 trigger format: `<digest>` (128-bit MD5) plus `<trigger>` (header: version, ui-mode, initiator, future-use, sessionid, length-identifier, server-identifier; body: vendor-specific). 5.1 nonce resynchronisation (special nonce 0x00000000). 6.2.10 Session Identifier ties package 0 to the package 1 SessionID. 7.1 delivery over WAP Push (MIME `application/vnd.syncml.notification`), 7.2 over OBEX.
- Notes: Windows enterprise MDM does not use OMA package 0 (Learn: "Remote DM server initiation notification using WAP Push over SMS. Not used by enterprise management"). Windows replaces it with WNS raw push (1.7) and polling. The DMAcc CSP `Ext/Microsoft/UseNonceResync` node is the only visible residue of this spec. Context only.

#### OMA Device Management Tree and Description V1.2.1 (OMA-TS-DM_TND-V1_2_1-20080617-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_TND-V1_2_1-20080617-A.pdf> · OMA · spec · Date 2008-06-17 · Maintenance
- Covers: 6 management tree (node characteristics, interior and leaf, URI addressing), 7 node properties (ACL, Format, Name, Size, Title, TStamp, Type, VerNo; property addressing `?prop=`), 8 tree serialisation requests (`?list=`), 9 Device Description Framework (DDF) XML and its elements, 10 DDF WBXML code page.
- Notes: Windows honours `?prop=Type` style property queries and publishes CSP DDF files (1.5). The Learn OMA-DM page adds Windows-specific node-name rules ('.' allowed, non-empty, not just `*`, LocURI cannot start with `/`). The DDF DTD itself is OMA-SUP-dtd_dm_ddf-V1_2-20070209-A.dtd, and every Microsoft DDF file's DOCTYPE points at it.

#### OMA Device Management Standardized Objects V1.2.1 (OMA-TS-DM_StdObj-V1_2_1-20080617-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_StdObj-V1_2_1-20080617-A.pdf> · OMA · spec · Date 2008-06-17 · Maintenance
- Covers: 5.3 the OMA DM management objects: DMAcc (5.3.1), DevInfo (5.3.2), DevDetail (5.3.3), Inbox URI (5.3.4). "Clients implementing OMA DM MUST support the OMA DM Account management object, DevInfo management object and the DevDetail management object. OMA DM servers MUST support all three."
- Notes: Windows implements all three as CSPs: `./DevInfo`, `./DevDetail`, `./SyncML/DMAcc/{AccountUID}` (1.4). DevInfo is sent in package 1 automatically; DevDetail must be queried.

#### OMA Device Management Security V1.2.1 (OMA-TS-DM_Security-V1_2_1-20080617-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Security-V1_2_1-20080617-A.pdf> · OMA · spec · Date 2008-06-17 · Maintenance
- Covers: 5.1 credentials, 5.2 initial provisioning, 5.3 authentication (`syncml:auth-basic`, `syncml:auth-md5`; clients without transport-layer client auth MUST support auth-md5), 5.4 integrity via HMAC-MD5 (`syncml:auth-MAC`, transported in the `x-syncml-hmac` header, nonce handling), 5.5 confidentiality (transport), 5.6 notification security, 5.7 bootstrap security.
- Notes: The Learn OMA-DM page and MS-MDM 1.3.1 point here for Basic and MD5 client auth, MD5 server auth, HMAC integrity, and the nonce-renewal rule ("The server MD5 nonce is renewed in each DM session for the next DM session ... The MD5 binary nonce is sent over XML in B64-encoded format, but the octal form of the binary data is used when the server calculates the hash"). The Windows w7 bootstrap constrains AAUTHTYPE: CLIENT level MUST be DIGEST, APPSRV may be BASIC or DIGEST.

#### OMA Device Management Bootstrap V1.2.1 (OMA-TS-DM_Bootstrap-V1_2_1-20080617-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Bootstrap-V1_2_1-20080617-A.pdf> · OMA · spec · Date 2008-06-17 · Maintenance
- Covers: 5.1 bootstrap scenarios (factory, smartcard, server-initiated), 5.2 profiles, 5.3 OMA Client Provisioning profile (w7 APPLICATION characteristic, references ac_w7_dm), 5.4 OMA Device Management profile (TNDS-serialised DMAcc), smartcard storage.
- Notes: Windows enrollment uses the OMA Client Provisioning profile: the WSTEP response carries a `wap-provisioningdoc` with `<characteristic type="APPLICATION">` APPID w7 (MS-MDE2 2.2.9.5). Windows does not use server-initiated bootstrap over WAP Push.

#### OMA DM w7 Application Characteristic (OMA-SUP-ac_w7_dm-V1_0_1-20080617-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-SUP-ac_w7_dm-V1_0_1-20080617-A.txt> · OMA · spec (support file) · Date 2008-06-17 · Maintenance
- Covers: APPID `w7`; parameters APPLICATION/APPID, PROVIDER-ID, NAME, APPADDR/ADDR, ADDRTYPE, PORT/PORTNBR, APPAUTH (AAUTHLEVEL APPSRV|CLIENT|OBEX, AAUTHTYPE HTTP-BASIC, HTTP-DIGEST, BASIC, DIGEST, HMAC, X509, and others; AAUTHNAME, AAUTHSECRET, AAUTHDATA), TO-NAPID, TO-PROXY, INIT.
- Notes: Windows's w7 APPLICATION CSP is this characteristic plus Microsoft extensions (ROLE, PROTOVER, DEFAULTENCODING, CONNRETRYFREQ, INITIALBACKOFFTIME, MAXBACKOFFTIME, BACKCOMPATRETRYDISABLED, USEHWDEVID, SSLCLIENTCERTSEARCHCRITERIA). Windows only accepts AAUTHTYPE BASIC and DIGEST and ignores INIT for enterprise.

#### OMA DM 1.2.1 Enabler Release Definition (OMA-ERELD-DM-V1_2_1-20080617-A)
- Source: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-ERELD-DM-V1_2_1-20080617-A.pdf> · OMA · spec (ERELD) · Date 2008-06-17 · Maintenance
- Covers: 14 pages listing the enabler's documents with reference keys: [DMRD] RD V1_2; [DMREPU] RepPro V1_2_1; [DMSTDOBJ] StdObj V1_2_1; [DMTND] TND V1_2_1; [DMTNDS] TNDS V1_2; [DMDDFDTD] dtd_dm_ddf V1_2; plus Protocol, Security, Bootstrap, Notification 1.2.1 and the three MO DDF files.
- Notes: Confirms which files changed in 1.2.1 (the seven TS documents) and which did not. Use it as the manifest for pinning OMA references.

### 1.2 SyncML 1.2.2 Common

#### SyncML Common V1.2.2 directory (Common/V1_2_2-20090724-A)
- Source: <https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/> · OMA · spec (directory listing) · Date 2009-07-24 · Maintenance
- Covers: ERELD, ERP zip, OMA-SUP-DTD_SyncML_MetaInfo-V1_2-20070221-A.txt, OMA-SUP-DTD_SyncML_RepPro-V1_2-20070221-A.txt, OMA-TS-SyncML-RepPro-V1_2_2, OMA-TS-SyncML_HTTPBinding-V1_2_1-20070611-A, OMA-TS-SyncML_MetaInfo-V1_2_2, OMA-TS-SyncML_OBEXBinding-V1_2, OMA-TS-SyncML_SAN-V1_2_1, OMA-TS-SyncML_WSPBinding-V1_2.
- Notes: The "SyncML DTD" and "Meta-Information DTD" are the two `.txt` SUP files here; there is no separate SyncML DTD PDF, the DTD is chapter 7 of RepPro. OBEX and WSP bindings and SAN are not used by Windows MDM.

#### SyncML Representation Protocol V1.2.2 (OMA-TS-SyncML-RepPro-V1_2_2-20090724-A)
- Source: <https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/OMA-TS-SyncML-RepPro-V1_2_2-20090724-A.pdf> · OMA · spec · Date 2009-07-24 · Maintenance
- Covers: 60 pages. 5 SyncML (packages and messages, commands, security types, XML and MIME usage, identifiers); 6 mark-up (6.1 common elements: Chal, Cmd, CmdID, CmdRef, Cred, Final, LocName, LocURI, MoreData, MsgID, MsgRef, NoResp, RespURI, SessionID, Source, SourceRef, Target, TargetRef, VerDTD, VerProto; 6.2 SyncML/SyncHdr/SyncBody; 6.3 Data/Item/Meta/Correlator; 6.4 Status; 6.5 commands Add, Alert, Atomic, Copy, Delete, Exec, Get, Map, MapItem, Move, Put, Replace, Results, Search, Sequence, Sync); 7 SyncML DTD; 8 WBXML definition (8.1 SyncML 1.2 = WBXML PUBLICID 0x1201, FPI `-//SYNCML//DTD SyncML 1.2//EN`; 8.2 code pages 00 SyncML, 01 MetInf; 8.3 tokens); 9 URI schemes; 10 response status codes (200, 202, 212, 213, 214, 215, 216, 400, 401, 403, 404, 405, 406, 407, 412, 413, 415, 416, 418, 420, 424, 425, 500, 507, 508, 516, and others). Root element attribute rule: `xmlns` "Value MUST be the text: 'SYNCML:SYNCML1.2'". Change history: V1_2 2007-02-21, V1_2_1 2007-06-12, V1_2_2 2009-07-24.
- Notes: MS-MDM 2.2.1 names this as the namespace reference (`xmlns='SYNCML:SYNCML1.2'`) and the Learn OMA-DM page cites section 8 for WBXML and section 10 for status codes. Windows uses only the DM subset (no Sync, Map, Put, Copy, Move, Search). Some Learn CSP samples still show `SYNCML:SYNCML1.1`; Windows accepts both, but the normative value for DM/1.2 is `SYNCML:SYNCML1.2`.

#### SyncML Representation Protocol DTD V1.2 (OMA-SUP-DTD_SyncML_RepPro-V1_2-20070221-A)
- Source: <https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/OMA-SUP-DTD_SyncML_RepPro-V1_2-20070221-A.txt> · OMA · spec (DTD) · Date 2007-02-21 · Maintenance
- Covers: FPI `-//SYNCML//DTD SYNCML 1.2//EN`; `<!ELEMENT SyncML (SyncHdr, SyncBody)>`; `SyncHdr (VerDTD, VerProto, SessionID, MsgID, Target, Source, RespURI?, NoResp?, Cred?, Meta?)`; `SyncBody ((Alert|Atomic|Copy|Exec|Get|Map|Put|Results|Search|Sequence|Status|Sync|Add|Move|Replace|Delete)+, Final?)`; VerDTD "For this version of the DTD, the value is '1.2'".
- Notes: Gives the exact element ordering the Go marshaller must respect (MS-MDM 2.2.2: "the XML in the document MUST adhere to the explicit order defined in the DTD"). `MoreData` is declared EMPTY inside Item.

#### SyncML Meta Information V1.2.2 and its DTD
- Source: <https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/OMA-TS-SyncML_MetaInfo-V1_2_2-20090724-A.pdf> and <https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/OMA-SUP-DTD_SyncML_MetaInfo-V1_2-20070221-A.txt> · OMA · spec · Date 2009-07-24 (DTD 2007-02-21) · Maintenance
- Covers: Namespace `syncml:metinf`, FPI `-//OMA//DTD SYNCML-METINF 1.2//EN`; elements MetInf, FieldLevel, Format, Type, Mark, Size, Anchor (Last, Next), Version, NextNonce, MaxMsgSize, MaxObjSize, EMI, Mem. WBXML code page 01.
- Notes: Windows emits `<Format xmlns="syncml:metinf">`, `<Type xmlns="syncml:metinf">`, `NextNonce`, and `MaxMsgSize`. Alert 1224 and 1226 items carry `Type`, `Format`, `Mark` from this DTD.

#### SyncML HTTP Binding V1.2.1 (OMA-TS-SyncML_HTTPBinding-V1_2_1-20070611-A)
- Source: <https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/OMA-TS-SyncML_HTTPBinding-V1_2_1-20070611-A.pdf> · OMA · spec · Date 2007-06-11 · Maintenance
- Covers: POST is the transfer method; the body MUST be a SyncML message with the SyncML content type; `Accept` header; `Cache-Control: no-store` or `private` MUST be supported; implementations MUST support `Transfer-Encoding: chunked`.
- Notes: MS-MDM 2.1 says MDM is "in compliance with [OMA-SyncML-HTTPBnd]" using `application/vnd.syncml.dm+xml` (default) or `+wbxml`. Windows-specific additions not in this binding: `?mode=Maintenance|Machine&Platform=...` query parameters, `User-Agent: MSFT OMA DM Client/1.2.0.1`, `Authorization: Bearer <Entra token>`, `DeviceToken:`, `MS-Signature`, `MDM-GenericAlert`, `client-request-id` headers (see MS-MDM).

#### SyncML Common V1.2.2 Enabler Release Definition
- Source: <https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/OMA-ERELD-SyncML_Common-V1_2_2-20090724-A.pdf> · OMA · spec (ERELD) · Date 2009-07-24 · Maintenance
- Covers: Manifest: [REPRO] RepPro V1_2_2; [SYNCMETA] MetaInfo V1_2_2; [SYNCHTTP] HTTPBinding V1_2_1; [SYNCOBEX] OBEXBinding V1_2; [SYNCWSP] WSPBinding V1_2; [SAN] SyncML_SAN V1_2_1; DTD support files.
- Notes: Use as the pin manifest for the SyncML side.

#### WAP Binary XML Content Format (W3C NOTE)
- Source: <https://www.w3.org/TR/wbxml/> · W3C (submission by Ericsson, IBM, Motorola, Phone.com) · spec · Date 1999-06-24 (WBXML 1.1) · Historical (W3C NOTE)
- Covers: Multi-byte integers, version, publicid and charset preamble, string table, tag tokens, global tokens (SWITCH_PAGE, END, ENTITY, STR_I, LITERAL, EXT_*, PI, STR_T, OPAQUE), 256 code pages per parser state, encoding examples.
- Notes: This NOTE is WBXML 1.1. OMA DM RepPro 1.2.1 5.2 requires acceptance of any WBXML 1.x; the WAP-192 (WBXML 1.3) PDF was not fetched (section 10). Windows selects WBXML via `DEFAULTENCODING=application/vnd.syncml.dm+wbxml`; token tables come from SyncML RepPro 1.2.2 section 8 and MetaInfo 1.2.2 section 7.

### 1.3 Microsoft Open Specifications

Current document PDFs live at `https://winprotocoldoc.z19.web.core.windows.net/<DOC>/[<DOC>].pdf`;
the `winprotocoldocs-*.azurefd.net` links on the landing pages redirect there. Use the landing-page
revision tables, not the errata pages, to decide currency: since January 2024 Microsoft republishes
documents instead of issuing errata.

#### [MS-MDE2]: Mobile Device Enrollment Protocol Version 2
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692> (PDF <https://winprotocoldoc.z19.web.core.windows.net/MS-MDE2/%5bMS-MDE2%5d.pdf>) · Microsoft · spec · Date 2026-08-11 (revision 19.0, major; page updated 2026-08-12) · Active
- Covers: 120-page PDF "[MS-MDE2] v20260811". Section map: 1 Introduction (1.3 Overview, 1.7 Versioning "None"); 2 Messages: 2.1 Transport (SOAP 1.1 over HTTPS), 2.2.1 Namespaces, 2.2.9 Common Data Structures (2.2.9.1 XML Provisioning Schema, 2.2.9.2 CertificateStore CSP, 2.2.9.3 DMClient CSP, 2.2.9.4 RootCATrustedCertificates CSP, 2.2.9.5 w7 APPLICATION CSP, 2.2.9.6 OSEdition enumeration); 2.2.10 Faults; 3.1 IDiscoveryService (3.1.4.1 Discover; DiscoveryRequest fields EmailAddress, OSEdition, DeviceType, RequestVersion "MUST be set to 1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, or 9.0", ApplicationVersion, AuthPolicies; DiscoveryResponse AuthPolicy "MUST be set to Federated, Certificate, or OnPremise", AuthenticationServiceUrl, EnrollmentPolicyServiceUrl, EnrollmentServiceUrl, DeviceAssociationMaaUrl, GatewayService, EnrollmentVersion "3.0, 4.0, 5.0, 6.0, 7.0, 8.0, or 9.0"); 3.2 Interaction with STS (federated WAB flow, `AuthenticationServiceUrl?appru=<appid>&login_hint=<UPN>`, POST back to `ms-app://` with wresult); 3.3 X.509 Certificate Enrollment Policy (GetPolicies per auth policy); 3.4 WS-Trust X.509v3 Token Enrollment (RequestSecurityToken per auth policy, RequestSecurityTokenResponseCollection); 3.5 Certificate Renewal; 3.6 Certificate Recovery; 4 Examples (4.1 Discovery, 4.2 GetPolicies including 4.2.3 Azure Attestation, 4.3 RequestSecurityToken); 5 Security; 6 Appendix A XSD; 7 Appendix B Product Behavior; 8 Change Tracking. Discovery: "The path portion of the URL '/EnrollmentServer/Discovery.svc' is always constant"; overview step 3: "The enrollment client sends an HTTP GET request to the Discovery Service to validate" before the SOAP POST. Faults 2.2.10: s:MessageFormat 80180001, s:Authentication 80180002, s:Authorization 80180003, s:CertificateRequest 80180004, s:EnrollmentServer 80180005, a:InternalServiceFault 80180006, a:InvalidSecurity 80180007; DeviceEnrollmentServiceError subcodes DeviceCapReached 80180013, DeviceNotSupported 80180014, NotSupported 80180015, NotEligibleToRenew 80180016, InMaintenance 80180017, UserLicense 80180018, InvalidEnrollmentData 80180019, CustomServerError 80180032.
- Notes: The newest document in the store and it supersedes every Learn enrollment page (which still show RequestVersion and EnrollmentVersion 3.0 examples). Revision history: 18.0 (2026-01-26), 17.0 (2025-11-21), 16.0 (2025-03-10), 15.0 (2024-04-23), back to 1.0 (2015-06-30). New in recent revisions: EnrollmentVersion 5.0 nodes (2022 10C), 6.0 and 7.0 nodes (attestation and key-storage related), DeviceAssociationMaaUrl and GatewayService (Azure attestation, provisioning gateway). go-oma-dm should accept RequestVersion up to 9.0 and answer with the highest EnrollmentVersion it actually implements (3.0 is still valid). The errata page is stale (targets v12.0).

#### MS-WINERRATA errata page for MS-MDE2
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-winerrata/a66b5d6f-6330-46ab-9fa9-34700ee29f63> · Microsoft · spec (errata) · Date 2024-10-30 (last erratum 2023-06-12) · Historical (applies to MDE2 v12.0)
- Covers: 2023-06-12 added CustomServerError (80180032) with the note "applicable to Windows 10 v20H2 and later and Windows 11 version 1 and later"; 2022-12-30 and 2022-10-03: RequestVersion and EnrollmentVersion 5.0 support widened to Windows 11 (2022 10C) and Windows 10 v2004 (2023 1C).
- Notes: All folded into MDE2 v13.0 and later. Historical evidence of when version 5.0 became valid on Windows 11.

#### [MS-MDM]: Mobile Device Management Protocol
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mdm/33769a92-ac31-47ef-ae7b-dc8501f7104f> (PDF <https://winprotocoldoc.z19.web.core.windows.net/MS-MDM/%5bMS-MDM%5d.pdf>) · Microsoft · spec · Date 2024-04-23 (revision 15.0, major; page updated 2024-10-30) · Active
- Covers: 48-page PDF "[MS-MDM] v20240423". 1.3.1 Server requirements for OMA DM (TLS server auth mandatory; Basic or MD5 application-layer client auth; SSL client cert auth; MD5 nonce renewed each session and returned via Status; nonce is base64 on the wire, raw bytes in the hash); 2.1 Transport (HTTP per RFC 2616 and the SyncML HTTP binding; `application/vnd.syncml.dm+xml` default or `+wbxml` chosen by DMAcc `Microsoft/DefaultEncoding`; note 1 URL `?mode=Maintenance|Machine&Platform=WoA`; note 2 `User-Agent: MSFT OMA DM Client/1.2.0.1`; note 3 `Authorization: Bearer <Entra token>`; note 4 `MS-Signature` CMS detached signature opt-in via DMClient `RequireMessageSigning`; note 5 `MDM-GenericAlert: <Type1><Type2>` header when a session is triggered by fatal or critical generic alerts; note 8 `client-request-id` = EntDMID; note 9 `DeviceToken:` header when `ForceAadToken` is set); 2.2.1 Namespaces (`xmlns='SYNCML:SYNCML1.2'`); 2.2.2 SyncML Message (SyncHdr with VerDTD 1.2, VerProto DM/1.2, SessionID, MsgID, Target LocURI = unique device ID, Source LocURI = management server URL); 2.2.3 common elements (2.2.3.4 Final only in the last message of a package; 2.2.3.8 SessionID opaque, MUST be in every SyncHdr, max 4 bytes); 2.2.5 Data/Item/Meta (client sends Meta Format, NextNonce, MaxMsgSize, Type; note 13: MaxMsgSize unsupported on 8.1 and 1507); 2.2.6.1 Status; 2.2.7 commands Add, Alert (1200, 1201, 1222, 1223, 1224, 1225, 1226; Correlator unsupported; custom Type `com.microsoft/MDM/LoginStatus`), Atomic, Delete, Exec, Get, Replace, Results (`(CmdID, MsgRef?, CmdRef, Cmd, Meta?, Item+)`, missing MsgRef means "1"); 3.1.5.1.2 Alert MUST be in the session init message; 3.1.5.1.3 Atomic rollback semantics; 3.1.5.2.1 Status "MUST be returned by the client in response to any command issued by the server"; 3.1.5.2.2 Results per successful Get; 3.1.7.1 ACLs; 3.1.7.2 UserAgentOrigin; 3.2 Azure details: 3.2.5.1.1 device session vs user session (mixed by default; AVD: device session with an Entra device token, per-user sessions with Entra user tokens), 3.2.5.1.2 Entra join (`com.microsoft/MDM/AADUserToken` alert; OSEdition 175 = AVD), 3.2.5.1.3 SyncApplicationVersion (query MaxSyncApplicationVersion, set 5.0+ for multi-user AVD), 3.2.5.1.4 MultipleSession poll nodes, 3.2.5.1.5 SyncType Alert 1224 Type `com.microsoft.mdm.synctype` Data user|device|mixed (a server sending the wrong scope gets Status 405), 3.2.5.1.6 DevicePrepSync Alert 1224 Type `com.microsoft/MDM/DevicePrepSync` Data PendingProvisioning|Bootstrapping|ExecutingProvisioning|ProvisioningComplete; 4 Examples; 6 Appendix A MSI application install; 7 Product Behavior; 8 Change Tracking.
- Notes: The normative Windows subset of OMA DM. Two flags: (a) 1.3.1 literally says "The OMA DM server is required to support the OMA-DM version 2.1 or later protocol", which contradicts everything else in the document (VerProto DM/1.2, references to [OMA-DMP1.2.1]); treat as a typo for 1.2.1. (b) There is no "DMS heartbeat" section; the closest constructs are the DMClient `EnableOmaDmKeepAliveMessage` node and the polling schedule. Chunking and MaxMsgSize: MS-MDM defers to OMA-DMP1.2.1 sections 6 and 7; the Learn OMA-DM page adds "In Windows 10, client support for uploading large objects to the server was added". Older than MS-MDE2 by two years; still current for management.

#### MS-WINERRATA errata page for MS-MDM
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-winerrata/1732d832-43da-40ed-b950-2d379050d8b7> · Microsoft · spec (errata) · Date 2024-10-30 (last erratum 2023-03-06) · Historical (applies to MDM v14.0)
- Covers: AVD multi-user session support backported to Windows 11 and Windows 10 v2004+; 2022-06-14 added transport note 9 (`DeviceToken:` header when DMClient `ForceAadToken` is set).
- Notes: Folded into MDM v15.0. Useful only for the version gates on AVD multi-session and DeviceToken.

#### [MS-MDE]: Mobile Device Enrollment Protocol (v1, legacy)
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde/5c841535-042e-489e-913c-9d783d741267> (PDF <https://winprotocoldoc.z19.web.core.windows.net/MS-MDE/%5bMS-MDE%5d.pdf>) · Microsoft · spec · Date 2024-04-23 (revision 8.0) · Maintenance (legacy Windows Phone 8 and 8.1)
- Covers: Same shape as MDE2 but older: 3.1 IDiscoveryService, 3.2 STS, 3.3 XCEP, 3.5 Certificate Renewal, 3.6 XML Provisioning Document Schema, 4 examples, 6 Appendix A full WSDL.
- Notes: Superseded by MS-MDE2 for Windows 10 and 11. Keep only because the MS-MDM landing description still says devices are "enrolled ... through [MS-MDE]" and the WSDL in Appendix A is the same IDiscoveryService contract. Do not implement MDE v1-only behaviour.

#### [MS-XCEP]: X.509 Certificate Enrollment Policy Protocol
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-xcep/08ec4475-32c2-457d-8c27-5a176660a210> (PDF <https://winprotocoldoc.z19.web.core.windows.net/MS-XCEP/%5bMS-XCEP%5d.pdf>) · Microsoft · spec · Date 2026-03-09 (revision 18.0) · Active
- Covers: GetPolicies and GetPoliciesResponse over SOAP 1.2; 2.2 message syntax (client, requestFilter, response/policies/policy attributes: policySchema, privateKeyAttributes/minimalKeyLength, hashAlgorithmOIDReference, cAs, oIDs), 2.3 directory schema elements, 3.1 IPolicy server details, 4 examples, 6 Appendix A WSDL and XSD.
- Notes: MDE2 3.3 profiles it: for Windows enrollment the policy service is optional (Learn: default 2048-bit and SHA-1 if absent), and the enrollment client only consumes minimalKeyLength, hashAlgorithmOIDReference and cryptoProviders. Endpoint action `http://schemas.microsoft.com/windows/pki/2009/01/enrollmentpolicy/IPolicy/GetPolicies`. No separate XCEP errata page exists.

#### [MS-WSTEP]: WS-Trust X.509v3 Token Enrollment Extensions
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-wstep/4766a85d-0d18-4fa1-a51f-e5cb98b752ea> (PDF <https://winprotocoldoc.z19.web.core.windows.net/MS-WSTEP/%5bMS-WSTEP%5d.pdf>) · Microsoft · spec · Date 2024-04-23 (revision 15.0) · Active
- Covers: RequestSecurityToken and RequestSecurityTokenResponseCollection extensions (BinarySecurityToken PKCS#10 in, X509v3 or CMC out, RequestID, DispositionMessage), 3.1 SecurityTokenService server details, 4 RST/RSTR sequence, 6 WSDL.
- Notes: MDE2 3.4 overrides the token type: Windows uses `TokenType http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentToken`, RequestType `.../ws-trust/200512/Issue` (or `/Renew` for ROBO), PKCS10 ValueType `http://schemas.microsoft.com/windows/pki/2009/01/enrollment#PKCS10`, and returns ValueType `.../DeviceEnrollmentProvisionDoc` (base64 wap-provisioningdoc) instead of a bare certificate. Action URIs `.../enrollment/RST/wstep` and `.../enrollment/RSTRC/wstep`.

#### MS-WINERRATA errata page for MS-WSTEP
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-winerrata/2d7f0db9-8fcb-4cde-8182-e3f14568da12> · Microsoft · spec (errata) · Date 2024-10-30 (erratum 2021-09-21) · Historical (applies to WSTEP v14.0)
- Covers: 3.1.4.1.3.2 clarified: the server SHOULD include the end-entity cert in RequestedSecurityToken with ValueType X509v3, and MUST include a CMC full PKI response in the collection; "Microsoft Windows always includes the requested end entity certificate".
- Notes: Folded into v15.0. Not directly applicable to MDE2's provisioning-doc token, but explains the RSTRC shape.

#### MS-WINERRATA: Windows Protocols Errata (main page)
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-winerrata/314fe022-28ea-4bd9-93ac-7941ecf9ca10> · Microsoft · spec (errata index) · Date 2024-10-30 · Maintenance
- Covers: Lists MS-MDE2, MS-MDM and MS-WSTEP among "documents with active errata" (stale), errata archive PDFs 2015 to 2023, and the January 2024 policy note that documents are now republished rather than errata'd.
- Notes: Use the landing-page revision tables, not this page, to decide currency.

### 1.4 Microsoft Learn client-management pages

All pages are under `https://learn.microsoft.com/en-us/windows/client-management/`. Publisher is
Microsoft. Most carry ms.date 2025-08-04, which is a single bulk commit that touched the whole
section, not a content refresh.

#### OMA DM protocol support
- Source: <https://learn.microsoft.com/en-us/windows/client-management/oma-dm-protocol-support> · Microsoft · learn · Date 2025-08-04 · Active
- Covers: The concrete Windows subset. Transport: client-initiated HTTPS DM session over TLS; WAP Push and SMS notification and bootstrap "Not used by enterprise management". Bootstrap XML: OMA Client Provisioning. Commands: Add (implicit Add supported), Alert (1226 Generic Alert "when the user triggers an MDM unenrollment action from the device or when a CSP finishes some asynchronous actions"; 1224 Device alert for device-triggered events), Atomic (Add-then-Replace on the same node unsupported; nested Atomic and Get inside Atomic return 500; parent Atomic 507), Delete (subtree), Exec, Get (interior nodes return URI-encoded child names), Replace, Result, Sequence, Status. A non-command element under SyncBody, Atomic or Sequence gives 400; a missing CmdID gives a blank CmdID plus 400; LocURI cannot start with `/`; "Meta XML tag in SyncHdr is ignored by the device". Standard objects: DevInfo, DevDetail, DMS account objects. Security: Basic and MD5 client auth, MD5 server auth, HMAC integrity, TLS client and server certs. Node-name rules. WBXML: both XML and WBXML, chosen by `DEFAULTENCODING`. "In Windows 10, client support for uploading large objects to the server was added." SessionID: "If the server doesn't notify the device that it supports a new version (through SyncApplicationVersion node in the DMClient CSP), the client returns the SessionID in integer in decimal format. If the server supports DM session sync version 2.0 ... the device client returns 2 bytes." Session: "All messages from the server must have a MsgID that is unique within the session, starting at 1 for the first message, and increasing by an increment of 1"; 212 to Cred means no further auth this session; Chal and next-nonce rules. User vs device: Alert 1224 with `<Type xmlns="syncml:metinf">com.microsoft/MDM/LoginStatus</Type>` Data `user|others|none` in package 1; the server prefixes LocURI with `./user` or `./device` (default device). Status code table: 200, 202, 212, 214, 215, 216, 400, 401, 403, 404, 405, 406, 415, 418, 425, 500, 507, 516 with Windows meanings (405 write to read-only node, 418 node exists, 425 ACL, 500 "SyncML DPU can't map the originating error code").
- Notes: The single most useful page for the SyncML engine. It does not state a numeric MaxMsgSize. It cites the 1.2 Protocol PDF for the protocol and the 1.2.1 directory for Security. Newer than MS-MDM (2024-04-23) and consistent with it; MS-MDM adds the synctype and DevicePrepSync alerts and the HTTP header extensions this page omits.

#### Mobile device enrollment
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mobile-device-enrollment> · Microsoft · learn · Date 2025-08-04 · Active
- Covers: Three phases (discovery, certificate installation, DM client provisioning "via DM SyncML over HTTPS"); discovery is "a simple HTTP post call that returns XML"; XCEP GetPolicies; WSTEP; provisioning XML contents; best-practice note not to hard-code checks on User-Agent, fixed URIs or value formats; domain-joined devices can enroll only user-scoped; the built-in administrator and standard users cannot enroll; `DisableRegistration` registry key; SOAP fault format with the s: and a: subcode table (80180001 to 80180007) and `deviceenrollmentserviceerror` (`errortype`, `message`, `traceid`) with DeviceCapReached through InvalidEnrollmentData (80180013 to 80180019).
- Notes: Missing CustomServerError 80180032 that MDE2 2.2.10 now has; MDE2 v19.0 supersedes it.

#### MDM enrollment of Windows devices
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm-enrollment-of-windows-devices> · Microsoft · learn · Date 2025-08-04 · Active
- Covers: End-user paths: Entra join (OOBE or Settings; auto-enrolls if the tenant has MDM configured), Entra register or add work account (BYOD), "Enroll only in device management" (email to discovery; on-premise vs federated UI), deep link `ms-device-enrollment:?mode=mdm&username=...&servername=...&accesstoken=...&deviceidentifier=...&tenantidentifier=...&ownership=1|2|3`, Info and Disconnect (AllowManualMDMUnenrollment policy blocks user disconnect; server-initiated unenroll required), export management logs.
- Notes: Confirms the discovery hostname is derived from the email domain and that `servername` can bypass autodiscovery.

#### Federated authentication device enrollment
- Source: <https://learn.microsoft.com/en-us/windows/client-management/federated-authentication-device-enrollment> · Microsoft · learn · Date 2025-08-04 · Active
- Covers: Full worked flow: host `enterpriseenrollment.<domain>`; first request is a bare HTTP GET to `/EnrollmentServer/Discovery.svc` (expects 200, empty body), then an HTTPS SOAP POST `Discover` (namespace `http://schemas.microsoft.com/windows/management/2012/01/enrollment/`, action `.../IDiscoveryService/Discover`; fields EmailAddress, OSEdition, RequestVersion, DeviceType, ApplicationVersion, AuthPolicies); DiscoverResponse with AuthPolicy Federated, EnrollmentVersion, EnrollmentPolicyServiceUrl, EnrollmentServiceUrl, AuthenticationServiceUrl; "must not set Transfer-Encoding to Chunked"; WAB: `AuthenticationServiceUrl?appru=<ms-app://...>&login_hint=<UPN>`, the server returns an HTML form POSTing `wresult` to `ms-app://appid`; GetPolicies with `wsse:BinarySecurityToken ValueType=http://schemas.microsoft.com/5.0.0.0/ConfigurationManager/Enrollment/DeviceEnrollmentUserToken`; RST with TokenType DeviceEnrollmentToken, PKCS10 BST, AdditionalContext items (OSEdition, OSVersion, DeviceName, MAC, IMEI, EnrollmentType Full, DeviceType CIMClient_Windows, ApplicationVersion, DeviceID, TargetedUserLoggedIn); RSTR with a `DeviceEnrollmentProvisionDoc` BST; full `wap-provisioningdoc` sample (CertificateStore Root/System and My/User with PrivateKeyContainer, My/WSTEP/Renew ROBOSupport/RenewPeriod/RetryInterval, APPLICATION w7 with ADDR, CONNRETRYFREQ, INITIALBACKOFFTIME, MAXBACKOFFTIME, BACKCOMPATRETRYDISABLED, DEFAULTENCODING, SSLCLIENTCERTSEARCHCRITERIA, APPAUTH CLIENT/DIGEST and APPSRV/BASIC, DMClient/Provider/<ProviderID> with UPN, EntDeviceName, Poll/*); the policy and enrollment services must share a host name; at most one root and one intermediate CA in the doc.
- Notes: Best end-to-end reference for the go-oma-dm enrollment server; uses RequestVersion 3.0 samples (MDE2 allows up to 9.0). The sample RSTR envelope uses the SOAP 1.1 namespace while requests use SOAP 1.2; follow MDE2 2.1 with care and test against a real client.

#### On-premises authentication device enrollment
- Source: <https://learn.microsoft.com/en-us/windows/client-management/on-premise-authentication-device-enrollment> · Microsoft · learn · Date 2025-08-04 · Active
- Covers: Same flow with AuthPolicy OnPremise: `wsse:UsernameToken` (Username plus Password `#PasswordText`) in GetPolicies and RST headers; `domain\username` form when no email; identical wap-provisioningdoc sample.
- Notes: Simplest path for a self-hosted server. The DiscoverResponse text says "OnPremise is the supported value" and "Federated is added"; MDE2 lists all three.

#### Certificate authentication device enrollment
- Source: <https://learn.microsoft.com/en-us/windows/client-management/certificate-authentication-device-enrollment> · Microsoft · learn · Date 2025-08-04 · Active
- Covers: Discovery with `<AuthPolicies>Certificate</AuthPolicies>` and OSVersion, response AuthPolicy Certificate; GetPolicies with `BinarySecurityToken ValueType="X509v3"`; RST signed with `ds:Signature` referencing the X509 BST via `wsse:SecurityTokenReference`, plus an `EnrollmentData` context item; provisioning-package prerequisite.
- Notes: Requires a pre-provisioned client cert (provisioning package); niche for Windows 11 but part of MDE2 3.x, and it is the AuthPolicy WinDC uses for Entra registered devices.

#### Certificate renewal
- Source: <https://learn.microsoft.com/en-us/windows/client-management/certificate-renewal-windows-mdm> · Microsoft · learn · Date 2025-08-04 · Active
- Covers: Manual (user-prompted) and automatic ROBO renewal; ROBO uses client TLS with the existing MDM cert, no user token, "only supported with Microsoft PKI" (note), mandatory for Federated enrollments; EntDMID must be set first; RST with RequestType `http://docs.oasis-open.org/ws-sx/ws-trust/200512/Renew`, BST ValueType `...wssecurity-secext-1.0.xsd#PKCS7` (PKCS#7 signed by the old cert, containing the PKCS#10); the server checks signature, renewal window, issuer, same requester; the response is a wap-provisioningdoc with the new cert (plus optional root) and `APPLICATION/PROVIDER-ID`; the renewal schedule is set at enrollment via CertificateStore `My/WSTEP/Renew` (RenewPeriod, RetryInterval, ROBOSupport); the device refuses HTTP redirects during ROBO; recommends a 40 to 60 day renewal window and 4 to 5 day retry; supported CSPs during enrollment and renewal: CertificateStore, w7 APPLICATION, DMClient, EnterpriseAppManagement.
- Notes: MDE2 3.5 (Certificate Renewal) and 3.6 (Certificate Recovery) are the normative versions; MDE2 v19.0 also documents `My/WSTEP/Renew/ServerURL`. The "EnterpriseAppManagement" link on this page points at the App-V CSP page, a broken cross-reference.

#### Support for Windows Information Protection (WIP) on Windows
- Source: <https://learn.microsoft.com/en-us/windows/client-management/implement-server-side-mobile-application-management> · Microsoft · learn · Date 2025-08-04 · Maintenance (WIP deprecated since July 2022)
- Covers: MAM enrollment is an MDE2 "MAM extension": no MDM discovery, Entra federated auth only, no client cert and no XCEP, Entra token for policy sync over one-way TLS; minimal wap-provisioningdoc (APPLICATION w7 with ADDR and `DEFAULTENCODING application/vnd.syncml.dm+xml`, no APPAUTH, default 24 h poll); allowed CSP list; MAM-to-MDM conversion via `ManagementServerToUpgradeTo`.
- Notes: Out of scope for a Windows 11 MDM (WIP sunset), but the only Learn text stating DMClient's `ManagementServerToUpgradeTo` purpose.

#### Mobile Device Management overview
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm-overview> · Microsoft · learn · Date 2025-08-04 · Active
- Covers: Enrollment client plus management client; third-party servers use MS-MDE2 and MS-MDM; "Only one MDM is allowed"; MDM security baselines; `dmwappushsvc` (the WAP push routing service that "initiates and orchestrates management sync sessions") must not be disabled.
- Notes: Entry point; no protocol detail.

#### Enroll a Windows device automatically using Group Policy
- Source: <https://learn.microsoft.com/en-us/windows/client-management/enroll-a-windows-10-device-automatically-using-group-policy> · Microsoft · learn · Date 2025-08-04 (updated 2026-05-28) · Active · Third-party MDM: yes, if the server is registered as the tenant's MDM application
- Covers: GPO "Enable automatic MDM enrollment using default Microsoft Entra credentials" with User Credential or Device Credential (1903+); a scheduled task under `Microsoft\Windows\EnterpriseMgmt` runs every 5 minutes for a day; requires hybrid Entra join and the tenant's MDM discovery URL; "In Windows 10, version 1709, the enrollment protocol was updated to check whether the device is domain-joined" (MDE2 4.3.1); error 0x80180026 MENROLL_E_DEVICE_MANAGEMENT_BLOCKED.
- Notes: Most recently touched page in the set (2026-05-28).

#### Known issues in MDM
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm-known-issues> · Microsoft · learn · Date 2025-08-04 · Active
- Covers: Get inside Atomic unsupported; CDATA in SyncML data unsupported; SCEP IIS SSL setting; enrollment fails behind an authenticating proxy; server-initiated unenroll silently fails for add-work-account enrollments and is disabled for Entra-joined devices (wipe instead); ClientCertificateInstall dual-store install bug; DevDetail OSPlatform mismatch on Windows 11; multi-cert Wi-Fi and VPN EAP filtering; "MDM client will immediately check in with the MDM server after client renews WNS channel URI" and the server "should send a GET request for ProviderID/Push/ChannelURI" at every check-in; `./User` provisioning fails on Entra-joined devices unless signed in as an Entra user; push-button reset keeps enrollment but deletes task schedules.
- Notes: Two of these (no CDATA, re-read ChannelURI every session) are hard requirements for the server.

#### Collect MDM logs
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm-collect-logs> (the old `diagnose-mdm-failures-in-windows-10` URL redirects here) · Microsoft · learn · Date 2025-08-04 · Active
- Covers: Settings, Access work or school, Info, Create report (`C:\Users\Public\Documents\MDMDiagnostics`); `mdmdiagnosticstool.exe -area "DeviceEnrollment;DeviceProvisioning;Autopilot" -zip ...`; zip contents (MDMDiagHtmlReport.html, MDMDiagReport.xml, registry dump, evtx); Event Viewer channel `Microsoft-Windows-DeviceManagement-Enterprise-Diagnostics-Provider` Admin and Debug; DiagnosticLog CSP SyncML samples (all use `xmlns="SYNCML:SYNCML1.2"`); ETW provider 3DA494E4-0FE2-415C-B895-FB5265C5C83B; `DeviceStateData/MdmConfiguration` snapshot.
- Notes: Cite the canonical `mdm-collect-logs` URL.

#### DMClient CSP (full node inventory)
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/dmclient-csp> · Microsoft · learn · Date 2025-07-22 · Active
- Covers: `./Device/Vendor/MSFT/DMClient` (HWDevID, Unenroll [Exec], UpdateManagementServiceAddress) and `Provider/{ProviderID}/` nodes: AADDeviceID, AADResourceID, AADSendDeviceToken, CertRenewTimeStamp, CommercialID, ConfigLock/*, ConfigRefresh/* (Enabled, Cadence 30 to 1440 default 90, PausePeriod), CustomEnrollmentCompletePage/*, EnableOmaDmKeepAliveMessage, EnhancedAppLayerSecurity/* (Cert0, Cert1, SecurityMode 0 to 3, UseCertIfRevocationCheckOffline), EnrollmentType (Device|Full), EntDeviceName, EntDMID, ExchangeID, FirstSyncStatus/* (ExpectedPolicies, ExpectedNetworkProfiles, ExpectedMSIAppPackages, ExpectedModernAppPackages, ExpectedPFXCerts, ExpectedSCEPCerts, ServerHasFinishedProvisioning, IsSyncDone, WasDeviceSuccessfullyProvisioned, TimeOutUntilSyncFailure, Skip*StatusPage, BlockInStatusPage, AllowCollectLogsButton, CustomErrorText), ForceAadToken, Help*, HWDevID, LinkedEnrollment/* (see 1.6), ManagementServerAddressList (`<URL1><URL2>`), ManagementServerToUpgradeTo, ManagementServiceAddress, MaxSyncApplicationVersion (Get), MultipleSession/*, NumberOfDaysAfterLostContactToUnenroll, Poll/* (IntervalForFirstSetOfRetries default 15, NumberOfFirstRetries default 10, second and remaining sets, PollOnLogin, AllUsersPollOnFirstLogin), PublisherDeviceID, Push/PFN (Add/Delete/Get/Replace), Push/ChannelURI (Get), Push/Status (Get, 0 success, 1 to 8 failures), Recovery/*, RequireMessageSigning, SignedEntDMID, SyncApplicationVersion ("1.0 or 2.0 ... once set to 2.0, won't revert"), Unenroll [Exec], UPN. User scope `./User/Vendor/MSFT/DMClient/Provider/{ProviderID}/FirstSyncStatus/*`.
- Notes: Central to bootstrap (the `DMClient` characteristic in the wap-provisioningdoc), polling, push, unenroll and the session-version negotiation. MS-MDM 3.2.5.1.3 refers to `MaxSyncApplicationVersion` 5.0 for AVD; this page documents values 1.0 and 2.0 only.

#### DMAcc CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/dmacc-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: "allows an OMA Device Management (DM) version 1.2 server to handle OMA DM account objects"; `./SyncML/DMAcc/{AccountUID}` (UID is the SHA-256 of the w7 PROVIDER-ID when bootstrapped), AAuthPref (BASIC|DIGEST), AppAddr/{1}/Addr, AddrType (URI|IPv4), Port/{1}/PortNbr, AppAuth/{CLCRED|SRVCRED}/AAuthLevel, AAuthType (CLCRED: BASIC|DIGEST; SRVCRED: DIGEST), AAuthName, AAuthSecret, AAuthData (next nonce, bin), AppID (w7 only), Name, PrefConRef, ServerID, Ext/Microsoft/{BackCompatRetryDisabled, ConnRetryFreq (3), CRLCheck, DefaultEncoding (`application/vnd.syncml.dm+xml` default or `+wbxml`), DisableOnRoaming, InitialBackOffTime (16000 ms), InitiateSession (Add starts a session, 1703+), MaxBackOffTime (86400000), ProtoVer (1.1|1.2, sets VerDTD in package 1), Role, SSLCLIENTCERTSEARCHCRITERIA, UseHwDevID, UseNonceResync}.
- Notes: Device scope only. The OMA StdObj DMAcc with Microsoft extensions; the w7 bootstrap populates it, but `Ext/Microsoft/InitiateSession` and `DefaultEncoding` are useful. The AAuthLevel value names here (CLCRED/SRVCRED) differ from the w7 names (CLIENT/APPSRV).

#### w7 APPLICATION CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/w7-application-csp> · Microsoft · learn · Date 2017-06-26 (updated 2024-01-18) · Maintenance
- Covers: APPADDR/ADDR, ADDRTYPE, PORT/PORTNBR; APPAUTH/AAUTHDATA (base64 nonce for DIGEST; octal form used in the hash), AAUTHLEVEL (APPSRV = client authenticates to server; CLIENT = server authenticates to client), AAUTHNAME, AAUTHSECRET, AAUTHTYPE (BASIC = syncml:auth-basic, DIGEST = syncml:auth-md5; CLIENT level MUST be DIGEST); APPID (w7); BACKCOMPATRETRYDISABLED (valueless; presence disables re-send with the older VerDTD); CONNRETRYFREQ (3); DEFAULTENCODING; INIT (mobile-operator only, fails for enterprise); INITIALBACKOFFTIME (16000); MAXBACKOFFTIME (86400000); NAME; PROTOVER (1.1|1.2); PROVIDER-ID; ROLE (8 operator, 32 enterprise; enterprise set only by the enrollment client); TO-NAPID; USEHWDEVID; SSLCLIENTCERTSEARCHCRITERIA (`Subject=...&Stores=My%5CUser`, U+F000 separators). All names uppercase and case-sensitive; both APPSRV and CLIENT credentials must be provided.
- Notes: Oldest ms.date in the set but still the only Learn page for these parameters; MDE2 2.2.9.5 (2026) is the normative and newer description and takes precedence where they differ.

#### DeviceStatus CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/devicestatus-csp> · Microsoft · learn · Date 2025-10-03 · Active
- Covers: `./Vendor/MSFT/DeviceStatus` (device scope, Get only): Antispyware/*, Antivirus/*, Battery/*, CellularIdentities/{IMEI}/*, CertAttestation/MDMClientCertAttestation (Windows 11 21H2 KB5018483+; XML blob), Compliance/EncryptionCompliance, DeviceGuard/*, DMA/BootDMAProtectionStatus, DomainName, Firewall/Status, NetworkIdentifiers/{MAC}/*, OS/Edition, OS/Mode, SecureBoot/* (cert update status and error), SecureBootState, TPM/*, UAC/Status.
- Notes: Read-only inventory and compliance source for Windows 11; the newest CSP page here.

#### DevDetail CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/devdetail-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: `./DevDetail` (Get): DevTyp, OEM, FwV, HwV, SwV (`Major.Minor.Build.QFE`), LrgObj (bool: OMA DM Large Object Handling supported), URI/MaxDepth, MaxTotLen, MaxSegLen (0 = unlimited), Ext/Microsoft/* (CommercializationOperator, DeviceName [Get/Replace], DNSComputerName [Get/Replace, `%RAND:n%` and `%SERIAL%`, server must reboot], FreeStorage, LocalTime, MobileID, OSPlatform, ProcessorArchitecture, ProcessorType, RadioSwV, Resolution, SMBIOSSerialNumber, SMBIOSVersion, SystemSKU, TotalRAM, TotalStorage), Ext/DeviceHardwareData (opaque base64 hardware hash), Ext/WLAN*, Ext/VoLTEServiceSetting. Not sent automatically.
- Notes: `LrgObj` and `URI/*` are the OMA StdObj capability signals the server should read before chunking; the known-issues page says OSPlatform is wrong on Windows 11.

#### DevInfo CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/devinfo-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: `./DevInfo` "automatically sent to the OMA DM server at the beginning of each OMA DM session": DevId (app-specific GUID on desktop regardless of UseHWDevID), DmV, Lang (RFC 1766), Man, Mod, Ext/ICCID.
- Notes: DevId is the value in package 1's `Source/LocURI` and the Replace items; the server should key device identity on the enrollment DeviceID and certificate rather than on DevId format.

#### CertificateStore CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/certificatestore-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: `./Vendor/MSFT/CertificateStore` with Root/System, CA/System, My/User, My/System (each `{CertHash}/EncodedCertificate`, IssuedBy, IssuedTo, ValidFrom, ValidTo, TemplateName), My/SCEP/{UniqueID}/Install/* and Status, My/WSTEP/Renew/* (ServerURL, RenewPeriod, RetryInterval, ROBOSupport, Status, ErrorCode, LastRenewalAttemptTime, RenewNow [Exec]), My/WSTEP/CertThumbprint.
- Notes: Used during enrollment (root, intermediate and client cert install and renewal schedule) and over DM. MDE2 2.2.9.2 duplicates the enrollment-relevant subset and is newer.

#### ClientCertificateInstall CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/clientcertificateinstall-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: `PFXCertInstall/{UniqueID}/*` (KeyLocation, ContainerName, PFXCertBlob, PFXCertPassword, PFXCertPasswordEncryptionType, PFXKeyExportable, Thumbprint, Status, PFXCertPasswordEncryptionStore) and `SCEP/{UniqueID}/Install/*` (ServerURL, Challenge, EKUMapping, KeyUsage, SubjectName, KeyProtection, RetryDelay, RetryCount, TemplateName, KeyLength, HashAlgorithm, CAThumbprint, SubjectAlternativeNames, ValidPeriod, ValidPeriodUnits, ContainerName, CustomTextToShowInPrompt, Enroll [Exec], AADKeyIdentifierList) plus Status, ErrorCode, CertThumbprint, RespondentServerUrl; device and user scope.
- Notes: Post-enrollment certificate delivery; the known-issues page documents the dual-store install bug.

#### RootCATrustedCertificates CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/rootcacertificates-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: `./Device|User/Vendor/MSFT/RootCATrustedCertificates/{Root, CA, TrustedPublisher, TrustedPeople, UntrustedCertificates, TPMTrustedRoot}/{CertHash}/...`.
- Notes: MDE2 2.2.9.4 allows this CSP in the enrollment wap-provisioningdoc as well.

#### RemoteWipe CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/remotewipe-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: `./Device/Vendor/MSFT/RemoteWipe/` doWipe, doWipePersistProvisionedData, doWipePersistUserData, doWipeProtected, doWipeCloud, doWipeCloudPersistProvisionedData, doWipeCloudPersistUserData (Windows 11 22H2+), AutomaticRedeployment/doAutomaticRedeployment [Exec] plus LastError and Status. All wipe nodes are Exec; "The return status code shows whether the device accepted the Exec command".
- Notes: Exec returns synchronously (Status 200 = accepted); no completion alert is promised.

#### Reboot CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/reboot-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: `RebootNow` (Exec/Get; reboots within 5 minutes or at the end of the sync session), `Schedule/Single`, `Schedule/DailyRecurrent`, `Schedule/WeeklyRecurrent` (Windows 11 24H2+), ISO 8601 values, an empty value deletes the schedule.
- Notes: Simple Exec/Replace target; a good conformance test case.

#### EnrollmentStatusTracking CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/enrollmentstatustracking-csp> · Microsoft · learn · Date 2019-05-21 (updated 2024-01-18) · Maintenance · Third-party MDM: yes
- Covers: Device and user trees: DevicePreparation/PolicyProviders/{Provider}/InstallationState (1 to 4), LastError, Timeout (15 min default), TrackedResourceTypes/Apps; Setup/Apps/PolicyProviders/{Provider}/TrackingPoliciesCreated; Setup/Apps/Tracking/{Provider}/{App}/TrackingUri, InstallationState, RebootRequired; Setup/HasProvisioningCompleted. Written by policy providers, read by the Enrollment Status Page; pairs with DMClient FirstSyncStatus.
- Notes: Only needed if go-oma-dm wants to drive the ESP experience; the DMClient FirstSyncStatus nodes are the server-facing half.

#### DeviceManageability CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/devicemanageability-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: `Capabilities/CSPVersions` (Get, xml, versions of all CSPs), `Provider/{ProviderID}/ConfigInfo`, `EnrollmentInfo`, `PayloadTransfer`.
- Notes: Query CSPVersions once after enrollment to gate features per device.

#### EnterpriseModernAppManagement CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/enterprisemodernappmanagement-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: AppManagement/{AppStore, nonStore, System}/{PFN}/* inventory and DoNotUpdate, NonRemovable, AppUpdateSettings, AppInventoryQuery/AppInventoryResults, UpdateScan, ResetPackage [Exec, Windows 11 21H2+], ReleaseManagement; AppInstallation/{PFN}/StoreInstall and HostedInstall [Exec], Status, LastError, ProgressStatus; AppLicenses/StoreLicenses/{LicenseID}/AddLicense, GetLicenseFromStore [Exec]. Device and user scope.
- Notes: The `./user/vendor/MSFT/EnterpriseModernAppManagement/AppInstallation/<PFN>/StoreInstall` path is the OMA-DM page's example of a user-scoped LocURI.

#### EnterpriseDesktopAppManagement CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/enterprisedesktopappmanagement-csp> · Microsoft · learn · Date 2025-03-12 · Active
- Covers: `./Device|User/Vendor/MSFT/EnterpriseDesktopAppManagement/MSI/{ProductID}/` DownloadInstall (Add then Exec with `MsiInstallJob` XML: Product@Version, Download/ContentURLList/ContentURL, Validation/FileHash SHA-256, Enforcement/CommandLine, TimeOut, RetryCount, RetryInterval, DownloadFromAad), InstallDate, InstallPath, LastError, Name, Publisher, Status, Version; MSI/UpgradeCode/{Guid}; Delete = uninstall; "Atomic Required: True" on {ProductID}; async completion reported via Alert 1224 with `Type Reversed-Domain-Name:com.microsoft.mdm.win32csp_install`, Format int, Mark informational; per-user vs per-machine MSI context table.
- Notes: The samples use `xmlns="SYNCML:SYNCML1.1"`, inconsistent with MS-MDM's `SYNCML:SYNCML1.2`. The Alert example is the best worked instance of a CSP-generated asynchronous alert.

#### Win32AppInventory CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/win32appinventory-csp> · Microsoft · learn · Date 2017-06-26 (updated 2024-01-18) · Maintenance
- Covers: `./Vendor/MSFT/Win32AppInventory/Win32InstalledProgram/{InstalledProgram}/Name, Publisher, Version, Language, RegKey, Source, MsiProductCode, MsiPackageCode` (Get only).
- Notes: Read-only inventory. The legacy `enterpriseappmanagement-csp` page no longer exists (section 10).

### 1.5 CSP reference and DDF v2

#### Configuration service provider DDF files
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/configuration-service-provider-ddf> · Microsoft · learn · ms.date 2020-09-18, updated 2026-02-20 (gitcommit `0fca580e`) · Active
- Covers: Current bundle "DDF v2 Files, February 2026" (<https://download.microsoft.com/download/015bd9f5-9cca-4821-8a85-a4c5f9a5d0f2/DDFv2Feb2026.zip>); inline DDF v2 XSD (targetNamespace `http://tempuri.org/DM_DDF-V1_2`, imports `DDFv2Msft.xsd`) and MSFT XSD (`http://schemas.microsoft.com/MobileDevice/DM`); older bundles: DDFv2Sept25.zip, DDFv2July25.zip, DDFv2Feb25.zip, DDFv2September24.zip, DDFv2May24.zip, DDFv2September2023.zip, DDFv2December2022.zip, then per-version Windows 10 zips (2004 back to 1607) and standalone `PolicyDDF_all_*.xml` files (20H2 back to 1607).
- Notes: This page is the XSD source; the XSDs are not shipped inside the zips. The MSFT XSD documents `Applicability/OsBuildVersion` as "the first build that a feature is released to. If the feature was backported, multiple OS versions will be listed, such that the OS build version without a minor number is the first major release"; also `CspVersion`, `EditionAllowList` (hex edition IDs; "0x88* refers to Windows Holographic for Business"), `RequiresAzureAd`, `AllowedValues@ValueType` in {XSD, RegEx, ADMX, JSON, ENUM, Flag, Range, SDDL, None}, `ConflictResolution` enum, `ReplaceBehavior` {Append, Replace}, `RebootBehavior` {None, Automatic, ServerInitiated}, `GpMapping`, `DependencyBehavior`, `AtomicRequired`, `Deprecated@OsBuildDeprecated`. HTTP `Last-Modified` for the zips: Feb2026 2026-02-19, Sept25 2025-09-23, July25 2025-07-08, Feb25 2025-02-28. Feb 2026 supersedes Sept 2025; it is the newest bundle and predates 26H2.

#### Policy DDF file
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/policy-ddf-file> · Microsoft · learn · 301 redirect to configuration-service-provider-ddf · Historical
- Covers: Nothing standalone any more; the Policy area DDFs are now the `*_AreaDDF.xml` files in the combined bundle plus the legacy `PolicyDDF_all_*.xml` links (newest 20H2).
- Notes: Do not cite as a separate source; go-oma-dm should treat `Policy.xml` plus `*_AreaDDF.xml` in the bundle as the Policy DDF.

#### Configuration service provider reference (index)
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/> · Microsoft · learn · ms.date 2024-10-07, updated 2024-10-09 (the old `configuration-service-provider-reference` slug 301-redirects here) · Active
- Covers: Landing page linking support scenarios, DDF files, Contribute, Declared Configuration protocol, Policy CSP, ADMX policy lists, HoloLens 2 and Surface Hub support lists.
- Notes: There is no longer a single flat "all CSPs" list page; the authoritative CSP inventory is the bundle (52 standalone CSP DDFs, listed in the bundle inspection) and the TOC.

#### Configuration service provider support
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/configuration-service-provider-support> · Microsoft · learn · ms.date 2020-09-18, updated 2024-08-08 · Maintenance
- Covers: Now only a short intro (WAP vs SyncML, MDM Bridge WMI provider link, DDF link); 110 words.
- Notes: The historical edition matrix is gone from this page. Edition applicability now lives per node in `MSFT:EditionAllowList` in the DDFs and in each CSP page's "Scope / Editions / Applicable OS" table (Pro, Enterprise, Education, IoT Enterprise/LTSC; Home is not listed for any CSP checked).

#### Policy CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/policy-configuration-service-provider> · Microsoft · learn · ms.date 2026-05-08, updated 2026-05-15 · Active
- Covers: `./[Device|User]/Vendor/MSFT/Policy/{Config|Result}/{AreaName}/{PolicyName}`; `./Device/Vendor/MSFT/Policy/ConfigOperations/ADMXInstall/{AppName}/{SettingsType}/{AdmxFileId}` and `.../Properties/{SettingsType}/{AdmxFileId}/Version`; `./Vendor/MSFT/Policy/...` equivalence for device scope; 0xF000 list separator; Atomic recommendation; 260+ policy areas listed (including 2025 and 2026 additions such as `Sudo`, `WindowsAI`, `SecureBoot`, `TenantDefinedTelemetry`, `AppDeviceInventory`, `CloudDesktop`, `FederatedAuthentication`).
- Notes: `{PolicyName}` leaf format is `null` at the generic level; the real types come from the area DDFs. Nothing here is 26H2-specific.

#### Understanding ADMX policies
- Source: <https://learn.microsoft.com/en-us/windows/client-management/understanding-admx-backed-policies> · Microsoft · learn · ms.date 2025-08-04 · Active
- Covers: ADMX-backed policy payload grammar: `<enabled/>` / `<disabled/>` / Delete = Not Configured; `<data id="..." value="..."/>` per ADMX element (text to REG_SZ, multiText to REG_MULTI_SZ with `&#xF000;` separators, list to name/value pairs `&#xF000;`-separated, enum to decimal value, decimal, boolean "true"/"false"); manifest mapping `<ADMXPolicy area="appv~AT~System~CAT_AppV~CAT_Publishing" ...>`; SyncML uses `SYNCML:SYNCML1.2`.
- Notes: Essential for go-oma-dm's Policy CSP encoder. DDF `AllowedValues ValueType="ADMX"` plus `<MSFT:AdmxBacked Area= Name= File=>` gives the ADMX file and policy per node (2,518 such nodes in the Feb 2026 bundle) so the generated schema can carry the link.

#### Win32 and Desktop Bridge app ADMX policy ingestion
- Source: <https://learn.microsoft.com/en-us/windows/client-management/win32-and-centennial-app-policy-configuration> · Microsoft · learn · ms.date 2025-08-04, updated 2025-11-20 · Active
- Covers: `Add` of ADMX text to `./Vendor/MSFT/Policy/ConfigOperations/ADMXInstall/{AppName}/{SettingType}/{FileUid}`; derived area name `{AppName}~{SettingType}~{CategoryPathFromAdmx}` (categories joined by `~`); class Machine to `./Device`, User to `./User`, Both to either; registry allow-list for ingested policies; `Replace` support from 1709 plus KB.
- Notes: The example SyncML mixes `SYNCML:SYNCML1.2` (ADMXInstall) and `SYNCML:SYNCML1.1` (policy set); Windows accepts both.

#### Enable ADMX policies in MDM
- Source: <https://learn.microsoft.com/en-us/windows/client-management/enable-admx-backed-policies-in-mdm> · Microsoft · learn · ms.date 2025-08-04 · Active
- Covers: Step by step: find the GP name and ADMX file in the policy page, read `<elements>` from `C:\Windows\PolicyDefinitions\*.admx`, build the `<data id>` payload; `<Data><Enabled/></Data>` unencoded example; CDATA recommendation.
- Notes: Note the inconsistent casing `<Enabled/>` vs `<enabled/>` across pages; the client is case-insensitive here but the generator should normalise.

#### ADMX-backed policies in Policy CSP (list)
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/policies-in-policy-csp-admx-backed> · Microsoft · learn · ms.date 2024-06-28 · Maintenance
- Covers: Index of about 150 ADMX policy areas.
- Notes: Older than the DDF; use the DDF's `AdmxBacked` elements as the source of truth.

#### MicrosoftDocs/windows-itpro-docs (GitHub)
- Source: <https://github.com/MicrosoftDocs/windows-itpro-docs> · Microsoft · repo · HTTP 404 on 2026-09-06 (gh API 404 while authenticated; raw.githubusercontent 404; GitHub search returns only third-party forks; last Wayback snapshot 2025-06-14) · Historical
- Covers: Was the public mirror of `windows/client-management/mdm/`, default branch `public`.
- Notes: The public mirror is gone, so per-file commit history cannot be queried. The private upstream is `MicrosoftDocs/windows-docs-pr` (branch `live`), exposed only via each Learn page's `gitcommit` front matter. Best available proxy for the most recent change to each file: `declared-configuration*.md`, `oma-dm-protocol-support.md`, `mobile-device-enrollment.md`, `understanding-admx-backed-policies.md`, `enable-admx-backed-policies-in-mdm.md`, `new-in-windows-mdm-enrollment-management.md` all carry `1493a5a72db82d6a7915960d885588b7a47ac3b2`, 2025-08-04 (bulk commit); `mdm/dmclient-csp.md` 2025-07-22; `mdm/declaredconfiguration-csp.md` `aeb8f2bbd5d9`, 2025-03-12; `mdm/declaredconfiguration-ddf-file.md` `80cc10b7eda7`, 2025-02-14; `mdm/configuration-service-provider-ddf.md` `0fca580e8fc5`, 2026-02-20; `mdm/policy-configuration-service-provider.md` `f57c1fe2374b`, 2026-05-15; `win32-and-centennial-app-policy-configuration.md` `290feeb1f425`, 2025-11-20.

---

#### DDF bundle inspection (DDFv2Feb2026.zip)

- Archive: 727,650 bytes; HTTP `Last-Modified: Thu, 19 Feb 2026 18:04:42 GMT`; zip entries dated 2026-02-06; single top-level folder `DDFDrop012026/` (the Sept 2025 bundle used `DDFDrop09112025/`, entries dated 2025-09-12, 724,597 bytes).
- Contents: 313 files, all `.xml`. No XSD or DTD files (the DDF v2 and MSFT XSDs exist only inline on the Learn page; `VPNv2.xml` references an external `EapHostConfig.xsd` via `schemaLocation` that is also not shipped). Every file: `<?xml version="1.0" encoding="UTF-8"?>` with a UTF-8 BOM, `<!DOCTYPE MgmtTree PUBLIC " -//OMA//DTD-DM-DDF 1.2//EN" "http://www.openmobilealliance.org/tech/DTD/DM_DDF-V1_2.dtd" [<?oma-dm-ddf-ver supported-versions="1.2"?>]>`, `<MgmtTree xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM">`, `<VerDTD>1.2</VerDTD>`, an empty `<MSFT:Diagnostics/>`, then one or more root `<Node>`.
- Namespaces actually used: `xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM"` in all 313 files. The DDF elements themselves are unprefixed and undeclared: no file declares `http://tempuri.org/DM_DDF-V1_2` as the default namespace, even though the XSD's `targetNamespace` is that URI. A strict XSD-validating parser will reject the files; a Go generator must parse the DDF elements in no namespace (or inject the default namespace). Other namespaces seen only inside embedded XSD payloads: `xmlns:xs` (17 files), `xmlns:xsd` (1), `xmlns:q1="http://www.microsoft.com/provisioning/EapHostConfig"` (2).
- Naming pattern: `<CSPName>.xml` for 52 standalone CSPs; `<AreaName>_AreaDDF.xml` for 261 Policy CSP areas (145 `ADMX_*` plus 116 native areas). `Policy.xml` holds only the Policy CSP skeleton (Config/Result/ConfigOperations, 23 nodes); each area file is rooted at `./[User|Device]/Vendor/MSFT/Policy/Config/<Area>`. 109 files carry both a `./User/...` and a `./Device/...` root tree.
- Standalone CSP names (52): ActiveSync, AppLocker, ApplicationControl, AssignedAccess, BitLocker, CertificateStore, ClientCertificateInstall, CloudDesktop, DMAcc (`./SyncML/DMAcc`), DMClient, DeclaredConfiguration, Defender, DevDetail (`./DevDetail`), DevInfo (`./DevInfo`), DeviceManageability, DevicePreparation, DeviceStatus, DiagnosticLog, DnsClient, EMAIL2, EnterpriseDesktopAppManagement, EnterpriseModernAppManagement, Firewall, HealthAttestation, LAPS, LanguagePackManagement, MultiSIM, NetworkProxy, NetworkQoSPolicy, NodeCache, Office, PDE, PassportForWork, Personalization, Policy, PrinterProvisioning, Reboot, RemoteLock, RemoteRemediation, RemoteWipe, RootCATrustedCertificates, SUPL (path typo `./Vendor/MSFT//SUPL`), SecureAssessment, SharedPC, VPNv2, WiFi, WindowsBackupAndRestore, WindowsDefenderApplicationGuard, WindowsLicensing, WiredNetwork, WirelessNetworkPreference, eUICCs. 18 of these use legacy root paths without a `./Device` or `./User` prefix (for example `./Vendor/MSFT/Firewall`, `./Vendor/MSFT/DeviceStatus`).
- Size: 5,955 `<Node>` elements; 5,146 leaves. `DFFormat` census: chr 3,467; int 1,379; node 809; bool 218; b64 28; null 28; xml 21; bin 3; time 2 (no `date` or `float` used). Largest files: InternetExplorer_AreaDDF (536 nodes), ADMX_UserExperienceVirtualization (247), EnterpriseModernAppManagement (224), VPNv2 (204), Firewall (138).
- MSFT extension census: AllowedValues 4,489 (ValueType: ADMX 2,518; ENUM 1,284; None 488; Range 270; RegEx 33; XSD 18; Flag 11; SDDL 1; JSON 1); Applicability 4,120 (OsBuildVersion 4,118; CspVersion 4,115; EditionAllowList 3,767; RequiresAzureAd 0); ConflictResolution 3,714; AdmxBacked 2,518; GpMapping 907; List 197; DynamicNodeNaming 127 (UniqueName 65, ClientInventory 33, ServerGeneratedUniqueIdentifier 29); DependencyBehavior 125; Deprecated 31 (OsBuildDeprecated: 10.0.22000 x20, 10.0.14393 x2, 10.0.26100.712, 10.0.22631.2361); AtomicRequired 21; RebootBehavior 18. Only `DnsClient.xml` lacks Applicability entirely.
- OsBuildVersion values: a comma-separated list, first entry the "major release" (no revision), later entries backports with revisions. Highest real values: `10.0.26100.7019` and `10.0.26200.7019` (Firewall `MdmStore/Global/EnableAuditMode`), and `11.0.26200.7019, 11.0.26100.7019` (LocalPoliciesSecurityOptions `UserAccountControl_BehaviorOfTheElevationPromptForAdministratorProtection` and `_TypeOfAdminApprovalMode`; the anomalous `11.0.` major occurs nowhere else and should be treated as a data quirk). Other 26100 revisions present: .2314, .3323, .3360, .3613, .3624, .3775, .3915, .4770, .4946, .5074, .6725 (28 files mention 26100). `26200` appears in only 2 files; `26300` and `28000` appear nowhere. Sentinels: `99.9.99999` (141 nodes, "Insider"), `99.9.9999`, `88.8.88888`; `10.0.25000`, `10.0.26000`, `10.0.25145/25398/25965` (Insider and Server builds) also appear. CspVersion values: 1.0 to 1.6, 2.0 to 9.0, 9.9, 10/10.0, 11.0.
- Delta vs DDFv2Sept25 (same 313 count): added `SecureBoot_AreaDDF.xml` (Policy/SecureBoot: EnableSecurebootCertificateUpdates, ConfigureHighConfidenceOptOut, ConfigureMicrosoftUpdateManagedOptIn; applicability 10.0.19044), removed `SpeakForMe_AreaDDF.xml`; 19 files changed. Notable additions: Update/`MaintenanceWindow*` (17 nodes, 99.9.99999), WindowsAI (DisableAgentWorkspaces, DisableAgentConnectors, DisableRemoteAgentConnectors, AgentConnectorMinimumPolicy, RemoveMicrosoftCopilotApp, DisableRecallDataProviders), ApplicationManagement/EnableMsixAllowedZones and EnableMsixSmartScreenCheck (10.0.26000), DeviceStatus/SecureBoot/{SecurebootCertificateUpdateStatus,SecurebootCertificateUpdateError}, InternetExplorer *ZoneEnableProtectedMode, FileExplorer/DisableFileExplorerPrelaunch, Games/DisableGamingFullScreenExperience, eUICCs `DownloadServers/{id}/ErrorDetail`; WirelessNetworkPreference root moved from `./Device/Vendor/MSFT` to `./Vendor/MSFT`; several 99.9.99999 nodes resolved to real builds (for example SettingsSync/EnableWindowsBackup to 10.0.26100, 10.0.22621.5682, 10.0.19041.6141).
- Verdict for a generated-schema source: DDF v2 is viable and is the only machine-readable, versioned CSP description Microsoft publishes (roughly semi-annual drops; Feb 2026 supersedes Sept 2025). Caveats a generator must encode: undeclared default namespace; `OsBuildVersion` as a list with sentinels and the `11.0.` quirk; dynamic nodes with empty `<NodeName>` plus `DFTitle`; `Path` only on root nodes; Applicability inheritance to children; `AllowedValues` payloads of nine kinds (ADMX ones need the ADMX files, which are not in the bundle); legacy root paths; and the DeclaredConfiguration DDF being incomplete relative to the protocol docs.

### 1.6 Windows declared configuration (WinDC)

#### Windows declared configuration protocol (overview)
- Source: <https://learn.microsoft.com/en-us/windows/client-management/declared-configuration> · Microsoft · learn · ms.date 2025-08-04 · Active
- Covers: WinDC is a desired-state model over OMA-DM SyncML through a separate, linked OMA-DM enrollment that requires an existing primary MDM enrollment; two phases (discovery, enrollment); features (resource access, extensibility); supported platforms; refresh interval; troubleshooting.
- Notes: Supported platforms: Entra joined, all Windows 10/11; Entra registered, 24H2 with KB5040529 (26100.1301), 23H2 with KB5040527 (22631.3958), 22H2 with KB5040527 (22621.3958), Windows 10 22H2 with KB5040525 (19045.4717). Refresh interval: a task is created when a WinDC document exists; it runs every 4 hours by default; drift triggers reapply; if instance data is missing the document is marked drifted and a sync session is triggered. URI `./Device/Vendor/MSFT/DeclaredConfiguration/ManagementServiceConfiguration/RefreshInterval` (Get, Replace `int` such as `30`, Delete restores default). Contradiction: this node is absent from the DeclaredConfiguration CSP page and from the Feb 2026 DDF, which only has `ManagementServiceConfiguration/ConflictResolution`. SyncML namespace in every WinDC sample is `SYNCML:SYNCML1.1`. Enrollment name: event log samples show `Enrollment Name: (MicrosoftManagementPlatformCloud)` and `Enroll Type: (0x1A)`. Event log channels: `Application and Service Logs\Microsoft\Windows\DeviceManagement-Enterprise-Diagnostics-Provider\Admin` and `\Operational`. Sample errors: scope/context mismatch gives "The system cannot find the file specified"; DocID mismatch gives `0x8000FFFF`; a URI typo gives `0x86000002` plus operational `0x82d00007` (ErrorAtDocLevel).

#### Windows declared configuration discovery
- Source: <https://learn.microsoft.com/en-us/windows/client-management/declared-configuration-discovery> · Microsoft · learn · ms.date 2025-08-04 · Active
- Covers: JSON discovery against the discovery service (DS) endpoint.
- Notes: Request headers: `Content-Type: application/json` (required); `MS-CV` and `client-request-id` optional. Request body: `userDomain`, `upn`, `tenantId`, `emmDeviceId` (all optional), `enrollmentType` (`"Device"` = Entra joined, server must answer `AuthPolicy: Federated`; `"User"` = Entra registered, `AuthPolicy: Certificate`; absent = legacy joined), `osVersion` (required, such as `"10.0.00000.0"`; the server may gate on it). Response body: `EnrollmentServiceUrl`, `EnrollmentPolicyServiceUrl`, `AuthenticationServiceUrl`, `AuthPolicy` (all required; AuthPolicy is `Federated` or `Certificate`), plus optional `EnrollmentVersion`, `ManagementResource`, `TouUrl`, `errorCode`, `message`. Auth: joined uses the Entra device token (Federated); registered uses the MDM certificate of the parent enrollment (Certificate). Errors: `errorCode: "UPNRequired"` makes the client retry with a UPN; `WINHTTP_QUERY_RETRY_AFTER` is honoured for throttling. Doc defect: the "Entra registered" request example says `"enrollmentType" : "Device"` although the table says registered must send `"User"`. The endpoint URL is set by the primary MDM via `LinkedEnrollment/DiscoveryEndpoint`; Microsoft's own value is `https://discovery.dm.microsoft.com/EnrollmentConfiguration?api-version=1.0`.

#### Windows declared configuration enrollment
- Source: <https://learn.microsoft.com/en-us/windows/client-management/declared-configuration-enrollment> · Microsoft · learn · ms.date 2025-08-04 · Active
- Covers: Two SyncML messages: `Replace` on `./Device/Vendor/MSFT/DMClient/Provider/MS%20DM%20SERVER/LinkedEnrollment/DiscoveryEndpoint` with the discovery URL, then `Exec` on `.../LinkedEnrollment/Enroll`. Namespace `SYNCML:SYNCML1.1`. Enrollment itself then runs MS-MDE2.
- Notes: Only 88 words; the node semantics are on the DMClient page.

#### DMClient CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/dmclient-csp> · Microsoft · learn · ms.date 2025-07-22, updated 2025-07-23 · Active
- Covers: `Provider/{ProviderID}/LinkedEnrollment/` nodes: `DiscoveryEndpoint` (chr, Add/Delete/Get/Replace; Get returns empty string with S_OK when unset; DDF applicability `99.9.99999`); `Enroll` (null, Exec; "silent Declared Configuration enrollment, using the Microsoft Entra device token"; fails `ERROR_FILE_NOT_FOUND (0x80070002)` if DiscoveryEndpoint unset); `Unenroll` (null, Exec; rolls back all WinDC-set settings); `EnrollStatus` (int, Get: 0 Undefined, 1 Enrollment Not started, 2 In Progress, 3 Failed, 4 Succeeded, 5 Unenrollment Not started, 6 In Progress, 7 Failed, 8 Succeeded); `LastError` (int, Get; HRESULT). Applicability for Enroll/Unenroll/EnrollStatus/LastError: 10.0.19042/19043/19044.2193 (KB5018482), 22000.918 (KB5016691), 22621+ (CspVersion 1.6). Also `ConfigRefresh/{Enabled,Cadence(30 to 1440, default 90),PausePeriod}` (22000.2836 / 22621.3235), `ConfigLock`, `Recovery`, `MultipleSession`.
- Notes: The Feb 2026 DDF agrees exactly with these applicability values.

#### DeclaredConfiguration CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/declaredconfiguration-csp> · Microsoft · learn · ms.date 2025-03-12 · Active (flagged "Windows Insider Preview" throughout)
- Covers: Tree (all Device scope; editions Pro, Enterprise, Education, IoT Enterprise): `./Device/Vendor/MSFT/DeclaredConfiguration` with `Host/Complete/Documents/{DocID}/Document` (chr; Add/Delete/Get/Replace; DocID is a server-generated GUID), `Host/Complete/Documents/{DocID}/Properties/Abandoned` (int, default 0), `Host/Complete/Results/{DocID}/Document` (chr; Get), `Host/Inventory/Documents/{DocID}/Document`, `Host/Inventory/Results/{DocID}/Document`, and `ManagementServiceConfiguration/ConflictResolution` (int; 0 off, 1 on). Document XML: `<DeclaredConfiguration schema="1.0" context="Device|User" id="{GUID}" checksum="..." osdefinedscenario="...">`; `checksum` is the server-supplied document version (any string; samples use SHA-256 hex or "A0"/"A3"); syntax is validated synchronously, applied asynchronously by the orchestrator. Generic alert on every client message: `<Alert><Data>1224</Data><Item><Meta><Type xmlns="syncml:metinf">com.microsoft.mdm.declaredconfigurationdocuments</Type></Meta><Data><DeclaredConfigurations schema="1.0"><DeclaredConfiguration context= id= checksum= result_checksum= state=/></DeclaredConfigurations></Data></Item></Alert>` (the sample uses 1224, device alert, not 1226). States (`DCCSPURIState`): transient 0 NotDefined, 1 ConfigRequest, 2 ConfigInprogress, 3 ConfigInProgressAsyncPending, 10 DeleteRequest, 11 DeleteInprogress, 20 GetRequest, 21 GetInprogress, 40 ConstructURIStorageSuccess; permanent 60 ConfigCompletedSuccess, 61 ConfigCompletedError, 62 ConfigInfraError, 63 ConfigCompletedSuccessNoRefresh, 70/71/72 Delete Success/Error/InfraError, 80/81/82 Get Success/Error/InfraError. Abandon (`Properties/Abandoned`=1) stops drift refresh and hands the resource back to legacy MDM; unabandon (=0) reapplies immediately; `Delete` of `Document` removes the document but settings persist.
- Notes: Gaps vs the other WinDC pages: no `RefreshInterval`, no `Host/BulkTemplate` subtree, no `BulkVariables/Value`, and the page says User scope is not supported while the resource-access page uses `./User/Vendor/MSFT/DeclaredConfiguration/...` with `context="user"`. The `MSFTPolicies` scenario shown in the extensibility result sample is in no scenario table. Treat the CSP page plus DDF as lagging the protocol pages.

#### DeclaredConfiguration DDF file
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm/declaredconfiguration-ddf-file> · Microsoft · learn · ms.date 2025-02-13, updated 2025-02-14 · Active
- Covers: Identical to `DeclaredConfiguration.xml` in the Feb 2026 bundle: root applicability `OsBuildVersion 99.9.99999`, `CspVersion 9.9`, a long `EditionAllowList`; DOCTYPE `-//OMA//DTD-DM-DDF 1.2//EN`, `<MgmtTree xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM">`, `VerDTD 1.2`.
- Notes: `99.9.99999` is the sentinel Microsoft uses for "Insider / not yet in a released build" even though WinDC ships in production; a generator must not treat it as a real build.

#### Windows declared configuration resource access
- Source: <https://learn.microsoft.com/en-us/windows/client-management/declared-configuration-resource-access> · Microsoft · learn · ms.date 2025-08-04 · Active
- Covers: `<CSP name="./Vendor/MSFT/VPNv2"><URI path="..." type="int|chr|bool">value</URI></CSP>` document body; scenario table: `MSFTWiredNetwork` (WiredNetwork), `MSFTResource` (ActiveSync), `MSFTVPN` (VPNv2), `MSFTWifi` (Wifi), `MSFTInventory` (certificate inventory), `MSFTClientCertificateInstall` (SCEP, PFX, bulk template data); context must match LocURI scope (`./User/...` with `context="user"`); result document `<DeclaredConfigurationResult context schema id osdefinedscenario checksum result_checksum result_timestamp operation="Set" state="60"><CSP name state><URI path status="200" state="60" type/>`; resource ownership transfers to WinDC (legacy MDM writes to the same resource fail with `0x86000031`) until the document is deleted or abandoned; bulk template at `./Device/Vendor/MSFT/DeclaredConfiguration/Host/BulkTemplate/Documents/{DocID}/Document` with `@#var#` placeholders and `<ReflectedProperties><Property name type>`, instance data via `.../BulkTemplate/Documents/{DocID}/BulkVariables/Value` containing `<InstanceBlob schema="1.0"><Instance><InstanceData variable="...">`, results at `.../BulkTemplate/Results/{DocID}/Document`.
- Notes: The only source for the BulkTemplate subtree and for User-scope WinDC; neither is in the CSP page or DDF. `result_timestamp` samples are from 2024-08-06.

#### Windows declared configuration extensibility
- Source: <https://learn.microsoft.com/en-us/windows/client-management/declared-configuration-extensibility> · Microsoft · learn · ms.date 2025-08-04 · Active
- Covers: Extensibility is via native WMI/MI providers, not PowerShell scripts: the class must implement `GetTargetResource`, `TestTargetResource`, `SetTargetResource`; only string properties; document body `<DSC namespace="root/Microsoft/Windows/DesiredStateConfiguration" className="MSFT_FileDirectoryConfiguration"><Key name="...">...</Key><Value name="...">...</Value></DSC>`; scenarios `MSFTExtensibilityMIProviderConfig` (configure) and `MSFTExtensibilityMIProviderInventory` (inventory, via `Host/Inventory/Documents/{DocID}/Document`); Device scope only; results at `Host/Complete/Results/{DocID}/Document`.
- Notes: There is no PowerShell-script scenario documented; scripts would have to be wrapped in an MI provider. The result sample's `osdefinedscenario="MSFTPolicies"` is an undocumented scenario name (it is what Intune Endpoint Privilege Management uses; see below).

#### Mobile device enrollment (linked-enrollment note)
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mobile-device-enrollment> · Microsoft · learn · ms.date 2025-08-04 · Active
- Covers: MS-MDE2 phases (discovery, MS-XCEP policy, MS-WSTEP enrollment, provisioning XML); SOAP fault codes 80180001 to 80180019.
- Notes: No mention of linked or dual enrollment; WinDC's linked enrollment reuses this exact MS-MDE2 flow against the URLs returned by the JSON discovery.

#### Plan and prepare for Endpoint Privilege Management deployment (Intune)
- Source: <https://learn.microsoft.com/en-us/intune/epm/deployment-planning> · Microsoft · learn · ms.date 2026-01-26, updated 2026-07-01 · Active · Third-party MDM: Intune-gated
- Covers: EPM prerequisites (24H2; 23H2 22631.2506+; 22H2 22621.2215+; 21H2 22000.2713+; Entra joined or hybrid joined; Intune or co-managed; line of sight without SSL inspection to `*.dm.microsoft.com`). The EpmTools module includes `Get-DeclaredConfiguration` ("Retrieves a list of WinDC documents that identify the policies targeted to the device") and `Get-DeclaredConfigurationAnalysis` ("WinDC documents of type MSFTPolicies").
- Notes: Microsoft's only explicit on-Learn statement that EPM policy is delivered as WinDC documents, and it names the `MSFTPolicies` scenario. Entra registered devices are not supported for EPM despite WinDC registered support.

#### Network endpoints for Microsoft Intune
- Source: <https://learn.microsoft.com/en-us/intune/fundamentals/endpoints> · Microsoft · learn · ms.date 2026-08-21, updated 2026-08-24 · Active · Third-party MDM: Intune-gated
- Covers: `*.dm.microsoft.com` is in the core "Intune client and host service" set alongside `*.manage.microsoft.com`; the EPM section says `*.dm.microsoft.com` "supports the cloud-service endpoints that are used for enrollment, check-in, and reporting"; SSL inspection unsupported for `*.dm.microsoft.com`.
- Notes: Confirms the WinDC (MMP-C) service domain is `dm.microsoft.com` (discovery host `discovery.dm.microsoft.com`).

#### What's new in Microsoft Intune
- Source: <https://learn.microsoft.com/en-us/intune/whats-new/> · Microsoft · learn · fetched 2026-09-06 · Active · Third-party MDM: Intune-gated
- Covers: 2607 (week of 2026-07-27) registry inventory; 2605 local AI agent detection; 2604 EPM support-approved requests from all users.
- Notes: Zero occurrences of "declared configuration", "linked enrollment", "MMP-C" or "resource access"; Microsoft does not publicly label Intune features as WinDC-delivered.

#### MMP-C: onboarding your Microsoft tenant, LinkedEnrollment (call4cloud)
- Source: <https://call4cloud.nl/mmp-c-onboarding-linked-enrollment-dual-enrollment/> · Rudy Ooms · blog · 2025-01-20 · Active · Third-party MDM: unclear
- Covers: Linked enrollment shows on the device as a second enrollment named MicrosoftManagementPlatformCloud under `HKLM\SOFTWARE\Microsoft\Enrollments\`; service `dcsvc`; tenant onboarding via Graph beta `POST /deviceManagement/enableEndpointPrivilegeManagement`; the Device Inventory Agent now triggers MMP-C enrollment for all devices; error `0x8018000b` seen in DeviceManagement event logs.
- Notes: Community source; consistent with Learn's event-log name.

#### In the name of MMP-C (call4cloud)
- Source: <https://call4cloud.nl/in-the-name-of-mmp-c/> · Rudy Ooms · blog · 2023-08-02 (updated 2025-02-21) · Active · Third-party MDM: unclear
- Covers: The expansion is "Microsoft Management Platform Cloud" (from binaries and logs: `MicrosoftManagementPlatformCloudMdm`), not "Managed"; Microsoft's own term is "Declared Configuration Enrollment".
- Notes: Corrects the "Microsoft Managed Platform Cloud" wording that circulates.

#### MMP-C: the new era of Windows client management with Microsoft Intune (MSEndpointMgr)
- Source: <https://msendpointmgr.com/2025/11/11/mmp-c-the-new-era-of-windows-client-management-with-microsoft-intune/> · Anders Ahl · blog · 2025-11-11 · Active · Third-party MDM: Intune-gated
- Covers: Workloads on MMP-C today: EPM, Windows advanced device inventory (properties catalog), resource access (VPN, Wi-Fi, certificates); expectation that more policy areas move to WinDC.
- Notes: Community synthesis; no Microsoft citation for the roadmap.

#### MMP-C: the future of Windows device management with Intune (Patch My PC)
- Source: <https://patchmypc.com/blog/mmp-c-the-future-of-windows-device-management-with-intune/> · Rudy Ooms · blog · 2025-10-22 · Active · Third-party MDM: Intune-gated
- Covers: Dual services `OMADMClient` (legacy) and `dcsvc` (WinDC); "get once, set, self-heal"; discovery against `dm.microsoft.com`; planned: Defender configs, security baselines.
- Notes: Community.

#### Be prepared for Windows declared configuration in Intune (tbone.se)
- Source: <https://www.tbone.se/2025/03/17/be-prepared-for-windows-declared-configuration-in-intune/> · Torbjörn Granheden · blog · 2025-03-17 (updated 2026-04-01) · Active · Third-party MDM: Intune-gated
- Covers: 4-hour refresh, linked enrollment auto-enabled when the device is updated; DSC analogy.
- Notes: No endpoint or Microsoft-reference detail.

---

### 1.7 WNS push for MDM

#### Push notification support for device management
- Source: <https://learn.microsoft.com/en-us/windows/client-management/push-notification-windows-mdm> · Microsoft · learn · Date 2025-08-04 · Active · Third-party MDM: yes
- Covers: Flow: the server provisions `DMClient/Provider/{ID}/Push/PFN`; the device registers a WNS channel and exposes `Push/ChannelURI` and `Push/Status`; the server authenticates to WNS with its SID and client secret, gets a token, sends a raw push to the ChannelURI; the device opens a DM session. Use `X-WNS-Cache-Policy: cache` and optionally `X-WNS-TTL`. Restrictions: raw notifications only, no payload used; Battery Saver and Data Sense can drop the WNS connection; "A ChannelURI ... is only valid for 30 days. The device automatically renews the ChannelURI after 15 days and triggers a management session on successful renewal"; the server should re-query ChannelURI every session; push is not a replacement for polling; WNS may block an abusive PFN; retry logic. Credentials: create a Microsoft Store app in Partner Center (reserve a name), Product Identity shows the PFN, WNS/MPNS App Registration portal gives the Application ID and secrets.
- Notes: The only MDM-specific WNS page; it does not spell out the token endpoint or scope (see the two WNS pages below). MDM push uses the legacy Partner Center and login.live.com credential model (Package SID plus secret), not the Windows App SDK Entra model; the WNS overview marks that legacy flow as "not compatible with Windows App SDK push notifications" but it remains the documented path for DMClient push.

#### Windows Push Notification Services (WNS) overview
- Source: <https://learn.microsoft.com/en-us/windows/apps/develop/notifications/push-notifications/wns-overview> · Microsoft · learn · Date 2026-08-30 · Active
- Covers: Channel request, URI and cloud-service flow; "channel URIs expire after 30 days"; the service must verify the URI host is under `notify.windows.com`; 410 on an expired channel; legacy auth (warning box): OAuth 2.0 client_credentials POST to `https://login.live.com/accesstoken.srf` with `client_id=<Package SID ms-app://S-1-15-2-...>&client_secret=...&scope=notify.windows.com`, response `{"access_token":..., "token_type":"bearer"}`; send: `POST <channel URI>` with `Authorization: Bearer`, `X-WNS-Type`, Content-Type; WNS does not guarantee delivery; offline caching; battery saver behaviour.
- Notes: Newest WNS page; explicitly labels the Package SID and login.live.com flow "legacy UWP" and points Windows App SDK apps at Entra. For an MDM server the legacy flow is still the applicable one (the DMClient CSP registers by PFN, a Store identity).

#### Push notification service request and response headers
- Source: <https://learn.microsoft.com/en-us/windows/apps/develop/notifications/push-notifications/push-request-response-headers> · Microsoft · learn · Date 2024-05-03 (updated 2025-07-29) · Active
- Covers: Token request parameters (grant_type client_credentials, client_id = Package SID, client_secret, scope `notify.windows.com`), response (`access_token`, `token_type bearer`, `expires_in 86400`), 400 on auth failure; notification request headers: Authorization (required), Content-Type (`application/octet-stream` for raw), Content-Length, X-WNS-Type (`wns/raw`), X-WNS-Cache-Policy (`cache|no-cache`; raw defaults to no-cache), X-WNS-RequestForStatus, X-WNS-Tag, X-WNS-TTL (seconds), MS-CV; response headers X-WNS-Debug-Trace, X-WNS-DeviceConnectionStatus (connected|disconnected|tempconnected), X-WNS-Error-Description, X-WNS-Msg-ID, X-WNS-Status (received|dropped|channelthrottled), MS-CV; response codes 200, 400, 401 (refresh token), 403 (token and app mismatch), 404 (bad channel, stop sending), 405, 406 (throttled, honour Retry-After), 410 Gone (channel expired, request new) or Domain Blocked, 413 (payload over 5000 bytes), 500, 503; WNS does not support chunked transfer or pipelining.
- Notes: The wire spec for the go-oma-dm push sender. Combine with the MDM page's "set X-WNS-Cache-Policy: cache" advice, which overrides the raw default of no-cache.

#### Raw notification overview
- Source: <https://learn.microsoft.com/en-us/windows/apps/develop/notifications/push-notifications/raw-notification-overview> · Microsoft · learn · Date 2026-08-24 · Active
- Covers: Raw notifications have no UI and an app-defined payload; `Content-Type: application/octet-stream`, `X-WNS-Type: wns/raw`, payload "smaller than 5 KB ... must not be an empty string"; cached only if X-WNS-Cache-Policy is set, and only one; delivered to a running app, to a background task, or dropped.
- Notes: The "must not be an empty string" rule matters: an MDM push needs a non-empty body even though the MDM page says payloads are unused.

#### Quickstart: push notifications in the Windows App SDK
- Source: <https://learn.microsoft.com/en-us/windows/apps/develop/notifications/push-notifications/push-quickstart> · Microsoft · learn · Date 2026-07-15 · Active (app push only)
- Covers: Entra-based model: multi-tenant app registration, PFN-to-AppId mapping by e-mail, token from `https://login.microsoftonline.com/{tenantID}/oauth2/v2.0/token` with `scope=https://wns.windows.com/.default`, raw send with the same headers; "We do NOT support using Windows App SDK push notifications with Microsoft Partner Center".
- Notes: Included for contrast only. Nothing in the MDM docs says DMClient push has moved to this model; the MDM page still instructs Partner Center registration. Open question in section 10.

#### Send notifications to UWP apps using Azure Notification Hubs (WNS credential steps)
- Source: <https://learn.microsoft.com/en-us/azure/notification-hubs/notification-hubs-windows-store-dotnet-get-started-wns-push-notification> · Microsoft · learn · Date 2025-05-01 (updated 2025-10-08) · Active
- Covers: Partner Center steps referenced from the MDM push page: create a new app and reserve a name; Product Identity shows the Package SID, Package/Identity/Name, Publisher; WNS/MPNS, App Registration portal, Certificates and secrets, new client secret; Package SID form `ms-app://<SID>`; warns the secret is shown once and that MPNS is retired.
- Notes: Confirms the Package SID plus client secret pair is what `client_id` and `client_secret` expect at login.live.com.

#### Microsoft Q&A: WNS push-initiated MDM session never starts on Windows 11 24H2/25H2
- Source: <https://learn.microsoft.com/en-my/answers/questions/5907951/wns-push-initiated-mdm-session-never-starts-on-win> · Victor Lyuboslavsky (asker, Fleet); reply by an independent advisor · qna · Date 2026-06-01 · Experimental (unresolved, no Microsoft reply) · Third-party MDM: yes
- Covers: Windows 11 24H2 (26100) and 25H2 (26200.8457), Azure Trusted Launch VMs, device-only and UPN enrollments. Push/PFN provisioned, Push/Status = 0, ChannelURI on *.notify.windows.com; the server sends `X-WNS-Type: wns/raw`, `application/octet-stream`; WNS returns 200 `X-WNS-Status: received`; the device logs Events 1010 and 2419, the PushLaunch task runs deviceenroller.exe (result 0), then Event 4603 "Getting push alert info for push initiated session failed. NotificationId: notificationIdNotRetrieved, HRESULT: Unspecified error". The advisor claims an undocumented 24H2 change requiring a JSON payload (`NotificationId`, `IsCritical`, `Type`) and mentions `Feature_Servicing_DMPushMessageSetCritical`; the asker tested that payload and reproduced the identical failure. The asker's later update states GetPushAlertInfo fails and logs 4603 but "the device degrades gracefully. It runs the OMA-DM session and delivers anyway. The error does not block the session."
- Notes: Anecdotal, with an internal contradiction (the question says no session; the final update says the session does run despite 4603). No Microsoft acknowledgement, KB, or workaround; the advisor's payload theory was falsified in-thread. Takeaway for go-oma-dm: send a non-empty raw body, keep `X-WNS-Cache-Policy: cache`, never rely on push alone, re-read ChannelURI each session, and log Event 4603 as a known ambiguous symptom on 24H2 and 25H2 until Microsoft documents otherwise.

### 1.8 Entra and identity enrollment paths for a third-party MDM

The generic enrollment flows (federated, on-premises, certificate, GPO auto-enroll, the deep link)
are catalogued in 1.4. This section holds what is specific to Entra: how a third-party MDM is
registered in a tenant, what the tokens carry, what attestation exposes on the wire, and where the
line between "any MDM" and "Intune only" actually falls.

#### Microsoft Entra integration with MDM
- Source: <https://learn.microsoft.com/en-us/windows/client-management/azure-active-directory-integration-with-mdm> · Microsoft · learn · Date 2025-08-04 (updated 2026-05-11) · Active · Third-party MDM: yes
- Covers: The contract a third-party MDM implements to take part in Entra-integrated enrollment: cloud (multitenant) vs on-premises (single-tenant) MDM app, Terms-of-Use endpoint, MDM enrollment endpoint, Entra-issued bearer tokens, compliance reporting via Graph.
- Notes: The foundational document for go-oma-dm's Entra path. An on-premises third-party MDM is registered via Mobility (MDM and MAM), Add application, Create your own application; the admin configures the enrollment and Terms-of-Use URLs; the product must expose config for the client ID and key. Two-step flow: (1) a passive full-page redirect to the Terms-of-Use URL carrying `redirect_uri`, `client-request-id`, `api-version`, `mode=azureadjoin` (only for org-owned), with an Entra bearer token whose claims are ObjectID, UPN, TID, Resource (no device ID yet); the user returns with `IsAccepted=true&OpaqueBlob=...`. (2) The active OMA-DM enrollment. Enrollment-type table: Entra join gives EnrollmentType Device, device cert in My/System, CSR subject = Device ID; add work account gives EnrollmentType Full, user cert in My/User. AuthenticationServiceURL is skipped for Entra enrollments (the discovery URL is provisioned in Entra, no discovery phase). Warning to encode: after 1 July 2026 new on-premises MDM apps get v2.0 tokens by default, so `aud` validation must accept only the appId. Compliance reporting uses Graph PATCH `.../devices/{id}` with `isManaged` and `isCompliant`, and "This API is only applicable for approved MDM apps on Windows devices." Windows 11 renders the enrollment web view in an iframe.

#### [MS-MDE2] RequestSecurityToken using federated authentication (AdditionalContext catalogue)
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/3ea67409-e871-47cc-9a06-1fcb51bffb40> · Microsoft · spec · Date 2026-08-12 (v19.0) · Active · Third-party MDM: yes
- Covers: The full `ac:ContextItem` list the client sends in the RST: `EnrollmentType` (Full or Device), `DeviceType` (`CIMClient_Windows`), OSEdition, OSVersion, DeviceName, DeviceID, EnrollmentData, MAC, IMEI, TargetedUserLoggedIn, plus optional `BulkAADJ`, `ZeroTouchProvisioning`, `OfflineAutoPilotEnrollmentCorrelator`, `UXInitiated`, `ExternalMgmtAgentHint`, `DomainName`, `BootstrapDomainJoin`, `PlugandForget`, `WhiteGlove`, `WhiteGloveHybridJoin`, `NotInOobe`; with EnrollmentVersion 5.0 or later the attestation items `AIKAttestationClaim`, `AIKPub`, `AIKCert`, `AadAIKAttestationClaim`, `AADPub`, `EmmDeviceId`, `RequestVersion`; 6.0 adds `AzureAttestationBlob`; 7.0 adds `AttestationStatus`, `AttestationStatusHResult`, `AzureAttestationCorrelationVector`; 9.0 adds `AIKAlgorithm`.
- Notes: This is where enrollment attestation surfaces on the wire to the MDM server. The AIK claim is an NCryptCreateClaim blob with the MDM private key as subject and the device AIK as authority. A third-party MDM receives this material if it advertises EnrollmentVersion 5.0 or higher; what is Intune-gated is the reporting and recovery tooling.

#### [MS-MDE2] Interaction with the STS and the renewal RST
- Source: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/27ed8c2c-0140-41ce-b2fa-c3d1a793ab4a> and <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/e96d7d12-e178-4ab2-a2f7-62b9d6375e57> · Microsoft · spec · Date 2026-08-12 · Active · Third-party MDM: yes
- Covers: The STS phase (the client is "agnostic ... all protocol flows ... are passive, browser-implemented"; the POST form `action="ms-app://windows.immersivecontrolpanel"` returns `wresult`); the Renew RST variant (RequestType `.../Renew`, PKCS7 BinarySecurityToken).
- Notes: Confirms the Entra token reaches the server opaquely inside `<wsse:BinarySecurityToken>`; the enrollment client never interprets it. For Entra join there is no discovery or AuthenticationServiceUrl step.

#### mobilityManagementPolicy resource type (Graph beta)
- Source: <https://learn.microsoft.com/en-us/graph/api/resources/mobilitymanagementpolicy?view=graph-rest-beta> · Microsoft · learn · Date 2024-07-08 (updated 2025-12-03) · Active (beta) · Third-party MDM: yes
- Covers: The Entra auto-enrollment object for an MDM or MAM app: `appliesTo` (none, all, selected), `complianceUrl`, `discoveryUrl`, `termsOfUseUrl`, `displayName`, `isValid`, `includedGroups`. "Only applicable to devices based on Windows 10 OS and its derivatives."
- Notes: These URL fields are exactly what the Entra admin pastes for a third-party MDM. Programmable via `mobileDeviceManagementPolicies` (beta), which a go-oma-dm setup command could automate.

#### Manage device identities and device identity overview (Entra)
- Source: <https://learn.microsoft.com/en-us/entra/identity/devices/manage-device-identities> (ms.date 2026-06-17, updated 2026-08-25) and <https://learn.microsoft.com/en-us/entra/identity/devices/overview> (ms.date 2025-06-27) · Microsoft · learn · Active · Third-party MDM: yes
- Covers: The three join types and their `trustType` mapping: AzureAD is Entra joined, Workplace is Entra registered, ServerAD is Entra hybrid joined; the MDM column; device settings (users may join, MFA to join, max devices).
- Notes: Informs EnrollType handling: Entra joined gives a device (multi-user) enrollment; registered or add-work-account gives a full (single-user) enrollment; hybrid uses GPO user-credential auto-enroll. Only Entra joined and registered devices honour the automatic MDM-enrollment (Mobility) policy.

#### ADFS: Living in the Legacy of DRS, and xpn/adfstoolkit
- Source: <https://specterops.io/blog/2025/01/07/adfs-living-in-the-legacy-of-drs/> (the blog.xpnsec.com URL redirects here) · Adam Chester, SpecterOps · blog · Date 2025-01-07 · Active; <https://github.com/xpn/adfstoolkit> · Python · repo · Experimental (1 commit, 39 stars) · Third-party MDM: n/a (identity layer)
- Covers: Device registration service (DRS) mechanics that precede MDM: SOAP `DeviceEnrollmentWebService.svc` and REST `/EnrollmentServer/device/` (a clone of `enterpriseregistration.windows.net/EnrollmentServer/device/`), PKCS#10 CSR to device certificate, `msDS-Device` object and `msDS-KeyCredentialLink`, transport key, PRT and `x-ms-RefreshTokenCredential`.
- Notes: A mental model of what Entra device registration issues before the MDM enrollment cert: the device identity cert and PRT the OMA-DM client later presents as the Entra token and `DeviceToken` header. Not an MDM protocol source.

#### Windows enrollment attestation (Intune), DeviceStatus CSP, and Graph key recovery
- Source: <https://learn.microsoft.com/en-us/intune/device-enrollment/windows/attestation> (ms.date 2025-02-14, updated 2026-07-01); <https://learn.microsoft.com/en-us/windows/client-management/mdm/devicestatus-csp> (2025-10-03); <https://learn.microsoft.com/en-us/graph/api/intune-devices-manageddevice-initiatemobiledevicemanagementkeyrecovery?view=graph-rest-beta> (2024-08-01) · Microsoft · learn · Active · Third-party MDM: the wire claims are exposed to any MDM; the report, filter, action and Graph call are Intune-gated
- Covers: TPM 2.0 attestation of the MDM cert private key and the Entra key at enrollment; requirements (Windows 11 22000.2713+ or 22621.2792+, TPM 2.0, physical devices only; VMs, vTPM, AVD and Windows 365 cannot attest); the attestation status report; the "Attest device" action; DMClient Recovery CSP nodes. `./Vendor/MSFT/DeviceStatus/CertAttestation/MDMClientCertAttestation` (chr, Get) returns "an XML blob containing the relevant attestation fields" (Windows 11 21H2 with KB5018483, 22H2+). The Graph call requires an active Intune licence and `DeviceManagementManagedDevices.PrivilegedOperations.All`.
- Notes: State this clearly in the build plan: the attestation material is sent to the enrollment server as MS-MDE2 AdditionalContext items and via a generic DeviceStatus Get, so go-oma-dm can collect, verify and act on attestation itself. What is Intune-internal is the packaged feature: the device attestation status report, the `IsTpmAttested` enrollment-restriction filter (error 0x80180032), the "Attest device" action, and the Graph recovery call. MDM key recovery is driven client-side via DMClient `Recovery/InitiateRecovery` plus `AllowRecovery`, which any MDM issuing that SyncML could invoke.

#### Third-party device compliance partners (Conditional Access)
- Source: <https://learn.microsoft.com/en-us/intune/device-security/compliance/third-party-partners> · Microsoft · learn · Date 2026-06-24 (updated 2026-07-01) · Active · Third-party MDM: Intune-gated, and not Windows
- Covers: How a third-party MDM feeds compliance to Entra for Conditional Access by becoming the MDM authority for assigned groups; the GA partner list (Fleet, Jamf, Ivanti, Omnissa, SOTI, 42Gears and others); onboarding form.
- Notes: A hard constraint for the Conditional Access story: licensing requires an Intune subscription with licences assigned, and platform support is Android, iOS/iPadOS and macOS only. Windows is not listed. For Windows a third-party MDM reports compliance only via the Graph `isCompliant` PATCH from the Entra-integration page, which is limited to approved MDM apps.

#### Disabling MDM enrollment when adding a work or school account
- Source: <https://petervanderwoude.nl/post/disabling-mdm-enrollment-when-adding-work-or-school-account/> · Peter van der Woude · blog · Date 2026-03-02 · Active · Third-party MDM: yes (Entra tenant setting)
- Covers: The new Entra toggle "Disable MDM enrollment when adding work or school account", which affects only the app-initiated add-account flow.
- Notes: A tenant-side switch that silently changes whether registered devices reach a third-party MDM.

### 1.9 Autopilot and provisioning ecosystem

#### Windows Autopilot overview, requirements, and registration (v1)
- Source: <https://learn.microsoft.com/en-us/autopilot/overview> (ms.date 2025-06-13, updated 2026-02-05); <https://learn.microsoft.com/en-us/autopilot/requirements> (ms.date 2025-07-08, updated 2026-05-21); <https://learn.microsoft.com/en-us/autopilot/registration-overview> (ms.date 2025-03-25, updated 2026-06-22) · Microsoft · learn · Active · Third-party MDM: the enrollment target can be a third-party MDM; registration and profiles are Intune-gated
- Covers: v1 as a cloud service on top of the OEM image; post-deployment management by "Intune ... or other similar tools from non-Microsoft parties"; auto-enroll "Requires a Microsoft Entra ID P1 or P2 subscription for configuration". Licensing lists "Microsoft Entra ID P1 or P2 and Microsoft Intune subscription or an alternative MDM service", and "If using a different MDM service, contact the vendor for the specific URLs or configuration needed for those services." Registration is the hardware hash uploaded to the Autopilot service and associated to a tenant; devices appear under Intune, Devices, Enrollment, Windows Autopilot. Self-deploying and pre-provisioning need TPM attestation endpoints (`*.microsoftaik.azure.net`, vendor EK cert endpoints); the Autopilot service is `ztd.dds.microsoft.com`.
- Notes: There is no third-party registration API. Third parties ride Autopilot v1 only by being the Entra auto-enrollment target; the hardware hash and the Autopilot profile are handled in a Microsoft portal.

#### Autopilot user-driven, self-deploying, and pre-provisioning modes
- Source: <https://learn.microsoft.com/en-us/autopilot/user-driven> (2025-06-13, updated 2026-06-22); <https://learn.microsoft.com/en-us/autopilot/self-deploying> (2024-09-13, updated 2026-06-22); <https://learn.microsoft.com/en-us/autopilot/pre-provision> (2025-04-09, updated 2026-06-22) · Microsoft · learn · Active · Third-party MDM: user-driven yes; self-deploying and pre-provisioning Intune-gated
- Covers: User-driven: "Enroll in Microsoft Intune or another mobile device management (MDM) service." Self-deploying: TPM 2.0 attestation, Entra join only, VMs fail with `0x800705B4`. Pre-provisioning: "Pre-provisioned deployments use Microsoft Intune", an Intune subscription is required, TPM 2.0 plus attestation, an ESP profile is required.
- Notes: User-driven mode is third-party-capable through standard Entra auto-enroll. Self-deploying and pre-provisioning depend on TPM device attestation into the tenant and on Intune-hosted profiles; the `WhiteGlove` and `ZeroTouchProvisioning` context items are sent to any MDM, but the orchestration is Microsoft's.

#### Windows Autopilot device preparation (v2)
- Source: <https://learn.microsoft.com/en-us/autopilot/device-preparation/overview> (ms.date 2026-08-07, updated 2026-08-27); <https://learn.microsoft.com/en-us/autopilot/device-preparation/faq> (ms.date 2025-04-04, updated 2026-05-01) · Microsoft · learn · Active · Third-party MDM: planned, Intune-only today
- Covers: The v2 re-architecture; requires Windows 11 22H2 with KB5035942, 23H2, or 24H2; Entra join only; enrollment-time grouping; device-based targeting; does not use the ESP.
- Notes: The FAQ states verbatim: "Can Windows Autopilot Device preparation be used by non-Microsoft mobile device management (MDM) providers?" "Windows Autopilot device preparation will support non-Microsoft MDMs. In this initial release, configuration is only possible via Intune." Pre-provisioning and self-deploying "will be supported in the future, but aren't part of the initial release"; hybrid join is not supported. Watch item, not a build target.

#### Windows Autopilot device association (UEFI tenant affinity)
- Source: <https://learn.microsoft.com/en-us/autopilot/device-preparation/device-association/overview> (ms.date 2026-08-25, updated 2026-08-27); <https://techcommunity.microsoft.com/blog/intunecustomersuccess/introducing-device-association-for-windows-autopilot-device-preparation/4550603> (Maggie Dakeva, 2026-08-27); <https://call4cloud.nl/windows-autopilot-device-association/> (Rudy Ooms, 2026-08-31, updated 2026-09-05) · learn and blog · Active · Third-party MDM: Intune-gated
- Covers: Binds a Windows 11 device to the tenant before enrollment by writing a tenant-affinity marker (a compressed JWT, `DeviceLinkJwtCompressed` per call4cloud) into UEFI firmware, persisting across reset and reinstall; pre-associate, associate, remove; TPM-backed plus Microsoft Azure Attestation; auto-marks corporate-owned. VMs are unsupported.
- Notes: The newest Autopilot feature (August 2026). Pre-association is done in the Intune admin center; a third-party MDM cannot write or consume the UEFI marker today.

#### Enrollment Status Page, EnrollmentStatusTracking CSP, and FirstSyncStatus
- Source: <https://learn.microsoft.com/en-us/autopilot/enrollment-status> (ms.date 2025-06-13, updated 2026-04-07); <https://learn.microsoft.com/en-us/intune/intune-service/enrollment/windows-enrollment-status> (ms.date 2026-01-20, updated 2026-07-01); the EnrollmentStatusTracking CSP page in 1.4 · Microsoft · learn · Active · Third-party MDM: the CSPs are generic; the ESP profile is Intune-gated
- Covers: The ESP tracks device-preparation, device-setup and account-setup phases; uses the EnrollmentStatusTracking CSP (Win32 apps) and the DMClient FirstSyncStatus nodes (MSI and modern apps); InstallationState enums, timeouts (default 60 minutes), "Block device use until all apps and profiles are installed". "ESP doesn't apply to a Windows device that was enrolled with Group Policy (GPO)."
- Notes: The practical path for go-oma-dm: populate `FirstSyncStatus/ExpectedPolicies`, `ExpectedMSIAppPackages`, `ExpectedModernAppPackages`, `ExpectedSCEPCerts`, and set `IsSyncDone` and `WasDeviceSuccessfullyProvisioned` to feed the built-in OOBE progress. Full ESP app-blocking with the Intune Management Extension sidecar is an Intune experience.

#### Bulk enrollment via provisioning package and the Provisioning CSP
- Source: <https://learn.microsoft.com/en-us/windows/client-management/bulk-enrollment-using-windows-provisioning-tool> (ms.date 2025-08-04); <https://learn.microsoft.com/en-us/windows/client-management/mdm/provisioning-csp> (ms.date 2017-06-26, updated 2024-01-18); <https://learn.microsoft.com/en-us/intune/device-enrollment/windows/create-bulk-package> (ms.date 2025-09-29, updated 2026-07-01) · Microsoft · learn · Active · Third-party MDM: on-premises and certificate ppkg yes; the Entra bulk token is Entra and Intune-gated
- Covers: Windows Configuration Designer builds a ppkg with `Enrollments/{UPN}` carrying AuthPolicy (OnPremise or Certificate), DiscoveryServiceFullUrl, EnrollmentServiceFullUrl, PolicyServiceFullUrl, Secret (password, cert thumbprint, or federated token). Intune bulk join uses a 180-day bulk token and the `package_{GUID}` account; MFA must be excluded; federated accounts unsupported.
- Notes: Directly usable: go-oma-dm can be a bulk-enrollment target with AuthPolicy OnPremise or Certificate in the ppkg, and Malcolm/local-mdm already generates and signs such packages (section 3). "Bulk-join isn't supported in Microsoft Entra join" for third-party targets.

#### Windows 365 and Cloud PC
- Source: <https://learn.microsoft.com/en-us/windows-365/overview> · Microsoft · learn · Date 2025-04-15 (updated 2026-05-07) · Active · Third-party MDM: Intune-gated
- Covers: Cloud PCs are Entra-joined VMs with "full integration with Microsoft Intune"; auto-provisioned by licence; excluded from enrollment attestation.
- Notes: Out of scope for go-oma-dm. Recorded only to mark that Cloud PC management is Intune-bound.

---

## 2. Protocol surface checklist

Every capability the server must implement, with the source that defines it. The build plan's
phases and the decision records cite these rows.

| Capability | Defining source | Section or anchor | Note |
|---|---|---|---|
| Discovery GET (EnrollmentServer/Discovery.svc) | MS-MDE2 v19.0 (2026-08-11); Learn federated and on-prem pages (2025-08-04) | MDE2 1.3 overview step 3; 3.1.4.1 Discover | The client first sends a bare HTTP GET to `http(s)://enterpriseenrollment.<domain>/EnrollmentServer/Discovery.svc` expecting 200 with an empty body; the path is "always constant". |
| Discovery POST | MS-MDE2 | 3.1.4.1.2.1 Discover, 3.1.4.1.3.1 DiscoveryRequest, 3.1.4.1.3.2 DiscoveryResponse | SOAP action `.../2012/01/enrollment/IDiscoveryService/Discover`; RequestVersion 1.0 to 9.0; the response must not be chunked. |
| AuthPolicy values | MS-MDE2 | 3.1.4.1.3.2 ("MUST be set to Federated, Certificate, or OnPremise") | Federated requires AuthenticationServiceUrl and the `appru` and `wresult` WAB handshake (MDE2 3.2). |
| EnrollmentPolicy (XCEP GetPolicies) | MS-XCEP v18.0 (2026-03-09) profiled by MS-MDE2 | MDE2 3.3.4.1 GetPolicies and GetPoliciesResponse; XCEP 3.1 | Optional for Windows; action `.../enrollmentpolicy/IPolicy/GetPolicies`; only minimalKeyLength, hashAlgorithmOIDReference, cryptoProviders consumed. |
| EnrollmentService (WSTEP RequestSecurityToken, BinarySecurityToken, wap-provisioningdoc) | MS-WSTEP v15.0 profiled by MS-MDE2 | MDE2 3.4.4.1, 3.4.4.1.1.2 RequestSecurityTokenResponseCollection, 2.2.9.1 XML Provisioning Schema | TokenType DeviceEnrollmentToken; PKCS10 BST in; RSTRC BST ValueType DeviceEnrollmentProvisionDoc = base64 `wap-provisioningdoc`. |
| DMClient bootstrap | MS-MDE2; Learn DMClient CSP (2025-07-22); Learn w7 APPLICATION CSP | MDE2 2.2.9.3, 2.2.9.5, 2.2.9.2; Learn federated page provisioning sample | `APPLICATION` (APPID w7, PROVIDER-ID, ADDR, DEFAULTENCODING, APPAUTH x2, SSLCLIENTCERTSEARCHCRITERIA) plus `DMClient/Provider/<PROVIDER-ID>/Poll/*`; ProviderID must equal PROVIDER-ID. |
| SyncML session | OMA DM Protocol 1.2.1; MS-MDM v15.0; Learn OMA-DM page | OMA 8.3 to 8.6 (packages 1 to 4), 6.2 Final and 1222; MS-MDM 2.2.2, 3.1.5.1.2 | Package 1: Alert 1200/1201 plus Replace DevInfo plus Final; server MsgID starts at 1 and increments; SessionID constant, max 4 bytes. |
| Alert codes | OMA DM RepPro 1.2.1 alert table; MS-MDM 2.2.7.2; Learn OMA-DM page | MS-MDM 2.2.7.2 (1200, 1201, 1222, 1223, 1224, 1225, 1226); MS-MDM 3.2.5.1.5 and 3.2.5.1.6 | Windows-specific 1224 Types: `com.microsoft/MDM/LoginStatus` (user/others/none), `com.microsoft.mdm.synctype` (user/device/mixed), `com.microsoft/MDM/DevicePrepSync`, `Reversed-Domain-Name:com.microsoft.mdm.win32csp_install`, `com.microsoft.mdm.declaredconfigurationdocuments`; 1226 for unenroll and async CSP completion. |
| Status codes | SyncML RepPro 1.2.2 section 10; Learn OMA-DM page table | RepPro 10 (200 to 516); MS-MDM 3.1.5.2.1 | The client MUST Status every server command; 405 also used for wrong-scope settings in AVD (MS-MDM 3.2.5.1.5); 213 for chunk accepted (OMA 7). |
| Add, Replace, Get, Delete, Exec, Atomic | MS-MDM; Learn OMA-DM page | MS-MDM 2.2.7.1 to 2.2.7.7, 3.1.5.1.1 to 3.1.5.1.7 | Implicit Add; Get on an interior node returns the URI-encoded child list; nested Atomic gives 500/507; Get inside Atomic unsupported; Sequence supported per Learn (not in the MS-MDM list). |
| Results | MS-MDM | 2.2.7.8, 3.1.5.2.2 | `(CmdID, MsgRef?, CmdRef, Cmd, Meta?, Item+)`; absent MsgRef means 1; one Results per successful Get. |
| Final | OMA DM Protocol 1.2.1; MS-MDM | OMA 6.2; MS-MDM 2.2.3.4 | Only in the last message of a package; the server sends Final on every message when possible; the client withholds Final until the server's Final. |
| Chunking, MaxMsgSize, Large Object | OMA DM Protocol 1.2.1; SyncML MetaInfo 1.2.2; MS-MDM; Learn OMA-DM page | OMA 6 (1222 Next Message) and 7 (MoreData, 213, MaxObjSize, 1225); MS-MDM 2.2.5.3 note 13, 2.2.7.2 note 14; DevDetail `LrgObj` | The client may send Meta MaxMsgSize; large-object upload to the server was added in Windows 10; no numeric limit published. |
| User vs device scope | Learn OMA-DM page; MS-MDM; Learn known issues | Learn "User targeted vs. Device targeted configuration"; MS-MDM 3.2.5.1.1, 3.2.5.1.5; MDE2 3.4 `TargetedUserLoggedIn`; DMClient `EnrollmentType` | LocURI prefix `./user` vs `./device` (default device); Alert 1224 LoginStatus tells the server whether user settings are acceptable; `./User` fails on Entra-joined devices without an Entra sign-in. |
| WBXML | SyncML RepPro 1.2.2 section 8; OMA DM RepPro 1.2.1 5.1 and 5.2; MS-MDM 2.1; DMAcc `DefaultEncoding`; w7 `DEFAULTENCODING` | RepPro 8.1 publicid 0x1201, 8.2 code pages 00 and 01, 8.3 tokens; MetaInfo 1.2.2 section 7 | MIME `application/vnd.syncml.dm+wbxml`; XML is the default; W3C WBXML NOTE (1999) for the container format. |
| Push | Learn push-notification-windows-mdm; DMClient CSP Push/*; WNS headers page | DMClient `Push/PFN`, `Push/ChannelURI`, `Push/Status`; push-request-response-headers; raw-notification-overview | Raw push, non-empty body, Package SID and secret at login.live.com, 30-day channel, re-read ChannelURI each session, 24H2/25H2 caveat. |
| Cert renewal (ROBO) | Learn certificate-renewal-windows-mdm; MS-MDE2 | MDE2 3.5 Certificate Renewal, 2.2.9.2 `My/WSTEP/Renew/*` | RequestType `.../200512/Renew`, BST `#PKCS7` signed by the old cert, client TLS auth, response wap-provisioningdoc with the new cert; EntDMID required first. |
| Unenroll | Learn DMClient CSP; Learn OMA-DM page; Learn known issues | DMClient `Provider/{ID}/Unenroll` and `./Device/Vendor/MSFT/DMClient/Unenroll` (Exec); Alert 1226 on user-initiated unenroll | Server Exec on Unenroll; fails silently for add-work-account enrollments and is disabled for Entra-joined devices (wipe instead). |
| WinDC dual enrollment | Learn declared-configuration-discovery, -enrollment; DMClient CSP LinkedEnrollment | 1.6 | JSON discovery, then MS-MDE2 against the returned URLs; Federated for Entra joined, Certificate for Entra registered. |
| WinDC documents | Learn declaredconfiguration-csp, resource-access, extensibility | 1.6 | `Host/Complete/Documents/{DocID}/Document` Replace with the XML document; results via `Host/Complete/Results`; state via the 1224 declaredconfigurationdocuments alert. |
| SyncML namespace `SYNCML:SYNCML1.2` vs `SYNCML:SYNCML1.1` | SyncML RepPro 1.2.2; MS-MDM | RepPro 7 DTD ("Value MUST be the text: 'SYNCML:SYNCML1.2'"); MS-MDM 2.2.1, 2.2.2; OMA Protocol 8.3 VerDTD '1.2' and VerProto 'DM/1.2' | The normative value is `SYNCML:SYNCML1.2`; `SYNCML:SYNCML1.1` appears in older Learn CSP samples and every WinDC sample and corresponds to w7 `PROTOVER 1.1`; emit 1.2, parse both. |

---

## 3. Open source servers and PoCs

Every repository fact (language, license, last push, stars) was read from the GitHub API on
2026-09-06. Go projects first. A consistent finding across all of them: nobody uses a SOAP,
WS-Trust, or SyncML library; every project hand-rolls the XML with the language's XML encoder.

#### fleetdm/fleet
- Repo: <https://github.com/fleetdm/fleet> · Go · MIT core with proprietary `ee/` (API reports NOASSERTION) · Active (last push 2026-09) · 6,813 stars
- Implements: discovery, XCEP GetPolicies, WSTEP RequestSecurityToken, wap-provisioningdoc, SyncML session, command batching, Add/Replace, Exec (SCEP `Enroll` only), Get (internal only), Delete/Atomic parse-only, user scope (hold until login), WNS (built, disabled), cert renewal (partial), unenroll, Windows test client. No WinDC, no DDF.
- Notes: The most complete open Windows MDM server, and the best-documented. Key paths: protocol constants and endpoints in `server/mdm/microsoft/microsoft_mdm.go` (`/EnrollmentServer/Discovery.svc`, `/ManagementServer/MDM.svc`); SyncML constants and status codes in `server/mdm/microsoft/syncml/syncml.go`; enrollment SOAP handlers in `server/service/microsoft_mdm.go` (`ProcessMDMMicrosoftDiscovery`, `GetMDMWindowsPolicyResponse`, `GetMDMWindowsEnrollResponse`, `mdmMicrosoftManagementEndpoint`, and the provisioning-doc builders `NewCertStoreProvisioningData`, `NewApplicationProvisioningData`, `NewDMClientProvisioningData`, `NewProvisioningDoc`); WSTEP CA in `server/mdm/microsoft/wstep.go` (`SignClientCSR`, `GetClientCSR` accepts both PKCS#10 and PKCS#7 and verifies signature and signing time) plus a vendored copy of Go's x509 CSR parser in `wstep_csr.go` (1,585 lines) to tolerate the non-printable characters Windows puts in the CSR subject; cert thumbprint is uppercased SHA-1 hex. Session auth in `isTrustedRequest`: mTLS cert CN match, else an MD5 digest challenge via `Chal` and `NextNonce` (`syncml:auth-md5`, OMA DM Security 5.3). All pending commands for an enrollment go in one server-to-device SyncML (`createResponseSyncML`). Command queue in `server/datastore/mysql/microsoft_mdm.go`. Profile validation in `server/fleet/windows_mdm.go` (`validTopLevelElements = {Replace, Add, Exec, Atomic}`). Poll schedule: `NewDMClientProvisioningData` sets DMClient `Poll` `NumberOfFirstRetries=0, IntervalForFirstSetOfRetries=1` (a 1-minute infinite poll), relaxed to 480 minutes for fleetd-wakeable hosts; on-demand wake is fleetd calling `deviceenroller.exe /o <GUID> /c`, not WNS. Docs: `docs/Contributing/architecture/mdm/windows-mdm-architecture.md` and `docs/Contributing/product-groups/mdm/windows-mdm-glossary-and-protocol.md` (enrollment sequence diagram plus a full `HKLM\...\Enrollments` registry key reference). Dependencies: `smallstep/scep`, `smallstep/pkcs7`, `google/go-tpm`, `google/go-tpm-tools`, `foxboron/go-tpm-keyfiles`, `golang-jwt/jwt/v4`, `russellhaering/goxmldsig`, `antchfx/xmlquery`, `mattermost/xml-roundtrip-validator`. Pitfalls from its issues are in section 7.

#### Malcolm/local-mdm
- Repo: <https://github.com/Malcolm/local-mdm> · Go · MIT · Active (created 2026-02, last push 2026-05) · 0 stars
- Implements: discovery, XCEP, WSTEP, wap-provisioningdoc, SyncML session, Add/Replace/Get/Exec, WNS (real client), cert renewal (ROBO in the provisioning doc), unenroll, test client (`tools/windows-enrollment-agent`, e2e `mdmb_enrollment_test`). No Atomic, user scope, WinDC, or DDF.
- Notes: A fresh, well-structured Go server (Windows agentless plus macOS via a NanoMDM wrapper). Windows code in `internal/platform/windows/`: `discovery.go`, `enrollment.go` (`ExtractCSR`, `GenerateProvisioningXML`), `management.go` (`HandleSyncML`, `deliverPendingCommands`), `syncml.go` (typed builder: `AddStatus`, `AddGet`, `AddExec`, `AddReplace`), `csp.go` (Policy, WiFi, VPN, DeviceLock builders), `wns.go` (a real `WNSClient`: OAuth token then raw push), `ppkg.go` and `ppkg_signing.go` (provisioning-package generation). Uses `micromdm/scep`, `smallstep/pkcs7`, `micromdm/nanodep`. PostgreSQL, HTMX UI. Unproven, but the closest architectural sibling to what go-oma-dm will be.

#### mjoliver/Windows-MDM (Latchz)
- Repo: <https://github.com/mjoliver/Windows-MDM> · Go · no license file · Experimental (last push 2026-07) · 1 star
- Implements: discovery, XCEP, WSTEP (zero-touch), SyncML with Add/Replace/Get/Exec/Atomic (`internal/mdm/syncml.go`), DDF parse (`cmd/ddf-compiler` compiles Microsoft DDF XML into a policy catalog), WNS references in `internal/mdm/session.go`, renewal via XCEP. No user scope, no WinDC.
- Notes: Single-binary Windows MDM with a built-in RSA-4096 CA, OIDC (Google, Entra, Okta), React dashboard, Let's Encrypt auto-TLS. Author-declared PoC. `internal/enrollment/{discovery,wstep,xcep}.go`, `internal/mdm/{handler,syncml,session,commands}.go`, `internal/pki/ca.go`. Notable for the DDF-compiler approach to a policy catalog. No license means all rights reserved; read only.

#### marcosd4h/mdm and marcosd4h/MDMatador
- Repo: <https://github.com/marcosd4h/mdm> · Go and C++ · MIT · Maintenance (last push 2024-08) · 8 stars; <https://github.com/marcosd4h/MDMatador> · Go · MIT · Maintenance (last push 2023-08) · 15 stars
- Implements: discovery, XCEP, WSTEP, wap-provisioningdoc, SyncML session, Add/Replace/Get, unenroll detection. No Exec or Atomic.
- Notes: Black Hat USA 2023 "Windows Agentless C2: (Ab)using the MDM Client Stack" research by Marcos Oviedo, who later wrote Fleet's Windows MDM. `mdm/mdm_server_poc/` is a forked and expanded oscartbeaumont server with `mde_auth.go` (STS), `x509_wrapper.go` (the vendored x509 CSR parser Fleet later adopted), `mdm_manage.go` (`isDeviceUnenrollmentMessage`, `isSessionInitializationMessage`), and a large `sample_syncml_commands/` library (DiagnosticLog ETW traces, EnterpriseDesktopAppManagement MSI install, WMI provider, accounts, Defender and firewall). Includes a custom CSP DLL (`c2runch_csp/`) and enrollment-exploit client PoCs (C++). MDMatador is the productised "MDM-based agentless C2" server: `internal/mdm/{command_manager,command_handlers,protocol_helpers,wstep_manager,wstep_csr}.go`, SQLite migrations (`mdm_enrollments`, `mdm_certificates`, `mdm_device_settings`, `mdm_pending_device_operations`), about 40 device-info handlers (SecureBoot, BitLocker, Defender, TPM, HVCI). Fleet's helper naming (`NewSyncMLCmd*`, `processIncomingProtocolCommands`) originates here.

#### oscartbeaumont/windows_mdm
- Repo: <https://github.com/oscartbeaumont/windows_mdm> · Go · MIT · Historical (last push 2021-06) · 39 stars
- Implements: discovery, XCEP (partial), WSTEP (partial), wap-provisioningdoc, SyncML session (regex-parsed), Add/Replace/Get. Nothing else.
- Notes: The seminal minimal Go PoC that most later work forks. `mde_discovery.go`, `mde_policy.go`, `mde_enrollment.go` (parses the `wsse:BinarySecurityToken` PKCS10 via regex, signs with the root CA, emits a `wap-provisioningdoc` v1.1 with CertificateStore and DMClient), `mdm_manage.go` (regexes SessionID and MsgID out of the body). Ships `patch/patch.go`, which monkey-patches GOROOT's x509 to accept extra characters in PrintableString. README says not production and recommends Mattrax. Federated and OnPremise auth.

#### mattrax/Mattrax
- Repo: <https://github.com/mattrax/Mattrax> · Rust (current `rust` branch) and Go (older) · source-available, no longer open source per README · Maintenance (default branch pushed 2025-03; `rust` branch 2025-01) · 129 stars
- Implements (Rust crates): discovery, XCEP, WSTEP, wap-provisioningdoc (`ms-mde/src/wap.rs`), SyncML, Add/Replace/Get/Delete/Exec/Atomic (one module each), DDF v2 parse (`ms-ddf`). No WinDC.
- Notes: Read for design, not for code. The cleanest typed protocol decomposition to model in Go: `crates/ms-mde/src/` (`discovery.rs`, `policy.rs`, `enrollment.rs` with `RequestSecurityTokenResponseCollection`, `header.rs` with `BinarySecurityToken` and WS-Security `UsernameToken`, `wap.rs`, `soap.rs`); `crates/ms-mdm/src/` (`add.rs alert.rs atomic.rs cmd.rs data.rs delete.rs exec.rs final.rs get.rs item.rs meta.rs replace.rs results.rs status.rs sync_body.rs sync_hdr.rs sync_ml.rs`, with comments noting deviations from spec to match the real Windows client); `crates/ms-ddf/src/` (`ddf_v2.rs` with `MgmtTree`, `Node`, `DFProperties`, `DFFormat`, `AccessType`; `msft.rs` with `MSFT:AllowedValues`, ADMX-backed). The old Go version survives in the fork <https://github.com/PhenixH/Mattrax> (AGPL-3.0, 2022): `pkg/syncml/`, `pkg/soap/{discovery,enrollment,policy,fault,soap}.go` with typed XCEP structs, `mdm/windows/win_enroll_{discover,policy,enrollment}.go`; it handles PKCS7 and Federated and OnPremise and uses `github.com/mattrax/xml`.

#### AmberWolfCyber/NachoMDM
- Repo: <https://github.com/AmberWolfCyber/NachoMDM> · Python · no license file · Active (created 2026-08) · 15 stars
- Implements: discovery, XCEP GetPolicies, WSTEP (PKCS#10), wap-provisioningdoc (CertificateStore, APPLICATION, DMClient, ROBO), SyncML session, Get, Exec, OnPremise WS-Security UsernameToken auth, MSI first-sync deploy via EnterpriseDesktopAppManagement. No Atomic, Delete, user scope, WNS, or WBXML (the README calls WBXML out as missing).
- Notes: Security-research server that serves an MSI to an enrolling machine. `mdmserver/{syncml,soap,provisioning,crypto,store,server}.py`, SQLite state. The README is an unusually clear plain-English checklist of the MS-MDE2 and OMA-DM pieces, useful as a spec cross-check. No licence, so read only.

#### WSO2 carbon-device-mgt-plugins and Entgra device-mgt-plugins
- Repo: <https://github.com/wso2/carbon-device-mgt-plugins> · Java · Apache-2.0 · Maintenance (last push 2023-12) · 43 stars; maintained successor <https://github.com/entgra/device-mgt-plugins> · Java · Apache-2.0 · Active (last push 2026-09)
- Implements: discovery, enroll, policy, WSTEP (`BSTValidator`), SyncML (`SyncmlCommandType`), wap-provisioningdoc, OAuth2 token auth.
- Notes: The oldest full Windows 10 OMA-DM enrollment server in the open. The Windows plugin is under `components/mobile-plugins/windows-plugin/org.wso2.carbon.device.mgt.mobile.windows.api/`; its committed `win10-wap-provisioning.xml` and `wap-provisioning.xml` are a canonical provisioning-doc reference (`SSLCLIENTCERTSEARCHCRITERIA`, `DEFAULTENCODING=application/vnd.syncml.dm+wbxml`, WSTEP Renew ROBOSupport). Protocol knowledge only.

#### Minor and thin references
- <https://github.com/dejiblue/windows-mdm> · Go · MIT · Historical (2019) · 3 stars. Discovery, enrollment (`SignCSR.go`), minimal management; README says it was folded into Mattrax.
- <https://github.com/46y9qkpkjc-ui/apexaegis-management-plane> · Go · no license · Active (2026-09) · 0 stars. Typed SyncML structs with Add/Replace/Delete/Exec/Status/Alert in `internal/mdm/oma_dm.go`; early and thin.
- <https://github.com/Manik8286/MDM-SaaS> · Python · no license · Active (2026-04) · 0 stars. `app/mdm/windows/` with `syncml.py`, `omadm.py`, `ca.py` (hand-rolled DER CSR parse); Apple-first.

## 4. Libraries and building blocks

#### SOAP, WS-Trust, XCEP, WSTEP
- Finding: there is no Go WS-Trust or XCEP library. Fleet, Mattrax (Go and Rust), oscartbeaumont, marcosd4h, NachoMDM and local-mdm all build the SOAP request and response structs by hand with the language's XML encoder. Generic Go SOAP tooling exists but is not MDM-aware: <https://github.com/hooklift/gowsdl> (MPL-2.0, 1,222 stars), <https://github.com/fiorix/wsdl2go> (481 stars), <https://github.com/tiaguinho/gosoap> (MIT, 544 stars); useful only for scaffolding.
- <https://github.com/Sleepw4lker/TameMyCerts.WSTEP> · C# · Apache-2.0 · 17 stars · 2024. A standalone MS-WSTEP plus MS-XCEP server. `Models/MS-WSTEP/*.cs` (`RequestSecurityTokenType`, `RequestSecurityTokenResponseCollectionType`, `BinarySecurityTokenType`, `AdditionalContextType`) and `Models/MS-XCEP/*.cs` (`GetPoliciesResponseType`, `CertificateValidityType`) are the clearest typed schema map for these two protocols.

#### XML encoding pitfalls
- <https://github.com/mattrax/xml> · Go · BSD-3-Clause · 2021. A fork of the standard library `encoding/xml` "with better support for SOAP namespaces", created because Go's encoder mishandles the namespace prefixes these SOAP responses need. Evaluate whether Go 1.27's encoder still needs this before adopting.
- Fleet's `SyncMLCmd` and `SyncHdr` types (`server/fleet/microsoft_mdm.go`) use `xml:",chardata"` for `<Data>` content (no CDATA, plain entity-escaped char data; the Learn known-issues page says CDATA is unsupported). `Data *string` with `omitempty` distinguishes Get (no data) from Replace. Namespace quirks are handled by putting `xmlns` on `<Meta>` children such as `<Format xmlns="syncml:metinf">`.
- <https://github.com/russellhaering/goxmldsig> (Apache-2.0, 181 stars) for XML-DSIG (the Certificate auth policy signs the RST); <https://github.com/mattermost/xml-roundtrip-validator> guards against XML round-trip attacks; <https://github.com/antchfx/xmlquery> (MIT) and <https://github.com/beevik/etree> (BSD-2) for XPath or DOM when struct decoding is not enough.

#### SyncML and WBXML codecs
- <https://github.com/kapytein/syncml> · Go · GPL-3.0 · 2023. SyncML message builder only (parsing "planned"). GPL, so do not vendor.
- <https://github.com/getprimo/csp-builder> · TypeScript · MIT · Active 2026-09. Client-side ADMX/ADML plus Policy CSP to SyncML `<Replace>`/`<Delete>` generator; emits `./Device/Vendor/MSFT/Policy/Config/{Area}~Policy~{Cat}/{Policy}`. Good reference for the ADMX to OMA-URI mapping.
- WBXML (needed only for `application/vnd.syncml.dm+wbxml`; XML is the default and every open server negotiates XML): <https://github.com/remdev/go-activesync> (Go, MIT, 2026, the most modern pure-Go WBXML codec, written for Exchange ActiveSync), <https://github.com/magicmonty/wbxml-go> (Go, 2012, encoder and decoder), <https://github.com/netxfly/dewbxml> (Go, 2018, decode only), <https://github.com/libwbxml/libwbxml> (C, 2025), and the WBXML module in <https://github.com/zvikara/OmaSharp> (C#, 2022, token-page encoder and decoder, OMA CP `WapProvisioning`, SyncML code pages).

#### x509 and CSR handling for the enrollment certificate
- The parse pitfall: Windows sends CSRs whose subject contains characters illegal in Go's `PrintableString` parser (`asn1: syntax error: PrintableString contains invalid character`). Three approaches seen: oscartbeaumont patches GOROOT at build time; Fleet and marcosd4h vendor a full copy of the x509 CSR parser with `isPrintable` relaxed (`server/mdm/microsoft/wstep_csr.go`), the recommended approach; Manik8286 parses the DER by hand.
- PKCS#10 vs PKCS#7: the `BinarySecurityToken` `ValueType` is `...enrollment#PKCS10` on first enrollment and `#PKCS7` on ROBO renewal. Fleet's `GetClientCSR` handles both; PKCS#7 is parsed with <https://github.com/smallstep/pkcs7> (MIT) and its signature and signing time verified before extracting the inner CSR. Most PoCs handle PKCS#10 only.
- SCEP for `ClientCertificateInstall/SCEP` payloads: <https://github.com/micromdm/scep> (MIT, 391 stars; note the go-apple-dm rule that `micromdm/plist` is the only accepted micromdm dependency, so prefer <https://github.com/smallstep/scep> here).

#### TPM attestation
- <https://github.com/google/go-attestation> (Apache-2.0, 441 stars), <https://github.com/google/go-tpm> (670 stars), <https://github.com/google/go-tpm-tools> (308 stars), <https://github.com/foxboron/go-tpm-keyfiles>. Fleet depends on the last three. Relevant to the MDE2 v19 attestation nodes (DeviceAssociationMaaUrl, 4.2.3 Azure Attestation) and DeviceStatus MDMClientCertAttestation.

#### WNS HTTP clients
- <https://github.com/oniestel/go-wns> · Go · MIT · 9 stars · 2021. `Client.Init(SID, secret)` plus `Send(channelURL, notification)` for toast, tile, badge, raw. The only dedicated Go WNS client; MDM push needs the raw path with `X-WNS-Type: wns/raw`, `X-WNS-Cache-Policy: cache`, optional `X-WNS-TTL`. Fleet's and local-mdm's WNS clients are hand-rolled; local-mdm's `internal/platform/windows/wns.go` shows the full pattern (OAuth2 client-credentials to `https://login.live.com/accesstoken.srf` scope `notify.windows.com`, then POST to the ChannelURI). <https://github.com/AzureAD/microsoft-authentication-library-for-go> (MIT) if Entra tokens are needed for other calls.

#### Same-organisation building blocks
- <https://github.com/deploymenttheory/go-sdk-windowscsp> · Go · MIT · Active (last push 2026-09-04). A Go DDF v2 parser plus typed CSP SDK: `internal/ddf/parse.go` unmarshals `<MgmtTree>`, `<Node>`, `<DFProperties>`, `<DFFormat>` (b64, bin, bool, chr, int, node, null, xml) and `MSFT:AllowedValues` (ENUM), splitting `./Device` and `./User` scopes; `internal/codegen/generator.go` emits Get/Add/Replace/Delete/Exec Go packages per CSP with OMA-URIs and allowed-value constants; `windowscsp/syncml/` builds SyncML fragments (all verbs, `<Final/>`, entity escaping) with an `Executor` (live client) and a `Recorder` (offline authoring); `windowscsp/clienttest/` is an in-memory fake CSP tree keyed by OMA-URI; `cmd/fetchddf` pulls the DDF zip from download.microsoft.com. The closest existing Go building block for CSP and DDF work; the build plan must decide whether go-oma-dm depends on it, absorbs it, or stays independent.
- <https://github.com/deploymenttheory/go-bindings-win32> (Go, MIT, 2026-09; generated Win32 bindings including `mobiledevicemanagementregistration`) and <https://github.com/deploymenttheory/go-bindings-winrt> (Go, MIT, 2026-09). Device-side bindings for a Go test client that calls `mdmregistration.dll` and `mdmlocalmanagement.dll`.

#### Storage schema patterns (Fleet)
- From Fleet's `server/datastore/mysql/schema.sql` and migrations: `mdm_windows_enrollments` (mdm_device_id, mdm_hardware_id UNIQUE, device_state, device_type, enroll_type, enroll_user_id, host_uuid, `credentials_hash binary(16)`, credentials_acknowledged, awaiting_configuration, poll_schedule_relaxed, has_pending_commands, hardware_serial, ztd_registration_id, last_login_status, enrolled_activity_at); `windows_mdm_commands` (command_uuid PK, raw_command mediumtext, target_loc_uri); `windows_mdm_command_queue` (enrollment_id plus command_uuid PK, acked_at); `windows_mdm_command_results`; `windows_mdm_responses` (`raw_response_gz mediumblob` since migration 20260626); `wstep_serials`, `wstep_certificates` (serial PK, certificate_pem, revoked), `wstep_cert_auth_associations`; `mdm_windows_configuration_profiles`, `host_mdm_windows_profiles`, `mdm_windows_enrollment_config`, `host_autopilot_devices`, `windows_updates`. WSTEP CA datastore ops in `server/datastore/mysql/wstep.go`.

## 5. Clients, viewers, and test tooling

#### okieselbach/SyncMLViewer
- Repo: <https://github.com/okieselbach/SyncMLViewer> · C# (WPF, .NET) · MIT · Active (last push 2026-05; latest release v1.4.0 2025-01-21) · 233 stars
- Implements: client-side SyncML capture and local SyncML injection.
- Notes: Real-time OMA-DM SyncML stream viewer via ETW. Providers (`MainWindow.xaml.cs`): `Microsoft.Windows.DeviceManagement.OmaDmClient` `{0EC685CD-64E4-4375-92AD-4086B6AF5F1D}` and `Microsoft-WindowsPhone-OmaDm-Client-Provider` `{3B9602FF-E09B-4C6C-BC19-1A3DFA8F2250}`. Parses `OmaDmSessionStart`, `OmaDmSessionComplete`, `OmaDmSyncmlVerboseTrace`. MMP-C support: a separate MMP-C sync button, reads both `OmaDmAccountIdMDM` and `OmaDmAccountIdMMPC` enrollment GUIDs, triggers sync via the scheduled task `Microsoft\Windows\EnterpriseMgmt\{GUID}\Schedule #3 created by enrollment client` or `DeviceEnroller.exe /o {GUID} /c`. WinDC: menu items open `HKLM\SOFTWARE\Microsoft\DeclaredConfiguration` and `C:\ProgramData\microsoft\DC\HostOS`. Local MDM management (v1.2.0+): `SyncMLViewer.Executer.exe` calls `mdmlocalmanagement.dll` (`RegisterDeviceWithLocalManagement`, `ApplyLocalManagementSyncML`, `UnregisterDeviceWithLocalManagement`). `MdmDiagnostics.cs` reads enrollment info from `HKLM\SOFTWARE\Microsoft\Enrollments\{GUID}`. v1.3.0 added an HTML diagnostics report, status-code lookup, cert decode; v1.4.0 added Autopilot hardware-hash decode. The single most useful tool for watching what the real client sends to go-oma-dm.

#### MDM local management with mdmlocalmanagement.dll
- Source: <https://oliverkieselbach.com/2024/02/05/mdm-local-management-using-syncml-viewer/> · Oliver Kieselbach · blog · 2024-02-05 · Active
- Covers: Applying SyncML locally without a real MDM server: set `AllowEmbeddedMode` (`HKLM\SYSTEM\CurrentControlSet\Services\EmbeddedMode\Parameters\Flags`), `RegisterDeviceWithLocalManagement` (creates a temporary `Local_Management` enrollment), `ApplyLocalManagementSyncML(request, out result)`, then unregister. Requires admin. Standard OMA-DM verbs.
- Notes: Some settings persist after unregister; subject to normal CSP support. Fleet's `mdm_bridge` osquery table uses the same DLL. Directly reusable for a Go test harness that applies SyncML to a local Windows box and captures results, via the same-org Win32 bindings.

#### Fleet's Windows MDM test client and load tooling
- Source: <https://github.com/fleetdm/fleet/blob/main/pkg/mdm/mdmtest/windows.go> · Fleet · repo · Active
- Covers: `TestWindowsMDMClient` (`NewTestMDMClientWindowsProgramatic`, `NewTestMDMClientWindowsAutomatic`, an empty-BinarySecurityToken variant, options for LoginStatus, not-in-OOBE, signing key plus tenant) is a full in-Go Windows MDM device simulator that drives Discovery, XCEP, WSTEP, then SyncML. `windows_scep.go` parses SCEP CSP command layouts. `cmd/osquery-perf/agent.go` (`runWindowsMDMLoop`, `doWindowsMDMCheckIn`) simulates enrolled Windows hosts at scale; the plan in `infrastructure/loadtesting/terraform/windows-mdm-loadtest.md` targets 100k hosts. `tools/mdm/windows/programmatic-enrollment/main.go` calls `mdmregistration.dll` for real enrollment.
- Notes: The model for go-oma-dm's own simulator package, in the same role as `jessepeterson/mdmb` for Apple.

#### Device emulators from the security community
- <https://github.com/hotnops/Imitune> · Python · GPL-3.0 · 2025-09 · 1 star. An OMA-DM client that impersonates an Intune-managed device: `oma_dm_client.py`, `oma_dm_session.py`, `management_tree.py`, `omadm_message.py`, `oma_dm_commands.py`. Client side only; GPL.
- <https://github.com/secureworks/pytune> · Python · Apache-2.0 · 2025-08 · 297 stars. A fake device that Entra-joins and Intune-enrolls (Windows, Android, Linux), checks in, steals VPN and Wi-Fi config, downloads apps. `device/windows.py`. Black Hat EU 2024. The best-known Intune device emulator and the best reference for the Entra-join enrollment handshake from the client side.
- <https://github.com/Gerenios/AADInternals> · PowerShell · MIT · 2025-09 · 1,690 stars. `MDM.ps1` and `MDM_utils.ps1`: `Enroll-DeviceToMDM`, `New-SyncMLRequest`, `Parse-SyncMLResponse`, `Invoke-SyncMLRequest` against Intune's `cimhandler.ashx`; sends a WSTEP RequestSecurityToken with a PKCS10 BinarySecurityToken and parses the returned `wap-provisioningdoc`. Its DeviceEnrollment declares CSP nodes including `./Device/Vendor/MSFT/DeclaredConfiguration` and `./User/Vendor/MSFT/DeclaredConfiguration`, a WinDC reference.

#### MDM diagnostics on the device
- Source: <https://learn.microsoft.com/en-us/windows/client-management/mdm-collect-logs> · Microsoft · learn · 2025-08-04 · Active
- Covers: See 1.4. `mdmdiagnosticstool.exe -area "DeviceEnrollment;DeviceProvisioning;Autopilot" -zip out.zip`; `MDMDiagHtmlReport.html` (management URL, MDM server device ID, certs, policies), `MdmDiagReport_RegistryDump.reg`, `.evtx`; Event Viewer channel DeviceManagement-Enterprise-Diagnostics-Provider Admin and Debug; remote collection via the DiagnosticLog CSP.
- Notes: The first thing to read when an enrollment against go-oma-dm fails.

#### Test environments: guestweave (same organisation)
- Repo: <https://github.com/deploymenttheory/guestweave-cli-windows> · Go · MIT · Active (last push 2026-09-01; pre-alpha) · 0 stars; <https://github.com/deploymenttheory/guestweave-cli-macos> · Go · MIT · Active (last push 2026-09-03) · 0 stars; <https://github.com/deploymenttheory/guestweave-agent> · Go · MIT · Active (last push 2026-09-01); <https://github.com/deploymenttheory/go-sdk-vtpm2> · Go · MIT · Active, GA v1.0.2 (last push 2026-08-26) · public
- Note: the three guestweave repositories are private on 2026-09-07; they resolve only with an authenticated GitHub session, so the link check reports them as 404. go-sdk-vtpm2 is public.
- Implements: VM lifecycle for Windows 11 guests (create, run, snapshot, clone, suspend, console), unattended Windows install from Microsoft retail media, an in-guest agent driven from the host, and an authenticated HTTP plus MCP API for driving all of it from tests. Test environment only; no MDM protocol code.
- Notes: The project's test environments come from these two CLIs; no third-party hypervisor tooling is needed. On a Windows host, `weave create win --from-windows pro-25h2` downloads Microsoft's current multi-edition x64 retail ISO, attaches an answer file on a side volume, and installs unattended into a local admin (`weave`/`weave`) with OOBE skipped, OpenSSH enabled and a completion marker; `latest` tracks whatever Microsoft's consumer download serves (25H2 today, 26H2 once it reaches that channel), and `--from-windows <iso>` takes a Release Preview ISO by path. The guest runs on the Host Compute Service in guest-isolation mode and gets a real TPM 2.0 and Secure Boot by default (`Get-Tpm` reports MSFT, `Confirm-SecureBootUEFI` True, BitLocker turns itself on), so the Windows 11 install checks are satisfied rather than bypassed; `--no-vtpm` gives a plain VM. The vTPM state lives in the guest-state file and travels with snapshots and clones, so a snapshot of a clean, un-enrolled guest can be reverted to reset an enrollment without breaking BitLocker. `weave serve` exposes create, start, stop, snapshot revert, IP resolution, an MJPEG console and agent calls over HTTP (`/weave/vms/...`, bearer token, OpenAPI at `/openapi.json`) or MCP, and `weave agent call <vm> <tool>` runs tools inside the guest; that is the seam an e2e harness uses to enroll a real Windows client against go-oma-dm, capture SyncML with SyncMLViewer, and revert. On a macOS host, `weave create --from-windows 11 <name>` creates a Windows 11 ARM64 guest on guestweave's own Hypervisor.framework VMM (the Virtualization framework cannot boot Windows on ARM), with a TPM 2.0 provided in-process by go-sdk-vtpm2, a pure Go TPM built from the TCG specification (125 of 136 commands, EK and SRK provisioning, PCRs, sealing, Quote, Certify, MakeCredential and ActivateCredential, versioned persistent state), exposed to the guest over the standard CRB interface with a `MSFT0101` ACPI identity so Windows drives it with its stock driver; the guest agent handles Windows shutdown and restart, and `winmedia` builds the ARM64 install media. Either host therefore gives a Windows 11 guest with a TPM 2.0. Two limits carry over from Microsoft, not from guestweave: Intune's enrollment attestation feature is unsupported on any VM, but go-oma-dm is itself the verifier of the MS-MDE2 AIK claims it receives, so that flow can be exercised against a vTPM guest with trust anchored to the emulator's provisioned endorsement key, and only validation against vendor EK certificate chains needs physical hardware; and the unattended profile skips OOBE, so Autopilot and OOBE-time enrollment flows need the retail or noprompt media profile instead.

#### Authoring helpers
- <https://github.com/getprimo/csp-builder> (TypeScript, MIT, 2026) and <https://github.com/Weatherlights/PolicyApplicator-for-Microsoft-Intune> (PowerShell, 26 stars, 2025-09) convert ADMX, ini, json, or registry into OMA-URI SyncML. Useful for generating realistic test payloads.

## 6. Community knowledge

Newest first per author. Only posts that inform the protocol or server side are listed. Where a
post documents Intune internals (MMP-C, EPM, device inventory), it is kept because WinDC is the
same wire protocol go-oma-dm must speak, but it is marked Intune-gated.

#### Oliver Kieselbach (oliverkieselbach.com)
- <https://oliverkieselbach.com/2025/01/27/syncml-viewer-update-with-autopilot-hash-decoding/> · blog · 2025-01-27 · Active · yes. SyncMLViewer v1.4.0: Autopilot hardware-hash decode, NodeCache-to-registry lookup, hex viewer.
- <https://oliverkieselbach.com/2024/02/05/mdm-local-management-using-syncml-viewer/> · blog · 2024-02-05 · Active · yes. Local SyncML via `mdmlocalmanagement.dll` and EmbeddedMode (see section 5).
- <https://oliverkieselbach.com/2023/12/12/new-syncml-viewer-version/> · blog · 2023-12-12 (updated 2024-02-24) · Active · yes. MMP-C sync button; WinDC traffic is still SyncML over the same ETW provider; the 64 KB ETW buffer truncation indicator.
- <https://oliverkieselbach.com/2022/09/21/deep-dive-of-scep-certificate-request-renewal-on-intune-managed-windows-clients/> · blog · 2022-09-21 · Maintenance · partial. `omadmclient.exe`, ClientCertificateInstall CSP, the encrypted one-hour SCEP challenge, `dmcertinst.exe` scheduled task, NodeCache-driven renewal by thumbprint. The client-side flow go-oma-dm must emit profiles for.
- Older but still correct: "Windows 10 MDM client activity monitoring with SyncML Viewer" (2019-10-11), "Intune Policy Processing on Windows 10 explained" (2019-07-18), both under <https://oliverkieselbach.com/tag/syncml/>.

#### Rudy Ooms (call4cloud.nl, and posts syndicated on patchmypc.com)
- <https://patchmypc.com/blog/what-really-happens-when-you-press-remote-sync-in-intune/> · blog · 2025-12-29 (updated 2026-08-01) · Active · yes. The push-to-session chain: WNS push, `WNF_ENTR_PUSH_RECEIVED` (WNF state `75E0BEA328998213`), PushLaunch task, `deviceenroller.exe` (immediate or 5-minute queue), `omadmclient.exe`; Event 50001. The on-device WNF state is where `GetPushAlertInfo` reads, which ties to the 24H2 push symptom in section 7.
- <https://patchmypc.com/blog/intune-on-demand-device-sync-now-uses-ic3-for-ime-workloads/> · blog · 2026-08-01 (updated 2026-08-14) · Active · partial. Intune moved on-demand sync for the Intune Management Extension to IC3/Trouter, but WNS still wakes the Windows MDM client. Confirms WNS remains the MDM wake path.
- <https://call4cloud.nl/windows-autopilot-device-association/> · blog · 2026-08-31 (updated 2026-09-05) · Active · Intune-gated. Deepest public detail on the UEFI association marker.
- <https://call4cloud.nl/mmp-c-onboarding-linked-enrollment-dual-enrollment/> · blog · 2025-01-20 · Active · Intune-gated. Linked and dual enrollment uses the Entra device token; the enrollment type integer flips from 6 to 14; Graph `enableEndpointPrivilegeManagement`; `OnboardedToMicrosoftManagedPlatform`. The enrollment-type integers and dual-enrollment registry layout are reverse-engineered, not documented.
- <https://call4cloud.nl/in-the-name-of-mmp-c/> · blog · 2023-08-02 (updated 2025-02-21) · Active · Intune-gated. The expansion is "Microsoft Management Platform Cloud" (from binaries: `MicrosoftManagementPlatformCloudMdm`); Microsoft's own term is "Declared Configuration Enrollment".
- <https://call4cloud.nl/epm-enrollment-mmpc-mmp-c/> · blog · 2023-04-29 (updated 2025-12-12) · Active · Intune-gated. MMP-C discovery at `enrollment.dm.microsoft.com`, check-in at `checkin.dm.microsoft.com`; the EPM MSI is delivered via the EnterpriseDesktopAppManagement CSP.
- <https://call4cloud.nl/mmp-c-discovery-epm-agent-not-installed/> · blog · 2023-06-16 · Maintenance · Intune-gated. `discovery.dm.microsoft.com` JSON discovery (`{'errorCode':'InvalidRequest'}`), the four-phase MDE-style flow; a multi-level-domain bug.
- <https://call4cloud.nl/when-does-a-device-sync-with-mmp-c/> · blog · 2024-04-01 (updated 2025-02-21) · Active · Intune-gated. MMP-C Schedule #3 every 4 hours (classic MDM every 8 hours); `dcsvc.dll`.
- <https://call4cloud.nl/declared-configuration-windc-dcsvc/> · blog · 2023-08-23 (updated 2025-02-21) · Active · Intune-gated. dcsvc.dll origins, `PolicyConsistencyEnsurerJob` (about 15 minutes), DeclaredConfigurationStore.
- <https://call4cloud.nl/dcsvc-windc-refresh-schedule/> · blog · 2023-07-11 (updated 2025-02-21) · Active · Intune-gated. Drift detection cadence for EPM documents.
- <https://call4cloud.nl/2023/09/along-with-the-gods-the-two-pushlaunch-tasks> · blog · 2023-09-26 (updated 2024-10-16) · Active · yes. PushLaunch error `0x82AC0204` leads to the queued 5-minute task (`/q` vs `/z` to deviceenroller); `WindowsMMPCPush`.
- <https://call4cloud.nl/intune-sync-issue-dmwappushservice-missing/> · blog · 2022-11-03 · Maintenance · yes. dmwappushservice orchestrates push sync; if deleted the PushLaunch and PushRenewal tasks are never created and "Sync could not be initiated (0)".
- Attestation series, Intune-gated feature with generic wire: <https://call4cloud.nl/mdm-hardening-windows-enrollment-attestation/> (2024-07-17), <https://call4cloud.nl/windows-enrollment-attestation/> (2024-07-16), <https://call4cloud.nl/istpmattested/> (2024-07-31), <https://call4cloud.nl/initiaterecovery-enrollment-attestation/> (2023). `UseTPMForEnrollmentKey`, `AllowRecovery` and `InitiateRecovery` renewing the MDM cert with the key in the TPM, `DmGetTargetAik`, RecoveryStatus codes, and `device.IsTpmAttested -eq "False"` blocking enrollment with 0x80180032.
- Also relevant: the EmmDeviceID and Config Refresh bug (2024-04-02, `GetEnrollmentEntDmID` picks the first alphabetical DMClient subkey and breaks linked enrollment and cert renewal), MDM-to-MMP-C resource access policies (2024-12-30), fixing corrupted WinDC documents (2023-09-20, state 60 healthy, 81 failing), and device inventory from MMP-C (2024-12-11), all on call4cloud.nl.

#### Joymalya Basu Roy (joymalya.com)
- <https://joymalya.com/modern-windows-provisioning-autopilot-internals-part-2/> · blog · 2026-03-03 · Active · yes. OOBE network flow, TPM EK and AIK attestation, Autopilot discovery via `login.live.com` and `ztd.dds.microsoft.com`, profile cache `HKLM\SOFTWARE\Microsoft\Provisioning\AutopilotPolicyCache`.
- <https://joymalya.com/autopilot-v1-or-autopilot-v2-a-strategic-guide-to-modern-windows-provisioning/> · blog · 2026-02-18 · Active · Intune-gated. v2 has no hardware hash and no ESP.
- <https://joymalya.com/modern-windows-provisioning-internals-part-1/> · blog · 2025-12-12 · Active · yes. Boot to OOBE internals; `ImageState`; defaultuser0.
- <https://joymalya.com/from-omadm-to-mmpc-the-evolution-of-modern-windows-management/> · blog · 2025-10-29 · Active · Intune-gated. Imperative OMA-DM vs declarative MMP-C, dual enrollment, 4 h vs 8 h, EPM and device inventory workloads.
- <https://joymalya.com/windows-devices-stopped-syncing-with-intune/> · blog · 2021-12-06 · Historical · yes. dmwappushservice disabled means silent sync death; WNS on port 443.

#### Patch My PC blog (patchmypc.com)
- <https://patchmypc.com/blog/autopilot-identifying-security-policies-esp/> · Rudy Ooms · blog · 2025-05-20 (updated 2025-08-15) · Active · yes. `FirstSyncStatus/ExpectedPolicies` is materialised at `C:\ProgramData\Microsoft\DMClient\<guid>\FirstSync\ExpectedPolicies`; default only `EntDMID`; `IsServerProvisioningDone` locks the node. Concrete mechanics for how go-oma-dm should populate ExpectedPolicies.
- <https://patchmypc.com/blog/mmp-c-the-future-of-windows-device-management-with-intune/> · Rudy Ooms · blog · 2025-10-22 · Active · Intune-gated.
- <https://patchmypc.com/blog/the-history-of-intune-from-oma-dm-to-modern-windows-management/> · Rudy Ooms · blog · 2025-10-19 (updated 2025-11-07) · Active · background. US patent 20070113186A1 DM tree, CSP and ConfigManager, the Windows Phone 8.1 EDM protocol, WinDC single-document get and set.
- <https://patchmypc.com/blog/enhancing-device-security-device-attestation/> · blog · Active · Intune-gated. TPM, AIK, IsTPMAttested, 0x80180032, the Intune report.

#### Others
- <https://msendpointmgr.com/2025/11/11/mmp-c-the-new-era-of-windows-client-management-with-microsoft-intune/> · Anders Ahl · blog · 2025-11-11 · Active · Intune-gated.
- <https://jannikreinhard.com/2026/06/23/intune-troubleshooting-guide> · Jannik Reinhard · blog · 2026-06-23 · Active · yes. Event IDs 75, 76, 208, 813, 814, 809; WNS wake; an explicit note that the MMP-C acronym, the enrollment-type numbers, the `*.dm.microsoft.com` hosts and the 4-hour cadence were reverse-engineered by Rudy Ooms and never published by Microsoft; `deviceenroller.exe /DeclaredConfigurationRefresh`; two enrollment GUIDs under `HKLM\SOFTWARE\Microsoft\Enrollments`.
- <https://joostgelijsteen.com/declared-configuration/> (2025-02-28), <https://joostgelijsteen.com/device-inventory-mmp-c/>, <https://joostgelijsteen.com/windows-attestation/> (2024-07-16) · Joost Gelijsteen · blog · Active · Intune-gated. The attestation post says "this feature is presented as an Intune MDM capability."
- <https://petervanderwoude.nl/post/getting-started-with-windows-enrollment-attestation/> · Peter van der Woude · blog · 2024-09-30 · Active · Intune-gated. The `IsTpmAttested` filter, DMClient Recovery, the Graph recovery call.
- <https://www.tbone.se/2025/03/17/be-prepared-for-windows-declared-configuration-in-intune/> · Torbjörn Granheden · blog · 2025-03-17 (updated 2026-04-01) · Active · Intune-gated.
- <https://mikemdm.de/2024/09/15/troubleshooting-intune-endpoint-privilege-management/> · blog · 2024-09-15 · Active · Intune-gated. Dual enrollment, the EPM agent path, `HKLM\SOFTWARE\Microsoft\EPMAgent\Policies`.
- <https://www.vansurksum.com/2021/05/25/mdm-policy-processing-on-windows-10-with-microsoft-endpoint-manager-a-closer-look/> · Kenneth van Surksum · blog · 2021-05-25 · Historical · yes. The 8-hour interval, `HKLM\SOFTWARE\Microsoft\PolicyManager\current\device`, the diagnostics provider logs.

#### Marcos Oviedo: Windows Agentless C2, (Ab)using the MDM Client Stack
- Source: whitepaper <https://github.com/marcosd4h/presentations/blob/main/Blackhat/2023/Whitepaper_Windows_Agentless_C2_Abusing_the_MDM_Client_Stack.pdf> (Black Hat USA, 2023-08-09, with Zach Wasserman); slides <https://typhooncon.com/wp-content/uploads/2024/08/Abusing_the_MDM_Client_Stack_Typhooncon_2024-2.pdf> (2024); Ekoparty 2023 talk <https://www.youtube.com/watch?v=LEAtGGEd1aU> · talk · Active · Third-party MDM: yes (it is a rogue third-party MDM server)
- Covers: A working Go MDM server that enrolls Windows via MS-MDE2 and manages via MS-MDM; CSP architecture (ConfigManager2 COM, `IConfigServiceProvider2`, `HKLM\SOFTWARE\Microsoft\Provisioning\CSPs`, 60+ CSPs); sample SyncML for DevInfo Get, Policy CSP `Defender/AllowRealtimeMonitoring`, Update `AllowAutoUpdate`, Firewall, and the WMI Bridge CSP `./cimv2/Win32_Service`.
- Notes: The closest public prior art to go-oma-dm as a design narrative; the code is in section 3. The enrolling user must be a local admin.

#### Fleet's own writing on Windows MDM
- <https://fleetdm.com/guides/windows-mdm-setup> · Noah Talerman · guide · Active · yes. Registers Fleet in Entra via Mobility (MDM and WIP), Add application, Create your own; MDM user scope All; App ID URI = the Fleet URL; the MDM URLs come from Fleet's settings. "Enrolling against an IdP that isn't federated with Entra isn't currently supported."
- <https://fleetdm.com/guides/creating-windows-csps> · Harrison Ravazzolo · guide · Active · yes. Only `<Add>` and `<Replace>` shown; CDATA data (contradicting the Learn known-issues page, which says CDATA is unsupported; Fleet strips it before sending); ADMXInstall.
- <https://fleetdm.com/announcements/fleet-introduces-windows-mdm> · announcement · Maintenance.

## 7. Known problems and pitfalls

| Pitfall | Source and date | What go-oma-dm does about it |
|---|---|---|
| WNS push-initiated session on 24H2 and 25H2 logs Event 4603 "Getting push alert info for push initiated session failed, notificationIdNotRetrieved". The asker first reported no OMA-DM session, then updated that the session does run and "the error does not block the session". No Microsoft response. | Microsoft Q&A 5907951, 2026-06-01 (1.7); Fleet issue #43773, closed 2026-06-01 | Never rely on push alone: keep the DMClient poll schedule. Send a non-empty raw body with `X-WNS-Cache-Policy: cache`. Treat 4603 as a known ambiguous symptom and verify session arrival server-side rather than trusting WNS `received`. |
| `dmwappushservice` missing or disabled (often a hardening baseline `sc delete`) means PushLaunch and PushRenewal tasks are never created, `SslClientCertReference` is missing, and the device silently stops syncing with no error logged. | Learn troubleshoot page "cannot sync Windows 10 devices" (ms.date 2026-03-30); call4cloud 2022-11-03; joymalya 2021-12-06 | Document the service as a prerequisite; provide a server-side "has not checked in" alert since the client logs nothing. |
| Aggressive polling is harmful: Fleet's 1-minute poll made the Audit Policy CSP clear and reapply on every session (about 60 Event 4719 per minute per host) and produced 333 requests per second at 20k hosts. | Fleet issue #43773, opened 2026-04-20, closed 2026-06-01 (not planned) | Use WNS push for immediacy with a long poll fallback (Microsoft's default schedule); never re-send unchanged configuration each session; diff desired vs acknowledged state. |
| A SyncML engine that only accepts `<Add>` and `<Replace>` cannot trigger Exec-only nodes (LinkedEnrollment, Recovery, RemoteWipe, Reboot, SCEP Enroll), cannot Delete, cannot use Atomic, and cannot target `./User/Vendor/MSFT`; one-shot SCEP certs cannot be renewed. | Fleet issue #32919, opened 2025-09-12, open | Support Add, Replace, Get, Delete, Exec, Atomic and Sequence and both scopes from the first release; model SCEP certificate lifecycle (renew, expiry, removal). |
| Storing the full SyncML response envelope per session was Fleet's top database writer at 10k hosts (about 34 percent of writer time). | Fleet issue #44188, opened 2026-04-25, closed 2026-07-15 | Persist per-command results, not raw envelopes; keep the raw message only in a bounded debug ring or under an opt-in flag; plan retention and pruning up front (Fleet #43897 and #49746 never pruned). |
| Same-second commands delivered in random order because the queue orders by a second-resolution timestamp. | Fleet issue #49616, open | Order the command queue by a monotonic sequence, never by wall-clock time. |
| Client certificate back-dated by the renewal period (`NotBefore = now - 180d`), halving its useful life; no ROBO renewal because enrollments are federated and `ROBOSupport` is false. | Fleet issues #52601 and #52492, open | Issue certs with `NotBefore = now`; implement ROBO renewal with client TLS on a dedicated renewal URL; set RenewPeriod 40 to 60 days and RetryInterval 4 to 5 days in the provisioning doc. |
| Duplicate `HWDevID` from cloned images without sysprep silently steals an existing enrollment. | Fleet issue #50612, open | Key enrollments on the issued certificate and the MDE2 DeviceID, and treat a repeated HWDevID as a conflict to surface, not a silent re-enrollment. |
| User-scope commands before any user has signed in fail with 500 or 507 and roll back the Atomic. | Fleet issues #50196 and #48931; Learn known issues | Read the package-1 `LoginStatus` alert; hold `./User` commands until it reports `user`; never wrap device and user commands in one Atomic (Fleet #40713). |
| WinDC common errors: Device-scope LocURI with a User-context document gives "The system cannot find the file specified"; Document ID mismatch between LocURI and body gives `0x8000FFFF`; an OMA-URI typo gives `0x86000002` plus operational `0x82d00007`. | Learn declared-configuration page (2025-08-04) | Validate WinDC documents against the schema before sending: DocID in LocURI must equal the `id` attribute; `context` must match the scope; every CSP URI must exist in the DDF. |
| Enrollment error codes: SOAP faults 0x80180001 to 0x80180019 plus CustomServerError 0x80180032; auto-enroll failures 0x8018002b (unverified UPN suffix or MDM user scope None), 0x80180014 (Windows MDM disabled in the tenant), 0x8018000a (already enrolled), 0x80180026 (device management blocked), 0x8007064c (machine already enrolled). | MS-MDE2 2.2.10 (2026-08-11); Learn Entra integration page; Learn troubleshoot Windows enrollment errors (ms.date 2026-03-30) | Return the exact SOAP fault subcode and `deviceenrollmentserviceerror` so the client shows the right message; implement device-cap and licence rejections deliberately. |
| `0x80180032` (IsTPMAttested) blocks enrollment when an Intune enrollment restriction requires attestation. | call4cloud 2024-07-31 | Not something a third-party MDM sets, but go-oma-dm can read attestation via the MDE2 AdditionalContext items and choose to reject non-attested enrollments itself. |
| Large objects and MaxMsgSize: the server MUST support large-object handling (`MoreData`, 213, MaxObjSize); no numeric MaxMsgSize is published; the ETW capture tools truncate at 64 KB. | MS-MDM 2.2.5.3 and 2.2.7.2 notes (2024-04-23); OMA DM Protocol 1.2.1 sections 6 and 7; SyncMLViewer | Implement chunked send and receive with `MoreData` and status 213; read DevDetail `LrgObj` and `URI/MaxSegLen` before chunking; keep individual messages small enough for the capture tools. |
| Certificate renewal edge cases: ROBO needs client TLS with the existing MDM cert; auto-renew is the only method for federated enrollments; the device will not auto-renew an already expired cert; the device refuses redirects during renewal; behind an SSL-bridging proxy the server sees the proxy cert. | Learn certificate-renewal page (2025-08-04); Microsoft Q&A 1254209 (2023-04-24) | Terminate TLS at the server or forward the client cert; support `WSTEP/Renew/ServerURL`; never redirect on the renewal path; validate the PKCS#7 signature, window, issuer and same-requester. |
| SyncML 1.1 vs 1.2: management SyncML MUST be `SYNCML:SYNCML1.2` with VerDTD 1.2 and VerProto DM/1.2; Microsoft's own WinDC and some CSP samples use 1.1; elements MUST follow DTD order; w7 parameter names are uppercase and case-sensitive; LocURI cannot start with `/`; nested Atomic gives 500 and the parent 507; Add-then-Replace in one Atomic is unsupported; Get inside Atomic is unsupported; CDATA is unsupported. | Learn OMA-DM protocol support (2025-08-04); MS-MDM 2.2.1 and 2.2.2 (2024-04-23); Learn known issues | Emit 1.2, parse both; marshal in DTD order; entity-escape data instead of CDATA; enforce the Atomic rules in the command builder so invalid batches cannot be constructed. |
| Transport: enrollment responses must not be chunked; the client sends `?mode=Maintenance|Machine&Platform=WoA`, `User-Agent: MSFT OMA DM Client/1.2.0.1`, and optionally `Authorization: Bearer`, `DeviceToken`, `MS-Signature`, `MDM-GenericAlert`, `client-request-id`. | MS-MDM 2.1 (2024-04-23); Learn federated enrollment page | Set Content-Length on every enrollment response; parse and log the query parameters and headers; do not hard-code checks on the User-Agent (Learn best practice). |
| After the device renews its WNS channel it checks in immediately; the server must re-read `Push/ChannelURI` every session or push goes to a dead channel (410). | Learn known issues; WNS headers page | Issue a Get on `Push/ChannelURI` in every session response and update the stored URI; treat WNS 404 and 410 as "channel dead, wait for poll". |
| `./User` provisioning fails on Entra-joined devices unless an Entra user is signed in; server-initiated unenroll silently fails for add-work-account enrollments and is disabled for Entra-joined devices. | Learn known issues (2025-08-04) | Gate user-scope delivery on LoginStatus; for Entra-joined devices offer wipe or Entra-side removal instead of Unenroll. |

## 8. Summary matrix

Y = implemented, P = partial, C = client or device side, M = mock or parse only, blank = none.

| Project (language) | Discovery | XCEP | WSTEP | Prov doc | SyncML session | Batch | Add/Replace | Get | Delete | Exec | Atomic | User scope | WNS | Renew | Unenroll | WinDC | DDF | Test client |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| fleetdm/fleet (Go) | Y | Y | Y | Y | Y | Y | Y | Y (internal) | M | Y (SCEP only) | M | Y | P (built, off) | P | Y | | | Y |
| Malcolm/local-mdm (Go) | Y | Y | Y | Y | Y | Y | Y | Y | | Y | | | Y | P | Y | | | Y |
| mjoliver/Windows-MDM (Go) | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | | P | P | Y | | Y | |
| marcosd4h/mdm PoC (Go) | Y | Y | Y | Y | Y | P | Y | Y | | | | | | | Y | | | C |
| marcosd4h/MDMatador (Go) | Y | Y | Y | Y | Y | Y | Y | Y | | P | | | | P | Y | | | |
| oscartbeaumont/windows_mdm (Go) | Y | P | P | Y | Y | | Y | Y | | | | | | | | | | |
| Mattrax rust branch (Rust) | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | Y | | | P | Y | | Y | |
| Mattrax Go (PhenixH fork) | Y | Y | Y | Y | Y | P | Y | Y | | | | | | P | P | | | |
| NachoMDM (Python) | Y | Y | Y | Y | Y | P | Y | Y | | Y | | | | P | | | | |
| WSO2 and Entgra windows plugin (Java) | Y | Y | Y | Y | Y | P | Y | Y | | | | | | P | Y | | | |
| deploymenttheory/go-sdk-windowscsp (Go) | | | | | Y (fragments) | | Y | Y | Y | Y | Y | Y (./User) | | | | | Y | Y (fake tree) |
| SyncMLViewer (C#) | | | | | C | | | C | | C | | | | | Y (cleanup) | Y | | C |
| pytune (Python) | C | | C | C | C | | C | C | | | | | | | C | | | C |
| AADInternals MDM (PowerShell) | C | | C | C | C | | C | C | | | | C | | | C | Y (CSP decl) | | C |
| Imitune (Python) | C | | C | C | C | | C | C | | C | | | | | | | | C |

No open source project implements WinDC (DeclaredConfiguration) server-side. No open source Go
server implements WBXML. Only local-mdm ships a working WNS client in production code; Fleet built
one and turned it off.

---

## 9. Observations for go-oma-dm

These are conclusions drawn from the evidence above. They are inputs to the implementation plan
and the decision records, not decisions themselves.

### Which source is authoritative for what

| Question | Authority | Runner-up and why it loses |
|---|---|---|
| Enrollment wire protocol (discovery, XCEP, WSTEP, provisioning doc, faults, renewal) | MS-MDE2 v19.0, 2026-08-11 | The Learn enrollment pages (2025-08-04) still show RequestVersion 3.0 samples and lack fault 80180032. |
| Management wire protocol (SyncML shape, commands, alerts, headers, scope) | MS-MDM v15.0, 2024-04-23 | The Learn OMA-DM page (2025-08-04) is newer and consistent but omits the synctype and DevicePrepSync alerts and the HTTP header extensions. |
| Status codes, DTD element order, WBXML tokens | OMA SyncML RepPro 1.2.2 and its DTD | MS-MDM defers to it by reference. |
| Session mechanics (packages, Final, Large Object, MD5 challenge) | OMA DM Protocol 1.2.1 | The 1.2 PDF the Learn page links is editorially identical. |
| CSP tree, formats, applicability, allowed values | DDFv2Feb2026.zip, Last-Modified 2026-02-19 | Per-CSP Learn pages are generated from the same data but lag (DeclaredConfiguration 2025-03-12). |
| WinDC document schema, scenarios, bulk template, user scope | The five `declared-configuration*` Learn pages, 2025-08-04 | The DeclaredConfiguration CSP page and DDF describe only the leaf node and are missing RefreshInterval, BulkTemplate and User scope. |
| Entra registration of a third-party MDM | The Entra integration page, updated 2026-05-11, and the Graph `mobilityManagementPolicy` resource | Fleet's setup guide shows the same steps from the admin's side. |
| Push | Learn push-notification-windows-mdm (2025-08-04) plus the WNS headers page (2025-07-29) | Fleet issue #43773 and Q&A 5907951 are the only real-world evidence on 24H2 and 25H2, and they disagree with each other. |
| What the real client actually sends | SyncMLViewer captures and Fleet's test client | Every spec sample is illustrative; the DTD order and the 1.1 vs 1.2 namespace mix only show up in captures. |

### What the 26H2 target means in practice

- 26H2 is an enablement package on the 24H2 servicing branch. Compliance with 26H2 is compliance
  with build 26100.x plus the current LCU. No CSP or protocol surface is documented as new in 26H2,
  and the February 2026 DDF bundle has no 26300 applicability values.
- The build plan should therefore pin: MS-MDE2 v19.0, MS-MDM v15.0, OMA DM 1.2.1, SyncML 1.2.2,
  DDFv2Feb2026, and re-check all six when 26H2 reaches general availability, when Microsoft ships the
  next DDF drop (historically February, July and September), and when the MS-MDE2 RSS feed changes
  (four revisions in the last twelve months).
- The "What's new in MDM" page stopped at 22H2. Per-version deltas now have to be derived from DDF
  `OsBuildVersion` values, which argues for keeping every DDF drop under version control and
  diffing them, the way go-apple-dm diffs Apple's schema repository.

### What the build must cover that the references do not

- No open source project implements WinDC server-side. The protocol is fully documented on Learn
  (JSON discovery, linked MS-MDE2 enrollment, DeclaredConfiguration CSP, document schema, result
  documents, the 1224 alert), and the Windows client is generic: the primary MDM sets
  `LinkedEnrollment/DiscoveryEndpoint` to any URL. Only the Microsoft-hosted instance
  (`*.dm.microsoft.com`) is Intune-gated. This is the largest differentiator available to
  go-oma-dm, in the same way DDM was for go-apple-dm, and it is the part with the least prior art.
- No open source Go server implements WBXML. Windows defaults to XML and every reference negotiates
  XML, so WBXML can be a later phase, but the DMAcc and w7 `DEFAULTENCODING` hooks must be left open.
- Only local-mdm ships a working WNS client. Fleet built one, proved it wakes devices, and turned it
  off in favour of its agent. go-oma-dm has no agent, so WNS push plus a sane poll schedule is the
  only path to timely delivery.
- Nobody except Mattrax (Rust) and Latchz treats the DDF as a schema source. The same-organisation
  `go-sdk-windowscsp` already parses DDF v2 into typed Go and generates per-CSP packages, which proves
  the approach works. It is a reference, not a component to absorb (decided 2026-09-06); the
  dependency question is deferred until this project's architecture has formed. Whatever the
  answer, the generator must handle the
  caveats found in the bundle inspection: no default namespace, sentinel build values, the `11.0.`
  quirk, dynamic nodes, applicability inheritance, and nine kinds of allowed-value payload.
- Enrollment attestation is available to any MDM that advertises EnrollmentVersion 5.0 or higher:
  the AIK claim, certificate and public key arrive as AdditionalContext items. None of the open
  servers verify them. Verifying the AIK chain with go-attestation would be a genuine improvement,
  and it mirrors go-apple-dm's Managed Device Attestation work.
- Test environments are solved in-house. guestweave on either host gives a Windows 11 guest with a
  TPM 2.0 (the HCS vTPM on Windows, go-sdk-vtpm2 on macOS) and Secure Boot, OOBE skipped, SSH and
  an in-guest agent, snapshots that carry the TPM state, and an HTTP and MCP API to drive it, so
  go-oma-dm's e2e tier can enroll a real client, capture the SyncML, and revert to a clean snapshot
  without any third-party hypervisor. The simulator remains the unit-test client; guestweave is the
  conformance client. Even the AIK attestation claims can be exercised against a vTPM guest, since
  go-oma-dm is the verifier; only vendor EK certificate chain validation needs physical hardware.
- Every open server hand-rolls the SOAP and SyncML XML, and two of them vendor Go's x509 CSR parser
  to survive Windows's PrintableString subjects. The typed message model in Mattrax's Rust crates
  (one type per verb, one crate per protocol) is the design to follow in Go; the vendored CSR parser
  is the pitfall to test for on day one.

### What the references get wrong, so the tests can prove we do not

- Accepting only Add and Replace (Fleet): Exec-only nodes (LinkedEnrollment, Recovery, RemoteWipe,
  Reboot, SCEP Enroll) become unreachable.
- Polling every minute (Fleet): the Audit Policy CSP reapplies destructively each session and the
  server melts at scale.
- Storing the raw SyncML envelope per session (Fleet #44188): the database writer saturates at 10k
  hosts.
- Ordering the command queue by a second-resolution timestamp (Fleet #49616): same-second commands
  arrive in random order.
- Back-dating the client certificate by the renewal period (Fleet #52601) and never implementing
  ROBO renewal (Fleet #52492): certificates expire early and cannot renew.
- Keying enrollments on HWDevID (Fleet #50612): cloned images silently steal enrollments.
- Sending device and user commands in one Atomic before sign-in (Fleet #40713, #48931): the whole
  batch rolls back with 507.
- Regex-parsing SyncML (oscartbeaumont): SessionID and MsgID extraction breaks on any reordering.
- Patching GOROOT to parse the CSR (oscartbeaumont): the build is not reproducible.
- Using CDATA in profile data (Fleet's authoring guide) when the client does not support it: works
  only because Fleet strips it before sending.

### Rules that carry over from go-apple-dm unchanged

- Read at least two references before each feature; record what they do and where they fail in a
  decision record; prove the improvement with a failing-path test.
- Never take a code dependency on Fleet, Mattrax, MDMatador, or their forks; they are read-only
  references under `third_party/refs/`. The `micromdm/plist` exception does not apply here; the
  equivalent candidate exceptions are `smallstep/pkcs7` and `smallstep/scep`, to be decided in a
  decision record.
- Generated code lives only under a schema tier and is produced from a pinned, versioned upstream:
  here the DDF drop, pinned by URL and SHA-256 rather than as a git submodule, because Microsoft
  publishes a zip, not a repository.
- The tier layout (foundation, schema, protocol, PKI, platform services, storage, simulator, server,
  app) maps directly: `schema/` from DDF, `mdmprotocol/` for SyncML and MS-MDM plus a `windc`
  package, `pki/` for the WSTEP CA, XCEP, SCEP and attestation, `appleplatformservices/` becomes
  `msplatformservices/` for WNS, Entra and Graph, and `simulator/` becomes the Windows client
  simulator modelled on Fleet's `mdmtest/windows.go`.

## 10. Not covered and open questions

### Open questions to settle before the plan is written

1. Which EnrollmentVersion to advertise. MS-MDE2 allows 3.0 to 9.0; 5.0 unlocks attestation items and
   9.0 adds `AIKAlgorithm`. The behaviour differences between 5.0 and 9.0 beyond the context items
   are not documented in any fetched source; a capture from a 26H2 client against a server
   advertising 9.0 is needed.
2. Whether Windows 11 honours DMClient `Poll/NumberOfFirstRetries = 0` as "infinite", as Fleet
   assumes. Not independently tested.
3. Whether DMClient push will move from the Partner Center Package SID and `login.live.com`
   credential model to the Entra `login.microsoftonline.com` model that the Windows App SDK now
   requires. The MDM push page (2025-08-04) still documents Partner Center; the WNS overview
   (2026-08-30) calls that flow legacy. No Microsoft statement either way.
4. What Event 4603 (`notificationIdNotRetrieved`) on 24H2 and 25H2 actually means, and whether the
   session always runs despite it. The only source is an unresolved Q&A thread whose author changed
   their conclusion mid-thread.
5. Whether `SYNCML:SYNCML1.1` is required, tolerated, or merely habitual in WinDC traffic. Every
   Microsoft WinDC sample uses 1.1; MS-MDM mandates 1.2. A capture of the Microsoft-hosted linked
   enrollment via SyncMLViewer would settle it.
6. Whether the `ManagementServiceConfiguration/RefreshInterval`, `Host/BulkTemplate` and User-scope
   WinDC nodes exist on a current 26100 build, given that they are documented on the protocol pages
   but absent from the CSP page and the DDF.
7. How go-oma-dm relates to `deploymenttheory/go-sdk-windowscsp`. Decided 2026-09-06: it is proof
   that DDF v2 can drive generated Go, and a reference; it will not be absorbed, and no dependency
   decision is made until this project's architecture has formed. Revisit when the schema tier is
   designed.
8. What a third-party MDM must do to become an "approved MDM app" for the Graph `isCompliant` PATCH,
   and whether that is achievable outside the Intune partner programme. The Entra page states the
   restriction without a process.
9. Whether Microsoft will publish anything new for MDM in 26H2 between Release Preview and GA. Every
   page checked was silent.

Questions 1, 2, 4, 5 and 6 are closed by a guestweave Windows 11 guest (section 5) running
SyncMLViewer against a go-oma-dm test server; none of them needs physical hardware, including the
attestation items behind question 1, which a vTPM guest produces and go-oma-dm verifies.

### Not covered by this document

- Windows 10, HoloLens, Surface Hub, IoT Core, Windows Phone, and Windows Server; only where a page
  could not be separated from Windows 11.
- Generic OMA-DM for telecom and IoT (libdmclient, LwM2M, FUMO), by the scope decision.
- Windows 365 and Azure Virtual Desktop multi-session specifics beyond the MS-MDM 3.2 notes.
- Intune-internal services (MMP-C hosting, EPM agent, device inventory agent, IC3/Trouter) beyond
  what they reveal about the WinDC wire protocol.
- Windows Information Protection and MAM-only enrollment (deprecated July 2022).
- The Microsoft Graph Intune API surface, beyond the two calls named in 1.8.
- Licensing terms of the Open Specifications (Microsoft Open Specification Promise) and whether
  they constrain a Go implementation; to be read before the first commit of protocol code.

### Could not verify

Items the research agents could not resolve on 2026-09-06, kept so nobody re-searches for them.

**Bucket A**

- `mdm/enterpriseappmanagement-csp`: HTTP 404. The legacy Windows Phone EnterpriseAppManagement CSP page appears to have been removed; the certificate-renewal page's "EnterpriseAppManagement" link points at the App-V CSP.
- `diagnose-mdm-failures-in-windows-10`: not a page; it redirects to `mdm-collect-logs`.
- The old `send-notifications-from-a-cloud-service-to-a-windows-app` page: HTTP 404; the content now lives at `wns-overview` and `push-request-response-headers`.
- WAP-192 (WBXML 1.3) PDF: not fetched; only the W3C NOTE (WBXML 1.1) was verified.
- MS-MDE2 section 8 change tracking and Appendix B narrative for v19.0: the PDF's TOC and normative clauses were extracted, but the change-tracking narrative could not be isolated cleanly; the revision number and date come from the landing page and PDF title page.
- MS-MDM 2.2.6.1 (Status element syntax) body text did not extract cleanly; the Learn status table and MS-MDM 3.1.5.2.1 were used instead.
- A published numeric `MaxMsgSize` for the Windows DM client: none found.
- Any Microsoft statement that DMClient push has moved from Partner Center Package SID and `login.live.com` credentials to Entra `login.microsoftonline.com` credentials: none.

**Bucket B**

- `gh api` commit history for `MicrosoftDocs/windows-itpro-docs`: the repository returns 404 (authenticated and anonymous, 2026-09-06); GitHub search finds only forks; last Wayback snapshot 2025-06-14. Upstream `MicrosoftDocs/windows-docs-pr` is private. Learn `gitcommit` and `updated_at` metadata substituted above.
- `https://learn.microsoft.com/en-us/windows/whats-new/whats-new-windows-11-version-26h2`: 404.
- `mdm/policy-ddf-file` and `mdm/configuration-service-provider-reference`: both 301-redirect; no standalone content.
- GA date for 26H2: not announced; only "later this calendar year" (IT Pro blog 2026-06-19).
- Any Microsoft document defining "MMP-C" or expanding "Microsoft Management Platform Cloud": none found on Learn; the string `MicrosoftManagementPlatformCloud` appears only in event-log samples on the WinDC overview page.
- Any 2025 or 2026 content change to the five WinDC protocol pages beyond the 2025-08-04 bulk commit: none detectable from public metadata.
- Community blogs (call4cloud, MSEndpointMgr, Patch My PC, tbone.se): dates and claims recorded as stated on the pages; not independently verifiable against Microsoft sources.

**Bucket C**

- mymdm/mymdm: the marketing sites <https://mymdm.org> and <https://getmymdm.com> describe a Go plus TypeScript, AGPL-3.0, self-hostable MDM with "Windows OMA-DM" and Autopilot, repo at `github.com/mymdm/mymdm`. `gh api repos/mymdm/mymdm` returns 404 and the `mymdm` GitHub user (created 2024-10) has no public repositories. Treat as not publicly available; no code, licence, or activity could be inspected.
- ilius/mattrax: no such repository. The living Mattrax forks with Go code are PhenixH/Mattrax (2022) and mihaicoli/Mattrax (2020).
- akivaron/product-mdm: a fork of manojgunayadev/product-mdm (Apache-2.0, last push 2016); stale WSO2-derived Java, superseded by wso2/product-iots and Entgra; not inspected.
- headwindmdm and multunus/onemdm-server: Android-focused; no Windows OMA-DM code found, excluded.
- Fleet's `NumberOfFirstRetries=0` "infinite 1-minute poll" is confirmed in code, but whether current Windows 11 honours 0 as infinite was not independently tested.
- Fleet's licence shows as NOASSERTION via the API; the MIT core plus proprietary `ee/` split is from the repository's known licensing, not re-verified file by file.

**Bucket D**

- The Typhooncon 2024 slide PDF was fetched as binary and text-extracted locally; the exact presentation date is not on the slides. The dated instance of the talk is Black Hat USA 2023 (2023-08-09).
- The old Learn slugs `azure-ad-and-microsoft-intune-automatic-mdm-enrollment-using-the-new-portal` and `entra/identity/devices/device-management-azure-portal` return 404 or 403; they were superseded by `azure-ad-and-microsoft-intune-automatic-mdm-enrollment-in-the-new-portal` (2025-08-04) and `manage-device-identities` (2026-06-17), both used above.
- The Tech Community device-attestation and device-association posts return title-only to a plain fetch; dates and quotes were confirmed through a rendering proxy (Lior Bela 2024-05-06, Maggie Dakeva 2026-08-27).
- No public source code for Mattrax's current MS-MDE2 or SyncML implementation beyond the retained `rust` branch (section 3).
- No dedicated petervanderwoude.nl post on MMP-C or linked enrollment was found.
