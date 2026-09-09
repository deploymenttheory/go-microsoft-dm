# 0011: Management session engine

## Context

Once a device is enrolled it opens OMA DM sessions to the management endpoint: package 1 with
its device information and a session-type alert, package 2 with the server's commands, package
3 with the client's statuses and results, package 4 with more commands, and so on until a side
sends an empty package. MS-MDM is the Windows subset of OMA DM 1.2.1; the two disagree on one
sentence (MS-MDM 1.3.1 says "OMA-DM version 2.1 or later", which contradicts the VerProto
DM/1.2 everywhere else in the document and is a typo for 1.2.1). The engine has to authenticate
each session, keep the message and session identifiers straight, honour the flow-control and
large-object rules, route the alerts the client raises, and never do the destructive things the
reference implementations did: an aggressive poll that reapplied policy every minute, a SyncML
engine that only understood Add and Replace, storing the whole envelope per session.

## Decision

`mdmprotocol/mdm.Service.Handle` processes one request and returns the reply, over interfaces the
storage tier implements (`Authenticator`, `CommandQueue`, `SessionStore`). It decodes and
validates the message with `mdmprotocol/syncml`, finds or opens the `Session` keyed by DeviceID
and SessionID, and enforces the sequence rules: a message for an unknown session must be MsgID 1
and must be package 1 (a 1200 or 1201 alert and the DevInfo Replace); a message for a known
session must carry the next client MsgID. The server's own MsgID starts at 1 and increments per
reply. The reply answers the SyncHdr first, then every command the client sent.

Application-layer authentication is remembered once it succeeds within a session. TLS
certificate authentication is checked on every request, including when a session continues on a
new connection. `Transport.VerifiedChains` comes from Go's TLS connection state; only chains
whose leaf equals the current peer reach `Authenticator.TrustCertificate`. The listener must
use `tls.VerifyClientCertIfGiven` or `tls.RequireAndVerifyClientCert` with `ClientCAs` containing
only enrollment CA roots. `RequestClientCert` does not establish certificate trust. The default
storage authenticator matches both the leaf's serial and SHA-1 thumbprint to the active
enrollment. The thumbprint is an additional binding of an already verified certificate, using
the enrollment store's existing Windows-compatible identifier. Common names and intermediate
certificates are not identity evidence. A custom binding policy may replace the default, but
cannot accept unverified peers; a rejection is final.

Otherwise the SyncHdr `Cred` is verified: `syncml:auth-md5` against the stored credential hash
and a nonce the engine issued (a bounded record in the `state` store, renewed every session as
MS-MDM requires, base64 on the wire and raw bytes in the hash), or `syncml:auth-basic` when the
account was provisioned that way and the deployment opted in. Basic uses
`mdm.HashBasicCredential` to store a versioned PBKDF2-HMAC-SHA256 verifier of `username:password`
with a random 16-byte salt, 600,000 iterations and a 32-byte derived key. Verification compares
fixed-size derived keys with `subtle.ConstantTimeCompare`. `Identity` and `MDMCredential` contain
only `CredentialHash`; neither stores plaintext Basic credentials. Missing or malformed
verifiers fail closed. A missing or wrong wire credential receives Status 407 or 401 on the
SyncHdr with a `Chal` and no commands. Unverified certificates can still accompany valid
application-layer credentials.

This binding and password-storage policy is project policy. MS-MDM supplies the transport
certificate authentication option and the OMA DM credential formats. The reference server
currently requests no client certificate and provisions MD5 credentials; these defaults remain.

Package 1 facts (the DevInfo leaves, the LoginStatus, the AVD SyncType and DevicePrepSync
alerts) are recorded on the session and handed to a `PackageOne` hook once. On the first
session the engine queues, once, the reads the design wants: `./DevDetail` (SwV, LrgObj,
URI/MaxSegLen), the DeviceManageability CSP versions, and the DMClient push channel URI (Phase
8 consumes it; the known-issues page says re-read it every session). Client events (1224) the
engine does not consume and generic alerts (1226) go to `ClientEvent` and `GenericAlert` hooks;
a user-initiated unenroll alert ends the enrollment through an `Unenrolled` hook. Flow control
is handled directly: a "Next Message" request (Alert 1222) is answered with the continuation of
a large object in flight or the 1222 shape, a Session Abort (1223) ends the session, and a
No-End-of-Data (1225) resets the client-upload assembler.

Every server message carries `Final` except while a large object is being sent. The reply is
built from the `CommandQueue`: deliverable commands in the queue's monotonic sequence order,
user-scoped commands held until LoginStatus reports a user and refused for the wrong AVD
SyncType with the engine skipping them, each command's single large item split with `MoreData`
when it exceeds the client's `MaxObjSize`, and the whole message bounded by a byte budget so
the remaining commands roll to the next message. A command is marked sent when it goes out and
acknowledged or failed when its Status arrives; a Get's Results are reassembled across messages
(213 while chunks arrive) and filed with the Status that follows.

Transport facts are parsed but, beyond the content type, never enforced: the mode and platform
query parameters, the bearer and device tokens, the MS-Signature, the MDM-GenericAlert header,
the client-request-id and the UserAgentOrigin. WBXML is answered with 415 until Phase 14. The
Learn OMA DM page's rule against checking User-Agent, fixed URIs or value formats is followed.

## Rationale

