// Package support answers whether a CSP node applies to a device, from the
// MSFT:Applicability the DDF records and the build the device reports.
//
// # Design
//
// OsBuildVersion is a comma-separated list: the first entry is the major
// release a node shipped in and later entries are backports with the
// revision they arrived in. A device reports its build in DevDetail/SwV as
// Major.Minor.Build.Revision. Windows 11 24H2, 25H2 and 26H2 share one
// servicing branch (26100, 26200 and 26300 receive the same cumulative
// update), so a node that applies to 26100.x applies to a 26200 or 26300
// device at the same or later revision. Sentinel values (99.9.99999,
// 99.9.9999, 88.8.88888) mean "not in a released build"; Insider and Server
// builds are reported as such; the two nodes stamped 11.0.x are read as 10.0.x.
//
// The package reads DDF data and a build string; it does not fetch either.
// Edition allow lists are exposed but not interpreted beyond membership,
// because Microsoft publishes no table of the hex edition ids.
//
// # References
//
//   - Decision record 0006: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0006-schema-generator-over-ddf-v2.md
//   - Research store, section 0 (build lineage): https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research.md
//   - Microsoft, Windows 11 release information: https://learn.microsoft.com/en-us/windows/release-health/windows11-release-information
package support
