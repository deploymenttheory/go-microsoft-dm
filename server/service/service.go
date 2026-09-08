package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/pki/ca"
	"github.com/deploymenttheory/go-microsoft-dm/pki/wstep"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// ErrConfig reports an unusable configuration.
var ErrConfig = errors.New("service: invalid configuration")

// Store is everything the service persists through: the durable contracts
// plus the device-facts and event log the reference server keeps.
type Store interface {
	storage.Store
	storage.CredentialStore
	FactsStore
	EventLog
}

// FactsStore records device facts captured from a session.
type FactsStore interface {
	PutFacts(ctx context.Context, f sqlstore.Facts) error
}

// EventLog appends server events.
type EventLog interface {
	LogEvent(ctx context.Context, deviceID, kind string, detail any) error
}

// Config configures the composed service.
type Config struct {
	// BaseURL is the externally reachable https base, e.g.
	// https://mdm.example.com. Endpoint URLs are derived from it.
	BaseURL string
	// Paths override the endpoint paths; zero values use the defaults.
	Paths Paths
	// CA issues enrollment certificates. Required.
	CA ca.Issuer
	// Store persists everything. Required.
	Store Store
	// Queue is the command queue over the same store. Required.
	Queue mdm.CommandQueue
	// Authenticator verifies on-premise enrollment credentials. Required.
	Authenticator enroll.Authenticator
	// Registry validates queued commands; nil skips schema validation.
	Registry *csp.Registry
	// ProviderID is the DMClient provider id the provisioning doc and the
	// session engine use. Default "go-microsoft-dm".
	ProviderID string
	// PushPFN optionally configures WNS during enrollment.
	PushPFN string
	// Name is the APPLICATION display name in the provisioning doc.
	Name string
	// Clock; default real time.
	Clock clock.Clock
	// EnrollmentVersion advertised at discovery; default 3.0.
	EnrollmentVersion string
	// AllowBasicAuth permits syncml:auth-basic management sessions.
	AllowBasicAuth bool
}

// Paths are the four endpoint paths.
type Paths struct {
	Discovery  string
	Policy     string
	Enrollment string
	Management string
}

func (p Paths) withDefaults() Paths {
	if p.Discovery == "" {
		p.Discovery = enroll.DiscoveryPath
	}
	if p.Policy == "" {
		p.Policy = "/EnrollmentServer/Policy.svc"
	}
	if p.Enrollment == "" {
		p.Enrollment = "/EnrollmentServer/Enrollment.svc"
	}
	if p.Management == "" {
		p.Management = "/ManagementServer/MDM.svc"
	}
	return p
}

// Service is the composed reference-server logic.
type Service struct {
	Enroll     *enroll.Service
	Management *mdm.Service
	Paths      Paths
	cfg        Config
	store      Store
}

// New composes the service.
func New(cfg Config) (*Service, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("%w: BaseURL is required", ErrConfig)
	}
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return nil, fmt.Errorf("%w: BaseURL %q is not an absolute https URL", ErrConfig, cfg.BaseURL)
	}
	if cfg.CA == nil || cfg.Store == nil || cfg.Queue == nil || cfg.Authenticator == nil {
		return nil, fmt.Errorf("%w: CA, Store, Queue and Authenticator are required", ErrConfig)
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.ProviderID == "" {
		cfg.ProviderID = "go-microsoft-dm"
	}
	if cfg.Name == "" {
		cfg.Name = "go-microsoft-dm"
	}
	if cfg.EnrollmentVersion == "" {
		cfg.EnrollmentVersion = enroll.DefaultEnrollmentVersion
	}
	paths := cfg.Paths.withDefaults()
	abs := func(p string) string { u := *base; u.Path = p; return u.String() }
	managementURL := abs(paths.Management)

	issuer, err := wstep.NewIssuer(cfg.CA)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	creds := &credentialSource{store: cfg.Store, clock: cfg.Clock}
	prov, err := enroll.NewProvisioner(enroll.ProvisionConfig{
		ManagementURL: managementURL, ProviderID: cfg.ProviderID, Name: cfg.Name,
		PushPFN:     cfg.PushPFN,
		Credentials: creds,
		EntDMID:     func(e *enroll.Enrollment) string { return e.Request.Context.DeviceID },
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	recorder := &storage.Recorder{Store: cfg.Store, Conflicts: conflictSink{log: cfg.Store}}
	enrollSvc, err := enroll.New(enroll.Config{
		EnrollmentServiceURL:       abs(paths.Enrollment),
		EnrollmentPolicyServiceURL: abs(paths.Policy),
		EnrollmentVersion:          cfg.EnrollmentVersion,
		Authenticator:              cfg.Authenticator,
		Issuer:                     issuer,
		Provisioner:                prov,
		Recorder:                   recorder,
		Clock:                      cfg.Clock,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	mgmtSvc, err := mdm.New(mdm.Config{
		ServerURL:  managementURL,
		Auth:       &storage.MDMAuthenticator{Enrollments: cfg.Store, Credentials: cfg.Store},
		Queue:      cfg.Queue,
		Hooks:      &hooks{store: cfg.Store, clock: cfg.Clock},
		Registry:   cfg.Registry,
		Clock:      cfg.Clock,
		ProviderID: cfg.ProviderID,
		AllowBasic: cfg.AllowBasicAuth,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	return &Service{Enroll: enrollSvc, Management: mgmtSvc, Paths: paths, cfg: cfg, store: cfg.Store}, nil
}

// Store returns the durable store.
func (s *Service) Store() Store { return s.store }

// randRead is the entropy source for randomSecret; a test may replace it.
var randRead = rand.Read

// randomSecret returns n bytes of hex.
func randomSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := randRead(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