Interfaces for authentication, the queue and sessions keep the engine transport- and
storage-neutral and let the same code run in tests against in-memory backends and in the Phase
6 server against SQL. Recording facts and routing alerts to hooks rather than acting on them
keeps policy out of the protocol layer. Answering a certificate-authenticated session without a
challenge, and challenging only when there is no trusted certificate and no valid credential,
matches the Windows authentication flow. The pinned Fleet `isTrustedRequest` checks whether
a peer CN contains the DeviceID without binding it to the stored enrollment certificate; this
project requires verified chains and exact enrollment binding. The pinned local-mdm
`HandleSyncML` acknowledges the header and resolves the device without authenticating a
credential in that method, so it supplies no certificate-verification policy to reuse. Reading the
LoginStatus alert and holding user commands is the fix for the pitfall where user settings sent
before sign-in fail with 500 or 507 and roll back an Atomic. Treating MS-MDM 1.3.1's "2.1"
as a typo for 1.2.1 is the only reading consistent with the rest of the document and with every
reference implementation.

## Constraints

Only the XML encoding is implemented; WBXML is 415 until Phase 14. MS-Signature is parsed and
reported but not verified (Phase 9 supplies the PKCS#7 verification). The bearer and device
tokens are not validated (Phase 10). The multi-user AVD session policy itself is out of scope:
the SyncType alert is recorded and the wrong-scope case is refused, but running parallel user
sessions is not modelled. The default in-memory `SessionStore` expires sessions after an hour
and loses them on restart; the SQL store arrives in Phase 6. SessionID is treated as an opaque
string; the two-byte form that `SyncApplicationVersion` 2.0 selects is carried through
unchanged.

## Verification

`mdmprotocol/mdm` tests drive full sessions through a scripted client: the MD5 challenge and
success with a Get result filed, an Atomic whose members' statuses are recorded as children,
server-to-client chunking of a 5,000-byte value reassembled by the client, user-scoped
commands held without a user and delivered after sign-in, the SyncType and DevicePrepSync facts
recorded, custom client events and a user-initiated unenroll routed to hooks, session abort,
basic authentication, and a tiny per-message budget forcing delivery across messages. Failure
tests cover a new session opened with the wrong MsgID, a first message that is not package 1, a
LocURI starting with "/", an unenrolled device, malformed XML, an oversize body, a wrong MsgID
mid-session, a SessionID change mid-flight, every hook returning an error, a failing nonce
store, and an expired nonce. The HTTP handler tests check Content-Length on every reply, no
chunking, WBXML as 415, malformed as 400, unenrolled as 403, oversize as 413 and GET as 405.
`FuzzHandle` fuzzes the request path. The `simulator` package enrolls and then runs the same
scenarios end to end over TLS, including real mTLS with enrollment CA verification. TLS regression tests reject unverified,
self-signed, expired and wrong-EKU identities, other enrollments sharing a CN, and another CA
using the same serial. Tests also cover custom rejection, loss of certificate verification
mid-session, salted Basic verifiers and continued MD5 fallback.

## References

- [mdmprotocol/mdm](../../../mdmprotocol/mdm), [mdmprotocol/syncml](../../../mdmprotocol/syncml), [simulator](../../../simulator)
- [Research store](../../research.md), sections 1.1 (OMA DM Protocol 1.2.1 sections 6 to 9), 1.3 (MS-MDM), 1.4 (OMA DM protocol support, Known issues), 7 (pitfalls)
- Microsoft, [MS-MDM]: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mdm/33769a92-ac31-47ef-ae7b-dc8501f7104f>
- OMA Device Management Protocol 1.2.1: <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Protocol-V1_2_1-20080617-A.pdf>
- OMA Device Management Security 1.2.1 (MD5 digest): <https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Security-V1_2_1-20080617-A.pdf>
- Microsoft, OMA DM protocol support: <https://learn.microsoft.com/en-us/windows/client-management/oma-dm-protocol-support>

Reference source identifiers and paths (relative to the named project):

- `fleetdm/fleet`, `server/service/microsoft_mdm.go` (`isTrustedRequest` certificate and MD5 flow, `processIncomingMDMCmds`, `getPendingMDMCmds`, `createResponseSyncML`, the poll-schedule reconcile that answered issue #43773)
- `deploymenttheory/local-mdm`, `internal/platform/windows/management.go` (`HandleSyncML` package 1 to 2)
- `sonicaj/MDMatador`, `internal/mdm/command_manager.go` (`processIncomingProtocolCommands`, `processPendingOperations`, `GetResponseSyncMLCommand`)
- `mattrax/Mattrax` (`rust` branch), `crates/ms-mdm`

## Desktop conformance

Session lookup is additionally bound to EnrollmentKey. A new enrollment may restart SessionID and MsgID at 1; it cannot continue an unfinished session or inherit authentication from the previous enrollment. Unit and server e2e regressions seed old authenticated state and require a fresh challenge.

## Compatibility and upgrade

Custom `Authenticator.TrustCertificate` implementations and `MDMAuthenticator.TrustCert`
callbacks now receive `[][]*x509.Certificate` verified chains. Direct `Service.Handle` callers
must supply actual TLS verification evidence in `Transport.VerifiedChains` and the matching
peer in `Transport.Certificates`; forwarded certificate headers do not establish trust.
Certificate deployments must configure the TLS verifier with enrollment CA roots. Custom
identity lookups using the default storage binding must supply `CertificateThumbprint` as well
as `EnrollmentKey`.

Basic callers replace `BasicUsername`/`BasicPassword` with `CredentialHash` from
`mdm.HashBasicCredential`. SQL startup migrates legacy rows transactionally and clears the old
columns; see [0014](0014-sql-storage-backends.md) for operational requirements. Clear credentials
with both username and password empty have no distinguishable legacy marker and remain invalid;
reprovision such an account if it is needed.

- Go TLS verification contract: <https://pkg.go.dev/crypto/tls#ConnectionState>
- Go PBKDF2 implementation: <https://pkg.go.dev/crypto/pbkdf2>
