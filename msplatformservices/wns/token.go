package wns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
)

// ErrConfig reports invalid push configuration or input.
var ErrConfig = errors.New("wns: invalid configuration")

// TokenSource supplies a bearer token. refresh requests replacement after a 401.
type TokenSource interface {
	Token(ctx context.Context, refresh bool) (string, error)
}

// Credentials selects the legacy MDM or Entra Windows App SDK token flow.
// Entra authentication does not imply DMClient supports that package identity.
type Credentials struct {
	ClientID     string
	ClientSecret string
	TenantID     string
	HTTP         *http.Client
	Clock        clock.Clock
}

// OAuthSource caches tokens until shortly before their reported expiry.
type OAuthSource struct {
	mu                     sync.Mutex
	cfg                    Credentials
	endpoint, scope, token string
	expires                time.Time
}

// NewLegacy authenticates a Partner Center package SID and secret.
func NewLegacy(cfg Credentials) (*OAuthSource, error) {
	return newOAuth(cfg, "https://login.live.com/accesstoken.srf", "notify.windows.com")
}

// NewEntra authenticates a tenant app using the Windows App SDK WNS scope.
func NewEntra(cfg Credentials) (*OAuthSource, error) {
	if cfg.TenantID == "" || strings.ContainsAny(cfg.TenantID, "/\\?#% \t\r\n") {
		return nil, fmt.Errorf("%w: tenant ID required", ErrConfig)
	}
	return newOAuth(cfg, "https://login.microsoftonline.com/"+cfg.TenantID+"/oauth2/v2.0/token", "https://wns.windows.com/.default")
}

func newOAuth(cfg Credentials, endpoint, scope string) (*OAuthSource, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("%w: client ID and secret required", ErrConfig)
	}
	cfg.HTTP = noRedirects(cfg.HTTP)
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	return &OAuthSource{cfg: cfg, endpoint: endpoint, scope: scope}, nil
}

// Token implements TokenSource. Failed responses never disclose credentials or bodies.
func (s *OAuthSource) Token(ctx context.Context, refresh bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !refresh && s.token != "" && s.cfg.Clock.Now().Before(s.expires) {
		return s.token, nil
	}
	s.token = ""
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {s.cfg.ClientID}, "client_secret": {s.cfg.ClientSecret}, "scope": {s.scope}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.cfg.HTTP.Do(req)
	if err != nil {
		return "", safeTransportError(ctx, "token request", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wns: token endpoint returned HTTP %d", resp.StatusCode)
	}
	var wire struct {
		AccessToken string      `json:"access_token"`
		TokenType   string      `json:"token_type"`
		ExpiresIn   json.Number `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&wire); err != nil {
		return "", errors.New("wns: invalid token response")
	}
	if wire.AccessToken == "" || strings.ContainsAny(wire.AccessToken, "\r\n") || !strings.EqualFold(wire.TokenType, "bearer") {
		return "", errors.New("wns: invalid bearer token")
	}
	// Some legacy responses omit expires_in. Use that token once, without
	// inventing a lifetime; a subsequent call obtains a fresh token.
	lifetime := time.Duration(0)
	if wire.ExpiresIn != "" {
		seconds, err := wire.ExpiresIn.Int64()
		if err != nil || seconds <= 0 || seconds > int64((1<<63-1)/time.Second) {
			return "", errors.New("wns: invalid token lifetime")
		}
		lifetime = time.Duration(seconds) * time.Second
		margin := min(time.Minute, lifetime/10)
		lifetime -= margin
	}
	s.token, s.expires = wire.AccessToken, s.cfg.Clock.Now().Add(lifetime)
	return s.token, nil
}

func noRedirects(client *http.Client) *http.Client {
	clone := http.Client{Timeout: 30 * time.Second}
	if client != nil {
		clone = *client
	}
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &clone
}

func safeTransportError(ctx context.Context, operation string, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// url.Error includes the channel's capability token. Do not expose it.
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("wns: %s failed", operation)
	}
	return fmt.Errorf("wns: %s failed", operation)
}
