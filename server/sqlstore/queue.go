package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
)

// Enqueue implements mdm.CommandQueue.
func (s *Queue) Enqueue(ctx context.Context, deviceID string, cmd *mdm.Command, at time.Time) (*mdm.QueuedCommand, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("%w: empty device id", mdm.ErrNotFound)
	}
	if err := mdm.Validate(cmd); err != nil {
		return nil, err
	}
	body, err := syncml.EncodeCommand(cmd.Body, syncml.EncodeOptions{})
	if err != nil {
		return nil, fmt.Errorf("sqlstore: encode command: %w", err)
	}
	var out mdm.QueuedCommand
	err = s.tx(ctx, func(tx *sql.Tx) error {
		id := cmd.ID
		if id != "" {
			var exists int
			if err := s.rebindRow(ctx, tx, `SELECT 1 FROM commands WHERE device_id = ? AND id = ?`, deviceID, id).Scan(&exists); err == nil {
				return fmt.Errorf("%w: command %q exists for device %q", mdm.ErrConflict, id, deviceID)
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		// The per-device counter yields a gapless, monotonic sequence even
		// under concurrent enqueues and after Prune deletes earlier commands.
		// The upsert holds a row lock until commit, so two concurrent
		// enqueues for one device never read the same value.
		if _, err := tx.ExecContext(ctx, s.d.rebind(
			`INSERT INTO command_seq (device_id, next_seq) VALUES (?, 1)`+s.d.incrementSeq()), deviceID); err != nil {
			return err
		}
		var seq int64
		if err := s.rebindRow(ctx, tx, `SELECT next_seq FROM command_seq WHERE device_id = ?`, deviceID).Scan(&seq); err != nil {
			return err
		}
		if id == "" {
			id = "cmd-" + strconv.FormatInt(seq, 10)
			// Ensure the generated id is unique even if a caller mixed
			// explicit and generated ids.
			for {
				var exists int
				err := s.rebindRow(ctx, tx, `SELECT 1 FROM commands WHERE device_id = ? AND id = ?`, deviceID, id).Scan(&exists)
				if errors.Is(err, sql.ErrNoRows) {
					break
				}
				if err != nil {
					return err
				}
				id += "x"
			}
		}
		_, err := tx.ExecContext(ctx, s.d.rebind(
			`INSERT INTO commands (device_id, id, seq, scope, internal, state, body, enqueued_at, last_sent_at, sent_msg_id, attempts, completed_at, result)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, '', 0, 0, NULL)`),
			deviceID, id, seq, string(cmd.Scope), boolInt(cmd.Internal), string(mdm.StatePending), body, nanos(at))
		if err != nil {
			return err
		}
		out = mdm.QueuedCommand{Command: *cmd, Seq: seq, State: mdm.StatePending, EnqueuedAt: at.UTC()}
		out.Command.ID = id
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

const cmdCols = `device_id, id, seq, scope, internal, state, body, enqueued_at, last_sent_at, sent_msg_id, attempts, completed_at, result`

func scanCommand(sc interface{ Scan(...any) error }) (*mdm.QueuedCommand, error) {
	var (
		qc                                  mdm.QueuedCommand
		deviceID, scope, state, sentMsgID   string
		internal                            int64
		body, result                        []byte
		enqueuedAt, lastSentAt, completedAt int64
		attempts                            int64
	)
	if err := sc.Scan(&deviceID, &qc.Command.ID, &qc.Seq, &scope, &internal, &state, &body,
		&enqueuedAt, &lastSentAt, &sentMsgID, &attempts, &completedAt, &result); err != nil {
		return nil, err
	}
	qc.Command.Scope = mdm.Scope(scope)
	qc.Command.Internal = internal != 0
	qc.State = mdm.State(state)
	qc.EnqueuedAt, qc.LastSentAt, qc.CompletedAt = fromNanos(enqueuedAt), fromNanos(lastSentAt), fromNanos(completedAt)
	qc.SentMsgID = sentMsgID
	qc.Attempts = int(attempts)
	cmdBody, err := syncml.DecodeCommand(body)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: decode command: %w", err)
	}
	qc.Command.Body = cmdBody
	if len(result) > 0 {
		var r mdm.Result
		if err := json.Unmarshal(result, &r); err != nil {
			return nil, fmt.Errorf("sqlstore: unmarshal result: %w", err)
		}
		qc.Result = &r
	}
	return &qc, nil
}

// Deliverable implements mdm.CommandQueue.
func (s *Queue) Deliverable(ctx context.Context, deviceID string, limit int) ([]mdm.QueuedCommand, error) {
	q := `SELECT ` + cmdCols + ` FROM commands WHERE device_id = ? AND state IN (?, ?) ORDER BY seq`
	args := []any{deviceID, string(mdm.StatePending), string(mdm.StateSent)}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mdm.QueuedCommand
	for rows.Next() {
		qc, err := scanCommand(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *qc)
	}
	return out, rows.Err()
}

// MarkSent implements mdm.CommandQueue.
func (s *Queue) MarkSent(ctx context.Context, deviceID string, deliveries []mdm.Delivery) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		for _, d := range deliveries {
			res, err := tx.ExecContext(ctx, s.d.rebind(
				`UPDATE commands SET state = ?, last_sent_at = ?, sent_msg_id = ?, attempts = attempts + 1 WHERE device_id = ? AND id = ?`),
				string(mdm.StateSent), nanos(d.At), d.MsgID, deviceID, d.CommandID)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return fmt.Errorf("%w: command %q", mdm.ErrNotFound, d.CommandID)
			}
		}
		return nil
	})
}

