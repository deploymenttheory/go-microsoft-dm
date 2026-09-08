package app

import (
	"fmt"
	"os"

	"github.com/deploymenttheory/go-microsoft-dm/msplatformservices/wns"
)

// PushConfig is opt-in. A PFN may be provisioned without cloud credentials;
// sending requires credentials for the same package identity.
type PushConfig struct {
	PFN          string
	Auth         string
	ClientID     string
	ClientSecret string
	TenantID     string
}

func loadPush() PushConfig {
	return PushConfig{
		PFN: os.Getenv("DM_WNS_PFN"), Auth: getenv("DM_WNS_AUTH", "legacy"),
		ClientID: os.Getenv("DM_WNS_CLIENT_ID"), ClientSecret: os.Getenv("DM_WNS_CLIENT_SECRET"),
		TenantID: os.Getenv("DM_WNS_TENANT_ID"),
	}
}

// PushSender builds the configured sender. Nil means push delivery is disabled.
func (c PushConfig) PushSender() (*wns.Client, error) {
	if c.ClientID == "" && c.ClientSecret == "" {
		return nil, nil //nolint:nilnil // No configured credentials intentionally disables this optional sender.
	}
	if c.PFN == "" {
		return nil, fmt.Errorf("%w: DM_WNS_PFN required with push credentials", ErrConfig)
	}
	creds := wns.Credentials{
		ClientID: c.ClientID, ClientSecret: c.ClientSecret, TenantID: c.TenantID,
	}
	var source *wns.OAuthSource
	var err error
	switch c.Auth {
	case "", "legacy":
		source, err = wns.NewLegacy(creds)
	case "entra":
		source, err = wns.NewEntra(creds)
	default:
		return nil, fmt.Errorf("%w: DM_WNS_AUTH must be legacy or entra", ErrConfig)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: WNS credentials: %w", ErrConfig, err)
	}
	client, err := wns.New(wns.Config{Tokens: source, Retries: 2})
	if err != nil {
		return nil, fmt.Errorf("%w: WNS sender: %w", ErrConfig, err)
	}
	return client, nil
}
