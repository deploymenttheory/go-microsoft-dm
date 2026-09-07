package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/wapprov"
)

// Conflict reports that a hardware identifier already belongs to another
// device identifier. It is an event, not an error: the new enrollment is
// recorded and the operator decides what a repeated HWDevID means.
type Conflict struct {
	HWDevID string
	// Existing are the earlier enrollments with the same HWDevID and a
	// different DeviceID.
	Existing []Enrollment
	// New is the enrollment just recorded.
	New Enrollment
}

// ConflictSink receives Conflicts. Its error aborts the enrollment.
type ConflictSink interface {
	HWDevIDConflict(ctx context.Context, c Conflict) error
}

// ConflictSinkFunc adapts a function to ConflictSink.
type ConflictSinkFunc func(ctx context.Context, c Conflict) error

// HWDevIDConflict implements ConflictSink.
func (f ConflictSinkFunc) HWDevIDConflict(ctx context.Context, c Conflict) error { return f(ctx, c) }

// Recorder implements enroll.Recorder over a Store.
type Recorder struct {
	Store Store
	// Conflicts receives HWDevID conflicts; nil ignores them.
	Conflicts ConflictSink
}

var _ enroll.Recorder = (*Recorder)(nil)

// Record stores the issued certificate and the enrollment, supersedes the
// device's previous enrollment, and reports a HWDevID conflict when the
// hardware was last seen under another DeviceID.
func (r *Recorder) Record(ctx context.Context, e *enroll.Enrollment, _ *wapprov.Document) error {
	if r == nil || r.Store == nil {
		return errors.New("storage: recorder has no store")
	}
	if e == nil || e.Certificate == nil || e.Request == nil {
		return fmt.Errorf("%w: enrollment without certificate", ErrInvalid)
	}
	cert := e.Certificate
	rec := Enrollment{
		Serial: cert.SerialNumber.String(), Thumbprint: wapprov.Thumbprint(cert.Raw),
		DeviceID: e.Request.Context.DeviceID, HWDevID: e.Request.Context.HWDevID,
		EnrollmentType: e.Request.Context.EnrollmentType, UPN: e.Request.Principal.UPN,
		Context: e.Request.Context, EnrolledAt: e.EnrolledAt,
	}
	if err := r.Store.PutCertificate(ctx, &Certificate{
		Serial: rec.Serial, Thumbprint: rec.Thumbprint, Subject: cert.Subject.String(), DeviceID: rec.DeviceID,
		NotBefore: cert.NotBefore, NotAfter: cert.NotAfter, Raw: cert.Raw,
	}); err != nil {
		return fmt.Errorf("storage: record certificate: %w", err)
	}
	var existing []Enrollment
	if rec.HWDevID != "" {
		all, err := r.Store.ListByHWDevID(ctx, rec.HWDevID)
		if err != nil {
			return fmt.Errorf("storage: look up HWDevID: %w", err)
		}
		for _, prev := range all {
			if prev.DeviceID != rec.DeviceID {
				existing = append(existing, prev)
			}
		}
	}
	if err := r.Store.Create(ctx, &rec); err != nil {
		return fmt.Errorf("storage: record enrollment: %w", err)
	}
	if len(existing) > 0 && r.Conflicts != nil {
		if err := r.Conflicts.HWDevIDConflict(ctx, Conflict{HWDevID: rec.HWDevID, Existing: existing, New: rec}); err != nil {
			return fmt.Errorf("storage: HWDevID conflict: %w", err)
		}
	}
	return nil
}

// Unenroll marks the enrollment with the serial unenrolled and revokes its
// certificate.
func Unenroll(ctx context.Context, s Store, serial string, at time.Time) error {
	if err := s.SetState(ctx, serial, StateUnenrolled, at); err != nil {
		return err
	}
	return s.Revoke(ctx, serial, at)
}
