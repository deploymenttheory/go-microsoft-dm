// Package support resolves CSP node applicability against a device's reported build.
//
// # Design
//
// Phase 3 of the implementation plan fills this package from the generated
// schema: OsBuildVersion lists are resolved against DevDetail SwV using the
// 24H2, 25H2 and 26H2 shared servicing branch, with sentinel values treated
// as not released. Until then the package is empty and exists so the schema
// tier has a home. It never performs I/O.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 3: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - Microsoft: https://learn.microsoft.com/en-us/windows/client-management/mdm/configuration-service-provider-ddf
package support
