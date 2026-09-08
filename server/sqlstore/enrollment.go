package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// Create implements storage.EnrollmentStore. It supersedes the device's
// current active enrollment and inserts the new one, atomically.
func (s *Store) Create(ctx context.Context, e *storage.Enrollment) error {
	if err := e.Validate(); err != nil {
		return fmt.Errorf("%w: %w", storage.ErrInvalid, err)
	}
	ctxJSON, err := json.Marshal(e.Context)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal context: %w", err)
	}
	return s.tx(ctx, func(tx *sql.Tx) error {
		var exists int
		err := s.rebindRow(ctx, tx, `SELECT 1 FROM enrollments WHERE serial = ?`, e.Serial).Scan(&exists)
		if err == nil {
			return fmt.Errorf("%w: enrollment with serial %s exists", storage.ErrConflict, e.Serial)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := s.rebindRow(ctx, tx, `SELECT 1 FROM enrollments WHERE thumbprint = ?`, e.Thumbprint).Scan(&exists); err == nil {
			return fmt.Errorf("%w: enrollment with thumbprint %s exists", storage.ErrConflict, e.Thumbprint)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		// Supersede the current active enrollment for this device.
		if _, err := tx.ExecContext(ctx, s.d.rebind(
			`UPDATE enrollments SET state = ?, updated_at = ? WHERE device_id = ? AND state = ?`),
			string(storage.StateSuperseded), nanos(e.EnrolledAt), e.DeviceID, string(storage.StateActive)); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, s.d.rebind(
			`INSERT INTO enrollments (serial, thumbprint, device_id, hwdevid, enrollment_type, upn, context, state, enrolled_at, updated_at, last_seen_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
			e.Serial, e.Thumbprint, e.DeviceID, e.HWDevID, string(e.EnrollmentType), e.UPN, ctxJSON,
			string(storage.StateActive), nanos(e.EnrolledAt), nanos(e.EnrolledAt), nanos(e.LastSeenAt))
		return err
	})
}

const enrollmentCols = `serial, thumbprint, device_id, hwdevid, enrollment_type, upn, context, state, enrolled_at, updated_at, last_seen_at`

func scanEnrollment(sc interface{ Scan(...any) error }) (*storage.Enrollment, error) {
	var (
		e                               storage.Enrollment
		etype, state                    string
		ctxJSON                         []byte
		enrolledAt, updatedAt, lastSeen int64
	)
	if err := sc.Scan(&e.Serial, &e.Thumbprint, &e.DeviceID, &e.HWDevID, &etype, &e.UPN, &ctxJSON, &state, &enrolledAt, &updatedAt, &lastSeen); err != nil {
		return nil, err
	}
	e.EnrollmentType = enroll.EnrollmentType(etype)
	e.State = storage.State(state)
	e.EnrolledAt, e.UpdatedAt, e.LastSeenAt = fromNanos(enrolledAt), fromNanos(updatedAt), fromNanos(lastSeen)
	if len(ctxJSON) > 0 {
		if err := json.Unmarshal(ctxJSON, &e.Context); err != nil {
			return nil, fmt.Errorf("sqlstore: unmarshal context: %w", err)
		}
	}
	return &e, nil
}

// Get implements storage.EnrollmentStore.
func (s *Store) Get(ctx context.Context, deviceID string) (*storage.Enrollment, error) {
	row := s.queryRow(ctx, `SELECT `+enrollmentCols+` FROM enrollments WHERE device_id = ? AND state = ?`, deviceID, string(storage.StateActive))
	e, err := scanEnrollment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: device %s", storage.ErrNotFound, deviceID)
	}
	return e, err
}

// GetBySerial implements storage.EnrollmentStore.
func (s *Store) GetBySerial(ctx context.Context, serial string) (*storage.Enrollment, error) {
	row := s.queryRow(ctx, `SELECT `+enrollmentCols+` FROM enrollments WHERE serial = ?`, serial)
	e, err := scanEnrollment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: serial %s", storage.ErrNotFound, serial)
	}
	return e, err
}

// GetByThumbprint implements storage.EnrollmentStore.
func (s *Store) GetByThumbprint(ctx context.Context, thumbprint string) (*storage.Enrollment, error) {
	row := s.queryRow(ctx, `SELECT `+enrollmentCols+` FROM enrollments WHERE thumbprint = ?`, thumbprint)
	e, err := scanEnrollment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: thumbprint %s", storage.ErrNotFound, thumbprint)
	}
	return e, err
}

// ListByHWDevID implements storage.EnrollmentStore.
func (s *Store) ListByHWDevID(ctx context.Context, hwDevID string) ([]storage.Enrollment, error) {
	if hwDevID == "" {
		return nil, fmt.Errorf("%w: empty HWDevID", storage.ErrInvalid)
	}
	rows, err := s.query(ctx, `SELECT `+enrollmentCols+` FROM enrollments WHERE hwdevid = ? ORDER BY enrolled_at, serial`, hwDevID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storage.Enrollment
	for rows.Next() {
		e, err := scanEnrollment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// SetState implements storage.EnrollmentStore.
func (s *Store) SetState(ctx context.Context, serial string, state storage.State, at time.Time) error {
	if !state.Valid() {
		return fmt.Errorf("%w: state %q", storage.ErrInvalid, state)
	}
	return s.tx(ctx, func(tx *sql.Tx) error {
		var deviceID string
		if err := s.rebindRow(ctx, tx, `SELECT device_id FROM enrollments WHERE serial = ?`, serial).Scan(&deviceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: serial %s", storage.ErrNotFound, serial)
			}
			return err
		}
		// Activating supersedes any other active enrollment for the device.
		if state == storage.StateActive {
			if _, err := tx.ExecContext(ctx, s.d.rebind(
				`UPDATE enrollments SET state = ?, updated_at = ? WHERE device_id = ? AND state = ? AND serial <> ?`),
				string(storage.StateSuperseded), nanos(at), deviceID, string(storage.StateActive), serial); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, s.d.rebind(`UPDATE enrollments SET state = ?, updated_at = ? WHERE serial = ?`),
			string(state), nanos(at), serial)
		return err
	})
}

// TouchLastSeen implements storage.EnrollmentStore.
func (s *Store) TouchLastSeen(ctx context.Context, serial string, at time.Time) error {
	res, err := s.exec(ctx, `UPDATE enrollments SET last_seen_at = ? WHERE serial = ? AND last_seen_at < ?`, nanos(at), serial, nanos(at))
	if err != nil {
		return err
	}
	// A no-op update (already newer) is fine; only a missing row is an error.
	if n, _ := res.RowsAffected(); n == 0 {
		var exists int
		if err := s.queryRow(ctx, `SELECT 1 FROM enrollments WHERE serial = ?`, serial).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: serial %s", storage.ErrNotFound, serial)
		} else if err != nil {
			return err
		}
	}
	return nil
}

// List implements storage.EnrollmentStore.
func (s *Store) List(ctx context.Context, q storage.EnrollmentQuery, p paging.Page) (paging.Result[storage.Enrollment], error) {
	where, args := enrollmentWhere(q)
	offset, err := offsetOf(p)
	if err != nil {
		return paging.Result[storage.Enrollment]{}, err
	}
	size := p.Size()
	args = append(args, size+1, offset)
	rows, err := s.query(ctx, `SELECT `+enrollmentCols+` FROM enrollments`+where+` ORDER BY enrolled_at, serial LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return paging.Result[storage.Enrollment]{}, err
	}
	defer rows.Close()
	var items []storage.Enrollment
	for rows.Next() {
		e, err := scanEnrollment(rows)
		if err != nil {
			return paging.Result[storage.Enrollment]{}, err
		}
		items = append(items, *e)
	}
	if err := rows.Err(); err != nil {
		return paging.Result[storage.Enrollment]{}, err
	}
	return pageResult(items, size, offset), nil
}

func enrollmentWhere(q storage.EnrollmentQuery) (string, []any) {
	var clauses []string
	var args []any
	if q.State != "" {
		clauses = append(clauses, "state = ?")
		args = append(args, string(q.State))
	}
	if q.EnrollmentType != "" {
		clauses = append(clauses, "enrollment_type = ?")
		args = append(args, string(q.EnrollmentType))
	}
	if q.UPN != "" {
		clauses = append(clauses, "upn = ?")
		args = append(args, q.UPN)
	}
	if q.DeviceID != "" {
		clauses = append(clauses, "device_id = ?")
		args = append(args, q.DeviceID)
	}
	return whereClause(clauses), args
}

// offsetOf reads the offset cursor.
func offsetOf(p paging.Page) (int, error) {
	if p.Cursor == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(p.Cursor)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: cursor %q", storage.ErrInvalid, p.Cursor)
	}
	return n, nil
}

// pageResult trims an over-fetched slice to size and sets the next cursor.
func pageResult[T any](items []T, size, offset int) paging.Result[T] {
	res := paging.Result[T]{}
	if len(items) > size {
		res.Items = items[:size]
		res.NextCursor = strconv.Itoa(offset + size)
	} else {
		res.Items = items
	}
	return res
}

func whereClause(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	out := " WHERE "
	for i, c := range clauses {
		if i > 0 {
			out += " AND "
		}
		out += c
	}
	return out
}
