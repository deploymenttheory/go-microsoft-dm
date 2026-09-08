# Windows enrollment investigation

Research date: 2026-09-08. Scope: native enrollment on the Windows desktop,
using the local reference server. Resolved: native enrollment succeeds after
adding initial digest nonces to the reference server's credentials.

## Resolution and verification

The nonce-only change succeeded on Windows 11 Enterprise build 26200.9278.
The reference server now generates 32 cryptographically random bytes and gives
each authentication entry a separate 16-byte portion, encoded as base64 by the
existing provisioning library. Random generation finishes before credentials
are persisted. Library interfaces and optional nonce validation are unchanged.

| Attempt (BST, 2026-09-08) | Result |
| --- | --- |
| 11:41:51, no initial nonces | Event 17, account configuration failed with `0x80070057`. |
| 12:21:35, initial nonces supplied | Events 16, 58 and 72: account configuration, provisioning and enrollment succeeded; four management requests followed. |
| 12:24:24, explicit read-only query | `Get ./DevInfo/Man` acknowledged with status 200 and returned `Gigabyte Technology Co., Ltd.` |
| 12:29:11, enrollment after unregistering the first | Events 16, 58 and 72 succeeded again; management requests followed. |
| 12:44:50, finalized scripts with freshly generated PKI | Native enrollment succeeded again through `scripts/enrollment/host.ps1`. |

APPAUTH ordering, encoding defaults and retry settings were not changed. This
demonstrates that the nonce-bearing response resolves the observed failure; it
does not prove that each of the two nonces is independently necessary on every
Windows version. Both are supplied for compatibility.

The simulation server uses the same `server/service` credential source.
Simulator-driven application tests verify base64 nonce round-tripping and
separate values for CLIENT and APPSRV. The server's e2e suite checks initial
nonces, an authenticated read-only Get, and unenrollment followed by fresh
certificates, secrets and nonces on reenrollment. Service tests cover nonce
generation failure without persisting or returning partial credentials.

Both modules' full tests, server e2e tests, and lint passed. During the test
expansion, repeated full runs exposed an authentication challenge
loop caused by parallel application tests sharing SQLite's named in-memory
database and overwriting the same device's credentials. The application test
helper now gives each test its own temporary SQLite database. Twenty repeats
of the application package passed after that correction, followed by the full
server suite, expanded e2e suite, and lint including e2e tests. Race testing
remains unavailable with the host's CGO-disabled toolchain.

Reusable desktop scripts and instructions are in
[scripts/enrollment](../../scripts/enrollment/README.md). Generated certificates,
passwords, databases and binaries remain under ignored `tmp/`; they are not
part of the reusable scripts. The scripts preserve timestamped server logs,
use the full username, select the test enrollment by provider and localhost
URL, and remove only the generated root identified by its ownership marker.

The finalized scripts also completed the read-only manufacturer query with
status 200. Cleanup was verified: the device reports `Registered = False`,
both temporary validation roots are absent from LocalMachine/Root, and the
local test servers are stopped. Runtime evidence remains in the ignored
`tmp/host-validation` and `tmp/enrollment` directories.

## Observed failure

The test username is `host-validation@example.com`. At 11:41 BST, discovery,
certificate policy and certificate enrollment each returned HTTP 200. Windows
reported that it parsed the certificate enrollment response, then logged
Enrollment event 17 at 11:41:51:

```text
OMA-DM client configuration failed. RAWResult: (0x80070057)
Result: (Provisioning failed, but a specific CSP is not indicated.).
```

The UI reported `0x80180023`. No management request appears in this attempt's
server log. This places the observed failure after enrollment authentication
and before the first management exchange; it does not identify a particular
invalid provisioning parameter.

Earlier attempts failed with event 56 naming `RootCATrustedCertificates`
(`0x83750005`). Correcting that CSP's root tree alone did not resolve the
bootstrap failure. Omitting the extra top-level characteristic from the
reference server response got past that parser error. The root certificate
remains in `CertificateStore/Root/System`.

Local evidence is in the Windows
`Microsoft-Windows-DeviceManagement-Enterprise-Diagnostics-Provider/Enrollment`
event channel and ignored `tmp/host-validation/server-stderr.log`. Server logs
are replaced on restart; the times above identify the specific attempt.

## Intune and its sidecar

