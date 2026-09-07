// Package wstep parses enrollment CSRs and applies issuance policy for MS-WSTEP.
//
// # Design
//
// ParseCSR reads the PKCS#10 request a Windows enrollment client puts in
// its BinarySecurityToken. Windows tags the subject common name as an ASN.1
// PrintableString even when it contains '!' or a NUL, which the standard
// library rightly refuses (research section 4, x509 pitfall). Instead of a
// vendored parser or a patched toolchain, ParseCSR retries once with a
// narrow relaxation: every PrintableString in the subject whose only
// offending bytes are in RelaxedCharacters is re-tagged as a UTF8String in a
// copy, the copy is parsed by crypto/x509, and the original bytes are put
// back on the parsed request so that CheckSignature verifies what the
// client signed. Any other unprintable byte is refused with ErrSubject.
// A PKCS#7 body, the renewal token of MS-MDE2 3.5, is recognised and refused
// with ErrPKCS7 until Phase 9.
//
// Issuer implements enroll.Issuer over a pki/ca Issuer: it parses the CSR,
// verifies its signature, lets an optional Subject hook choose the
// certificate subject (the request's own subject by default, re-encoded by
// the CA so the issued certificate is one every parser reads) and signs.
//
// The package does not build provisioning documents, does not persist and
// does not decide whether a device may enroll beyond the key policy.
//
// # References
//
//   - Decision record 0009: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0009-wstep-ca-and-csr-handling.md
//   - Microsoft MS-WSTEP: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-wstep/4766a85d-0d18-4fa1-a51f-e5cb98b752ea
//   - RFC 2986 PKCS #10: https://www.rfc-editor.org/rfc/rfc2986
//   - ITU-T X.680 section 41.4 PrintableString: https://www.itu.int/rec/T-REC-X.680
package wstep
