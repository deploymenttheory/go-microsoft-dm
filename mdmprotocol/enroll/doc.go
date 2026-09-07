// Package enroll implements MS-MDE2 discovery, XCEP policy and WSTEP enrollment messages and handlers.
//
// # Design
//
// The package has three layers. The codecs (DecodeDiscover, EncodeDiscover
// and their GetPolicies and RequestSecurityToken counterparts) turn the SOAP
// bodies of MS-MDE2 sections 3.1, 3.3 and 3.4 into typed Go values and back,
// in both directions so that a server and a client share one definition of
// each message. AdditionalContext is the typed view of every ContextItem the
// specification catalogues, with unknown items kept.
//
// Service is the transport-neutral flow: Discover validates the request
// version and the offered policies and advertises the on-premise policy and
// the configured endpoints; GetPolicies authenticates the WS-Security
// UsernameToken and returns the certificate policy; Enroll authenticates,
// checks the WS-Trust token and request types, refuses PKCS#7 and Renew with
// the NotEligibleToRenew detail until Phase 9, validates the context, calls
// an Issuer for the certificate, a Provisioner for the document and a
// Recorder for persistence, and returns the RequestSecurityTokenResponse.
// Every failure is a *soap.Fault with the exact subcode of MS-MDE2 2.2.10,
// and the returned bytes are always a complete envelope so a transport can
// write them without knowing what went wrong.
//
// Handler is the HTTP adapter: GET on the discovery path is the client's
// reachability probe, responses carry Content-Length and are never chunked,
// and faults go out with status 500. It performs no other checks on the
// request: no User-Agent, no fixed hostnames, no value formats beyond the
// specification's, as the Learn enrollment page advises.
//
// The package does not parse certificate requests or sign anything (an
// Issuer does, in pki/wstep), does not decide who may enroll (an
// Authenticator does) and does not persist (a Recorder does). Only the
// OnPremise policy is implemented; Federated and Certificate arrive in
// Phase 10, renewal in Phase 9.
//
// # References
//
//   - Decision record 0008: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0008-enrollment-protocol-and-provisioning-document.md
//   - Microsoft MS-MDE2: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692
//   - Microsoft MS-XCEP: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-xcep/08ec4475-32c2-457d-8c27-5a176660a210
//   - Microsoft MS-WSTEP: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-wstep/4766a85d-0d18-4fa1-a51f-e5cb98b752ea
//   - On-premises authentication device enrollment: https://learn.microsoft.com/en-us/windows/client-management/on-premise-authentication-device-enrollment
package enroll
