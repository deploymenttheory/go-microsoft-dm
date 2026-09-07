// Package state defines transactional, expiring byte records for protocol state.
//
// # Design
//
// MD5 challenge nonces (renewed every DM session), WSTEP and SCEP one-time
// challenges, Entra token replay guards and quotas need atomic updates without
// importing database drivers. Memory supports a single process; the server
// module adds shared SQL persistence. Transactions expose store time so expiry
// and quota decisions can use a consistent clock. Callers own serialization,
// namespaces, key selection and record lifetimes. In-memory state is lost on
// restart.
//
// # References
//
//   - Decision record 0004: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0004-go-baseline-and-foundation-contracts.md
//   - OMA DM Security 1.2.1 section 5.3 (nonce renewal): https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Security-V1_2_1-20080617-A.pdf
package state
