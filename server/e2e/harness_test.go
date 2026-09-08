//go:build e2e

package e2e_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/server/internal/app"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
)

// lazyHandler lets the httptest server exist before the app handler, so the
// app can be built with the server's own URL as its BaseURL.
type lazyHandler struct {
	mu sync.RWMutex
	h  http.Handler
}

func (l *lazyHandler) set(h http.Handler) { l.mu.Lock(); l.h = h; l.mu.Unlock() }

func (l *lazyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.mu.RLock()
	h := l.h
	l.mu.RUnlock()
	if h == nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	h.ServeHTTP(w, r)
}

// harness is a running reference server with its TLS test endpoint.
type harness struct {
	srv *httptest.Server
	app *app.App
}

// newHarness starts the server on the store named by E2E_STORE (default
// sqlite): "sqlite" uses a temp file, "inmem"/"memory" an ephemeral sqlite,
// "postgres" the DSN in E2E_POSTGRES_DSN (skipping when unset).
func newHarness(t *testing.T) *harness {
	t.Helper()
	cfg := app.Config{
		BaseURL: "https://placeholder", ProviderID: "e2e", Name: "e2e",
		Role: app.RoleAll, EnrollAllowAny: true,
	}
	switch store := os.Getenv("E2E_STORE"); store {
	case "", "sqlite":
		cfg.Store = sqlstore.SQLite
		cfg.DSN = "file:" + filepath.Join(t.TempDir(), "e2e.db") + "?_pragma=busy_timeout(5000)"
	case "inmem", "memory":
		cfg.Store, cfg.Memory = sqlstore.SQLite, true
	case "postgres":
		dsn := os.Getenv("E2E_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("E2E_STORE=postgres but E2E_POSTGRES_DSN is not set")
		}
		cfg.Store, cfg.DSN = sqlstore.Postgres, dsn
	default:
		t.Fatalf("unknown E2E_STORE %q", store)
	}

	lazy := &lazyHandler{}
	srv := httptest.NewTLSServer(lazy)
	t.Cleanup(srv.Close)
	cfg.BaseURL = srv.URL

	a, err := app.New(context.Background(), cfg, app.Options{})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	lazy.set(a.Handler)
	return &harness{srv: srv, app: a}
}

// client returns an HTTP client trusting the test server's certificate.
func (h *harness) client() *http.Client { return h.srv.Client() }
