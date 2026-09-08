package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
)

func TestPushConfiguration(t *testing.T) {
	for _, cfg := range []PushConfig{
		{ClientID: "id", ClientSecret: "secret"},
		{PFN: "pfn", ClientID: "id"},
		{PFN: "pfn", ClientSecret: "secret"},
		{PFN: "pfn", ClientID: "id", ClientSecret: "secret", Auth: "unknown"},
		{PFN: "pfn", ClientID: "id", ClientSecret: "secret", Auth: "entra"},
	} {
		if _, err := cfg.PushSender(); err == nil {
			t.Error("invalid push configuration accepted")
		}
	}
	for _, cfg := range []PushConfig{
		{}, {PFN: "pfn"},
		{PFN: "pfn", ClientID: "id", ClientSecret: "secret", Auth: "legacy"},
		{PFN: "pfn", ClientID: "id", ClientSecret: "secret", Auth: "entra", TenantID: "tenant"},
	} {
		if _, err := cfg.PushSender(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAppWiresOptionalPushWithDefaultProvider(t *testing.T) {
	cfg := Config{Store: sqlstore.SQLite, DSN: filepath.Join(t.TempDir(), "app.db"), BaseURL: "https://localhost:8443", Role: RoleAll, EnrollAllowAny: true, Push: PushConfig{PFN: "pfn", ClientID: "id", ClientSecret: "secret", Auth: "legacy"}}
	a, err := New(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	if a.Push.Sender == nil || a.Config.ProviderID != "go-microsoft-dm" {
		t.Fatal("push composition missing")
	}
	// Reject incomplete credentials before opening another database or serving enrollment.
	cfg.Push.ClientSecret = ""
	if invalid, err := New(context.Background(), cfg, Options{}); err == nil {
		_ = invalid.Close()
		t.Fatal("app accepted incomplete WNS credentials")
	}
}
