// Package ca defines the certificate authority interface and the in-memory and file-backed issuers.
//
// # Design
//
// Issuer is what the enrollment path calls: Issue signs a public key under a
// subject and returns the certificate; Certificate and Chain expose the
// signing certificate and its path to the root so the provisioning document
// can install the root under CertificateStore/Root and one intermediate
// under CertificateStore/CA. Local holds the signing key in memory; it is
// built from a certificate and key in memory (NewLocal), from PEM (LoadPEM),
// from files (LoadFiles) or generated (NewSelfSigned).
//
// Issuance follows Policy: NotBefore is the issuing clock's now and is never
// back-dated, because Windows treats the certificate's remaining life as the
// renewal budget and a certificate back-dated by the renewal period is half
// spent on arrival (research pitfall "Client certificate back-dated");
// validity defaults to one year; the key usage defaults to digital
// signature (and key encipherment for RSA) with client authentication; RSA
// keys below 2048 bits are refused; the subject comes from the request.
//
// The package does not parse certificate requests (pki/wstep does), does
// not persist what it issues (the enrollment Recorder does through the
// storage contracts) and does not revoke; revocation arrives with
// pki/revocation.
//
// # References
//
//   - Decision record 0009: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0009-wstep-ca-and-csr-handling.md
//   - RFC 5280: https://www.rfc-editor.org/rfc/rfc5280
package ca
