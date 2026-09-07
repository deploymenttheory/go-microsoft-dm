# 0008: Enrollment protocol and the provisioning document

## Context

A Windows 11 device becomes managed through MS-MDE2: it resolves the Discovery Service, posts
a Discover request, optionally fetches a certificate policy over MS-XCEP, and posts a
WS-Trust RequestSecurityToken over MS-WSTEP whose answer is a certificate wrapped in an OMA
Client Provisioning `wap-provisioningdoc` that also creates the OMA DM account and the DMClient
provider node. Every later phase depends on that document being right: the session engine
verifies the credentials it planted, the poll schedule it set decides when the device calls
back, and the renewal settings decide when Phase 9's ROBO flow runs. The reference
implementations show where a server drifts: a one-minute poll that breaks the CSP's own rule,
a back-dated client certificate, hand-typed fault envelopes, one struct per body that loses
the namespace when re-encoded, and no test against Microsoft's worked examples.

## Decision

MS-MDE2 revision 19.0 (2026-08-11) is the authority for message shapes and the fault table; the
Learn on-premises and MDM enrollment pages are read for behaviour the specification leaves
open. The layer split follows the tiers: `mdmprotocol/soap` owns the envelope, the
WS-Addressing header, the WS-Security header (UsernameToken, BinarySecurityToken, Timestamp)
and faults; `mdmprotocol/enroll` owns the three bodies, both directions, and the
transport-neutral flow; `mdmprotocol/wapprov` owns the provisioning document. `pki/wstep`
and `pki/xcep` sit above and are reached only through the `enroll.Issuer` interface and a
`PolicyResponse` value, so the enrollment package never imports the PKI tier.

Envelopes are decoded into `soap.Envelope[B]` with the caller's body type, accepting the SOAP
1.2 namespace every example uses and the SOAP 1.1 namespace MS-MDE2 2.1 names; header
identifiers are trimmed because Microsoft's own examples wrap them in whitespace. Responses
are written by hand in the prefix form of the section 4 examples (`s:`, `a:`, `u:`, `o:` for
the reply-side Security header), with `xmlns:xsi` and `xmlns:xsd` on the Body of Discover and
GetPolicies replies, an `ActivityId` on Discover replies and a five-minute `u:Timestamp` on
the token reply. Faults are SOAP 1.2 `s:Fault` with code `s:Receiver`, one of the seven
subcodes of 2.2.10 (`s:MessageFormat` 0x80180001 through `a:InvalidSecurity` 0x80180007),
`xml:lang="en-US"` reason text, and, when a detail applies, the `deviceenrollmentserviceerror`
element with one of the eight error types (`DeviceCapReached` 0x80180013 through
`CustomServerError` 0x80180032) and a trace id. `soap.Fault` is a Go error so a flow can
return it from any depth and every non-fault error becomes `a:InternalServiceFault`.

Discovery: `GET /EnrollmentServer/Discovery.svc` answers 200 with an empty body (the client's
reachability probe, MS-MDE2 1.3 step 3). The Discover body requires an EmailAddress and a
RequestVersion in 1.0 to 9.0, and the client must have offered the server's policy; this
phase implements `OnPremise` only (Federated and Certificate are Phase 10). The reply carries
`AuthPolicy`, `EnrollmentPolicyServiceUrl` when the policy service is configured,
`EnrollmentServiceUrl`, and `EnrollmentVersion` equal to the lower of the configured version
(default 3.0, raised in Phase 11) and the client's RequestVersion, omitted for clients below
3.0. `DeviceAssociationMaaUrl` and `GatewayService` are modelled and written when set but
unused. The `enterpriseenrollment.<domain>` hostname convention is what the simulator derives
and what documentation says; the server never checks the Host header.

GetPolicies: the WS-Security header must carry a UsernameToken with `#PasswordText` (a
BinarySecurityToken is refused with `a:InvalidSecurity` naming the unsupported policy); the
Authenticator decides `s:Authentication` (unknown) or `s:Authorization` (known, not allowed).
The reply is the MS-MDE2 4.2.2 shape: `policySchema` 3, `minimalKeyLength` 2048 by default,
`validityPeriodSeconds` and `renewalPeriodSeconds`, the two Windows crypto providers,
`hashAlgorithmOIDReference` into an `oIDs` table holding SHA-256 (`2.16.840.1.101.3.4.2.1`),
every field Windows does not read written as `xsi:nil="true"`. `pki/xcep.FromPolicy` derives
it from the CA policy so the promise matches the issuer.

