package inmem

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// Store implements storage.Store in memory.
type Store struct {
	mu          sync.Mutex
	enrollments map[string]*storage.Enrollment  // by serial
	active      map[string]string               // device id -> active serial
	byThumb     map[string]string               // thumbprint -> serial
	certs       map[string]*storage.Certificate // by serial
	certThumb   map[string]string               // thumbprint -> serial
	creds       map[string]storage.MDMCredential // device id -> OMA DM credential
}

// New returns an empty store.
func New() *Store {
	return &Store{
		enrollments: map[string]*storage.Enrollment{}, active: map[string]string{}, byThumb: map[string]string{},
		certs: map[string]*storage.Certificate{}, certThumb: map[string]string{},
		creds: map[string]storage.MDMCredential{},
	}
}

var _ storage.Store = (*Store)(nil)

// Create implements storage.EnrollmentStore.
func (s *Store) Create(_ context.Context, e *storage.Enrollment) error {
	if err := e.Validate(); err != nil {
		return fmt.Errorf("%w: %w", storage.ErrInvalid, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.enrollments[e.Serial]; ok {
		return fmt.Errorf("%w: enrollment with serial %s exists", storage.ErrConflict, e.Serial)
	}
	if _, ok := s.byThumb[e.Thumbprint]; ok {
		return fmt.Errorf("%w: enrollment with thumbprint %s exists", storage.ErrConflict, e.Thumbprint)
	}
	rec := *e
	rec.State = storage.StateActive
	rec.UpdatedAt = e.EnrolledAt
	rec.Context.Items = append([]enroll.ContextItem(nil), e.Context.Items...)
	rec.Context.Unknown = append([]enroll.ContextItem(nil), e.Context.Unknown...)
	if prev, ok := s.active[e.DeviceID]; ok {
		s.enrollments[prev].State = storage.StateSuperseded
		s.enrollments[prev].UpdatedAt = e.EnrolledAt
	}
	s.enrollments[e.Serial] = &rec
	s.active[e.DeviceID] = e.Serial
	s.byThumb[e.Thumbprint] = e.Serial
	return nil
}

// Get implements storage.EnrollmentStore.
func (s *Store) Get(_ context.Context, deviceID string) (*storage.Enrollment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	serial, ok := s.active[deviceID]
	if !ok {
		return nil, fmt.Errorf("%w: device %s", storage.ErrNotFound, deviceID)
	}
	return copyEnrollment(s.enrollments[serial]), nil
}

// GetBySerial implements storage.EnrollmentStore.
func (s *Store) GetBySerial(_ context.Context, serial string) (*storage.Enrollment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.enrollments[serial]
	if !ok {
		return nil, fmt.Errorf("%w: serial %s", storage.ErrNotFound, serial)
	}
	return copyEnrollment(e), nil
}

// GetByThumbprint implements storage.EnrollmentStore.
func (s *Store) GetByThumbprint(_ context.Context, thumbprint string) (*storage.Enrollment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	serial, ok := s.byThumb[thumbprint]
	if !ok {
		return nil, fmt.Errorf("%w: thumbprint %s", storage.ErrNotFound, thumbprint)
	}
	return copyEnrollment(s.enrollments[serial]), nil
}

// ListByHWDevID implements storage.EnrollmentStore.
func (s *Store) ListByHWDevID(_ context.Context, hwDevID string) ([]storage.Enrollment, error) {
	if hwDevID == "" {
		return nil, fmt.Errorf("%w: empty HWDevID", storage.ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []storage.Enrollment
	for _, e := range s.enrollments {
		if e.HWDevID == hwDevID {
			out = append(out, *copyEnrollment(e))
		}
	}
	sortEnrollments(out)
	return out, nil
}

// SetState implements storage.EnrollmentStore.
func (s *Store) SetState(_ context.Context, serial string, state storage.State, at time.Time) error {
	if !state.Valid() {
		return fmt.Errorf("%w: state %q", storage.ErrInvalid, state)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.enrollments[serial]
	if !ok {
		return fmt.Errorf("%w: serial %s", storage.ErrNotFound, serial)
	}
	e.State = state
	e.UpdatedAt = at.UTC()
	if state != storage.StateActive && s.active[e.DeviceID] == serial {
		delete(s.active, e.DeviceID)
	}
	if state == storage.StateActive {
		if prev, ok := s.active[e.DeviceID]; ok && prev != serial {
			s.enrollments[prev].State = storage.StateSuperseded
			s.enrollments[prev].UpdatedAt = at.UTC()
		}
		s.active[e.DeviceID] = serial
	}
	return nil
}

// TouchLastSeen implements storage.EnrollmentStore.
func (s *Store) TouchLastSeen(_ context.Context, serial string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.enrollments[serial]
	if !ok {
		return fmt.Errorf("%w: serial %s", storage.ErrNotFound, serial)
	}
	if at.After(e.LastSeenAt) {
		e.LastSeenAt = at.UTC()
	}
	return nil
}

// List implements storage.EnrollmentStore.
func (s *Store) List(_ context.Context, q storage.EnrollmentQuery, p paging.Page) (paging.Result[storage.Enrollment], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []storage.Enrollment
	for _, e := range s.enrollments {
		if q.State != "" && e.State != q.State {
			continue
		}
		if q.EnrollmentType != "" && e.EnrollmentType != q.EnrollmentType {
			continue
		}
		if q.UPN != "" && e.UPN != q.UPN {
			continue
		}
		if q.DeviceID != "" && e.DeviceID != q.DeviceID {
			continue
		}
		all = append(all, *copyEnrollment(e))
	}
	sortEnrollments(all)
	return page(all, p)
}

// PutCertificate implements storage.CertificateStore.
func (s *Store) PutCertificate(_ context.Context, c *storage.Certificate) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("%w: %w", storage.ErrInvalid, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.certs[c.Serial]; ok {
		return fmt.Errorf("%w: certificate with serial %s exists", storage.ErrConflict, c.Serial)
	}
	if _, ok := s.certThumb[c.Thumbprint]; ok {
		return fmt.Errorf("%w: certificate with thumbprint %s exists", storage.ErrConflict, c.Thumbprint)
	}
	rec := *c
	rec.Raw = append([]byte(nil), c.Raw...)
	s.certs[c.Serial] = &rec
	s.certThumb[c.Thumbprint] = c.Serial
	return nil
}

// Certificate implements storage.CertificateStore.
func (s *Store) Certificate(_ context.Context, serial string) (*storage.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.certs[serial]
	if !ok {
		return nil, fmt.Errorf("%w: certificate %s", storage.ErrNotFound, serial)
	}
	return copyCertificate(c), nil
}

// CertificateByThumbprint implements storage.CertificateStore.
func (s *Store) CertificateByThumbprint(_ context.Context, thumbprint string) (*storage.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	serial, ok := s.certThumb[thumbprint]
	if !ok {
		return nil, fmt.Errorf("%w: certificate thumbprint %s", storage.ErrNotFound, thumbprint)
	}
	return copyCertificate(s.certs[serial]), nil
}

// Revoke implements storage.CertificateStore.
func (s *Store) Revoke(_ context.Context, serial string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.certs[serial]
	if !ok {
		return fmt.Errorf("%w: certificate %s", storage.ErrNotFound, serial)
	}
	if !c.Revoked {
		c.Revoked = true
		c.RevokedAt = at.UTC()
	}
	return nil
}

// ListCertificates implements storage.CertificateStore.
func (s *Store) ListCertificates(_ context.Context, q storage.CertificateQuery, p paging.Page) (paging.Result[storage.Certificate], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []storage.Certificate
	for _, c := range s.certs {
		if q.DeviceID != "" && c.DeviceID != q.DeviceID {
			continue
		}
		if q.Revoked != nil && c.Revoked != *q.Revoked {
			continue
		}
		if !q.ExpiresBefore.IsZero() && !c.NotAfter.Before(q.ExpiresBefore) {
			continue
		}
		all = append(all, *copyCertificate(c))
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].NotBefore.Equal(all[j].NotBefore) {
			return all[i].NotBefore.Before(all[j].NotBefore)
		}
		return all[i].Serial < all[j].Serial
	})
	return page(all, p)
}

func sortEnrollments(es []storage.Enrollment) {
	sort.Slice(es, func(i, j int) bool {
		if !es[i].EnrolledAt.Equal(es[j].EnrolledAt) {
			return es[i].EnrolledAt.Before(es[j].EnrolledAt)
		}
		return es[i].Serial < es[j].Serial
	})
}

// page applies an offset cursor to a sorted slice.
func page[T any](all []T, p paging.Page) (paging.Result[T], error) {
	offset := 0
	if p.Cursor != "" {
		n, err := strconv.Atoi(p.Cursor)
		if err != nil || n < 0 {
			return paging.Result[T]{}, fmt.Errorf("%w: cursor %q", storage.ErrInvalid, p.Cursor)
		}
		offset = n
	}
	size := p.Size()
	if offset > len(all) {
		offset = len(all)
	}
	end := offset + size
	if end > len(all) {
		end = len(all)
	}
	res := paging.Result[T]{Items: all[offset:end]}
	if end < len(all) {
		res.NextCursor = strconv.Itoa(end)
	}
	return res, nil
}

func copyEnrollment(e *storage.Enrollment) *storage.Enrollment {
	c := *e
	c.Context.Items = append([]enroll.ContextItem(nil), e.Context.Items...)
	c.Context.Unknown = append([]enroll.ContextItem(nil), e.Context.Unknown...)
	c.Context.MAC = append([]string(nil), e.Context.MAC...)
	c.Context.IMEI = append([]string(nil), e.Context.IMEI...)
	return &c
}

func copyCertificate(c *storage.Certificate) *storage.Certificate {
	out := *c
	out.Raw = append([]byte(nil), c.Raw...)
	return &out
}

