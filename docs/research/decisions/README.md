# Design decisions

These records describe the implemented design. Each decision identifies the relevant context,
contracts, rationale, constraints and verification evidence. Use the [template](TEMPLATE.md)
for new decisions; omit empty sections. Integrate amendments into the relevant record and keep
its number and filename stable so existing links continue to resolve.

The [architecture guide](../../architecture.md) summarizes how the decisions fit together.
Numbers are reserved by the [implementation plan](../../implementation_plan.md) so phases can
cross-reference before the files exist; a reserved row names the phase that writes it.

| Decision | Design | Phase |
|---|---|---|
| 0001 | [Library-first architecture with a generated schema core and WinDC as an extension of MDM](0001-architecture.md) | 0 |
| 0002 | [Pinned references and re-check triggers](0002-pinned-references.md) | 0 |
| 0003 | [Reference projects and dependency policy](0003-reference-projects-and-dependency-policy.md) | 0 |
| 0004 | [Go 1.27 baseline and foundation contracts](0004-go-baseline-and-foundation-contracts.md) | 1 |
| 0005 | [SyncML message model and XML policy](0005-syncml-message-model-and-xml-policy.md) | 2 |
| 0006 | [Schema generator over DDF v2](0006-schema-generator-over-ddf-v2.md) | 3 |
| 0007 | [Relationship to go-sdk-windowscsp](0007-relationship-to-go-sdk-windowscsp.md) | 3 |
| 0008 | [Enrollment protocol and the provisioning document](0008-enrollment-protocol-and-provisioning-document.md) | 4 |
| 0009 | [WSTEP CA and CSR handling](0009-wstep-ca-and-csr-handling.md) | 4 |
| 0010 | [Storage interfaces and the contract suite](0010-storage-interfaces-and-contract-suite.md) | 4 |
| 0011 | [Management session engine](0011-management-session-engine.md) | 5 |
| 0012 | [Command queue, results and retention](0012-command-queue-results-and-retention.md) | 5 |
| 0013 | [Scope and the user channel](0013-scope-and-user-channel.md) | 5 |
| 0014 | [SQL storage backends](0014-sql-storage-backends.md) | 6 |
| 0015 | [Reference server roles and configuration](0015-reference-server-roles-and-configuration.md) | 6 |
| 0016 | [`dmctl` structure](0016-dmctl-structure.md) | 6 |
| 0017 | [Conformance testing on a Windows host](0017-conformance-testing-on-windows-host.md) | 7 |
| 0018 | [WNS push and poll policy](0018-wns-push-and-poll-policy.md) | 8 |
| 0019 | Certificate renewal and recovery (reserved) | 9 |
| 0020 | SCEP and certificate delivery (reserved) | 9 |
| 0021 | Entra integration and federated enrollment (reserved) | 10 |
| 0022 | Token validation (reserved) | 10 |
| 0023 | Compliance reporting constraints (reserved) | 10 |
| 0024 | Enrollment attestation (reserved) | 11 |
| 0025 | WinDC linked enrollment (reserved) | 12 |
| 0026 | WinDC document model and state (reserved) | 12 |
| 0027 | WinDC desired-state engine (reserved) | 12 |
| 0028 | CSP operations library (reserved) | 13 |
| 0029 | WBXML (reserved) | 14 |
| 0030 | Telemetry seam (reserved) | 15 |
| 0031 | Event sinks and audit (reserved) | 15 |
| 0032 | Admin API and authorization (reserved) | 15 |
| 0033 | Secrets at rest (reserved) | 15 |
