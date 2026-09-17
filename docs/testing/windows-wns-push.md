# WNS push validation on the Windows desktop

Use the [native enrollment procedure](../../scripts/enrollment/README.md).
All device work targets the local test enrollment. Set the WNS environment
before `Start` so both enrollment provisioning and `dmctl` use the same identity.
Keep secrets in the process environment or a private local credential loader.

For an isolated guestweave Windows test, use a headed Windows 11 VM with a NAT
NIC for WNS and a host-only NIC for host access. `weave net check <vm>` must
confirm the guest's NAT address and default route before enrollment. Install
PowerShell 7 and Go 1.27 in the guest, and run the repository, test server,
`dmctl` and the scripts below **inside the guest**: the native harness binds
`https://localhost:8443`, which means localhost must be the enrolled device.
Set the WNS environment in the guest session before `Start` and retain that
same identity for `Push` and `test-push.ps1`.

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

To obtain the legacy identity, register for the Windows developer program in
Partner Center, create an MSIX or PWA app under Apps and games, and reserve its
name. Product Identity shows the PFN and Package SID. Open that product's
WNS/MPNS App Registration portal to create the matching client secret; an
unrelated Entra app registration does not identify the same Store package.
An EXE or MSI app draft's Partner Center ID is not the Package SID and its
Application identity panel does not supply the PFN needed by DMClient.
Follow [Microsoft's MDM push setup](https://learn.microsoft.com/en-us/windows/client-management/push-notification-windows-mdm)
and [WNS credential walkthrough](https://learn.microsoft.com/en-us/azure/notification-hubs/notification-hubs-windows-store-dotnet-get-started-wns-push-notification).

For the documented MDM flow, set the three values in one guest PowerShell 7
session before `Start`, and run `Push` or `test-push.ps1` from that session too:

```powershell
$env:DM_WNS_PFN = '<PFN from Product Identity>'
$env:DM_WNS_CLIENT_ID = 'ms-app://<Package SID from Product Identity>'
$env:DM_WNS_CLIENT_SECRET = Read-Host -Prompt 'WNS client secret' -MaskInput
```

The secret prompt masks input and does not add the value to command history.

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
The headed Windows 11 VM enrolled with the Partner Center PFN
`DeploymentTheory.WeaveDeviceManagement_35ma6q3bna5z2`. Its native session
reported Push/Status 0 and a ChannelURI, and the server recorded `has_channel`
true. A queued read-only `./DevInfo/Man` Get completed with status 200.
The first credentialed native push could not authenticate: the legacy token
endpoint returned HTTP 400. A separate request from the guest returned
`invalid_request` with the description "Client credential flows against
login.live.com are no longer supported. New clients should use
login.microsoftonline.com instead." The same secret with the bare Package SID
returned `invalid_client` / "Invalid client id". No WNS delivery occurred.
The Partner Center linked app registration is Microsoft account only. With its
Application ID and a newly created secret, the tenant-specific Entra token
request returned `AADSTS9002346`, directing this client to `/consumers`.
The `/consumers` request for `https://wns.windows.com/.default` returned
`AADSTS9002332`: WNS is configured for Azure Active Directory users only.
The alternate `https://api.wns.windows.com/.default` resource was not found
in `/consumers`. No token or push was issued. These results are specific to
the new Store identity and current service state. Microsoft documents Entra
WNS tokens for Windows App SDK apps, but its channel identity uses the app ID
instead of a Partner Center PFN; that documentation does not establish a way
to authenticate this PFN-based DMClient channel. Q3 (which credential flow
works with this native DMClient identity) and Q4 (Event 4603 versus session
arrival) remain open.

An attempted change of the Partner Center linked app registration from
`PersonalMicrosoftAccount` to `AzureADandPersonalMicrosoftAccount` was rejected
by the Azure manifest editor. It required the registration's `ms-app://<Package
SID>` identifier URI to use a verified organizational domain or supported
multitenant format. The change was discarded; the portal again showed the
original `PersonalMicrosoftAccount` audience. Replacing the Package SID URI
was not attempted because its Store linkage and DMClient channel semantics
would need Microsoft confirmation.
