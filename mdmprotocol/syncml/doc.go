// Package syncml is the typed, DTD-ordered SyncML 1.2 codec for the OMA DM subset Windows uses.
//
// # Design
//
// Phase 2 of the implementation plan fills this package: one Go type per
// element in DTD order, a sealed command interface, entity-escaped Data with
// no CDATA, the Windows alert and status vocabularies as constants, and
// large-object primitives. It performs no I/O. Until Phase 2 the package is
// empty.
//
// # References
//
//   - Decision record 0001: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0001-architecture.md
//   - Implementation plan, Phase 2: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/implementation_plan.md
//   - OMA SyncML RepPro 1.2.2: https://www.openmobilealliance.org/release/Common/V1_2_2-20090724-A/OMA-TS-SyncML-RepPro-V1_2_2-20090724-A.pdf
//   - Microsoft MS-MDM: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-mdm/33769a92-ac31-47ef-ae7b-dc8501f7104f
package syncml
