# 0004: Go 1.27 baseline and foundation contracts

## Context

Both modules need a consistent toolchain, an explicit policy for XML and JSON at trust
boundaries, and a set of domain-free packages every higher tier can rely on without pulling in
protocol knowledge.

## Decision

Both modules declare `go 1.27.0` with no `toolchain` line; `GOTOOLCHAIN=auto` fetches the
declared version and CI derives it from `go.mod`. `make tools` builds golangci-lint and the
other developer tools with that version so they can load the modules.

XML uses `encoding/xml`. SyncML is marshalled in DTD order from typed structs with
`xml:",chardata"` data and never CDATA (Phase 2); SOAP envelopes are typed structs with
explicit namespace attributes (Phase 4). `mattrax/xml` is adopted only if a Phase 2 test shows
the standard encoder cannot produce a shape the Windows client requires. JSON at trust
boundaries (WinDC discovery, Entra tokens, the admin API) uses `encoding/json/v2` with strict
decoding; `encoding/json` remains for internal manifests and configuration.

Foundation contracts:

- `clock`: `Clock{Now, Since, After}`, `Real`, and a lock-coordinated `Fake` with `Advance`,
  `Set` and `Pending`. Certificate validity, nonce lifetimes, token expiry and poll backoff
  take a `Clock`; scheduling policy stays with callers.
- `paging`: `Page{Cursor, Limit}.Size()` bounded by `DefaultPageSize` 100 and `MaxPageSize`
  1000, and `Result[T]`. Ordering, filters and cursor encoding belong to each backend.
- `secrets`: a `Secret` that redacts every formatting and marshalling path and yields its
  value only through `Bytes`; `Static`, `Env`, `Dir` (rooted with `os.Root`) and `Chain`
  providers. WNS secrets, Entra client secrets, OMA DM MD5 credentials and PFX passwords flow
  through it in later phases. Sealing at rest is a server concern (Phase 15).
- `telemetry`: `Config` with explicit metric and trace providers defaulting to no-ops, an
  outbound `RoundTripper` recording bounded method, server, status and error attributes,
  and `Vocabulary` mapping unknown values to `OtherValue`. The library depends on the
  OpenTelemetry API modules (`otel`, `otel/metric`, `otel/trace` v1.46.0) and installs no SDK,
  exporter or logs bridge; consumers own those. `telemetrytest` records measurements and spans
  without the SDK. `ScopeRoot` is this module's path.
- `state`: transactional, expiring byte records (`Store`, `Tx`, `Record`, `ValidKey`) with a
  single-process `Memory` implementation. MD5 challenge nonces, WSTEP and SCEP one-time
  challenges and quota state use it; SQL persistence arrives with the server module.
- `ratelimit`: atomic GCRA quotas over `state.Store` with hashed keys, bounded capacity,
  typed unavailable errors, `PeerKey` with explicit proxy trust and an HTTP `Middleware`.
  Disabled unless configured; admission policy is the caller's.
- `testpki`: an ephemeral CA issuing MDM client identities and TLS server identities, plain
  PKCS#10 requests, and `WindowsCSR`, which encodes the subject as a PrintableString carrying
  the characters the Windows enrollment client sends (`WindowsCSRSubject`, with `!`) so the
  standard parser rejects it the way it rejects a real device's request. Test-only; its roots
  are never trusted outside tests.

Tests use no assertion library. Every exported function has a failure-path test, and
`ratelimit` keeps fault injection in its own `failure_test.go`.

## Rationale

A declared toolchain makes local and CI behavior reproducible. The standard XML encoder is
the first choice because every reference that hand-rolled SyncML with it works against the
real client; the fork is a fallback with evidence attached. Strict JSON rejects duplicate
names and invalid UTF-8 where ambiguity would affect validation. The foundation packages are
copied in shape from go-apple-dm (same organisation, MIT) because their contracts are protocol
neutral and already carry failure-path tests; adapting them costs less than rediscovering
their edge cases. `WindowsCSR` exists in Phase 1 so that the Phase 4 parser is written against
a failing test rather than a vendored 1,585-line copy of the standard library.

## Constraints

The minimum Go version is a build requirement. `encoding/json/v2` strictness must be
established at each decoding call site; an import alone does not make a custom unmarshaler
strict. `testpki.WindowsCSR` reproduces the one subject shape observed in the field (Fleet's
comment records `!` and `0x00`); a Phase 7 capture from a real 26100 guest will confirm or
extend it. In-memory `state` is lost on restart.

## Verification

`make test` runs both modules with the race detector; `make coverage` holds 95% per non-exempt
package. `telemetry`'s `TestChannelURIAndDeviceIDNeverReachTelemetry` proves a WNS channel URI
and a device identifier in a request URL reach neither a metric attribute nor a span name.
`testpki`'s `TestWindowsCSRIsRejectedByTheStandardLibrary` proves the standard parser rejects
the Windows subject while the same encoder's conforming subject parses and verifies.
`internal/layout` shows the seven packages in the foundation tier with `ratelimit -> state`
the only edge between them.

## References

- [go.mod](../../../go.mod), [server/go.mod](../../../server/go.mod), [Makefile](../../../Makefile) (`tools`)
- [clock](../../../clock), [paging](../../../paging), [secrets](../../../secrets), [telemetry](../../../telemetry), [state](../../../state), [ratelimit](../../../ratelimit), [testpki](../../../testpki)
- [Research store](../../research.md), section 4 (XML encoding pitfalls; x509 and CSR handling) and section 7
- OpenTelemetry versioning and stability: <https://opentelemetry.io/docs/specs/otel/versioning-and-stability/>

Reference source identifiers and paths (relative to the named project):

- `deploymenttheory/go-apple-dm`, `clock/`, `paging/`, `secrets/`, `telemetry/`, `state/`, `ratelimit/`, `testpki/`, `docs/research/decisions/0018-go-1.27-baseline.md`, `0040-opentelemetry-seam.md`
- `fleetdm/fleet`, `server/mdm/microsoft/wstep_csr.go:49-52,67-91` (the admitted characters and the example subject)
- `oscartbeaumont/windows_mdm`, `patch/patch.go`
- `mattrax/xml`, `marshal.go`, `read.go`
