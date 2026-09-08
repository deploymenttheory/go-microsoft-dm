package service

import (
	"context"
	"errors"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// hooks records device facts and routes session events to the store.
type hooks struct {
	store Store
	clock clock.Clock
}

var _ mdm.Hooks = (*hooks)(nil)

// PackageOne persists the device facts reported at session start.
func (h *hooks) PackageOne(ctx context.Context, deviceID, sessionID string, f mdm.Facts) error {
	e, err := h.store.Get(ctx, deviceID)
	if err != nil {
		return err
	}
	if err := h.store.TouchLastSeen(ctx, e.Serial, h.clock.Now()); err != nil {
		return err
	}
	facts := sqlstore.Facts{
		DeviceID: deviceID, DevInfo: f.DevInfo, LoginStatus: f.LoginStatus,
		SyncType: f.SyncType, DevicePrepSync: f.DevicePrepSync, UpdatedAt: h.clock.Now(),
	}
	if err := h.store.PutFacts(ctx, facts); err != nil {
		return err
	}
	return h.store.LogEvent(ctx, deviceID, "session", map[string]string{"session_id": sessionID, "login_status": f.LoginStatus})
}

// GenericAlert logs a 1226 generic alert.
func (h *hooks) GenericAlert(ctx context.Context, e mdm.Event) error {
	return h.store.LogEvent(ctx, e.DeviceID, "generic_alert", map[string]any{"type": e.Type, "data": e.Data, "mark": e.Mark})
}

// ClientEvent logs a 1224 client event.
func (h *hooks) ClientEvent(ctx context.Context, e mdm.Event) error {
	return h.store.LogEvent(ctx, e.DeviceID, "client_event", map[string]string{"type": e.Type, "data": e.Data})
}

// Unenrolled ends the enrollment: it marks the active enrollment unenrolled,
// revokes its certificate and logs the event.
func (h *hooks) Unenrolled(ctx context.Context, deviceID string) error {
	now := h.clock.Now()
	e, err := h.store.Get(ctx, deviceID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return h.store.LogEvent(ctx, deviceID, "unenrolled", nil)
		}
		return err
	}
	if err := storage.Unenroll(ctx, h.store, e.Serial, now); err != nil {
		return err
	}
	return h.store.LogEvent(ctx, deviceID, "unenrolled", map[string]string{"serial": e.Serial})
}

// conflictSink logs an HWDevID conflict as an event; it never fails the
// enrollment.
type conflictSink struct {
	log EventLog
}

var _ storage.ConflictSink = conflictSink{}

// HWDevIDConflict records the conflict.
func (c conflictSink) HWDevIDConflict(ctx context.Context, cf storage.Conflict) error {
	existing := make([]string, 0, len(cf.Existing))
	for _, e := range cf.Existing {
		existing = append(existing, e.DeviceID)
	}
	return c.log.LogEvent(ctx, cf.New.DeviceID, "hwdevid_conflict", map[string]any{
		"hwdevid": cf.HWDevID, "existing_devices": existing, "at": time.Now().UTC(),
	})
}
