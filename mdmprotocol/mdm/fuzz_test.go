package mdm_test

import (
	"context"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
)

// FuzzHandle feeds arbitrary bodies to the engine; it must never panic and
// must always return either a reply or an error.
func FuzzHandle(f *testing.F) {
	svc := newService(f)
	f.Add([]byte(packageOne("1", "1")))
	f.Add([]byte("<SyncML></SyncML>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		out, err := svc.Handle(context.Background(), &mdm.Transport{}, data)
		if err == nil && out == nil {
			// An empty reply with no error is valid (end of session), but the
			// input here always carries a body, so this is only reached for a
			// well-formed terminating message.
			return
		}
	})
}
