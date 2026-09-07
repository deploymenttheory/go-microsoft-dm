# Contributing

Report bugs and propose changes through the [issue templates](https://github.com/deploymenttheory/go-microsoft-dm/issues/new/choose).
Follow the [code of conduct](CODE_OF_CONDUCT.md). Report vulnerabilities through the process in
[SECURITY.md](SECURITY.md).

## Development and validation

Use Go 1.27. The workspace contains two modules: the root library and `server`. The library
must not import the server, including in tests. The package import constraints are documented
in [architecture.md](docs/architecture.md) and enforced by `internal/layout`.

Run checks appropriate to the change. `make verify` checks the pinned DDF bundle and, from
Phase 3, generated output; `make test` runs both modules with the race detector; `make lint`
runs golangci-lint in both modules. Storage changes require the shared contract suite. Protocol
changes require simulator scenarios and failure-path tests. `make help` describes every target.
The coverage floor is 95% overall and per non-exempt package; exemptions are listed in
[scripts/coverage-exempt.txt](scripts/coverage-exempt.txt).

For a review that must preserve formatting, run golangci-lint with `--fix=false` in each module.
The checked-in configuration otherwise enables automatic fixes.

## Sources and references

The research store in [docs/research.md](docs/research.md) is the source of normative detail;
[decision record 0002](docs/research/decisions/0002-pinned-references.md) pins the revision of
each specification and the DDF bundle. Before implementing a feature, read at least two of the
reference implementations `make refs` clones, record what they do and where they fail in a
decision record, and add a failing-path test for the failure. Reference code is read-only;
several repositories carry no licence or a GPL licence. Do not copy it.

## Documentation and design

Describe current behavior in direct, neutral English. Explain contracts, operational
requirements, limitations and useful design rationale. Distinguish what MS-MDE2, MS-MDM, OMA DM
or a Learn page requires from project policy. Preserve exact Go identifiers and wire values:
`w7` parameter names, CSP node names, alert `Type` strings, SOAP actions, status codes.
Avoid development milestones, competitive claims and comments that repeat the code.

Use a package comment in `doc.go`: a `Package` summary, a `# Design` section that also states
what the package does not do, and `# References`. Function comments describe inputs, results,
errors and significant side effects or concurrency requirements.

Document significant design decisions using the [decision template](docs/research/decisions/TEMPLATE.md).
Integrate amendments into the current decision and preserve its number and filename. Update
[architecture.md](docs/architecture.md) when behavior changes, and the status table in
[docs/implementation_plan.md](docs/implementation_plan.md) when a phase completes.

## Pull requests

Use Conventional Commit titles. Explain the problem, resulting behavior, relevant tradeoffs
and validation. Link related issues and design decisions. Keep dependencies justified and
include migration requirements for persistent or protocol state. Release-please manages
versions and changelogs; ordinary documentation changes do not change release metadata.
