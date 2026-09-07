// Package soap holds the SOAP envelope, WS-Addressing, WS-Security header and fault types MS-MDE2 uses.
//
// # Design
//
// The package models the envelope shapes MS-MDE2 section 2 profiles and
// nothing beyond them. A request is decoded into Envelope[B], where B is the
// body type the calling package supplies, so that the enrollment package owns
// the Discover, GetPolicies and RequestSecurityToken bodies while this package
// owns the parts every message shares: the SOAP 1.2 envelope (the SOAP 1.1
// namespace is accepted on input because MS-MDE2 2.1 names it and one of its
// examples uses it), the WS-Addressing Action, MessageID, ReplyTo and To
// headers, and the WS-Security header with its UsernameToken,
// BinarySecurityToken and Timestamp children.
//
// Responses are written by a small writer that emits the exact prefixes and
// element order of the MS-MDE2 section 4 examples, because the Windows
// enrollment client is the only consumer and the examples are what it is
// tested against. Faults follow MS-MDE2 2.2.10: a SOAP 1.2 Fault with the
// s:Receiver code, one of the seven subcodes, a reason text, and the optional
// deviceenrollmentserviceerror detail.
//
// The package performs no I/O and decides no policy. It does not implement
// WS-Security signatures or the federated BinarySecurityToken exchange; the
// certificate and federated authentication policies arrive in Phase 10 of the
// implementation plan.
//
// # References
//
//   - Decision record 0008: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0008-enrollment-protocol-and-provisioning-document.md
//   - Microsoft MS-MDE2 sections 2.1, 2.2.1, 2.2.10 and 4: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692
//   - WS-Addressing 1.0 Core: https://www.w3.org/TR/ws-addr-core/
//   - WS-Security 2004 SOAP Message Security 1.0: https://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0.pdf
package soap
