// Package xcep derives the MS-XCEP GetPolicies response from the certificate authority's policy.
//
// # Design
//
// The enrollment client reads only a few fields of a GetPolicies response:
// the minimal key length, the hash algorithm OID, the crypto providers and
// the validity and renewal periods. FromPolicy fills an enroll.PolicyResponse
// from a pki/ca Policy so that what the policy service promises is what the
// authority will sign: the key length comes from MinRSABits, the validity
// from Validity, and the hash from the Options. The wire encoding lives in
// mdmprotocol/enroll because the enrollment service is below this tier.
//
// The package does not implement any other MS-XCEP operation or the
// certificate template model; Windows enrollment uses none of it.
//
// # References
//
//   - Decision record 0008: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0008-enrollment-protocol-and-provisioning-document.md
//   - Microsoft MS-XCEP: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-xcep/08ec4475-32c2-457d-8c27-5a176660a210
//   - Microsoft MS-MDE2 3.3.4.1.1.2: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692
package xcep
