# Windows desktop conformance

Phase 7 uses the local Windows device, as requested, with the reusable
[enrollment scripts](../../scripts/enrollment/README.md). The server and simulator
share the production application; a `host` build adds HTTP-body recording and
explicit experimental response overrides. Run `Start -Capture` to select it.
Use a fresh state directory and the same `-StateDirectory` on every action.

## Procedure

1. Run `Setup`, `Start -Capture`, and elevated `Trust`.
2. Run `Enroll` and `Submit` in the desktop session. The username needs a domain,
   for example `host-validation@example.com`. Verify registration and fresh events.
3. Queue `Query` or `Probe` with the active device ID, run elevated `Sync`, and
   inspect `Results`. `Probe` queues only read-only device-version, CSPVersions
   and DeclaredConfiguration queries.
4. Unenroll using elevated MTA PowerShell. Confirm registration is false before
   repeating enrollment. Finally run elevated `Cleanup` to remove the generated
   trust root and stop the test server. Keep the private evidence locally.

`Start -Capture` creates `experiment.json` in the state directory. It is read
on each request. `{ "label": "baseline" }` preserves application response bytes.
Optional fields are `enrollment_version` (`3.0`, `5.0`, `9.0`), `namespace`
(`SYNCML:SYNCML1.1` or `SYNCML:SYNCML1.2`), `max_msg_size`, `first_retries` and
`first_interval`. Enrollment and polling overrides require reenrollment;
namespace and message-size overrides can change between sessions. The harness
disables schema validation to allow the fixed capability probes. These switches
are excluded from the production server and do not change library defaults.

Raw bodies and metadata are in the ignored state directory's `captures` folder.
They contain credentials, certificates and device identifiers. Export selected
sanitized evidence with Python 3, then review it before committing:

```powershell
python ./scripts/enrollment/export-captures.py ./tmp/phase7/captures ./mdmprotocol/syncml/testdata/captures
go test ./mdmprotocol/syncml ./simulator
Push-Location server
go test -count=1 -tags host ./e2e/host/...
go test -count=1 -tags e2e ./e2e/...
Pop-Location
```

`make test-conformance-host` runs the offline capture, simulator and recorder
checks. It does not enroll the machine automatically. Native UI interaction and
elevated Windows operations follow the procedure above.

## Results: 2026-09-08, Windows 11 Enterprise 25H2, 26200.9278

| Experiment | Observed result |
|---|---|
| Native enrollment, authenticated management and unenrollment | Passed repeatedly; manufacturer Get returned 200; native user-request unenrollment emitted Alert 1226. |
| Advertised enrollment version 3.0 | RequestVersion 9.0; AttestationResultNoAttestation, no AIK algorithms provided, HRESULT 0x80070057. |
| Advertised enrollment version 5.0 | RequestVersion 5.0; AIKPub (380 characters) and AIKAttestationClaim (1624 characters); enrollment succeeded. |
| Advertised enrollment version 9.0 | RequestVersion 9.0; same no-attestation status as advertised 3.0; enrollment succeeded. |
| First retries 0, first interval 1 | Scheduled task repetition PT1M with no duration; successive automatic polls observed at 14:08:01, 14:09:01, 14:10:01 and 14:11:01 UTC. This demonstrates an unbounded schedule, not infinite elapsed observation. |
| Server namespace 1.1 versus 1.2 | Both accepted for manufacturer Get (200). Namespace-only experiment retained VerDTD 1.2 and VerProto DM/1.2. Client continued to send namespace 1.2. |
| SwV and DeviceManageability/Capabilities/CSPVersions | Both 200; software version 10.0.26200.9278; capability XML retained in the sanitized result fixture. |
| DeclaredConfiguration RefreshInterval, Host/BulkTemplate and user root | Each returned 406. This establishes unavailability to this primary enrollment, not absence on Windows or behavior under linked enrollment. |
| MaxMsgSize 2048 and 8192 with large CSPVersions result | Get status 200 but no large Results; client aborted with Alert 1223. No MoreData upload observed. Native DevDetail/LrgObj was false. |

The enrollment context summary preserves ordered item names, repeated MAC item
counts, value lengths and selected nonidentifying values. Raw physical-TPM
attestation stays local for Phase 11; its presence does not prove validation.

Native package 1 starts CmdIDs at 2, uses DevInfo DmV 1.3 and omits header Meta.
The simulator starts CmdIDs at 1 and allows explicit DevInfo and size settings;
the comparison test verifies matching identity, version, login and command
semantics with native settings. CmdIDs are correlation identifiers. Native
captures also contain repeated Results in successive messages; fixtures retain
the observed representation rather than imposing a byte-for-byte simulator match.

Repeated enrollment exposed stale session state keyed by device/session only.
The engine now compares enrollment identity before reusing a session. Unit tests
reproduce the rejected new MsgID 1 and possible reuse of authenticated old state;
the server e2e test seeds an unfinished old session and requires fresh authentication.

## Explicit limits and follow-up

Only build 26200.9278 was available. The 26100/26300 CSPVersions comparison
remains unverified. LoginStatus `none` needs a signed-out session and was not
captured on this working desktop; synthetic tests retain coverage. Successful
native chunking was not demonstrated; the two abort captures are evidence of the
actual result. Reboot Exec was replaced with a harmless Get for desktop testing.
Event 4603 and push behavior belong to Phase 8, attestation to Phase 11, and
linked-enrollment WinDC behavior to Phase 12. None of these are claimed as passed.
