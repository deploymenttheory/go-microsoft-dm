// Package ratelimit implements optional atomic GCRA quotas with bounded,
// expiring state.
//
// # Design
//
// Callers choose route families, bucket keys and quotas. Each decision accounts
// for all buckets or none, using transactional store time. Keys are hashed,
// capacity is bounded, and typed unavailable errors distinguish storage/capacity
// failures from quota exhaustion. A decision can carry a retry delay.
//
// The limiter is disabled unless configured. It does not choose enrollment
// admission policy or trust forwarded addresses. The reference server
// configures peer and aggregate buckets on the discovery, enrollment and
// management endpoints and serializes capacity accounting within its limiter
// namespace. Do not include raw credentials in bucket keys.
//
// # References
//
//   - Decision record 0004: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0004-go-baseline-and-foundation-contracts.md
//   - RFC 6585 section 4 (429), RFC 9110 section 10.2.3 (Retry-After)
//   - Research store, section 7 (aggressive polling row: 333 requests per second at 20k hosts): https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research.md
package ratelimit