// StoreResult implements mdm.CommandQueue.
func (s *Queue) StoreResult(ctx context.Context, deviceID, commandID string, r mdm.Result) error {
	blob, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal result: %w", err)
	}
	state := mdm.StateAcknowledged
	if !syncml.StatusCode(r.Status).Success() {
		state = mdm.StateFailed
	}
	res, err := s.exec(ctx, `UPDATE commands SET state = ?, result = ?, completed_at = ? WHERE device_id = ? AND id = ?`,
		string(state), blob, nanos(r.ReceivedAt), deviceID, commandID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: command %q for device %q", mdm.ErrNotFound, commandID, deviceID)
	}
	return nil
}

// Get implements mdm.CommandQueue.
func (s *Queue) Get(ctx context.Context, deviceID, commandID string) (*mdm.QueuedCommand, error) {
	row := s.queryRow(ctx, `SELECT `+cmdCols+` FROM commands WHERE device_id = ? AND id = ?`, deviceID, commandID)
	qc, err := scanCommand(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: command %q", mdm.ErrNotFound, commandID)
	}
	return qc, err
}

// List implements mdm.CommandQueue.
func (s *Queue) List(ctx context.Context, deviceID string, q mdm.Query, p paging.Page) (paging.Result[mdm.QueuedCommand], error) {
	clauses := []string{"device_id = ?"}
	args := []any{deviceID}
	if q.Internal != nil {
		clauses = append(clauses, "internal = ?")
		args = append(args, boolInt(*q.Internal))
	}
	if q.Scope != "" {
		clauses = append(clauses, "scope = ?")
		args = append(args, string(q.Scope))
	}
	if len(q.States) > 0 {
		ph := make([]byte, 0, len(q.States)*2)
		for i, st := range q.States {
			if i > 0 {
				ph = append(ph, ',')
			}
			ph = append(ph, '?')
			args = append(args, string(st))
		}
		clauses = append(clauses, "state IN ("+string(ph)+")")
	}
	offset, err := offsetOf(p)
	if err != nil {
		return paging.Result[mdm.QueuedCommand]{}, err
	}
	size := p.Size()
	args = append(args, size+1, offset)
	rows, err := s.query(ctx, `SELECT `+cmdCols+` FROM commands`+whereClause(clauses)+` ORDER BY seq LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return paging.Result[mdm.QueuedCommand]{}, err
	}
	defer rows.Close()
	var items []mdm.QueuedCommand
	for rows.Next() {
		qc, err := scanCommand(rows)
		if err != nil {
			return paging.Result[mdm.QueuedCommand]{}, err
		}
		items = append(items, *qc)
	}
	if err := rows.Err(); err != nil {
		return paging.Result[mdm.QueuedCommand]{}, err
	}
	return pageResult(items, size, offset), nil
}

// Cancel implements mdm.CommandQueue.
func (s *Queue) Cancel(ctx context.Context, deviceID string, ids []string, at time.Time) (int, error) {
	n := 0
	err := s.tx(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			res, err := tx.ExecContext(ctx, s.d.rebind(
				`UPDATE commands SET state = ?, completed_at = ? WHERE device_id = ? AND id = ? AND state IN (?, ?)`),
				string(mdm.StateCancelled), nanos(at), deviceID, id, string(mdm.StatePending), string(mdm.StateSent))
			if err != nil {
				return err
			}
			if c, _ := res.RowsAffected(); c > 0 {
				n++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// Prune implements mdm.CommandQueue.
func (s *Queue) Prune(ctx context.Context, before time.Time, limit int) (int, error) {
	// Select the ids to delete first so the count is exact and the limit is
	// honoured across dialects (MySQL cannot LIMIT a DELETE with a subquery
	// on the same table).
	q := `SELECT device_id, id FROM commands WHERE state IN (?, ?, ?) AND completed_at > 0 AND completed_at < ? ORDER BY completed_at`
	args := []any{string(mdm.StateAcknowledged), string(mdm.StateFailed), string(mdm.StateCancelled), nanos(before)}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	type key struct{ dev, id string }
	var keys []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.dev, &k.id); err != nil {
			rows.Close()
			return 0, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	n := 0
	err = s.tx(ctx, func(tx *sql.Tx) error {
		for _, k := range keys {
			if _, err := tx.ExecContext(ctx, s.d.rebind(`DELETE FROM commands WHERE device_id = ? AND id = ?`), k.dev, k.id); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}
