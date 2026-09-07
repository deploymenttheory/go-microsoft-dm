// Package clock abstracts time with a real clock and a manually advanced
// concurrent-safe fake.
//
// # Design
//
// Injected time makes certificate validity (NotBefore is always "now", never
// back-dated), MD5 nonce lifetimes, WNS token expiry, poll backoff and other
// time-dependent behavior testable without wall-clock delays. Fake coordinates
// Now, After and manual advancement under a lock. Scheduling and retry policy
// remain with callers; the package's timer interface is limited to After.
//
// # References
//
//   - Decision record 0004: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0004-go-baseline-and-foundation-contracts.md
//   - Research store, section 7 (back-dated certificate and polling rows): https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research.md
package clock
