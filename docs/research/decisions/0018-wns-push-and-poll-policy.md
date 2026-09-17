# 0018: WNS push and poll policy

## Decision

`msplatformservices/wns` owns OAuth token acquisition and raw WNS delivery.
Callers supply credentials, HTTP transport, clock and retry count. The reference
server opts in through `DM_WNS_*`; the library has no package identity, hidden
worker or polling policy. Partner Center SID/secret is the documented MDM path.
The separate Entra token source implements the Windows App SDK scope; it is
not evidence that Entra credentials can wake a PFN-based DMClient enrollment.

Validate HTTPS and the `notify.windows.com` domain boundary before requesting
a token. Reject userinfo, fragments and nonstandard ports. Disable redirects on
token and push requests, even when a caller supplies an HTTP client. Neither
channel capability URLs nor credentials enter errors or push event records.

Send a non-empty raw body with explicit Content-Length, octet-stream content
type and cache policy. TTL and correlation vector are caller options. Refresh
once after 401; never retry 403, 404, 410 or 413. Retry throttling and 5xx only
within the configured limit, honoring Retry-After and context cancellation.
The reference server allows two retries and a five-minute cached wake payload.
HTTP 200 with `dropped` is rejected and `channelthrottled` is throttled; WNS
acceptance is always distinct from a subsequent authenticated management session.

## Channel lifecycle

Optional Push/PFN provisioning preserves the existing Microsoft poll schedule.
Every authenticated management session queues separate ChannelURI and Status
Gets, regardless of whether first-session device-detail reads already completed.
The reference queue observer accepts only successful correlated Get results.
Channel state is keyed by enrollment certificate serial, so reenrollment starts
without the old channel. The additive `push_channels` table works with the
existing SQLite/PostgreSQL/MySQL schema setup.

First observation starts an estimated channel age. Re-reading an unchanged URI
does not reset that age. A different URI starts a new observation period. At
15 days delivery events flag renewal due; at 30 days the server stops sending.
Actual channel creation may predate observation, so WNS 404/410 remains
authoritative. Mark a failed URI dead conditionally, preserving concurrent
renewal. Seeing that same dead URI again does not revive it; polling continues.

Authenticated package one updates LastSeenAt. `dmctl checkins <duration>` reports
and logs active enrollments that exceeded the operator's threshold. Schedule
that command externally; alerts are observations, not a claim about the device's
local service state. Push delivery never cancels queued work or changes polling.

## Verification and limits

Tests cover token caching/expiry, both scopes, redirect refusal, invalid inputs,
response classes, retry timing, correlated channel updates, renewal races and
reenrollment isolation. The simulator receives a test push by initiating a real
authenticated session against the reference application and acknowledging work.

The [Windows procedure](../../testing/windows-wns-push.md) uses the local desktop.
The headed Windows 11 VM enrolled with the Partner Center PFN.
Native Push/Status was 0, the server captured a channel, and a queued Get
completed in an authenticated session. The first credentialed push did not
reach delivery: `login.live.com/accesstoken.srf` returned HTTP 400. A direct
guest request reported `invalid_request` and stated that client credential
flows against `login.live.com` are no longer supported for new clients. The
bare Package SID returned `invalid_client`. The Partner Center linked app
registration is Microsoft account only. Its newly created secret reached the
tenant-specific Entra endpoint, which returned `AADSTS9002346` and required
`/consumers`; `/consumers` then returned `AADSTS9002332` because the WNS
resource accepts Azure Active Directory users only. No token was issued.
The separate Entra token source remains unverified against this DMClient
channel; its Windows App SDK scope does not establish compatibility. A
supported credential migration or Partner Center repair needs confirmation
before further native delivery testing. Event 4603 and push-attributable
session arrival remain unverified. The native test skips explicitly without
credentials; it does not substitute simulator success for native evidence.
No VM or Windows service configuration change is required by automated suites.

A controlled audience test of the Partner Center linked app
registration also failed at Azure manifest validation. The portal rejected
`AzureADandPersonalMicrosoftAccount` because its `ms-app://<Package SID>`
identifier URI did not use a verified organizational domain. The unsaved edit
was discarded; the registration retained `PersonalMicrosoftAccount`. Changing
the Store identifier URI is outside this experiment because it may alter the
identity bound to the existing DMClient channel.

## References

- [MDM push and channel renewal](https://learn.microsoft.com/en-us/windows/client-management/push-notification-windows-mdm)
- [WNS authentication and channel validation](https://learn.microsoft.com/en-us/windows/apps/develop/notifications/push-notifications/wns-overview)
- [Request and response headers](https://learn.microsoft.com/en-us/windows/apps/develop/notifications/push-notifications/push-request-response-headers)
- [Windows App SDK push authentication](https://learn.microsoft.com/en-us/windows/apps/develop/notifications/push-notifications/push-quickstart)
- Pinned `third_party/ddf` DMClient definition and generated `schema/csp/dmclient`.

Phase 7 established the zero-retry
unbounded schedule on 26200.9278; it did not change the production poll defaults.
