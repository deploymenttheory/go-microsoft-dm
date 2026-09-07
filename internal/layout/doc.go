// Package layout builds repository import graphs for dependency-boundary tests.
//
// # Design
//
// LoadRepo reads both Go modules, records package edges and maps them to
// repository-relative paths. The graph supports transitive reachability and
// directory-level strongly connected components, and keeps test-only imports
// apart from production ones so the "library never imports the server, even in
// tests" rule can be checked. Tests define tier rules and their explicit
// exceptions; this package holds no policy.
//
// The lint workflow reports issues without a failing exit code, so these
// boundaries are enforced through Go tests.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
package layout
