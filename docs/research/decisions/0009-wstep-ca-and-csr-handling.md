# 0009: WSTEP CA and CSR handling

## Context

The RequestSecurityToken body carries a PKCS#10 request the enrollment client built with
Windows's own crypto stack. Its subject common name is tagged as an ASN.1 PrintableString
even when it contains an exclamation mark or a NUL, which X.680 forbids and crypto/x509
refuses ("invalid PrintableString"). The open implementations answer that three ways: a
1,585-line copy of the standard parser with two bytes added to `isPrintable` (Fleet), a
monkey patch of GOROOT (oscartbeaumont's PoC), or a different language (Mattrax). The server
then has to sign the key, give the certificate back with the right store path, and remember
what it issued for the session engine, renewal and revocation, while never back-dating the
certificate, which Fleet does by the renewal period and which halves the certificate's useful
life (research pitfall "Client certificate back-dated").

## Decision

`pki/ca.Issuer` is the signing interface: `Issue` takes a public key, a subject and an optional
per-request policy; `Certificate` and `Chain` expose the signing certificate and its path to
the root, issuer first, so the provisioning document can install the root under
`CertificateStore/Root` and one intermediate under `CertificateStore/CA`. `ca.Local` is the
implementation: built from a certificate and key (`NewLocal`), PEM (`LoadPEM`), files
(`LoadFiles`) or generated (`NewSelfSigned`, RSA 2048 by default, or a supplied key). Its
`Policy` sets validity (one year by default), an absolute `NotAfter` cap, key usages (digital
signature plus key encipherment for RSA, client authentication), a minimum RSA size (2048),
an allow list of key kinds, a subject override and extra extensions. `NotBefore` is the
issuing clock's now; there is no back-dating option. Serials are random 127-bit values.

`pki/wstep.ParseCSR` parses the client's request. It first refuses a PKCS#7 ContentInfo with
`ErrPKCS7`, then tries crypto/x509, and only when the error names PrintableString retries
through a narrow relaxation: the request is split into its TLV parts, each PrintableString in
the subject whose only offending bytes are in `RelaxedCharacters` (`!` and NUL, the two the
Windows client has been observed to send) is re-tagged as a UTF8String in a copy, the copy is
parsed by crypto/x509, and the original `Raw`, `RawTBSCertificateRequest` and `RawSubject`
bytes are restored on the result so `CheckSignature` verifies what the client signed. Any
other unprintable byte is refused with `ErrSubject`; a request whose signature does not verify
is refused with `ErrSignature`. Nothing is vendored and nothing in the toolchain is patched.

`pki/wstep.Issuer` implements `enroll.Issuer`: parse, verify, choose the subject (the CSR's
own `pkix.Name` by default, re-encoded by the CA so the issued certificate is one every
parser reads; a `SubjectFunc` can substitute the DeviceID or anything else), sign under the
CA policy or a WSTEP-specific override, and return the certificate with the CA's chain.
Policy refusals surface as `ca.ErrPolicy`, which the enrollment service renders as
`s:CertificateRequest`.

Keying and history live in the storage tier (decision record 0010): the enrollment is keyed by
the certificate serial and indexed by DeviceID and thumbprint; a repeated `HWDevID` under a
different DeviceID is reported as a `storage.Conflict` event by the Recorder and the
enrollment proceeds.

## Rationale

The relaxation touches only the bytes that are wrong, keeps the signed bytes intact, and is
tested against `testpki.WindowsCSR`, which reproduces the Fleet-documented subject; if
crypto/x509 ever accepts that subject, the test fails and the relaxation can be removed. A
vendored parser would have to track every crypto/x509 change; a GOROOT patch cannot ship.
Re-encoding the subject through `pkix.Name` (UTF8String for the odd characters) is what Fleet
does in production, so the certificate Windows receives is a known-good shape, and it is the
only way to return a `*x509.Certificate` at all. The CA never back-dates because the client's
renewal window is measured from the certificate's own dates. The CA does not persist because
persistence has an owner two tiers up; a depot interface here would duplicate it.

## Constraints

`RelaxedCharacters` is deliberately the observed set; a Windows build that emits another
character in a PrintableString will be refused with `ErrSubject` until the set is extended
with evidence. The issued certificate's subject is the request's subject re-encoded, so a
byte-for-byte comparison of CSR subject and certificate subject fails when the relaxation
applied; Windows compares by key, and the conformance run (Phase 7) confirms it. PKCS#7
renewal tokens are recognised and refused until Phase 9. The Microsoft DeviceID certificate
extension (1.3.6.1.4.1.311.66.1.0) is not written; nothing in the on-premise flow reads it.
`ca.Local` holds its key in memory; HSM-backed signers implement `ca.Issuer` themselves.

## Verification

`pki/wstep` tests prove crypto/x509 still refuses the Windows subject, parse it, verify the
signature over the restored bytes, check the raw subject keeps its PrintableString tag, parse a
NUL subject, refuse a `#` subject with `ErrSubject`, refuse tampered signatures on both paths,
refuse PKCS#7 (`IsPKCS7` true only for the 1.2.840.113549.1.7 arc), cover every structural
error of the relaxed path, issue through a CA with a subject hook and a policy override, and
fuzz `ParseCSR` asserting no accepted request has a bad signature. `pki/ca` tests cover
`NotBefore` equal to now, defaults, every policy rule and refusal, key kinds, chains through
an intermediate, PEM and file loading with every malformed input, and self-signed generation
with a supplied key.

## References

- [pki/ca](../../../pki/ca), [pki/wstep](../../../pki/wstep), [testpki](../../../testpki)
- [Research store](../../research.md), section 4 (SOAP finding; x509 CSR pitfall; PKCS#10 vs PKCS#7), section 7 ("Client certificate back-dated", "Duplicate HWDevID"), section 9 (GOROOT patch and vendored parser)
- ITU-T X.680 section 41.4, PrintableString: <https://www.itu.int/rec/T-REC-X.680>
- RFC 2986, PKCS #10: <https://www.rfc-editor.org/rfc/rfc2986>
- RFC 5280: <https://www.rfc-editor.org/rfc/rfc5280>
- Microsoft, [MS-WSTEP] 3.1.4.1: <https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-wstep/4766a85d-0d18-4fa1-a51f-e5cb98b752ea>

Reference source identifiers and paths (relative to the named project):

- `fleetdm/fleet`, `server/mdm/microsoft/wstep_csr.go` (`ParseCertificateRequestFromWindowsDevice`, the patched `isPrintable` admitting `!` and 0x00), `server/mdm/microsoft/wstep.go` (`SignClientCSR`, `NotBefore` back-dated by the renewal period)
- `oscartbeaumont/windows_mdm`, `patch/patch.go` (GOROOT monkey patch)
- `deploymenttheory/go-apple-dm`, `pki/ca/ca.go` (`Policy`, `Local`, `KindOf`, `SerialFrom`; this record drops its `Backdate` and `Depot`)
