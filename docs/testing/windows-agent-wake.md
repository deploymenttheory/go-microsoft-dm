# Windows agent wake workflow

Windows can start an OMA-DM session when a local process runs
`deviceenroller.exe /o <EnrollmentGUID> /c`. Fleet uses this mechanism through
its [fleetd agent](https://github.com/fleetdm/fleet/issues/46567). The
[agent wake script](../../scripts/enrollment/agent-wake.ps1) scopes the call
to one active enrollment with the expected provider and discovery URL.
Therefore the task cannot wake another MDM provider on the same device.

The reference server exposes `GET /agent/wake?device_id=<DeviceID>` over
HTTPS. It authenticates a per-enrollment bearer token and returns the newest
outstanding user command ID as `wake_id`, or an empty value when no wake is
needed. The token is random, stored as a SHA-256 digest, rotated by
`dmctl agent-token <DeviceID>`, and invalidated when the active enrollment
serial changes. The endpoint returns no command body or credential material
and disables caching. The agent must trust the server TLS certificate.

The [polling script](../../scripts/enrollment/agent-poll.ps1) requests this
signal and invokes the local wake script when `wake_id` is present. It
throttles repeat wakes for the same command to two minutes, then retries while
the command remains outstanding. The
[installer](../../scripts/enrollment/install-agent.ps1) copies both scripts
and a JSON config into `%ProgramData%\go-microsoft-dm\agent`, restricts the
directory to SYSTEM and administrators, and registers a SYSTEM task that polls
once per minute. The token is read from the config file, never from task
arguments.

## Provisioning

1. Enroll the device, then run `dmctl agent-token <DeviceID>` against the
   server store. Capture its one-time output securely. Rotating this token
   invalidates any previous agent config immediately.
2. On the Windows device, create a JSON file with `ServerURL` (the HTTPS
   server base URL), `DeviceID`, `Token`, `ProviderID`, and `DiscoveryURL`
   (the exact enrollment discovery base URL). Keep the token out of shell
   history, logs, and command arguments.
3. In elevated PowerShell 7, run
   `./scripts/enrollment/install-agent.ps1 -ConfigPath <config-file>`.
   Protect or remove the original config copy after installation.
4. Queue a harmless Get and check its result. The Windows task named
   `go-microsoft-dm agent wake` should run, followed by `Schedule to run
   OMADMClient by client` under the matching EnterpriseMgmt enrollment.
   Confirm that the server receives the session and command result before
   adjusting the Windows poll schedule.

This endpoint is a lightweight agent poll, not WNS or a persistent push
connection. It makes one HTTPS request per device per minute; operators must
size the server for that rate. The existing Windows MDM poll schedule stays
active as the fallback. Agent loss, invalid tokens, or network outages can
delay delivery until that fallback poll. The installer is a Windows
PowerShell deployment path; it does not distribute or upgrade the agent
automatically.

## Native validation

The headed Windows VM completed read-only `./DevInfo/Man` Gets after local
`deviceenroller` wakes. The client-triggered task and server session occurred
together while the next scheduled poll was about five minutes away. The
authenticated agent retrieved a pending `wake_id` from the HTTPS endpoint,
invoked the wake, and received a status 200 command result. The installed
SYSTEM task also processed a queued Get without manual invocation: its
nonsecret marker recorded the command ID, and the server captured the matching
session and status 200 result. After consolidation onto the enrollment server's
HTTPS port, the installed task processed another read-only Get (`cmd-35`) and
the server recorded status 200.

`deviceenroller.exe` returned `0x82AC0008` despite successful task execution
and command delivery. Its process exit is therefore not a delivery verdict.
Task execution, server captures, and command results establish success.
