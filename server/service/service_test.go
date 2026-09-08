package service_test

import (
	"context"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/pki/ca"
	"github.com/deploymenttheory/go-microsoft-dm/server/service"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
)

func testStore(t *testing.T) *sqlstore.Store {
	t.Helper()
	s, err := sqlstore.Open(context.Background(), sqlstore.SQLite, "file::memory:?cache=shared", sqlstore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func testCA(t *testing.T) *ca.Local {
	t.Helper()
	c, err := ca.NewSelfSigned(ca.SelfSignedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewValidatesConfig(t *testing.T) {
	t.Parallel()
	store := testStore(t)
	good := service.Config{
		BaseURL: "https://mdm.test", CA: testCA(t), Store: store, Queue: store.Queue(),
		Authenticator: &service.StaticAuthenticator{AllowAny: true}, Clock: clock.Real{},
	}
	if _, err := service.New(good); err != nil {
		t.Fatalf("good config rejected: %v", err)
	}
	bad := map[string]func(*service.Config){
		"no base url": func(c *service.Config) { c.BaseURL = "" },
		"http base":   func(c *service.Config) { c.BaseURL = "http://mdm.test" },
		"no ca":       func(c *service.Config) { c.CA = nil },
		"no store":    func(c *service.Config) { c.Store = nil },
		"no queue":    func(c *service.Config) { c.Queue = nil },
		"no auth":     func(c *service.Config) { c.Authenticator = nil },
	}
	for name, mutate := range bad {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := good
			mutate(&c)
			if _, err := service.New(c); err == nil {
				t.Errorf("%s accepted", name)
			}
		})
	}
}

func TestStaticAuthenticator(t *testing.T) {
	t.Parallel()
	a := &service.StaticAuthenticator{Users: map[string]string{"alice": "secret"}}
	ctx := context.Background()
	if p, err := a.Authenticate(ctx, enroll.Credentials{Username: "alice", Password: "secret"}); err != nil || p.UPN != "alice" {
		t.Errorf("valid = %+v, %v", p, err)
	}
	if _, err := a.Authenticate(ctx, enroll.Credentials{Username: "alice", Password: "wrong"}); err == nil {
		t.Error("wrong password accepted")
	}
	if _, err := a.Authenticate(ctx, enroll.Credentials{Username: "carol"}); err == nil {
		t.Error("unknown user accepted")
	}
	any := &service.StaticAuthenticator{AllowAny: true}
	if p, err := any.Authenticate(ctx, enroll.Credentials{Username: "anyone", Password: "x"}); err != nil || p.UPN != "anyone" {
		t.Errorf("allow-any = %+v, %v", p, err)
	}
}

func TestNewDefaultsAndStore(t *testing.T) {
	t.Parallel()
	store := testStore(t)
	// A config with no Clock exercises the clock default; Store() returns it.
	cfg := service.Config{
		BaseURL: "https://mdm.test", CA: testCA(t), Store: store, Queue: store.Queue(),
		Authenticator: &service.StaticAuthenticator{AllowAny: true},
	}
	svc, err := service.New(cfg)
	if err != nil {
		t.Fatalf("New with defaults: %v", err)
	}
	if svc.Store() != store {
		t.Error("Store() did not return the configured store")
	}
	// An out-of-range EnrollmentVersion is rejected by enroll.New, exercising
	// the dependency-construction error arm.
	bad := cfg
	bad.EnrollmentVersion = "1.0"
	if _, err := service.New(bad); err == nil {
		t.Error("invalid EnrollmentVersion accepted")
	}
}
