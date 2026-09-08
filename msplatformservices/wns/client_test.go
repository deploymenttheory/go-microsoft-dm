package wns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
)

type tokenFunc func(context.Context, bool) (string, error)

func (f tokenFunc) Token(ctx context.Context, refresh bool) (string, error) { return f(ctx, refresh) }

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Keep public URL validation intact; only the test transport routes to httptest.
func testHTTP(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	u, _ := url.Parse(s.URL)
	return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		copy := r.Clone(r.Context())
		copy.URL.Scheme, copy.URL.Host = u.Scheme, u.Host
		return http.DefaultTransport.RoundTrip(copy)
	})}
}

func TestSendResponses(t *testing.T) {
	for _, tc := range []struct {
		code    int
		status  string
		outcome Outcome
	}{
		{200, "received", Accepted}, {200, "dropped", Rejected}, {200, "channelthrottled", Throttled},
		{400, "", Rejected}, {401, "", Unauthorized}, {403, "", Forbidden}, {404, "", DeadChannel},
		{405, "", Rejected}, {406, "", Throttled}, {410, "", DeadChannel}, {413, "", TooLarge},
		{500, "", Transient}, {503, "", Transient}, {302, "", Rejected},
	} {
		t.Run(fmt.Sprintf("%d/%s", tc.code, tc.status), func(t *testing.T) {
			calls, refreshed := 0, 0
			client := testHTTP(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				body, _ := io.ReadAll(r.Body)
				if r.Method != "POST" || string(body) != "wake" || r.ContentLength != 4 || len(r.TransferEncoding) != 0 || r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("X-WNS-Type") != "wns/raw" || r.Header.Get("Content-Type") != "application/octet-stream" || r.Header.Get("X-WNS-Cache-Policy") != "cache" || r.Header.Get("X-WNS-TTL") != "30" || r.Header.Get("MS-CV") != "cv" {
					t.Error("invalid WNS wire request")
				}
				w.Header().Set("X-WNS-Status", tc.status)
				w.Header().Set("X-WNS-DeviceConnectionStatus", "connected")
				w.Header().Set("Retry-After", "7")
				w.Header().Set("Location", "https://example.org/steal")
				w.WriteHeader(tc.code)
			})
			c, err := New(Config{HTTP: client, Tokens: tokenFunc(func(_ context.Context, refresh bool) (string, error) {
				if refresh {
					refreshed++
				}
				return "token", nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			ttl := 30
			r, err := c.Send(context.Background(), "https://cloud.notify.windows.com/channel", Notification{Body: []byte("wake"), TTLSeconds: &ttl, CorrelationVector: "cv"})
			if r.Outcome != tc.outcome || r.DeviceConnectionStatus != "connected" || r.RetryAfter != 7*time.Second || (err == nil) != (tc.outcome == Accepted) {
				t.Fatalf("result=%+v err=%v", r, err)
			}
			want := 1
			if tc.code == 401 {
				want = 2
			}
			if calls != want || refreshed != want-1 {
				t.Fatalf("calls=%d refreshes=%d", calls, refreshed)
			}
		})
	}
}

func TestInvalidPushDoesNotAuthenticate(t *testing.T) {
	c, _ := New(Config{Tokens: tokenFunc(func(context.Context, bool) (string, error) {
		t.Fatal("authentication on invalid input")
		return "", nil
	})})
	for _, uri := range []string{"http://cloud.notify.windows.com/x", "https://notify.windows.com.evil.test/x", "https://evilnotify.windows.com/x", "https://user@cloud.notify.windows.com/x", "https://cloud.notify.windows.com:444/x", "https://cloud.notify.windows.com/x#fragment", "https://localhost/x"} {
		if _, err := c.Send(context.Background(), uri, Notification{Body: []byte{0}}); !errors.Is(err, ErrConfig) {
			t.Fatalf("accepted %s", uri)
		}
	}
	for _, body := range [][]byte{nil, make([]byte, 5001)} {
		if _, err := c.Send(context.Background(), "https://cloud.notify.windows.com/x", Notification{Body: body}); !errors.Is(err, ErrConfig) {
			t.Fatal(err)
		}
	}
}

func TestRetryAfterAndCancellation(t *testing.T) {
	clk := clock.NewFake(time.Unix(100, 0))
	var calls atomic.Int32
	h := testHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(406)
		} else {
			w.WriteHeader(200)
		}
	})
	c, _ := New(Config{Tokens: tokenFunc(func(context.Context, bool) (string, error) { return "t", nil }), HTTP: h, Clock: clk, Retries: 1})
	done := make(chan error, 1)
	go func() {
		_, err := c.Send(context.Background(), "https://cloud.notify.windows.com/x", Notification{Body: []byte{0}})
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for clk.Pending() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if clk.Pending() == 0 {
		t.Fatal("sender did not schedule retry")
	}
	clk.Advance(4 * time.Second)
	if calls.Load() != 1 {
		t.Fatal("retried before Retry-After")
	}
	clk.Advance(time.Second)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("retry hung")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Send(ctx, "https://cloud.notify.windows.com/x", Notification{Body: []byte{0}}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestTokenSourcesCacheRefreshAndExpiry(t *testing.T) {
	for _, entra := range []bool{false, true} {
		t.Run(fmt.Sprint(entra), func(t *testing.T) {
			clk := clock.NewFake(time.Unix(100, 0))
			calls := 0
			h := testHTTP(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				scope := "notify.windows.com"
				path := "/accesstoken.srf"
				if entra {
					scope = "https://wns.windows.com/.default"
					path = "/tenant/oauth2/v2.0/token"
				}
				if r.URL.Path != path || r.Form.Get("scope") != scope || r.Form.Get("client_id") != "client" || r.Form.Get("client_secret") != "secret&value" || r.Form.Get("grant_type") != "client_credentials" {
					t.Error("incorrect token request")
				}
				fmt.Fprintf(w, `{"access_token":"token%d","token_type":"bearer","expires_in":100}`, calls)
			})
			cfg := Credentials{ClientID: "client", ClientSecret: "secret&value", TenantID: "tenant", HTTP: h, Clock: clk}
			var s *OAuthSource
			var err error
			if entra {
				s, err = NewEntra(cfg)
			} else {
				s, err = NewLegacy(cfg)
			}
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			for i := 0; i < 2; i++ {
				got, err := s.Token(ctx, false)
				if got != "token1" || err != nil {
					t.Fatal(got, err)
				}
			}
			if got, err := s.Token(ctx, true); got != "token2" || err != nil {
				t.Fatal(got, err)
			}
			clk.Advance(91 * time.Second)
			if got, err := s.Token(ctx, false); got != "token3" || err != nil {
				t.Fatal(got, err)
			}
		})
	}
}

func TestTokenFailures(t *testing.T) {
	for _, body := range []string{`oops`, `{}`, `{"access_token":"secret","token_type":"basic"}`, `{"access_token":"secret","token_type":"bearer","expires_in":0}`, `{"access_token":"secret","token_type":"bearer","expires_in":"invalid"}`} {
		h := testHTTP(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })
		s, _ := NewLegacy(Credentials{ClientID: "id", ClientSecret: "private", HTTP: h})
		_, err := s.Token(context.Background(), false)
		if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private") {
			t.Fatalf("error disclosed token response: %v", err)
		}
	}
	var calls int
	h := testHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Location", "https://evil.test")
		w.WriteHeader(307)
	})
	s, _ := NewLegacy(Credentials{ClientID: "id", ClientSecret: "private", HTTP: h})
	if _, err := s.Token(context.Background(), false); err == nil || calls != 1 {
		t.Fatal("token redirect followed")
	}
}

