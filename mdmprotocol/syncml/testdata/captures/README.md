# Native Windows captures

Captured on Windows 11 Enterprise 25H2 build 26200.9278 on 2026-09-08 using
the localhost reference server's opt-in host recorder. `manifest.json` records
timestamps, experiment settings and each fixture's role. The namespace 1.1
response is server output; other XML fixtures are native client requests.

`scripts/enrollment/export-captures.py` preserves XML structure while replacing
device IDs, endpoint, manufacturer/model, credential names, digest values and
nonces. Enrollment SOAP is not published: `enrollment-context.json` contains
only ordered names, lengths and selected nonidentifying values. Raw credentials,
certificates and attestation are retained only in ignored local test state.

The two large-result fixtures show aborts, not successful chunked uploads.
There is no native LoginStatus `none` fixture or second-build comparison.
See [results](../../../../docs/testing/windows-host-conformance.md) for the
observations and their limits. These captures are codec and behavior evidence,
not replayable authentication credentials.
