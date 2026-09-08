# WNS push validation on the Windows desktop

Use the [native enrollment procedure](../../scripts/enrollment/README.md).
All device work targets the local test enrollment. Set the WNS environment
before `Start` so both enrollment provisioning and `dmctl` use the same identity.
Keep secrets in the process environment or a private local credential loader.

| Variable | Meaning |
|---|---|
| DM_WNS_PFN | Package family name provisioned under DMClient/Provider/Push/PFN. |
| DM_WNS_AUTH | `legacy` (default) or `entra`. |
| DM_WNS_CLIENT_ID | Partner Center package SID for legacy; application client ID for Entra. |
| DM_WNS_CLIENT_SECRET | Secret for that identity. |
| DM_WNS_TENANT_ID | Required only for Entra. |

The PFN, SID and secret must belong to the same registered package identity.
Microsoft's MDM guidance specifies the legacy flow; the Entra source is
implemented separately for the plan's alternate flow and is not native DMClient
compatibility evidence. PFN alone can be configured to test channel registration
without enabling server-side delivery. With no WNS configuration, enrollment
and polling retain their existing behavior.

1. Run `Setup`, `Start -Capture`, elevated `Trust`, then native `Enroll`/`Submit`.
2. Complete a management session and inspect `host.ps1 -Action PushState -DeviceID ...`.
   `has_channel` must be true. Channel metadata excludes the capability URI;
   Status 0 indicates registration success. Other statuses describe device-side
   failures; consult the pinned DMClient CSP description.
3. Queue the harmless manufacturer Get with `host.ps1 -Action Query`.
4. Run the push test from the repository root, using the same state directory:

```powershell
./scripts/enrollment/test-push.ps1 -DeviceID '<test-device-id>' -StateDirectory ./tmp/enrollment
```

The script sends through `dmctl push`, then waits up to two minutes for a newer
authenticated check-in. It does not invoke manual Sync. It saves WNS metadata,
session observation and Event 4603 record IDs/times in the ignored state folder.
Event-read failures are recorded separately from an empty event list. Review
the test enrollment's polling task times before attributing a session to push:
a coincident scheduled poll makes causation inconclusive. An accepted WNS
request alone does not pass native validation. Inspect `Results` for the queued
Get and retain the matching capture timestamps.

For the opt-in Go harness, set `HOST_WNS_STATE_DIR` to an absolute state path and
`HOST_WNS_DEVICE_ID` to the enrolled test device ID, keep the same `DM_WNS_*`
environment, and run from `server`:

```powershell
go test -count=1 -tags host -run TestNativePush -v ./e2e/host/...
```

It skips when the required environment is absent. After testing, use elevated
MTA `Unenroll`, verify registration is false, then `Cleanup`.

## Poll fallback and operational checks

The provisioning document keeps the existing Microsoft default schedule. No
one-minute schedule is introduced. `dmctl checkins 24h` logs and prints active
enrollments with no authenticated activity in the last day; choose the threshold
and schedule this command in your existing operations system. Repeated runs can
emit repeated alerts. This does not diagnose the local cause of missing activity.

On Windows, inspect `Get-Service dmwappushservice` and the matching enrollment's
PushLaunch/PushRenewal tasks when native pushes fail. Do not modify unrelated
enrollments or remove polling to force an outcome. During implementation the
desktop service existed with Manual start and was stopped; this snapshot alone
does not establish a fault in a trigger-start service.

## Implementation validation

Local tests cover both token flows, raw delivery, errors/retries, persistence,
reenrollment isolation, missed-check-in alerts and a simulator-triggered session.
Real WNS delivery was skipped because credentials were not configured. Q3
(which credential flow works with this native DMClient identity) and Q4 (Event
4603 versus session arrival) therefore remain open for the credentialed run.
