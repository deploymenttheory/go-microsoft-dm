package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Facts is the durable device facts the service captures from a session.
type Facts struct {
	DeviceID       string
	DevInfo        map[string]string
	LoginStatus    string
	SyncType       string
	DevicePrepSync string
	ChannelURI     string
	UpdatedAt      time.Time
}

// ErrNotFound reports a missing device-facts or event row.
var ErrNotFound = errors.New("sqlstore: not found")

// PutFacts records the facts a device reported, merging the DevInfo and the
// non-empty scalar fields into any existing row.
func (s *Store) PutFacts(ctx context.Context, f Facts) error {
	if f.DeviceID == "" {
		return fmt.Errorf("%w: empty device id", ErrConfig)
	}
	devinfo, err := json.Marshal(f.DevInfo)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal devinfo: %w", err)
	}
	cols := []string{"devinfo", "login_status", "sync_type", "device_prep_sync", "channel_uri", "updated_at"}
	_, err = s.exec(ctx,
		`INSERT INTO device_facts (device_id, devinfo, login_status, sync_type, device_prep_sync, channel_uri, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`+
			s.d.upsert([]string{"device_id"}, cols),
		f.DeviceID, devinfo, f.LoginStatus, f.SyncType, f.DevicePrepSync, f.ChannelURI, nanos(f.UpdatedAt))
	return err
}

// Facts returns a device's recorded facts, or ErrNotFound.
func (s *Store) Facts(ctx context.Context, deviceID string) (Facts, error) {
	var (
		f         Facts
		devinfo   []byte
		updatedAt int64
	)
	f.DeviceID = deviceID
	err := s.queryRow(ctx, `SELECT devinfo, login_status, sync_type, device_prep_sync, channel_uri, updated_at FROM device_facts WHERE device_id = ?`, deviceID).
		Scan(&devinfo, &f.LoginStatus, &f.SyncType, &f.DevicePrepSync, &f.ChannelURI, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Facts{}, fmt.Errorf("%w: facts for %q", ErrNotFound, deviceID)
	}
	if err != nil {
		return Facts{}, err
	}
	f.UpdatedAt = fromNanos(updatedAt)
	if len(devinfo) > 0 {
		if err := json.Unmarshal(devinfo, &f.DevInfo); err != nil {
			return Facts{}, fmt.Errorf("sqlstore: unmarshal devinfo: %w", err)
		}
	}
	return f, nil
}

// Event is one entry in the append-only event log.
type Event struct {
	ID       int64
	DeviceID string
	Kind     string
	Detail   json.RawMessage
	At       time.Time
}

// LogEvent appends an event. detail is stored as JSON; nil becomes "null".
func (s *Store) LogEvent(ctx context.Context, deviceID, kind string, detail any) error {
	blob, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal event: %w", err)
	}
	_, err = s.exec(ctx, `INSERT INTO events (device_id, kind, detail, at) VALUES (?, ?, ?, ?)`,
		deviceID, kind, blob, nanos(s.clock.Now()))
	return err
}

// Events returns a device's events, oldest first, up to limit (0 = all).
func (s *Store) Events(ctx context.Context, deviceID string, limit int) ([]Event, error) {
	q := `SELECT id, device_id, kind, detail, at FROM events WHERE device_id = ? ORDER BY id`
	args := []any{deviceID}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var (
			e  Event
			at int64
		)
		if err := rows.Scan(&e.ID, &e.DeviceID, &e.Kind, &e.Detail, &at); err != nil {
			return nil, err
		}
		e.At = fromNanos(at)
		out = append(out, e)
	}
	return out, rows.Err()
}
