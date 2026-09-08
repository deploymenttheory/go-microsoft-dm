package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/pki/ca"
	"github.com/deploymenttheory/go-microsoft-dm/server/httpapi"
	"github.com/deploymenttheory/go-microsoft-dm/server/service"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
)

func newService(t *testing.T) *service.Service {
	t.Helper()
	store, err := sqlstore.Open(context.Background(), sqlstore.SQLite, "file::memory:?cache=shared", sqlstore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	authority, err := ca.NewSelfSigned(ca.SelfSignedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := service.New(service.Config{
		BaseURL: "https://mdm.test", CA: authority, Store: store, Queue: store.Queue(),
		Authenticator: &service.StaticAuthenticator{AllowAny: true}, Clock: clock.Real{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestRoutesAndAccessLog(t *testing.T) {
	t.Parallel()
	var logged []string
	h := httpapi.New(newService(t))
	h.Log = func(method, path string, status int) { logged = append(logged, method+" "+path) }

	// GET the discovery probe -> 200 empty.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, enroll.DiscoveryPath, nil))
	if rec.Code != http.StatusOK {
		t.Errorf("discovery GET = %d", rec.Code)
	}
	// POST a malformed discovery -> the enroll handler answers a 500 fault.
	rec = httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, enroll.DiscoveryPath, strings.NewReader("<s:Envelope"))
	req.Header.Set("Content-Type", "application/soap+xml")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("malformed discovery = %d", rec.Code)
	}
	// An unknown path -> 404.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/nowhere", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown path = %d", rec.Code)
	}
	if len(logged) != 3 {
		t.Errorf("access log entries = %v", logged)
	}
	// Without a log hook the handler still serves.
	h2 := httpapi.New(newService(t))
	rec = httptest.NewRecorder()
	h2.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, enroll.DiscoveryPath, nil))
	if rec.Code != http.StatusOK {
		t.Errorf("no-log GET = %d", rec.Code)
	}
}
