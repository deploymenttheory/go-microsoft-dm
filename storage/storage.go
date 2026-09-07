package storage

import (
	"context"
	"errors"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
)

// Errors shared by every backend.
var (
	ErrNotFound = errors.New("storage: not found")
	ErrConflict = errors.New("storage: conflict")
	ErrInvalid  = errors.New("storage: invalid argument")
)

// State of an enrollment.
type State string

// Enrollment states.
const (
	// StateActive is the current enrollment for its DeviceID.
	StateActive State = "active"
	// StateSuperseded means the device enrolled again; the record is kept.
	StateSuperseded State = "superseded"
	// StateUnenrolled means the device or the server ended the enrollment.
	StateUnenrolled State = "unenrolled"
)

// Valid reports whether the state is one this package defines.
func (s State) Valid() bool {
	return s == StateActive || s == StateSuperseded || s == StateUnenrolled
}

// Enrollment is one issued identity and what the device said when it asked
// for it.
type Enrollment struct {
	// Serial is the decimal serial of the issued certificate; the key.
	Serial string
	// Thumbprint is the certificate's SHA-1 thumbprint, uppercase hex.
	Thumbprint string
	// DeviceID is the MDE2 AdditionalContext DeviceID.
	DeviceID string
	// HWDevID is the hardware identifier, when the client sent one.
	HWDevID string
	// EnrollmentType is Full (user) or Device.
	EnrollmentType enroll.EnrollmentType
	// UPN is the enrolling user's principal name; empty for a device
	// enrollment without one.
	UPN string
	// Context is everything the client sent in AdditionalContext.
	Context enroll.AdditionalContext
	State   State
	// EnrolledAt is when the certificate was issued; UpdatedAt the last
	// state change; LastSeenAt the last management session.
	EnrolledAt time.Time
	UpdatedAt  time.Time
	LastSeenAt time.Time
}

// Validate checks the fields every backend requires.
func (e *Enrollment) Validate() error {
	switch {
	case e == nil:
		return errors.New("storage: nil enrollment")
	case e.Serial == "":
		return errors.New("storage: enrollment needs a certificate serial")
	case e.Thumbprint == "":
		return errors.New("storage: enrollment needs a certificate thumbprint")
	case e.DeviceID == "":
		return errors.New("storage: enrollment needs a DeviceID")
	case !e.EnrollmentType.Valid():
		return errors.New("storage: enrollment type must be Full or Device")
	case e.EnrolledAt.IsZero():
		return errors.New("storage: enrollment needs EnrolledAt")
	}
	return nil
}

// EnrollmentQuery filters List. Zero values mean "any".
type EnrollmentQuery struct {
	State          State
	EnrollmentType enroll.EnrollmentType
	UPN            string
	// DeviceID lists every enrollment a device ever made, newest first
	// when combined with the default order.
	DeviceID string
}

// EnrollmentStore persists enrollments.
type EnrollmentStore interface {
	// Create stores a new enrollment in StateActive. ErrConflict when the
	// serial exists. An active enrollment with the same DeviceID becomes
	// superseded in the same operation.
	Create(ctx context.Context, e *Enrollment) error
	// Get returns the active enrollment for the DeviceID, or ErrNotFound.
	Get(ctx context.Context, deviceID string) (*Enrollment, error)
	// GetBySerial returns the enrollment bound to the certificate serial,
	// whatever its state, or ErrNotFound.
	GetBySerial(ctx context.Context, serial string) (*Enrollment, error)
	// GetByThumbprint returns the enrollment bound to the certificate
	// thumbprint, whatever its state, or ErrNotFound.
	GetByThumbprint(ctx context.Context, thumbprint string) (*Enrollment, error)
	// ListByHWDevID returns every enrollment the hardware ever made,
	// oldest first; empty, not ErrNotFound, when none.
	ListByHWDevID(ctx context.Context, hwDevID string) ([]Enrollment, error)
	// SetState changes the state of the enrollment with the serial and
	// stamps UpdatedAt. ErrNotFound for an unknown serial; ErrInvalid for
	// an unknown state.
	SetState(ctx context.Context, serial string, state State, at time.Time) error
	// TouchLastSeen records activity on the enrollment with the serial.
	TouchLastSeen(ctx context.Context, serial string, at time.Time) error
	// List pages through enrollments ordered by EnrolledAt then Serial.
	List(ctx context.Context, q EnrollmentQuery, p paging.Page) (paging.Result[Enrollment], error)
}

// Certificate is an issued certificate as the store keeps it.
type Certificate struct {
	// Serial is the decimal serial; the key.
	Serial string
	// Thumbprint is the SHA-1 thumbprint, uppercase hex; unique.
	Thumbprint string
	// Subject is the RFC 2253 subject string.
	Subject string
	// DeviceID links the certificate to the enrollment it was issued for.
	DeviceID  string
	NotBefore time.Time
	NotAfter  time.Time
	// Raw is the DER encoding.
	Raw []byte
	// Revoked and RevokedAt record revocation; RevokedAt is zero when not.
	Revoked   bool
	RevokedAt time.Time
}

// Validate checks the fields every backend requires.
func (c *Certificate) Validate() error {
	switch {
	case c == nil:
		return errors.New("storage: nil certificate")
	case c.Serial == "":
		return errors.New("storage: certificate needs a serial")
	case c.Thumbprint == "":
		return errors.New("storage: certificate needs a thumbprint")
	case len(c.Raw) == 0:
		return errors.New("storage: certificate needs its DER encoding")
	case c.NotBefore.IsZero() || c.NotAfter.IsZero() || !c.NotAfter.After(c.NotBefore):
		return errors.New("storage: certificate needs a validity period")
	}
	return nil
}

// CertificateQuery filters ListCertificates. Zero values mean "any".
type CertificateQuery struct {
	DeviceID string
	// Revoked, when set, selects revoked (true) or unrevoked (false).
	Revoked *bool
	// ExpiresBefore selects certificates whose NotAfter is before it.
	ExpiresBefore time.Time
}

// CertificateStore persists issued certificates.
type CertificateStore interface {
	// PutCertificate stores a certificate. ErrConflict when the serial or
	// thumbprint exists.
	PutCertificate(ctx context.Context, c *Certificate) error
	// Certificate returns the certificate with the serial, or ErrNotFound.
	Certificate(ctx context.Context, serial string) (*Certificate, error)
	// CertificateByThumbprint returns the certificate with the thumbprint,
	// or ErrNotFound.
	CertificateByThumbprint(ctx context.Context, thumbprint string) (*Certificate, error)
	// Revoke marks the certificate revoked at the time. Revoking twice
	// keeps the first time. ErrNotFound for an unknown serial.
	Revoke(ctx context.Context, serial string, at time.Time) error
	// ListCertificates pages through certificates ordered by NotBefore then
	// Serial.
	ListCertificates(ctx context.Context, q CertificateQuery, p paging.Page) (paging.Result[Certificate], error)
}

// Store is everything a backend implements.
type Store interface {
	EnrollmentStore
	CertificateStore
}
