// Package wapprov builds the wap-provisioningdoc returned by WSTEP enrollment.
//
// # Design
//
// Phase 4 of the implementation plan fills this package: a characteristic and
// parm model with the CertificateStore, APPLICATION w7, DMClient and
// RootCATrustedCertificates characteristics from MS-MDE2 2.2.9. Parameter
// names are uppercase and case-sensitive. It does not decide policy values;
// callers supply them. Until Phase 4 the package is empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 4: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - OMA DM w7 characteristic: https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-SUP-ac_w7_dm-V1_0_1-20080617-A.txt
package wapprov