func TestConfigurationAndTransportFailures(t *testing.T) {
	for _, cfg := range []Config{{}, {Tokens: pushTokens(), Retries: -1}, {Tokens: pushTokens(), Backoff: -time.Second}} {
		if _, err := New(cfg); !errors.Is(err, ErrConfig) {
			t.Fatal("invalid sender configuration accepted")
		}
	}
	if _, err := NewLegacy(Credentials{}); !errors.Is(err, ErrConfig) {
		t.Fatal(err)
	}
	if _, err := NewEntra(Credentials{TenantID: "../evil"}); !errors.Is(err, ErrConfig) {
		t.Fatal(err)
	}
	secretErr := errors.New("private channel and credentials")
	c, _ := New(Config{Tokens: tokenFunc(func(context.Context, bool) (string, error) { return "", secretErr })})
	if _, err := c.Send(context.Background(), "https://cloud.notify.windows.com/x", Notification{Body: []byte{0}}); !errors.Is(err, secretErr) {
		t.Fatal(err)
	}
	for _, token := range []string{"", "bad\r\ntoken"} {
		c, _ = New(Config{Tokens: tokenFunc(func(context.Context, bool) (string, error) { return token, nil })})
		if _, err := c.Send(context.Background(), "https://cloud.notify.windows.com/x", Notification{Body: []byte{0}}); !errors.Is(err, ErrConfig) {
			t.Fatal(err)
		}
	}
	h := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return nil, secretErr })}
	c, _ = New(Config{Tokens: pushTokens(), HTTP: h})
	if _, err := c.Send(context.Background(), "https://cloud.notify.windows.com/private", Notification{Body: []byte{0}}); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("transport error disclosed channel", err)
	}
	s, _ := NewLegacy(Credentials{ClientID: "id", ClientSecret: "secret", HTTP: h})
	if _, err := s.Token(context.Background(), false); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("token transport error leaked", err)
	}
	err := (&ResponseError{Result: Result{HTTPStatus: 403, Outcome: Forbidden}}).Error()
	if !strings.Contains(err, "403") {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	r := classify(&http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {now.Add(time.Minute).Format(http.TimeFormat)}}}, now)
	if r.RetryAfter != time.Minute || r.Outcome != Throttled {
		t.Fatalf("date Retry-After: %+v", r)
	}
}

func pushTokens() TokenSource {
	return tokenFunc(func(context.Context, bool) (string, error) { return "token", nil })
}
