package wake_test

import (
	"context"
	json "encoding/json/v2"
	"net/http/httptest"
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/server/agent/wake"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

type probeStore struct{}

func (probeStore) AgentWake(_ context.Context, id, token string) (string, error) {
	if id == "device" && token == "secret" {
		return "cmd-7", nil
	}
	return "", storage.ErrNotFound
}

func TestWakeEndpoint(t *testing.T) {
	h := wake.Handler(probeStore{})
	for _, tc := range []struct {
		method, auth string
		want         int
	}{
		{"GET", "Bearer secret", 200}, {"GET", "Bearer wrong", 401}, {"POST", "Bearer secret", 405},
	} {
		r := httptest.NewRequest(tc.method, "/agent/wake?device_id=device", nil)
		r.Header.Set("Authorization", tc.auth)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d", tc.method, tc.auth, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("wake response can be cached")
		}
		if tc.want == 200 {
			var body struct {
				WakeID string `json:"wake_id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.WakeID != "cmd-7" {
				t.Fatalf("body: %+v, %v", body, err)
			}
		}
	}
}
