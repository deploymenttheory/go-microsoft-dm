package storagetest

import (
	"context"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// Failing wraps a Store and returns the configured error from any method
// whose name is in Fail.
type Failing struct {
	storage.Store
	Fail map[string]error
}

func (f *Failing) fail(method string) error {
	if f.Fail == nil {
		return nil
	}
	return f.Fail[method]
}

// Create implements storage.EnrollmentStore.
func (f *Failing) Create(ctx context.Context, e *storage.Enrollment) error {
	if err := f.fail("Create"); err != nil {
		return err
	}
	return f.Store.Create(ctx, e)
}

// Get implements storage.EnrollmentStore.
func (f *Failing) Get(ctx context.Context, deviceID string) (*storage.Enrollment, error) {
	if err := f.fail("Get"); err != nil {
		return nil, err
	}
	return f.Store.Get(ctx, deviceID)
}

// GetBySerial implements storage.EnrollmentStore.
func (f *Failing) GetBySerial(ctx context.Context, serial string) (*storage.Enrollment, error) {
	if err := f.fail("GetBySerial"); err != nil {
		return nil, err
	}
	return f.Store.GetBySerial(ctx, serial)
}

// GetByThumbprint implements storage.EnrollmentStore.
func (f *Failing) GetByThumbprint(ctx context.Context, thumbprint string) (*storage.Enrollment, error) {
	if err := f.fail("GetByThumbprint"); err != nil {
		return nil, err
	}
	return f.Store.GetByThumbprint(ctx, thumbprint)
}

// ListByHWDevID implements storage.EnrollmentStore.
func (f *Failing) ListByHWDevID(ctx context.Context, hwDevID string) ([]storage.Enrollment, error) {
	if err := f.fail("ListByHWDevID"); err != nil {
		return nil, err
	}
	return f.Store.ListByHWDevID(ctx, hwDevID)
}

// SetState implements storage.EnrollmentStore.
func (f *Failing) SetState(ctx context.Context, serial string, state storage.State, at time.Time) error {
	if err := f.fail("SetState"); err != nil {
		return err
	}
	return f.Store.SetState(ctx, serial, state, at)
}

// TouchLastSeen implements storage.EnrollmentStore.
func (f *Failing) TouchLastSeen(ctx context.Context, serial string, at time.Time) error {
	if err := f.fail("TouchLastSeen"); err != nil {
		return err
	}
	return f.Store.TouchLastSeen(ctx, serial, at)
}

// List implements storage.EnrollmentStore.
func (f *Failing) List(ctx context.Context, q storage.EnrollmentQuery, p paging.Page) (paging.Result[storage.Enrollment], error) {
	if err := f.fail("List"); err != nil {
		return paging.Result[storage.Enrollment]{}, err
	}
	return f.Store.List(ctx, q, p)
}

// PutCertificate implements storage.CertificateStore.
func (f *Failing) PutCertificate(ctx context.Context, c *storage.Certificate) error {
	if err := f.fail("PutCertificate"); err != nil {
		return err
	}
	return f.Store.PutCertificate(ctx, c)
}

// Certificate implements storage.CertificateStore.
func (f *Failing) Certificate(ctx context.Context, serial string) (*storage.Certificate, error) {
	if err := f.fail("Certificate"); err != nil {
		return nil, err
	}
	return f.Store.Certificate(ctx, serial)
}

// CertificateByThumbprint implements storage.CertificateStore.
func (f *Failing) CertificateByThumbprint(ctx context.Context, thumbprint string) (*storage.Certificate, error) {
	if err := f.fail("CertificateByThumbprint"); err != nil {
		return nil, err
	}
	return f.Store.CertificateByThumbprint(ctx, thumbprint)
}

// Revoke implements storage.CertificateStore.
func (f *Failing) Revoke(ctx context.Context, serial string, at time.Time) error {
	if err := f.fail("Revoke"); err != nil {
		return err
	}
	return f.Store.Revoke(ctx, serial, at)
}

// ListCertificates implements storage.CertificateStore.
func (f *Failing) ListCertificates(ctx context.Context, q storage.CertificateQuery, p paging.Page) (paging.Result[storage.Certificate], error) {
	if err := f.fail("ListCertificates"); err != nil {
		return paging.Result[storage.Certificate]{}, err
	}
	return f.Store.ListCertificates(ctx, q, p)
}
