package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

const certCols = `serial, thumbprint, subject, device_id, not_before, not_after, raw, revoked, revoked_at`

func scanCertificate(sc interface{ Scan(...any) error }) (*storage.Certificate, error) {
	var (
		c                              storage.Certificate
		notBefore, notAfter, revokedAt int64
		revoked                        int64
	)
	if err := sc.Scan(&c.Serial, &c.Thumbprint, &c.Subject, &c.DeviceID, &notBefore, &notAfter, &c.Raw, &revoked, &revokedAt); err != nil {
		return nil, err
	}
	c.NotBefore, c.NotAfter, c.RevokedAt = fromNanos(notBefore), fromNanos(notAfter), fromNanos(revokedAt)
	c.Revoked = revoked != 0
	return &c, nil
}

// PutCertificate implements storage.CertificateStore.
func (s *Store) PutCertificate(ctx context.Context, c *storage.Certificate) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("%w: %w", storage.ErrInvalid, err)
	}
	return s.tx(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := s.rebindRow(ctx, tx, `SELECT 1 FROM certificates WHERE serial = ?`, c.Serial).Scan(&exists); err == nil {
			return fmt.Errorf("%w: certificate with serial %s exists", storage.ErrConflict, c.Serial)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := s.rebindRow(ctx, tx, `SELECT 1 FROM certificates WHERE thumbprint = ?`, c.Thumbprint).Scan(&exists); err == nil {
			return fmt.Errorf("%w: certificate with thumbprint %s exists", storage.ErrConflict, c.Thumbprint)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		_, err := tx.ExecContext(ctx, s.d.rebind(
			`INSERT INTO certificates (`+certCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`),
			c.Serial, c.Thumbprint, c.Subject, c.DeviceID, nanos(c.NotBefore), nanos(c.NotAfter), c.Raw, boolInt(c.Revoked), nanos(c.RevokedAt))
		return err
	})
}

// Certificate implements storage.CertificateStore.
func (s *Store) Certificate(ctx context.Context, serial string) (*storage.Certificate, error) {
	row := s.queryRow(ctx, `SELECT `+certCols+` FROM certificates WHERE serial = ?`, serial)
	c, err := scanCertificate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: certificate %s", storage.ErrNotFound, serial)
	}
	return c, err
}

// CertificateByThumbprint implements storage.CertificateStore.
func (s *Store) CertificateByThumbprint(ctx context.Context, thumbprint string) (*storage.Certificate, error) {
	row := s.queryRow(ctx, `SELECT `+certCols+` FROM certificates WHERE thumbprint = ?`, thumbprint)
	c, err := scanCertificate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: certificate thumbprint %s", storage.ErrNotFound, thumbprint)
	}
	return c, err
}

// Revoke implements storage.CertificateStore.
func (s *Store) Revoke(ctx context.Context, serial string, at time.Time) error {
	res, err := s.exec(ctx, `UPDATE certificates SET revoked = 1, revoked_at = ? WHERE serial = ? AND revoked = 0`, nanos(at), serial)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Either already revoked (keep the first time) or missing.
		var exists int
		if err := s.queryRow(ctx, `SELECT 1 FROM certificates WHERE serial = ?`, serial).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: certificate %s", storage.ErrNotFound, serial)
		} else if err != nil {
			return err
		}
	}
	return nil
}

// ListCertificates implements storage.CertificateStore.
func (s *Store) ListCertificates(ctx context.Context, q storage.CertificateQuery, p paging.Page) (paging.Result[storage.Certificate], error) {
	var clauses []string
	var args []any
	if q.DeviceID != "" {
		clauses = append(clauses, "device_id = ?")
		args = append(args, q.DeviceID)
	}
	if q.Revoked != nil {
		clauses = append(clauses, "revoked = ?")
		args = append(args, boolInt(*q.Revoked))
	}
	if !q.ExpiresBefore.IsZero() {
		clauses = append(clauses, "not_after < ?")
		args = append(args, nanos(q.ExpiresBefore))
	}
	offset, err := offsetOf(p)
	if err != nil {
		return paging.Result[storage.Certificate]{}, err
	}
	size := p.Size()
	args = append(args, size+1, offset)
	rows, err := s.query(ctx, `SELECT `+certCols+` FROM certificates`+whereClause(clauses)+` ORDER BY not_before, serial LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return paging.Result[storage.Certificate]{}, err
	}
	defer rows.Close()
	var items []storage.Certificate
	for rows.Next() {
		c, err := scanCertificate(rows)
		if err != nil {
			return paging.Result[storage.Certificate]{}, err
		}
		items = append(items, *c)
	}
	if err := rows.Err(); err != nil {
		return paging.Result[storage.Certificate]{}, err
	}
	return pageResult(items, size, offset), nil
}
