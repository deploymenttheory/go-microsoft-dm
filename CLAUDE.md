# go-microsoft-dm

Go libraries for Windows 11 device management: MS-MDE2 enrollment, OMA DM over SyncML
(MS-MDM), the DDF-generated CSP schema, WNS push and Windows declared configuration, with a
reference server. Read [CONTRIBUTING.md](CONTRIBUTING.md) for editorial and validation
requirements, [docs/architecture.md](docs/architecture.md) for the implemented design, and
[docs/implementation_plan.md](docs/implementation_plan.md) for what comes next. The research
store in [docs/research.md](docs/research.md) holds the normative detail; when it and the plan
disagree, the research store wins and the plan is fixed.

## Modules and dependencies

The root module is `github.com/deploymenttheory/go-microsoft-dm`; `server/` is
`github.com/deploymenttheory/go-microsoft-dm/server`. Both use Go 1.27. The server depends on
the library. Library code and tests must not import the server. `go.work` supports local
development. `internal/layout` enforces the tier order (foundation, schema, mdmprotocol, pki,
msplatformservices, storage, simulator, server, app) and its explicit exceptions, of which
there are none.

Fleet, local-mdm, Mattrax, MDMatador, NachoMDM and the other repositories `make refs` clones
under `third_party/refs` are read-only references, never code dependencies, and several carry
no licence or a GPL licence. `deploymenttheory/go-sdk-windowscsp` is a reference, not a
dependency. `smallstep/pkcs7` and `smallstep/scep` are the candidate exceptions and each needs
a decision record before its first import. Read at least two references before implementing a
feature, record what they do and where they fail in a decision record, and prove the
improvement with a failing-path test. Do not copy third-party code.

## Pinned references and generated files

Decision record 0002 pins the specifications and the DDF bundle. `make specs` downloads the
specifications into `third_party/specs` and checks them against the manifest; `make verify`
checks the checked-in DDF bundle against `third_party/ddf/MANIFEST.json`. From Phase 3,
`make generate` regenerates `schema/` from the bundle. Never hand-edit `*.gen.go`,
`schema/EXPORTED_IDENTIFIERS.lock` or `schema/GENERATED_FROM.json`. Preserve Microsoft's and
OMA's exact protocol identifiers: `w7` parameter names, CSP node names, alert `Type` strings,
SOAP action URIs, status codes.

## Design and comments

Use the current-design format in [docs/research/decisions/TEMPLATE.md](docs/research/decisions/TEMPLATE.md)
for significant decisions. Integrate amendments without changing decision filenames or numbers.
Explain behavior, contracts, rationale and limitations; omit implementation chronology and
comparative claims. Keep package comments in `doc.go` with a concise summary, a `# Design`
section that says what the package does not do, and `# References`. Configuration is `DM_*`;
`MDM` names the protocol.

## Checks

- `make verify`: DDF pin and, from Phase 3, deterministic regeneration.
- `make test`: both modules with race detection and coverage.
- `make lint`: golangci-lint in both modules; use `--fix=false` when a review must preserve formatting.
- `make fuzz-smoke`: bounded fuzz runs; `make coverage`: 95% overall and per non-exempt
  package, exemptions in `scripts/coverage-exempt.txt`.
- `make ci`: everything above in order.
- `make test-conformance-guest`: a real Windows guest through guestweave (Phase 7; skipped
  unless `WEAVE_URL` and `WEAVE_TOKEN` are set).

Add failure-path coverage for every exported function, and a named test for every pitfall
row a phase claims to address. Use Conventional Commits; release-please owns release versions
and changelogs. See `make help` for the complete target list.
