// Package syncml is the typed, DTD-ordered SyncML 1.2 codec for the OMA DM
// subset Windows uses.
//
// # Design
//
// One Go type per element the Windows client exchanges: Message, Header and
// Body; the commands Add, Alert, Atomic, Delete, Exec, Get, Replace, Results,
// Sequence and Status behind the sealed Command interface; Item, Data, Meta,
// Cred and Chal. Body keeps its commands in document order, because the
// client answers Status elements in the order it received the commands and a
// server must match them back. Encode writes elements in the order the SyncML
// Representation Protocol DTD prescribes, emits xmlns="SYNCML:SYNCML1.2" on
// the root and the syncml:metinf namespace on every Meta child, escapes Data
// as character data and never writes CDATA. Decode accepts SYNCML:SYNCML1.2
// and SYNCML:SYNCML1.1 roots (and no namespace, for the Learn fragments used
// as fixtures), normalises CDATA, preserves markup carried inside Data
// verbatim, tolerates the whitespace and missing metinf namespaces seen in
// Microsoft's own samples, and rejects the OMA commands Windows does not
// implement with a typed error. Copy, Map, Move, Put, Search and Sync are
// recognised and refused.
//
// Validate applies the rules the session engine needs before it acts on a
// message: version strings, a non-empty SessionID, a numeric MsgID, unique
// CmdIDs that are never "0", LocURI and node-name rules from the Learn OMA DM
// page, known alert and status codes, and the Atomic rules the Windows
// client enforces with 500 and 507 (no nested Atomic, no Get inside Atomic,
// no Add then Replace on one node). The MD5 helpers implement the OMA DM
// Security digest with the nonce as raw bytes and base64 on the wire. The
// large-object helpers split an Item into MoreData chunks and reassemble
// them. The response helpers build the "Next Message" and abort shapes.
//
// The package performs no I/O and holds no session state; it does not
// decide what to send, only how to say it. The session engine in
// mdmprotocol/mdm owns packages, authentication order and the command queue.
//
// # References
//
//   - Decision record 0005: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0005-syncml-message-model-and-xml-policy.md
//   - OMA SyncML RepPro 1.2.2 and its DTD: https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/OMA-TS-SyncML-RepPro-V1_2_2-20090724-A.pdf
//   - OMA DM Protocol 1.2.1 (sections 6, 7, 8 and 9): https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Protocol-V1_2_1-20080617-A.pdf
//   - OMA DM Security 1.2.1 (section 5.3): https://www.openmobilealliance.org/release/DM/V1_2_1-20080617-A/OMA-TS-DM_Security-V1_2_1-20080617-A.pdf
//   - Microsoft MS-MDM v15.0 (section 2.2): https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mdm/33769a92-ac31-47ef-ae7b-dc8501f7104f
//   - Microsoft, OMA DM protocol support: https://learn.microsoft.com/en-us/windows/client-management/oma-dm-protocol-support
package syncml
