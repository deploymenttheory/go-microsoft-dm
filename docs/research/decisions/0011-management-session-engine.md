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

Authentication runs per session and is remembered once it succeeds. A TLS client certificate
that belongs to the enrollment authenticates the whole session (MS-MDM 1.3.1, transport
client-certificate authentication); the default match is a subject common name containing the
DeviceID, which is how the enrollment CSR named it, and a deployment can pin the exact
certificate. Otherwise the SyncHdr `Cred` is verified: `syncml:auth-md5` against the stored
credential hash and a nonce the engine issued (a bounded record in the Phase 1 `state` store,
renewed every session as MS-MDM requires, base64 on the wire and raw bytes in the hash),
or `syncml:auth-basic` when the account was provisioned that way and the deployment opted in.
A missing or wrong credential is answered with a Status 407 or 401 on the SyncHdr carrying a
`Chal` with a fresh nonce and no commands; the client reverts to package 1 with the credential
(OMA DM Security 9). Success needs no repeat challenge for the rest of the session.

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
matches what a real Windows client expects and what Fleet's trust check does. Reading the
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
scenarios end to end over TLS, including real mTLS.

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
