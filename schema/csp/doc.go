// Package csp is the runtime model of a configuration service provider
// tree: the types the generated schema packages instantiate and the lookups
// the validation and support packages run over them.
//
// # Design
//
// A Tree is one root of one DDF file (a CSP such as DMClient, or a Policy
// area such as Policy/Config/BITS) with its Node hierarchy. Every Node
// carries what the DDF says about it: format, access verbs, scope, the
// MSFT applicability, allowed values, dynamic naming, dependencies and
// behaviour flags. Dynamic nodes have an empty Name and a placeholder Title,
// and a URI is matched against the tree segment by segment so that
// ./Device/Vendor/MSFT/DMClient/Provider/MS%20DM%20SERVER/Poll resolves to
// the node whose path is .../Provider/{ProviderID}/Poll.
//
// The package holds no generated data and does no I/O; cmd/ddfgen writes the
// data into schema/csp/<name> and schema/policy/<area>, and schema/registry
// joins them. Value validation and build applicability live in
// schema/validation and schema/support.
//
// # References
//
//   - Decision record 0006: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0006-schema-generator-over-ddf-v2.md
//   - Microsoft, DDF v2 files and XSD: https://learn.microsoft.com/en-us/windows/client-management/mdm/configuration-service-provider-ddf
//   - OMA DM Tree and Description 1.2.1: https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_TND-V1_2_1-20080617-A.pdf
package csp