Microsoft describes the Intune Management Extension as an agent installed on
enrolled devices when supported workloads are assigned. That makes its
installation a later stage than the current failure. The standard executable
path was checked on this host and was not present; no installer was run.
[Microsoft IME documentation](https://learn.microsoft.com/en-us/intune/device-management/tools/management-extension-windows)

The AADInternals `Enroll-DeviceToMDM` implementation sends a token-authenticated
enrollment request, decodes the returned provisioning document, and extracts
certificates. Its SyncML helper uses a client certificate for HTTPS requests.
It does not apply the returned provisioning document through the native Windows
enrollment client. Consequently, this source is useful for message flow but
cannot demonstrate that our account configuration is accepted by Windows.
It is historical client code, not a captured current Intune provisioning response.
[AADInternals source](https://github.com/Gerenios/AADInternals/blob/master/MDM_utils.ps1)

No successful current Intune provisioning capture was obtained in this research.
Microsoft's published enrollment examples are protocol examples, not evidence
of the exact values Intune currently sends.

## Fleet comparison

Reference: Fleet commit `770aab0f49b33f515d7b8b7ae092bbc870bd4e6f`,
`server/service/microsoft_mdm.go`, particularly
`NewApplicationProvisioningData` and `NewProvisioningDoc`.
[Pinned Fleet source](https://github.com/fleetdm/fleet/blob/770aab0f49b33f515d7b8b7ae092bbc870bd4e6f/server/service/microsoft_mdm.go)

| Setting | Fleet | Current reference server | Assessment |
| --- | --- | --- | --- |
| Top-level characteristics | CertificateStore, APPLICATION, DMClient | Same after the bootstrap correction | Extra root CSP rejection was observed on this host. |
| Digest `AAUTHDATA` | Present for CLIENT and APPSRV; literal `nonce` | Omitted for both | First isolated compatibility experiment. Not a proven cause. |
| APPAUTH order | CLIENT, APPSRV | APPSRV, CLIENT | A difference, but no ordering requirement established. |
| DEFAULTENCODING | Explicit SyncML XML | Omitted | Microsoft documents XML as the default. Lower priority. |
| Retry parameters | Explicit values and BACKCOMPATRETRYDISABLED | Defaults | No evidence these explain account creation failure. |
| SSLCLIENTCERTSEARCHCRITERIA | Omitted | Omitted | Not a difference between these implementations. |

Fleet comments that it challenges the first management request with a fresh
nonce. Its literal initial value should not be treated as an example of
base64-encoded random bytes. The library already supports supplying raw nonce
bytes and encoding them as base64.

## What Microsoft requires

Both APPSRV and CLIENT credential characteristics must be supplied. CLIENT
authentication must use DIGEST. `AAUTHDATA` is documented as optional and,
when supplied, carries a base64-encoded nonce. Its omission is therefore not
by itself evidence of a specification violation. No mandatory nonce rule
should be added to library validation based only on Fleet's choices.
[MS-MDE2 section 2.2.9.5](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/426201f4-2c7f-4ffe-843d-0deb3fad0a09)

Windows maps w7 provisioning into a DMAcc account. Its authentication data
node is binary and stores the next nonce. This explains why comparing the
account's authentication parameters is relevant, but does not establish which
parameter produced `0x80070057`.
[DMAcc CSP](https://learn.microsoft.com/en-us/windows/client-management/mdm/dmacc-csp)

Microsoft's Intune troubleshooting article associates `0x80180023` with a
missing `dmwappushservice` when the underlying error is `0x800706D9`.
On this host the service exists, is stopped with Manual startup, and the
underlying error is `0x80070057`. The documented missing-service remedy does
not match the observations. No service or registry changes were made.
[Microsoft troubleshooting](https://learn.microsoft.com/en-us/troubleshoot/mem/intune/comanage-configmgr/troubleshoot-co-management-auto-enrolling#a-microsoft-entra-hybrid-joined-windows-10-device-fails-to-enroll-in-intune-with-error-0x800706d9-or-0x80180023)

## Experiment selected before the fix

Provision explicit, independently generated initial digest nonces in the local
test configuration, keeping the remaining response identical. Retry native
enrollment with the same full username and collect event 17 and management
request evidence. If the same failure remains, do not retain the change as a
claimed fix. Compare APPAUTH ordering and explicit XML encoding separately.

A passing simulator test is insufficient evidence of native enrollment success.
Keep optional protocol settings optional and distinguish host compatibility
findings from library requirements.

## Desktop conformance

The remaining desktop conformance experiments are recorded in [Windows host conformance](../testing/windows-host-conformance.md): enrollment versions 3/5/9, zero-retry polling, namespace acceptance, capability probes, large-result aborts and sanitized fixtures. The session engine now rejects reuse of previous-enrollment session authentication. Other-build, signed-out and successful native-chunk evidence remains unavailable.
