// Package simulator is a Windows MDM client in software for tests and end-to-end scenarios.
//
// # Design
//
// Enroll performs the MS-MDE2 on-premise enrollment the way the Windows
// client does: the GET probe of the discovery URL, the Discover POST, the
// optional GetPolicies call when discovery advertised a policy service, key
// generation at the advertised key length, a PKCS#10 request (optionally with
// the PrintableString subject Windows sends) and the RequestSecurityToken
// call. The returned Enrollment holds the certificate and key, the
// provisioning document, and the document parsed into the OMA DM account and
// DMClient state the Phase 5 session client starts from.
//
// The simulator speaks only what the server package implements; a fault from
// the server is returned as a *soap.DecodedFault. It is the unit-test client;
// a real Windows guest on guestweave is the conformance client (Phase 7).
// It does not yet run management sessions.
//
// # References
//
//   - Decision record 0008: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0008-enrollment-protocol-and-provisioning-document.md
//   - Microsoft MS-MDE2 section 4: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692
package simulator