RequestSecurityToken: `TokenType` must be `DeviceEnrollmentToken` and `RequestType` `Issue`;
a `Renew` request or a `#PKCS7` token is refused with `s:CertificateRequest` and the
`NotEligibleToRenew` detail (Phase 9 accepts them); the `#PKCS10` token is base64-decoded and
handed to the Issuer unparsed. `AdditionalContext` is parsed into a typed struct covering all
41 items MS-MDE2 3.4.4.1.1.1 catalogues, with repeats (`MAC`, `IMEI`) as slices, booleans read
case-insensitively (the client writes `True`), unknown items kept, and every item kept in
wire order. `DeviceID` and a valid `EnrollmentType` are required (`InvalidEnrollmentData`
detail otherwise); `EnrollmentType` Full puts the certificate under `My/User` and Device under
`My/System`. The reply is `RequestSecurityTokenResponseCollection` with the provisioning
document base64-encoded in a `BinarySecurityToken` of ValueType `DeviceEnrollmentProvisionDoc`,
an empty `DispositionMessage` and `RequestID` 0.

The provisioning document is `wap-provisioningdoc` version 1.1 with: `CertificateStore` holding
`Root/System/<thumbprint>` for the root, `CA/System/<thumbprint>` for at most one
intermediate, `My/<User|System>/<thumbprint>` plus `PrivateKeyContainer` for the client
certificate, and `My/WSTEP/Renew` (`ROBOSupport` true, `RenewPeriod` 60 days, `RetryInterval`
4 days by default, `ServerURL` reserved for Phase 9), thumbprints being SHA-1 uppercase hex;
`APPLICATION` with `APPID` w7, `PROVIDER-ID`, `NAME`, `ADDR`, optional `PROTOVER`,
`CONNRETRYFREQ`, `INITIALBACKOFFTIME`, `MAXBACKOFFTIME`, `BACKCOMPATRETRYDISABLED`,
`DEFAULTENCODING`, `USEHWDEVID`, `SSLCLIENTCERTSEARCHCRITERIA` (U+F000 between values, RFC 2396
escaping, the comma left as Microsoft's example leaves it), and two `APPAUTH` children:
APPSRV (BASIC or DIGEST, `AAUTHNAME` required) and CLIENT (DIGEST only), both required, as
MS-MDE2 2.2.9.5 states; `ROLE` is never written. `DMClient/Provider/<PROVIDER-ID>` carries
`UPN` for Full enrollments, `EntDeviceName`, optional `EntDMID`, and `Poll` at Microsoft's
documented defaults (15 minutes times 5, 60 minutes times 10, then every 1440 minutes for
ever); the builder refuses `NumberOfFirstRetries` 0 and a remaining interval under 1440
minutes. Parameter names below `APPLICATION` are validated uppercase. Credentials come from a
`CredentialSource` the deployment supplies: the enrollment path does not invent secrets the
session engine would then have to guess.

Transport: `enroll.Handler` sets `Content-Length` on every response so nothing is chunked,
answers faults with status 500, rejects bodies over the configured bound with 413, and
performs no checks on `User-Agent`, hostnames or value formats beyond the specification's.

## Rationale

Typed bodies decoded through a generic envelope keep the shared parts in one place without
the enrollment package leaking into the SOAP package, and let the simulator reuse the same
codecs as a client. Hand-written responses in the example's exact prefix form remove the one
variable the real client is sensitive to that `encoding/xml`'s marshaller cannot control.
Typed faults with the specification's codes give the client the message Microsoft's own
troubleshooting page expects (research pitfall "Enrollment error codes"). Microsoft's poll
defaults, not Fleet's one-minute schedule, respect the DMClient CSP's stated rule and leave
frequency to Phase 8's push design. Refusing PKCS#7 and Renew with `NotEligibleToRenew`
rather than `MessageFormat` tells a renewing device the truth about why.

## Constraints

Only the OnPremise policy is implemented; a client that offers only Federated or Certificate
gets `s:Authentication`. `EnrollmentVersion` is capped at 3.0 by default, so the attestation
items of 5.0 and later are parsed but never requested. The WS-Security header is not signed
or verified beyond the UsernameToken; certificate authentication (Phase 10) adds the XML
signature. Passwords are compared by the Authenticator the deployment supplies; the library
ships none. Whether `EnrollmentPolicyServiceUrl` may be omitted for a Windows 11 client is
untested against a real device (Phase 7); the simulator tolerates both. The RSTRC example in
MS-MDE2 4.3.2 uses the SOAP 1.1 namespace while every other example uses 1.2; this
implementation replies in 1.2 and accepts either on input.

## Verification

`mdmprotocol/soap` tests cover envelope decoding for both namespaces, header trimming, token
decoding, the response writer against a byte-exact rendering of the 4.3.2 header shape, fault
round trips, every subcode and error type value, and `FuzzDecode`. `mdmprotocol/enroll` tests
decode the on-premise Discover, GetPolicies and RequestSecurityToken examples of MS-MDE2
section 4 (with a real CSR substituted), check every AdditionalContext item in the 4.3.1.4
example plus the rest of the catalogue, round-trip each codec, drive the service through
version negotiation, every fault path (malformed body, wrong action, missing email,
unsupported version, policy not offered, missing or wrong security header, unknown user,
blocked user, authenticator faults, wrong token or request type, Renew, PKCS#7, bad base64,
missing DeviceID, bad EnrollmentType, issuer and recorder failures), the provisioning document
for Full and Device enrollments, and the HTTP adapter's framing (Content-Length on every
response, no Transfer-Encoding, 500 on faults, 405, 413), and `FuzzService`.
`mdmprotocol/wapprov` tests decode the MS-MDE2 2.2.9.1 example, round-trip documents, check
the search criteria example byte for byte, and refuse the schedule Fleet ships. `simulator`
enrolls against the in-memory server over TLS with a Windows-shaped subject and checks the
parsed account, credentials, poll schedule and store contents.

