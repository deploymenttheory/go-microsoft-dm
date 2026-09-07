// Package paging defines cursor-page requests, generic results and shared
// page-size bounds.
//
// # Design
//
// Page and Result are transport-neutral values used by multiple storage and
// service contracts. Size applies the default and maximum before allocation or
// querying, keeping limits consistent across backends. The package does not
// define ordering, query filters or cursor encoding; each issuing backend owns
// those semantics. Callers should treat cursors as opaque.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Decision record 0004: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0004-go-baseline-and-foundation-contracts.md
package paging
