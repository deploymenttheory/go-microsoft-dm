# Native Windows enrollment validation

Run from the repository root with PowerShell 7 and Go installed. These scripts
exercise the reference server on the Windows desktop. No VM or Intune sidecar
is required. The simulation server uses the same server implementation.

`host.ps1` consolidates the setup, server, trust, enrollment-status, query, sync,
unenrollment and cleanup scripts used during desktop testing. `dialog.ps1`
handles the native enrollment form. `_setup/main.go` generates disposable PKI
and a random password using the repository's existing test PKI helper.

Runtime files go to ignored `tmp/enrollment`: certificates, private keys,
credentials, SQLite database, binaries, PID and timestamped server logs. Keep
these files out of commits. The test root is short-lived; use a new state
directory for a later run after it expires. `Setup` refuses to overwrite an
existing nonempty directory. All actions accept `-StateDirectory` when using
a different directory; supply that same directory on every action.

## Start and enroll

In a normal PowerShell window:

```powershell
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Setup
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Start
```

The default username is `host-validation@example.com`. To change it, pass
`-UserName user@domain.com` to `Setup`; use a full domain name. The server binds
only to `127.0.0.1:8443`. `Start` builds both `dmserver` and `dmctl` from the
current source. Run `Stop` before starting a rebuilt server; earlier logs are
retained.

In an **elevated** PowerShell window, install the generated test root:

```powershell
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Trust
```

Back in the normal desktop session:

```powershell
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Enroll
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Submit
```

Wait for the form before running `Submit`. If Windows initially asks for the
email and server URL, `Submit` fills that page; run it again when the password
page appears. On that page it fills the matching username and generated
password without printing the password. UI automation expects the English
“Microsoft account” window and the control IDs tested on Windows 11. Inspect
the form if Windows changes its UI:

```powershell
pwsh -NoProfile -File ./scripts/enrollment/dialog.ps1
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Status
```

Enrollment success requires `Registered = True`, Windows events 16, 58 and 72,
and subsequent management requests in the timestamped server log. Status shows
the most recent matching events from the last hour; correlate their timestamps
with the current attempt. HTTP 200 from the enrollment endpoint alone does not
prove that Windows accepted the provisioning response.

## Read-only management query

Get the active device ID from the local server database:

```powershell
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Results
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Query -DeviceID '<active-device-id>'
```

`Query` queues only `Get ./DevInfo/Man`. In an elevated window, trigger the
matching local test enrollment's polling task:

```powershell
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Sync
```

Then inspect the response:

```powershell
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Results -DeviceID '<active-device-id>'
```

Expect the Get command's status 200 and a manufacturer value. Automatic initial
device-information reads may also appear in the result list.

## Repeat and clean up

In an elevated window:

```powershell
pwsh -Mta -NoProfile -File ./scripts/enrollment/host.ps1 -Action Unenroll
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Status
```

Unenrollment uses the native Windows API and requires MTA threading. It selects
exactly one enrollment matching both `go-microsoft-dm-local-test` and
`https://localhost:8443`. It refuses ambiguous matches. For a repeat enrollment,
leave the test server and root in place, verify `Registered = False`, and run
`Enroll` and `Submit` again.

After the final unenrollment, in an elevated window:

```powershell
pwsh -NoProfile -File ./scripts/enrollment/host.ps1 -Action Cleanup
```

Cleanup verifies the device is no longer managed, removes only the test root
matching the generated certificate and saved thumbprint, and stops the server
whose executable matches this state directory. It retains the database and logs
for review. To stop the server without changing certificate trust, use `Stop`.

## Automated tests

The desktop steps are separate from simulator tests, which do not enroll or
change the Windows host:

```powershell
go test -count=1 ./...
Push-Location server
go test -count=1 ./...
go test -count=1 -tags e2e ./e2e/...
Pop-Location
```

The e2e suite verifies nonce-bearing enrollment, an authenticated read-only Get,
and unenrollment followed by fresh certificates, secrets and nonces on
reenrollment. [Observed desktop results](../../docs/research/windows-enrollment-investigation.md)
record the native compatibility fix and its limits.
