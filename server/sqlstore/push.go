package sqlstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

// PushChannel belongs to one enrollment certificate, never just a device ID.
type PushChannel struct {
	Serial     string
	URI        string `json:"-"`
	FirstSeen  time.Time
	ObservedAt time.Time
	Dead       bool
	Status     string
}

// PushChannel returns the last observed WNS channel for an enrollment.
func (s *Store) PushChannel(ctx context.Context, serial string) (PushChannel, error) {
	c := PushChannel{Serial: serial}
	var first, observed, dead int64
	err := s.queryRow(ctx, `SELECT uri, first_seen, observed_at, dead, push_status FROM push_channels WHERE serial = ?`, serial).Scan(&c.URI, &first, &observed, &dead, &c.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	c.FirstSeen, c.ObservedAt, c.Dead = fromNanos(first), fromNanos(observed), dead != 0
	return c, err
}

// ObservePush records a successful read. Nil fields were not reported. Re-reading
// the same URI does not reset its age or revive a known-dead channel.
func (s *Store) ObservePush(ctx context.Context, serial string, uri, status *string, at time.Time) error {
	_, err := s.exec(ctx, `INSERT INTO push_channels (serial, uri, uri_hash, first_seen, observed_at, dead, push_status) VALUES (?, ?, ?, ?, ?, ?, ?)`+s.d.upsert([]string{"serial"}, []string{"serial"}), serial, "", pushURIHash(""), int64(0), int64(0), int64(0), "")
	if err != nil {
		return err
	}
	if uri != nil {
		hash := pushURIHash(*uri)
		_, err = s.exec(ctx, `UPDATE push_channels SET first_seen = CASE WHEN uri_hash = ? THEN first_seen ELSE ? END, dead = CASE WHEN uri_hash = ? THEN dead ELSE 0 END, uri = ?, uri_hash = ?, observed_at = ? WHERE serial = ? AND observed_at <= ?`, hash, nanos(at), hash, *uri, hash, nanos(at), serial, nanos(at))
		if err != nil {
			return err
		}
	}
	if status != nil {
		_, err = s.exec(ctx, `UPDATE push_channels SET push_status = ? WHERE serial = ?`, *status, serial)
	}
	return err
}

// MarkPushDead only invalidates the URI that failed, preserving a concurrent renewal.
func (s *Store) MarkPushDead(ctx context.Context, serial, uri string) error {
	_, err := s.exec(ctx, `UPDATE push_channels SET dead = 1 WHERE serial = ? AND uri_hash = ?`, serial, pushURIHash(uri))
	return err
}

// Compare opaque tokens independently of the database's text collation.
func pushURIHash(uri string) string {
	sum := sha256.Sum256([]byte(uri))
	return hex.EncodeToString(sum[:])
}
