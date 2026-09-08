package wns

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
)

// Outcome classifies a WNS response; Accepted is not proof of device check-in.
type Outcome string

const (
	Accepted     Outcome = "accepted"
	Unauthorized Outcome = "unauthorized"
	Forbidden    Outcome = "forbidden"
	DeadChannel  Outcome = "dead_channel"
	Throttled    Outcome = "throttled"
	TooLarge     Outcome = "too_large"
	Transient    Outcome = "transient"
	Rejected     Outcome = "rejected"
)

// Result preserves response metadata without the secret channel URI or token.
type Result struct {
	HTTPStatus             int
	Outcome                Outcome
	NotificationStatus     string
	DeviceConnectionStatus string
	MessageID              string
	CorrelationVector      string
	RetryAfter             time.Duration
}

// ResponseError reports a response which did not accept the notification.
type ResponseError struct{ Result Result }

func (e *ResponseError) Error() string {
	return fmt.Sprintf("wns: %s (HTTP %d)", e.Result.Outcome, e.Result.HTTPStatus)
}

// Config configures the sender. Retries apply only to throttling and 5xx.
// Zero retries makes a single attempt; a 401 always refreshes at most once.
type Config struct {
	Tokens  TokenSource
	HTTP    *http.Client
	Clock   clock.Clock
	Retries int
	Backoff time.Duration
}

// Client sends raw WNS notifications and is safe for concurrent use if its TokenSource is.
type Client struct{ cfg Config }

// New constructs a sender with redirect following disabled.
func New(cfg Config) (*Client, error) {
	if cfg.Tokens == nil || cfg.Retries < 0 || cfg.Retries > 10 || cfg.Backoff < 0 {
		return nil, ErrConfig
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.Backoff == 0 {
		cfg.Backoff = time.Second
	}
	cfg.HTTP = noRedirects(cfg.HTTP)
	return &Client{cfg: cfg}, nil
}

// ValidateChannel checks the documented Microsoft domain boundary before auth.
func ValidateChannel(channel string) error {
	u, err := url.Parse(channel)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Opaque != "" || (u.Port() != "" && u.Port() != "443") {
		return fmt.Errorf("%w: channel must be a Microsoft HTTPS URL", ErrConfig)
	}
	host := strings.ToLower(u.Hostname())
	if host != "notify.windows.com" && !strings.HasSuffix(host, ".notify.windows.com") {
		return fmt.Errorf("%w: channel host must be under notify.windows.com", ErrConfig)
	}
	return nil
}

// Notification is an opaque, non-empty raw payload, with optional TTL and MS-CV.
// TTLSeconds nil omits the header; zero requests immediate expiry.
type Notification struct {
	Body              []byte
	TTLSeconds        *int
	CorrelationVector string
}

// Send authenticates and sends, respecting cancellation during retry delays.
func (c *Client) Send(ctx context.Context, channel string, n Notification) (Result, error) {
	if err := ValidateChannel(channel); err != nil {
		return Result{}, err
	}
	if len(n.Body) == 0 || len(n.Body) > 5000 || (n.TTLSeconds != nil && *n.TTLSeconds < 0) || strings.ContainsAny(n.CorrelationVector, "\r\n") {
		return Result{}, fmt.Errorf("%w: invalid raw notification", ErrConfig)
	}
	refreshed, force := false, false
	retries := 0
	for {
		token, err := c.cfg.Tokens.Token(ctx, force)
		if err != nil {
			return Result{}, err
		}
		force = false
		if token == "" || strings.ContainsAny(token, "\r\n") {
			return Result{}, fmt.Errorf("%w: invalid token", ErrConfig)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, channel, bytes.NewReader(n.Body))
		if err != nil {
			return Result{}, fmt.Errorf("%w: invalid channel", ErrConfig)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("X-WNS-Type", "wns/raw")
		req.Header.Set("X-WNS-Cache-Policy", "cache")
		req.Header.Set("X-WNS-RequestForStatus", "true")
		if n.TTLSeconds != nil {
			req.Header.Set("X-WNS-TTL", strconv.Itoa(*n.TTLSeconds))
		}
		if n.CorrelationVector != "" {
			req.Header.Set("MS-CV", n.CorrelationVector)
		}
		resp, err := c.cfg.HTTP.Do(req)
		if err != nil {
			return Result{}, safeTransportError(ctx, "push request", err)
		}
		result := classify(resp, c.cfg.Clock.Now())
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
		if result.Outcome == Accepted {
			return result, nil
		}
		if result.Outcome == Unauthorized && !refreshed {
			refreshed, force = true, true
			continue
		}
		if (result.Outcome == Transient || result.Outcome == Throttled) && retries < c.cfg.Retries {
			delay := max(result.RetryAfter, c.cfg.Backoff*time.Duration(1<<retries))
			retries++
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-c.cfg.Clock.After(delay):
			}
			continue
		}
		return result, &ResponseError{Result: result}
	}
}

func classify(resp *http.Response, now time.Time) Result {
	r := Result{HTTPStatus: resp.StatusCode, Outcome: Rejected, NotificationStatus: resp.Header.Get("X-WNS-Status"), DeviceConnectionStatus: resp.Header.Get("X-WNS-DeviceConnectionStatus"), MessageID: resp.Header.Get("X-WNS-Msg-ID"), CorrelationVector: resp.Header.Get("MS-CV")}
	switch resp.StatusCode {
	case 200:
		if r.NotificationStatus == "channelthrottled" {
			r.Outcome = Throttled
		} else if r.NotificationStatus == "" || r.NotificationStatus == "received" {
			r.Outcome = Accepted
		}
	case 401:
		r.Outcome = Unauthorized
	case 403:
		r.Outcome = Forbidden
	case 404, 410:
		r.Outcome = DeadChannel
	case 406, 429:
		r.Outcome = Throttled
	case 413:
		r.Outcome = TooLarge
	default:
		if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
			r.Outcome = Transient
		}
	}
	if value := resp.Header.Get("Retry-After"); value != "" {
		if seconds, err := strconv.ParseInt(value, 10, 32); err == nil && seconds >= 0 {
			r.RetryAfter = time.Duration(seconds) * time.Second
		} else if at, err := http.ParseTime(value); err == nil {
			r.RetryAfter = max(0, at.Sub(now))
		}
	}
	return r
}
