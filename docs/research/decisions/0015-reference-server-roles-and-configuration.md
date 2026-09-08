# 0015: Reference server roles and configuration

## Context

The library is transport- and storage-neutral: the enrollment and session engines leave the CA,
the credential policy, persistence and HTTP to the caller. Phase 6 provides that caller, an
installable `dmserver`, so the protocol work can be run and, in Phase 7, pointed at a real
Windows guest. It has to compose the engines over a store, expose the four MS-MDE2 and MS-MDM
endpoints, and be configured without code.

## Decision

`server/service` composes an `enroll.Service` and an `mdm.Service` over one store and one CA. It
supplies the seams the engines leave open: a `StaticAuthenticator` for on-premise enrollment
credentials (a username-to-password map, or accept-any for simulated devices); an `Issuer` from
`pki/wstep` over `pki/ca`; a `Provisioner` whose `CredentialSource` mints the device's OMA DM
account credentials and, crucially, persists the APPSRV secret's `H(name:secret)` to the
credential store so the management session can verify the device with `syncml:auth-md5`; a
`Recorder` that stores the enrollment and logs an HWDevID conflict as an event rather than
refusing it; an `mdm.Authenticator` over the enrollment and credential stores; and `Hooks` that
write the package-1 device facts to the store and route generic alerts, client events and
user-initiated unenrollment to the event log, revoking the certificate on unenrollment.

`server/httpapi` mounts the enrollment handler on the discovery, policy and enrollment paths and
the management handler on the management path, with an optional access log. The handlers already
answer the discovery GET probe, set Content-Length and never chunk. `server/internal/app` reads
`DM_*` configuration, opens the store, builds or loads the CA and returns the composed handler.
`dmserver` is a thin wrapper that listens (TLS when `DM_TLS_CERT`/`DM_TLS_KEY` are set, otherwise
plain HTTP behind a TLS-terminating proxy) with graceful shutdown.

Configuration is `DM_*` (as CLAUDE.md requires): `DM_STORE` (sqlite, postgres, mysql, or memory,
which is an ephemeral in-memory SQLite), `DM_DSN`, `DM_BASE_URL` (the externally reachable https
base, from which the endpoint URLs and the provisioning document's management URL are derived),
`DM_LISTEN`, `DM_TLS_CERT`/`DM_TLS_KEY`, `DM_CA_CERT`/`DM_CA_KEY` (a generated ephemeral CA when
unset, with a warning), `DM_PROVIDER_ID`, `DM_NAME`, `DM_ROLE` (all or mdm; windc joins in Phase
12), `DM_ENROLL_USERS` or `DM_ENROLL_ALLOW_ANY`, and `DM_DISABLE_SCHEMA_VALIDATION`. Enrollment
responses always carry Content-Length; no endpoint checks the User-Agent or a fixed host.

## Rationale

Keeping every seam in one composition package means the engines stay free of storage and policy
and the whole server is one `service.New` call the binaries and tests share. Persisting the
APPSRV credential at provisioning time is the join that makes enroll-then-manage work end to end:
the same secret the provisioning document hands the device is what the session verifies. Logging
the HWDevID conflict rather than refusing it honours the pitfall's guidance that the same
hardware may legitimately re-enrol. Deriving every URL from `DM_BASE_URL` keeps the provisioning
document pointing back at the right server behind a proxy, and lets a test run the whole server on
an ephemeral port. Generating an ephemeral CA when none is configured makes the server runnable in
one command for a demo while warning that it is not durable.

## Constraints

The reference server is single-node and single-tenant. `DM_ROLE` all and mdm expose the same
endpoints today; the windc role and its endpoints arrive in Phase 12. There is no admin API in
this phase (adminauth is Phase 15); `dmctl` operates on the store directly. The generated CA is
not persisted; a real deployment sets `DM_CA_CERT`/`DM_CA_KEY`. The static authenticator is a
reference credential policy, not an identity provider; Federated and Certificate enrollment are
Phase 10.

## Verification

`server/internal/app` has an end-to-end test that enrols a simulated device against the composed
server over TLS, drives a session that delivers a queued policy, records device facts and events,
and unenrols; plus configuration-loading tests for every `DM_*` variable and its errors.
`server/service` tests cover `New`'s validation and the `StaticAuthenticator`; `server/httpapi`
tests cover routing, the access log and the discovery probe. `server/e2e` runs the enrol, session,
reboot, policy, chunked-Get and unenroll scenarios against SQLite and the in-memory role.

## References

- [server/service](../../../server/service), [server/httpapi](../../../server/httpapi), [server/internal/app](../../../server/internal/app), [server/cmd/dmserver](../../../server/cmd/dmserver)
- [Decision record 0014](0014-sql-storage-backends.md), [decision record 0008](0008-enrollment-protocol-and-provisioning-document.md), [decision record 0011](0011-management-session-engine.md)
- Microsoft, on-premises authentication device enrollment: <https://learn.microsoft.com/en-us/windows/client-management/on-premise-authentication-device-enrollment>
