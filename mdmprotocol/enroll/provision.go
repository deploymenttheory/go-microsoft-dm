package enroll

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
)

// CredentialSource supplies the two OMA DM account credentials for an
// enrollment: how the client authenticates to the server (APPSRV) and how
// the server authenticates to the client (CLIENT, always DIGEST). The
// session engine verifies them later, so the caller owns them.
type CredentialSource interface {
	Credentials(ctx context.Context, e *Enrollment) (server, client wapprov.Credential, err error)
}

// CredentialSourceFunc adapts a function to CredentialSource.
type CredentialSourceFunc func(ctx context.Context, e *Enrollment) (server, client wapprov.Credential, err error)

// Credentials implements CredentialSource.
func (f CredentialSourceFunc) Credentials(ctx context.Context, e *Enrollment) (wapprov.Credential, wapprov.Credential, error) {
	return f(ctx, e)
}

// ProvisionConfig configures the default Provisioner.
type ProvisionConfig struct {
	// ManagementURL is the OMA DM endpoint written to APPLICATION ADDR and
	// DMClient. Required.
	ManagementURL string
	// ProviderID is PROVIDER-ID and the DMClient provider node name.
	// Required.
	ProviderID string
	// Name is the APPLICATION NAME shown to the user; optional.
	Name string
	// Credentials supplies the account credentials. Required.
	Credentials CredentialSource
	// Renew is My/WSTEP/Renew; default ROBOSupport with a 60-day period
	// and 4-day retries.
	Renew *wapprov.Renew
	// Poll is the DMClient schedule; default wapprov.DefaultPoll.
	Poll *wapprov.Poll
	// PushPFN is the optional WNS package family name provisioned under DMClient.
	PushPFN string
	// Application tunes the w7 characteristic. ProviderID, Name, Address,
	// ServerAuth and ClientAuth are filled from this config and the
	// enrollment; the remaining fields are copied.
	Application wapprov.ApplicationConfig
	// IncludeRootCATrusted adds a RootCATrustedCertificates characteristic
	// for the chain's root in addition to CertificateStore/Root.
	IncludeRootCATrusted bool
	// EntDMID, when set, names the device for the server; it receives the
	// enrollment and returns the identifier or "" to omit the parm.
	EntDMID func(e *Enrollment) string
}

// DefaultRenew is the My/WSTEP/Renew setting written when none is given.
func DefaultRenew() *wapprov.Renew {
	return &wapprov.Renew{ROBOSupport: true, RenewPeriod: 60, RetryInterval: 4}
}

// DefaultProvisioner builds provisioning documents from the enrollment
// and a fixed configuration.
type DefaultProvisioner struct {
	cfg ProvisionConfig
}

// NewProvisioner validates the configuration.
func NewProvisioner(cfg ProvisionConfig) (*DefaultProvisioner, error) {
	if cfg.ManagementURL == "" || cfg.ProviderID == "" {
		return nil, fmt.Errorf("%w: ManagementURL and ProviderID are required", ErrConfig)
	}
	if cfg.Credentials == nil {
		return nil, fmt.Errorf("%w: Credentials source is required", ErrConfig)
	}
	if cfg.Renew == nil {
		cfg.Renew = DefaultRenew()
	}
	if cfg.Poll == nil {
		p := wapprov.DefaultPoll()
		cfg.Poll = &p
	}
	if err := cfg.Poll.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	return &DefaultProvisioner{cfg: cfg}, nil
}

// Provision implements Provisioner.
func (p *DefaultProvisioner) Provision(ctx context.Context, e *Enrollment) (*wapprov.Document, error) {
	if e == nil || e.Certificate == nil || e.Request == nil {
		return nil, fmt.Errorf("%w: enrollment without certificate", ErrConfig)
	}
	roots, intermediates := splitChain(e.Chain)
	if len(roots) == 0 {
		return nil, fmt.Errorf("%w: issuer chain has no root", ErrConfig)
	}
	cs, err := wapprov.CertificateStore(wapprov.CertificateStoreConfig{
		Roots: roots, Intermediates: intermediates, Client: e.Certificate.Raw, Store: e.Store, Renew: p.cfg.Renew,
	})
	if err != nil {
		return nil, err
	}
	server, client, err := p.cfg.Credentials.Credentials(ctx, e)
	if err != nil {
		return nil, fmt.Errorf("enroll: credentials: %w", err)
	}
	appCfg := p.cfg.Application
	appCfg.ProviderID, appCfg.Name, appCfg.Address = p.cfg.ProviderID, p.cfg.Name, p.cfg.ManagementURL
	appCfg.ServerAuth, appCfg.ClientAuth = server, client
	app, err := wapprov.Application(appCfg)
	if err != nil {
		return nil, err
	}
	dmCfg := wapprov.DMClientConfig{ProviderID: p.cfg.ProviderID, EntDeviceName: e.Request.Context.DeviceName, Poll: p.cfg.Poll, PushPFN: p.cfg.PushPFN}
	if e.Request.Context.EnrollmentType == EnrollmentTypeFull {
		dmCfg.UPN = e.Request.Principal.UPN
	}
	if p.cfg.EntDMID != nil {
		dmCfg.EntDMID = p.cfg.EntDMID(e)
	}
	dm, err := wapprov.DMClient(dmCfg)
	if err != nil {
		return nil, err
	}
	doc := &wapprov.Document{Version: wapprov.Version, Characteristics: []wapprov.Characteristic{cs}}
	if p.cfg.IncludeRootCATrusted {
		rc, err := wapprov.RootCATrustedCertificates(roots)
		if err != nil {
			return nil, err
		}
		doc.Characteristics = append(doc.Characteristics, rc)
	}
	doc.Characteristics = append(doc.Characteristics, app, dm)
	return doc, nil
}

// splitChain separates a chain (issuer first, root last) into the root and
// the intermediates between the leaf's issuer and the root. A chain of one
// is a root that signed directly.
func splitChain(chain []*x509.Certificate) (roots, intermediates [][]byte) {
	if len(chain) == 0 {
		return nil, nil
	}
	root := chain[len(chain)-1]
	roots = [][]byte{root.Raw}
	for _, c := range chain[:len(chain)-1] {
		intermediates = append(intermediates, c.Raw)
	}
	return roots, intermediates
}
