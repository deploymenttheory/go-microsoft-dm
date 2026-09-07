package enroll

import (
	"context"
	"errors"
	"testing"
)

// FuzzService feeds arbitrary bodies to the three operations; each must
// return a complete envelope and, on failure, a fault.
func FuzzService(f *testing.F) {
	// Signing is not the subject here; a refusing issuer keeps every worker
	// free of RSA key generation.
	e := newEnvWithIssuer(f, &fakeIssuer{err: errors.New("fuzz: no signing")}, nil)
	f.Add(fixture(f, "discover-onprem-request.xml"))
	f.Add(fixture(f, "getpolicies-onprem-request.xml"))
	f.Add(fixture(f, "rst-onprem-request.xml"))
	f.Fuzz(func(t *testing.T, data []byte) {
		for name, fn := range map[string]func(context.Context, []byte) ([]byte, error){"Discover": e.svc.Discover, "GetPolicies": e.svc.GetPolicies, "Enroll": e.svc.Enroll} {
			out, err := fn(context.Background(), data)
			if len(out) == 0 {
				t.Fatalf("%s returned no envelope", name)
			}
			if err != nil {
				if _, ok := IsFault(err); !ok {
					t.Fatalf("%s returned a non-fault error: %v", name, err)
				}
			}
		}
	})
}
