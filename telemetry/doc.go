// Package telemetry provides explicit OpenTelemetry configuration, bounded
// vocabularies and outbound HTTP measurement.
//
// # Design
//
// Config accepts metric/trace providers and defaults to no-op implementations
// without reading global providers. RoundTripper returns the original transport
// when unconfigured; otherwise it records bounded method, server, status and
// error categories. URL paths, query strings, bodies and error messages are
// excluded from metrics and spans: a WNS channel URI is a bearer credential in
// a path, and an OMA DM session URL carries the device identifier.
//
// Vocabulary maps values outside a fixed set to OtherValue, so a status code,
// alert type or CSP name a device sends cannot grow a metric series without
// bound. Consumers own SDKs, exporters, sampling and any slog bridge; the
// library installs none. Recording fakes in telemetry/telemetrytest let tests
// inspect emitted attributes and verify that secrets are absent.
//
// # References
//
//   - Decision record 0004: https://github.com/deploymenttheory/go-microsoft-dm/blob/main/docs/research/decisions/0004-go-baseline-and-foundation-contracts.md
//   - OpenTelemetry: https://opentelemetry.io/docs/languages/go/libraries/
//   - OpenTelemetry: https://opentelemetry.io/docs/specs/semconv/http/http-metrics/
//   - OpenTelemetry: https://opentelemetry.io/docs/specs/otel/versioning-and-stability/
//   - Microsoft, WNS request and response headers: https://learn.microsoft.com/en-us/windows/apps/develop/notifications/push-notifications/push-request-response-headers
package telemetry
