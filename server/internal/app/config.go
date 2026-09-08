package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
)

// Role selects which endpoints the server exposes.
type Role string

// The roles. windc is added in Phase 12.
const (
	RoleAll Role = "all"
	RoleMDM Role = "mdm"
)

// Config is the reference server's configuration.
type Config struct {
	// Store is the backend: sqlite, postgres, mysql or memory.
	Store sqlstore.Kind
	// Memory selects an ephemeral in-memory sqlite (the "memory" role).
	Memory bool
	// DSN is the database DSN (a file path for sqlite; ignored for memory).
	DSN string
	// BaseURL is the externally reachable https base URL.
	BaseURL string
	// Listen is the address dmserver binds.
	Listen string
	// TLSCert and TLSKey are the server certificate files; empty serves
	// plain HTTP (behind a TLS-terminating proxy).
	TLSCert string
	TLSKey  string
	// CACert and CAKey are the enrollment CA files; empty generates an
	// ephemeral self-signed CA.
	CACert string
	CAKey  string
	// ProviderID and Name identify the management provider.
	ProviderID string
	Name       string
	// Role selects the endpoint set.
	Role Role
	// EnrollUsers maps username to password for on-premise enrollment.
	EnrollUsers map[string]string
	// EnrollAllowAny accepts any enrollment credential (simulated devices).
	EnrollAllowAny bool
	// DisableSchemaValidation turns off command schema checks.
	DisableSchemaValidation bool
}

// Load reads the configuration from DM_* environment variables.
func Load() (Config, error) {
	c := Config{
		Store:      sqlstore.Kind(getenv("DM_STORE", "memory")),
		DSN:        os.Getenv("DM_DSN"),
		BaseURL:    getenv("DM_BASE_URL", "https://localhost:8443"),
		Listen:     getenv("DM_LISTEN", ":8443"),
		TLSCert:    os.Getenv("DM_TLS_CERT"),
		TLSKey:     os.Getenv("DM_TLS_KEY"),
		CACert:     os.Getenv("DM_CA_CERT"),
		CAKey:      os.Getenv("DM_CA_KEY"),
		ProviderID: getenv("DM_PROVIDER_ID", "go-microsoft-dm"),
		Name:       getenv("DM_NAME", "go-microsoft-dm"),
		Role:       Role(getenv("DM_ROLE", string(RoleAll))),
	}
	if c.Store == "memory" {
		c.Memory, c.Store = true, sqlstore.SQLite
	}
	if users := os.Getenv("DM_ENROLL_USERS"); users != "" {
		c.EnrollUsers = parseUsers(users)
	}
	c.EnrollAllowAny = os.Getenv("DM_ENROLL_ALLOW_ANY") == "true"
	c.DisableSchemaValidation = os.Getenv("DM_DISABLE_SCHEMA_VALIDATION") == "true"
	if err := c.validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) validate() error {
	switch c.Store {
	case sqlstore.SQLite, sqlstore.Postgres, sqlstore.MySQL:
	default:
		return fmt.Errorf("%w: DM_STORE %q", ErrConfig, c.Store)
	}
	if c.Role != RoleAll && c.Role != RoleMDM {
		return fmt.Errorf("%w: DM_ROLE %q", ErrConfig, c.Role)
	}
	if !c.Memory && c.DSN == "" {
		return fmt.Errorf("%w: DM_DSN is required for store %q", ErrConfig, c.Store)
	}
	if len(c.EnrollUsers) == 0 && !c.EnrollAllowAny {
		return fmt.Errorf("%w: set DM_ENROLL_USERS or DM_ENROLL_ALLOW_ANY", ErrConfig)
	}
	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseUsers reads "user:pass,user2:pass2".
func parseUsers(s string) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		u, p, ok := strings.Cut(pair, ":")
		if ok && u != "" {
			out[u] = p
		}
	}
	return out
}
