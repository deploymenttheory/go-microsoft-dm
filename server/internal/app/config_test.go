package app

import (
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
)

func TestLoad(t *testing.T) {
	// Not parallel: mutates the environment.
	env := map[string]string{
		"DM_STORE": "memory", "DM_BASE_URL": "https://mdm.test", "DM_ENROLL_ALLOW_ANY": "true",
		"DM_PROVIDER_ID": "p", "DM_NAME": "n", "DM_ROLE": "mdm",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.Memory || c.Store != sqlstore.SQLite || c.BaseURL != "https://mdm.test" || !c.EnrollAllowAny || c.Role != RoleMDM {
		t.Errorf("config = %+v", c)
	}
	// Explicit users, sqlite with a DSN.
	t.Setenv("DM_STORE", "sqlite")
	t.Setenv("DM_DSN", "file:x.db")
	t.Setenv("DM_ENROLL_ALLOW_ANY", "")
	t.Setenv("DM_ENROLL_USERS", "alice:secret, bob:pw ,")
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Memory || c.EnrollUsers["alice"] != "secret" || c.EnrollUsers["bob"] != "pw" || len(c.EnrollUsers) != 2 {
		t.Errorf("users = %+v", c.EnrollUsers)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := map[string]map[string]string{
		"bad store": {"DM_STORE": "oracle", "DM_ENROLL_ALLOW_ANY": "true"},
		"bad role":  {"DM_STORE": "memory", "DM_ROLE": "weird", "DM_ENROLL_ALLOW_ANY": "true"},
		"no dsn":    {"DM_STORE": "postgres", "DM_ENROLL_ALLOW_ANY": "true"},
		"no auth":   {"DM_STORE": "memory"},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			// clear the relevant vars, then set the case's.
			for _, k := range []string{"DM_STORE", "DM_ROLE", "DM_DSN", "DM_ENROLL_ALLOW_ANY", "DM_ENROLL_USERS", "DM_BASE_URL"} {
				t.Setenv(k, "")
			}
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := Load(); err == nil {
				t.Errorf("%s: expected an error", name)
			}
		})
	}
}
