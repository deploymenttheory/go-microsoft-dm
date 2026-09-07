// Package wapprov builds and reads the wap-provisioningdoc returned by WSTEP enrollment.
//
// # Design
//
// A provisioning document is a tree of characteristic elements holding parm
// elements. Document, Characteristic and Parm model it exactly; Encode writes
// it in the form of the MS-MDE2 2.2.9.1 examples and Decode reads it back for
// the simulator and for tests.
//
// The builders produce the four characteristics MS-MDE2 2.2.9.2 to 2.2.9.5
// profile: CertificateStore (the root, at most one intermediate, the client
// certificate under My/User or My/System, and My/WSTEP/Renew), APPLICATION
// with APPID w7 (the OMA DM account, with the APPSRV and CLIENT credentials
// MS-MDE2 2.2.9.5 says MUST both be present and the CLIENT level fixed to
// DIGEST), DMClient (the provider node with its polling schedule at
// Microsoft's documented defaults) and RootCATrustedCertificates. Parameter
// names are the exact uppercase spellings of the w7 characteristic; Validate
// refuses anything else in an APPLICATION subtree because the client is case
// sensitive.
//
// The package does not decide values. Whether a device is a user or device
// enrollment, which management URL to hand out, which credentials to issue
// and how often to poll are the caller's choices; the builders check only
// the constraints the specification states. It performs no I/O.
//
// # References
//
//   - Decision record 0008: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0008-enrollment-protocol-and-provisioning-document.md
//   - Microsoft MS-MDE2 2.2.9: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mde2/4d7eadd5-3951-4f1c-8159-c39e07cbe692
//   - OMA DM w7 characteristic: https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-SUP-ac_w7_dm-V1_0_1-20080617-A.txt
//   - DMClient CSP: https://learn.microsoft.com/en-us/windows/client-management/mdm/dmclient-csp
package wapprov