## References

- [mdmprotocol/soap](../../../mdmprotocol/soap), [mdmprotocol/enroll](../../../mdmprotocol/enroll), [mdmprotocol/wapprov](../../../mdmprotocol/wapprov), [pki/xcep](../../../pki/xcep), [simulator](../../../simulator)
- [Research store](../../research.md), sections 1.3 (MS-MDE2, MS-XCEP, MS-WSTEP), 1.4 (enrollment pages, DMClient, w7, CertificateStore), 1.8 (AdditionalContext catalogue), 7 (pitfalls "Enrollment error codes", "Transport")
- Microsoft, [MS-MDE2] v19.0, sections 2.2.9, 2.2.10, 3.1, 3.3, 3.4, 4: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692>
- Microsoft, [MS-XCEP] 3.1.4.1: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-xcep/08ec4475-32c2-457d-8c27-5a176660a210>
- Microsoft, [MS-WSTEP] 3.1.4.1: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-wstep/4766a85d-0d18-4fa1-a51f-e5cb98b752ea>
- Microsoft, On-premises authentication device enrollment: <https://learn.microsoft.com/en-us/windows/client-management/on-premise-authentication-device-enrollment>
- Microsoft, DMClient CSP: <https://learn.microsoft.com/en-us/windows/client-management/mdm/dmclient-csp>
- Microsoft, w7 APPLICATION CSP: <https://learn.microsoft.com/en-us/windows/client-management/mdm/w7-application-csp>

Reference source identifiers and paths (relative to the named project):

- `fleetdm/fleet`, `server/service/microsoft_mdm.go` (`NewDiscoverResponse`, `NewGetPoliciesResponse`, `NewCertStoreProvisioningData`, `NewApplicationProvisioningData`, `NewDMClientProvisioningData` with the one-minute poll, `NewSoapFault`), `server/fleet/microsoft_mdm.go` (SOAP structs), `server/mdm/microsoft/syncml/syncml.go` (namespace constants)
- `deploymenttheory/local-mdm`, `internal/platform/windows/enrollment.go` (string-templated RSTRC and provisioning document)
- `mattrax/Mattrax` (`rust` branch), `crates/ms-mde/src/{discovery,policy,enrollment,header,fault}.rs`
