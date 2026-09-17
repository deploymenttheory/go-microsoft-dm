// Package wake exposes an authenticated, enrollment-bound wake signal
// to an optional Windows agent. The agent still initiates the OMA-DM session.
package wake

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"strings"

	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

type Store interface {
	AgentWake(context.Context, string, string) (string, error)
}

// Handler serves GET /agent/wake?device_id=... . Credentials belong only in
// Authorization; neither credentials nor command bodies enter the response.
func Handler(store Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		deviceID := r.URL.Query().Get("device_id")
		parts := strings.Fields(r.Header.Get("Authorization"))
		if deviceID == "" || len(deviceID) > 255 || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		wakeID, err := store.AgentWake(r.Context(), deviceID, parts[1])
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		body, err := json.Marshal(struct {
			WakeID string `json:"wake_id"`
		}{wakeID})
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	})
}
