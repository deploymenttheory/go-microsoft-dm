# 0017: Conformance testing on a Windows host

## Context

The user replaced the planned guest environment with the local Windows desktop.
Phase 7 needs native protocol evidence while keeping experimental choices out
of the library and preserving a usable working device.

## Decision

Use `scripts/enrollment` with a localhost reference server, disposable PKI and
a domain-qualified test username. Select the build-tagged `server/e2e/host`
recorder explicitly for experiments. Capture exact request and response bodies
privately; commit only reviewed sanitized fixtures and context summaries.
Use read-only Gets as the desktop management exit criterion and native
unenrollment to finish. Do not reboot or sign out the working user for coverage.

The recorder's enrollment-version, polling and response-namespace overrides
are test controls, not library policy. Production behavior changes only for
confirmed protocol defects, including enrollment-bound session isolation.

## Constraints

Results apply to the tested build and enrollment. Unsupported probes and aborted
large transfers are recorded as observations. They cannot establish linked
WinDC support, successful native chunking or other Windows versions. Physical
TPM data is retained privately, without claiming attestation verification.

## Verification

[Procedure and results](../../testing/windows-host-conformance.md) describe the
26200.9278 experiments and explicit outstanding evidence. Sanitized SyncML
fixtures round-trip through the codec; a simulator comparison checks package-one
semantics, and server e2e tests require fresh authentication after reenrollment
even when an old session remains unfinished.
