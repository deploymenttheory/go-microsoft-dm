package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/pki/ca"
	"github.com/deploymenttheory/go-microsoft-dm/schema/registry"
	"github.com/deploymenttheory/go-microsoft-dm/server/httpapi"
	"github.com/deploymenttheory/go-microsoft-dm/server/pushnotify"
	"github.com/deploymenttheory/go-microsoft-dm/server/service"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
)

// ErrConfig reports an unusable configuration.
var ErrConfig = errors.New("app: invalid configuration")

// App is the composed reference server.
type App struct {
	Handler http.Handler
	Service *service.Service
	Store   *sqlstore.Store
	CA      *ca.Local
	// Config is the effective configuration.
	Config Config
	Push   *pushnotify.Service

	closers []func() error
}

// Options tune New (for tests).
type Options struct {
	// Clock; default real time.
	Clock clock.Clock
	// Log receives access-log lines; nil disables the access log.
	Log func(method, path string, status int)
}

// New opens the store, builds the CA and composes the service.
func New(ctx context.Context, cfg Config, opts Options) (*App, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if cfg.ProviderID == "" {
		cfg.ProviderID = "go-microsoft-dm"
	}
	clk := opts.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	dsn := cfg.DSN
	if cfg.Memory {
		dsn = "file::memory:?cache=shared"
	}
	store, err := sqlstore.Open(ctx, cfg.Store, dsn, sqlstore.Options{Clock: clk})
	if err != nil {
		return nil, fmt.Errorf("app: open store: %w", err)
	}
	authority, err := buildCA(cfg, clk)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	reg := registry.Registry()
	if cfg.DisableSchemaValidation {
		reg = nil
	}
	svc, err := service.New(service.Config{
		BaseURL: cfg.BaseURL,
		CA:      authority,
		Store:   store,
		Queue: &pushnotify.TrackingQueue{
			CommandQueue: store.Queue(), Store: store, ProviderID: cfg.ProviderID,
		},
		Authenticator: &service.StaticAuthenticator{
			Users:    cfg.EnrollUsers,
			AllowAny: cfg.EnrollAllowAny,
		},
		Registry:       reg,
		ProviderID:     cfg.ProviderID,
		PushPFN:        cfg.Push.PFN,
		Name:           cfg.Name,
		Clock:          clk,
		AllowBasicAuth: false,
	})
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("app: build service: %w", err)
	}
	h := httpapi.New(svc)
	h.Log = opts.Log
	sender, err := cfg.Push.PushSender()
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	push := &pushnotify.Service{Store: store, Clock: clk}
	if sender != nil {
		push.Sender = sender
	}
	return &App{
		Push:    push,
		Handler: h, Service: svc, Store: store, CA: authority, Config: cfg,
		closers: []func() error{store.Close},
	}, nil
}

// Close releases resources.
func (a *App) Close() error {
	var err error
	for i := len(a.closers) - 1; i >= 0; i-- {
		if e := a.closers[i](); e != nil {
			err = e
		}
	}
	return err
}

// buildCA loads the CA from files or generates an ephemeral one.
func buildCA(cfg Config, clk clock.Clock) (*ca.Local, error) {
	if cfg.CACert != "" && cfg.CAKey != "" {
		l, err := ca.LoadFiles(cfg.CACert, cfg.CAKey)
		if err != nil {
			return nil, fmt.Errorf("%w: load CA: %w", ErrConfig, err)
		}
		return l, nil
	}
	if cfg.CACert != "" || cfg.CAKey != "" {
		return nil, fmt.Errorf("%w: set both DM_CA_CERT and DM_CA_KEY, or neither", ErrConfig)
	}
	l, err := ca.NewSelfSigned(ca.SelfSignedOptions{Clock: clk})
	if err != nil {
		return nil, fmt.Errorf("%w: generate CA: %w", ErrConfig, err)
	}
	return l, nil
}
