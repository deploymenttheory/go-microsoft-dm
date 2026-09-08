// Package mdm runs the OMA DM management session: packages 1 to 4, authentication, the command queue contract and the response builder.
//
// # Design
//
// Service.Handle takes one SyncML request with its transport facts and
// returns the reply. It decodes and validates the message with
// mdmprotocol/syncml, finds or opens the Session (keyed by DeviceID and
// SessionID; a new session must be package 1, a continuing one must carry the
// next MsgID), identifies the device through an Authenticator (a client
// certificate that belongs to the enrollment is trusted; otherwise the
// SyncHdr Cred is verified as syncml:auth-md5 against the stored credential
// hash and the nonce issued for the device, or as syncml:auth-basic when the
// account was provisioned that way; a missing or wrong Cred is answered with
// 407 or 401 and a Chal carrying a fresh nonce; success is answered once with
// 212 and a Chal holding the nonce for the next session), records the
// package 1 facts (DevInfo, LoginStatus, SyncType, DevicePrepSync), routes
// client events and generic alerts to Hooks, files every Status and Results
// against the queued command it answers (large objects reassembled with 213
// and 1222), and then fills the reply with deliverable commands from the
// CommandQueue: in sequence order, user-scoped commands held until LoginStatus
// reports a user and refused for the wrong AVD SyncType, each command's items
// chunked with MoreData when they exceed the client's MaxObjSize, the whole
// message bounded by MaxMsgSize, Final on every message except while a large
// object is in flight or when answering with the "Next Message" shape.
//
// Command builders (NewAdd, NewReplace, NewDelete, NewGet, NewExec, NewAtomic,
// NewSequence) refuse what the Windows client refuses: a nested Atomic, a Get
// inside an Atomic, an Add followed by a Replace on one node inside an
// Atomic, and an Atomic that mixes device and user scope. Check applies the
// generated schema before a command is queued.
//
// Persistence is behind interfaces this package defines and the storage tier
// implements: CommandQueue, SessionStore and Authenticator. Results are
// stored per command, never as raw envelopes; an opt-in DebugRing keeps a
// bounded number of raw messages for troubleshooting. The package does not
// decide what to send: callers enqueue commands and the engine delivers them.
// It does not implement WBXML (415 until Phase 14), message signing
// (MS-Signature is parsed and reported), Entra token validation (Phase 10) or
// the multi-user AVD session policy.
//
// # References
//
//   - Decision record 0011: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0011-management-session-engine.md
//   - Decision record 0012: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0012-command-queue-results-and-retention.md
//   - Decision record 0013: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0013-scope-and-user-channel.md
//   - Microsoft MS-MDM: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mdm/33769a92-ac31-47ef-ae7b-dc8501f7104f
//   - OMA DM Protocol 1.2.1 sections 6 to 9: https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Protocol-V1_2_1-20080617-A.pdf
//   - OMA DM protocol support: https://learn.microsoft.com/en-us/windows/client-management/oma-dm-protocol-support
package mdm
