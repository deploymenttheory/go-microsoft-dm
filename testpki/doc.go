// Package testpki creates ephemeral certificate authorities, identities,
// TLS server certificates and certificate signing requests for tests and the
// device simulator.
//
// # Design
//
// Shared fixtures provide MDM client identities, TLS server certificates for
// httptest servers, plain PKCS#10 requests and, in WindowsCSR, a request whose
// subject is encoded the way the Windows enrollment client encodes it: a
// PrintableString carrying characters outside the PrintableString alphabet.
// The standard library rejects that request, which is the pitfall every open
// Windows MDM server has had to work around, so the parser in pki/wstep is
// tested against it from Phase 1 onward.
//
// Keys are generated at test time and validity is short. These helpers do not
// apply production issuance policy and their roots must not be trusted outside
// tests. Production signing uses pki/ca with configured keys, policy and
// storage.
//
// # References
//
//   - Decision record 0004: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0004-go-baseline-and-foundation-contracts.md
//   - Research store, section 4 (x509 and CSR handling): https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research.md
//   - Microsoft MS-WSTEP: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-wstep/4766a85d-0d18-4fa1-a51f-e5cb98b752ea
package testpki
