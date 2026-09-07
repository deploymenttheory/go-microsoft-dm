package storagetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// Factory returns a fresh, empty store for one test.
type Factory func(t *testing.T) storage.Store

// RunAll runs every suite.
func RunAll(t *testing.T, newStore Factory) {
	t.Helper()
	t.Run("Enrollment", func(t *testing.T) { RunEnrollmentSuite(t, newStore) })
	t.Run("Certificate", func(t *testing.T) { RunCertificateSuite(t, newStore) })
	t.Run("Concurrency", func(t *testing.T) { RunConcurrencySuite(t, newStore) })
}

var t0 = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// Enrollment returns a valid enrollment for device n with serial s.
func Enrollment(n int, serial string, at time.Time) *storage.Enrollment {
	return &storage.Enrollment{
		Serial: serial, Thumbprint: "TP-" + serial, DeviceID: fmt.Sprintf("DEVICE-%02d", n), HWDevID: fmt.Sprintf("HW-%02d", n),
		EnrollmentType: enroll.EnrollmentTypeFull, UPN: fmt.Sprintf("user%d@contoso.com", n),
		Context: enroll.AdditionalContext{DeviceID: fmt.Sprintf("DEVICE-%02d", n), DeviceName: "PC", MAC: []string{"AA:BB"},
			Items: []enroll.ContextItem{{Name: "DeviceName", Value: "PC"}}},
		EnrolledAt: at,
	}
}

// Certificate returns a valid certificate record with serial s.
func Certificate(serial, deviceID string, notBefore time.Time) *storage.Certificate {
	return &storage.Certificate{
		Serial: serial, Thumbprint: "TP-" + serial, Subject: "CN=" + deviceID, DeviceID: deviceID,
		NotBefore: notBefore, NotAfter: notBefore.Add(365 * 24 * time.Hour), Raw: []byte("der-" + serial),
	}
}

// RunEnrollmentSuite exercises storage.EnrollmentStore.
func RunEnrollmentSuite(t *testing.T, newStore Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("CreateAndLookups", func(t *testing.T) {
		s := newStore(t)
		e := Enrollment(1, "100", t0)
		if err := s.Create(ctx, e); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := s.Get(ctx, "DEVICE-01")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.State != storage.StateActive || got.Serial != "100" || !got.UpdatedAt.Equal(t0) || got.UPN != e.UPN || got.HWDevID != "HW-01" {
			t.Errorf("Get = %+v", got)
		}
		if len(got.Context.Items) != 1 || len(got.Context.MAC) != 1 {
			t.Errorf("context lost: %+v", got.Context)
		}
		bySerial, err := s.GetBySerial(ctx, "100")
		if err != nil || bySerial.DeviceID != "DEVICE-01" {
			t.Errorf("GetBySerial = %+v, %v", bySerial, err)
		}
		byThumb, err := s.GetByThumbprint(ctx, "TP-100")
		if err != nil || byThumb.Serial != "100" {
			t.Errorf("GetByThumbprint = %+v, %v", byThumb, err)
		}
		// Returned records are copies.
		got.Context.Items[0].Value = "changed"
		got.Context.MAC[0] = "changed"
		again, _ := s.Get(ctx, "DEVICE-01")
		if again.Context.Items[0].Value != "PC" || again.Context.MAC[0] != "AA:BB" {
			t.Error("store shares memory with callers")
		}
		// The caller's struct is not retained either.
		e.Context.Items[0].Value = "changed"
		again, _ = s.Get(ctx, "DEVICE-01")
		if again.Context.Items[0].Value != "PC" {
			t.Error("store retained the caller's slice")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.Get(ctx, "nope"); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("Get: %v", err)
		}
		if _, err := s.GetBySerial(ctx, "nope"); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("GetBySerial: %v", err)
		}
		if _, err := s.GetByThumbprint(ctx, "nope"); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("GetByThumbprint: %v", err)
		}
		if err := s.SetState(ctx, "nope", storage.StateUnenrolled, t0); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("SetState: %v", err)
		}
		if err := s.TouchLastSeen(ctx, "nope", t0); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("TouchLastSeen: %v", err)
		}
		list, err := s.ListByHWDevID(ctx, "HW-99")
		if err != nil || len(list) != 0 {
			t.Errorf("ListByHWDevID = %v, %v", list, err)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		s := newStore(t)
		bad := []*storage.Enrollment{
			nil,
			{Thumbprint: "t", DeviceID: "d", EnrollmentType: enroll.EnrollmentTypeFull, EnrolledAt: t0},
			{Serial: "1", DeviceID: "d", EnrollmentType: enroll.EnrollmentTypeFull, EnrolledAt: t0},
			{Serial: "1", Thumbprint: "t", EnrollmentType: enroll.EnrollmentTypeFull, EnrolledAt: t0},
			{Serial: "1", Thumbprint: "t", DeviceID: "d", EnrolledAt: t0},
			{Serial: "1", Thumbprint: "t", DeviceID: "d", EnrollmentType: enroll.EnrollmentTypeFull},
		}
		for i, e := range bad {
			if err := s.Create(ctx, e); !errors.Is(err, storage.ErrInvalid) {
				t.Errorf("Create(bad %d): %v", i, err)
			}
		}
		if err := s.Create(ctx, Enrollment(1, "1", t0)); err != nil {
			t.Fatal(err)
		}
		if err := s.SetState(ctx, "1", "sleeping", t0); !errors.Is(err, storage.ErrInvalid) {
			t.Errorf("SetState invalid: %v", err)
		}
		if _, err := s.ListByHWDevID(ctx, ""); !errors.Is(err, storage.ErrInvalid) {
			t.Errorf("ListByHWDevID empty: %v", err)
		}
		if _, err := s.List(ctx, storage.EnrollmentQuery{}, paging.Page{Cursor: "x"}); !errors.Is(err, storage.ErrInvalid) {
			t.Errorf("List bad cursor: %v", err)
		}
	})

	t.Run("Conflict", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(ctx, Enrollment(1, "1", t0)); err != nil {
			t.Fatal(err)
		}
		if err := s.Create(ctx, Enrollment(2, "1", t0)); !errors.Is(err, storage.ErrConflict) {
			t.Errorf("duplicate serial: %v", err)
		}
		dup := Enrollment(2, "2", t0)
		dup.Thumbprint = "TP-1"
		if err := s.Create(ctx, dup); !errors.Is(err, storage.ErrConflict) {
			t.Errorf("duplicate thumbprint: %v", err)
		}
	})

	t.Run("ReenrollmentSupersedes", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(ctx, Enrollment(1, "1", t0)); err != nil {
			t.Fatal(err)
		}
		second := Enrollment(1, "2", t0.Add(time.Hour))
		if err := s.Create(ctx, second); err != nil {
			t.Fatal(err)
		}
		cur, err := s.Get(ctx, "DEVICE-01")
		if err != nil || cur.Serial != "2" || cur.State != storage.StateActive {
			t.Errorf("current = %+v, %v", cur, err)
		}
		old, err := s.GetBySerial(ctx, "1")
		if err != nil || old.State != storage.StateSuperseded || !old.UpdatedAt.Equal(t0.Add(time.Hour)) {
			t.Errorf("old = %+v, %v", old, err)
		}
		hist, err := s.ListByHWDevID(ctx, "HW-01")
		if err != nil || len(hist) != 2 || hist[0].Serial != "1" || hist[1].Serial != "2" {
			t.Errorf("history = %+v, %v", hist, err)
		}
		all, err := s.List(ctx, storage.EnrollmentQuery{DeviceID: "DEVICE-01"}, paging.Page{})
		if err != nil || len(all.Items) != 2 {
			t.Errorf("List by device = %d, %v", len(all.Items), err)
		}
	})

	t.Run("States", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(ctx, Enrollment(1, "1", t0)); err != nil {
			t.Fatal(err)
		}
		if err := s.SetState(ctx, "1", storage.StateUnenrolled, t0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, "DEVICE-01"); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("unenrolled device still current: %v", err)
		}
		e, _ := s.GetBySerial(ctx, "1")
		if e.State != storage.StateUnenrolled || !e.UpdatedAt.Equal(t0.Add(time.Minute)) {
			t.Errorf("state = %+v", e)
		}
		// Reactivating makes it current again.
		if err := s.SetState(ctx, "1", storage.StateActive, t0.Add(2*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if cur, err := s.Get(ctx, "DEVICE-01"); err != nil || cur.Serial != "1" {
			t.Errorf("reactivated = %+v, %v", cur, err)
		}
		// Activating another serial for the same device supersedes the current one.
		if err := s.Create(ctx, Enrollment(1, "2", t0.Add(3*time.Minute))); err != nil {
			t.Fatal(err)
		}
		if err := s.SetState(ctx, "1", storage.StateActive, t0.Add(4*time.Minute)); err != nil {
			t.Fatal(err)
		}
		cur, _ := s.Get(ctx, "DEVICE-01")
		two, _ := s.GetBySerial(ctx, "2")
		if cur.Serial != "1" || two.State != storage.StateSuperseded {
			t.Errorf("swap: current %s, other %s", cur.Serial, two.State)
		}
		// Unenrolling a superseded record leaves the current one alone.
		if err := s.SetState(ctx, "2", storage.StateUnenrolled, t0.Add(5*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if cur, err := s.Get(ctx, "DEVICE-01"); err != nil || cur.Serial != "1" {
			t.Errorf("current after unenrolling other = %+v, %v", cur, err)
		}
	})

	t.Run("LastSeen", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(ctx, Enrollment(1, "1", t0)); err != nil {
			t.Fatal(err)
		}
		if err := s.TouchLastSeen(ctx, "1", t0.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if err := s.TouchLastSeen(ctx, "1", t0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		e, _ := s.GetBySerial(ctx, "1")
		if !e.LastSeenAt.Equal(t0.Add(time.Hour)) {
			t.Errorf("LastSeenAt = %s, want the later time kept", e.LastSeenAt)
		}
	})

	t.Run("ListFiltersAndPaging", func(t *testing.T) {
		s := newStore(t)
		for i := 1; i <= 5; i++ {
			e := Enrollment(i, fmt.Sprintf("%d", i), t0.Add(time.Duration(i)*time.Minute))
			if i%2 == 0 {
				e.EnrollmentType = enroll.EnrollmentTypeDevice
				e.UPN = ""
			}
			if err := s.Create(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.SetState(ctx, "5", storage.StateUnenrolled, t0.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		res, err := s.List(ctx, storage.EnrollmentQuery{}, paging.Page{Limit: 2})
		if err != nil || len(res.Items) != 2 || res.Items[0].Serial != "1" || res.Items[1].Serial != "2" || res.NextCursor == "" {
			t.Fatalf("page 1 = %+v, %v", res, err)
		}
		res, err = s.List(ctx, storage.EnrollmentQuery{}, paging.Page{Limit: 2, Cursor: res.NextCursor})
		if err != nil || len(res.Items) != 2 || res.Items[0].Serial != "3" || res.NextCursor == "" {
			t.Fatalf("page 2 = %+v, %v", res, err)
		}
		res, err = s.List(ctx, storage.EnrollmentQuery{}, paging.Page{Limit: 2, Cursor: res.NextCursor})
		if err != nil || len(res.Items) != 1 || res.Items[0].Serial != "5" || res.NextCursor != "" {
			t.Fatalf("page 3 = %+v, %v", res, err)
		}
		res, err = s.List(ctx, storage.EnrollmentQuery{}, paging.Page{Limit: 2, Cursor: "99"})
		if err != nil || len(res.Items) != 0 || res.NextCursor != "" {
			t.Fatalf("past the end = %+v, %v", res, err)
		}
		cases := map[string]struct {
			q    storage.EnrollmentQuery
			want []string
		}{
			"active":  {q: storage.EnrollmentQuery{State: storage.StateActive}, want: []string{"1", "2", "3", "4"}},
			"device":  {q: storage.EnrollmentQuery{EnrollmentType: enroll.EnrollmentTypeDevice}, want: []string{"2", "4"}},
			"upn":     {q: storage.EnrollmentQuery{UPN: "user3@contoso.com"}, want: []string{"3"}},
			"devid":   {q: storage.EnrollmentQuery{DeviceID: "DEVICE-05"}, want: []string{"5"}},
			"nothing": {q: storage.EnrollmentQuery{UPN: "nobody"}, want: nil},
		}
		for name, tc := range cases {
			res, err := s.List(ctx, tc.q, paging.Page{})
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			var got []string
			for _, e := range res.Items {
				got = append(got, e.Serial)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("%s = %v, want %v", name, got, tc.want)
			}
		}
	})
}

// RunCertificateSuite exercises storage.CertificateStore.
func RunCertificateSuite(t *testing.T, newStore Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("PutAndGet", func(t *testing.T) {
		s := newStore(t)
		c := Certificate("100", "DEVICE-01", t0)
		if err := s.PutCertificate(ctx, c); err != nil {
			t.Fatal(err)
		}
		got, err := s.Certificate(ctx, "100")
		if err != nil || got.Thumbprint != "TP-100" || string(got.Raw) != "der-100" || got.Revoked {
			t.Errorf("Certificate = %+v, %v", got, err)
		}
		byThumb, err := s.CertificateByThumbprint(ctx, "TP-100")
		if err != nil || byThumb.Serial != "100" {
			t.Errorf("CertificateByThumbprint = %+v, %v", byThumb, err)
		}
		got.Raw[0] = 'X'
		c.Raw[1] = 'Y'
		again, _ := s.Certificate(ctx, "100")
		if string(again.Raw) != "der-100" {
			t.Error("store shares DER with callers")
		}
	})

	t.Run("NotFoundAndInvalid", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.Certificate(ctx, "nope"); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("Certificate: %v", err)
		}
		if _, err := s.CertificateByThumbprint(ctx, "nope"); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("CertificateByThumbprint: %v", err)
		}
		if err := s.Revoke(ctx, "nope", t0); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("Revoke: %v", err)
		}
		bad := []*storage.Certificate{
			nil,
			{Thumbprint: "t", Raw: []byte{1}, NotBefore: t0, NotAfter: t0.Add(time.Hour)},
			{Serial: "1", Raw: []byte{1}, NotBefore: t0, NotAfter: t0.Add(time.Hour)},
			{Serial: "1", Thumbprint: "t", NotBefore: t0, NotAfter: t0.Add(time.Hour)},
			{Serial: "1", Thumbprint: "t", Raw: []byte{1}},
			{Serial: "1", Thumbprint: "t", Raw: []byte{1}, NotBefore: t0, NotAfter: t0},
		}
		for i, c := range bad {
			if err := s.PutCertificate(ctx, c); !errors.Is(err, storage.ErrInvalid) {
				t.Errorf("PutCertificate(bad %d): %v", i, err)
			}
		}
		if _, err := s.ListCertificates(ctx, storage.CertificateQuery{}, paging.Page{Cursor: "-1"}); !errors.Is(err, storage.ErrInvalid) {
			t.Errorf("bad cursor: %v", err)
		}
	})

	t.Run("Conflict", func(t *testing.T) {
		s := newStore(t)
		if err := s.PutCertificate(ctx, Certificate("1", "d", t0)); err != nil {
			t.Fatal(err)
		}
		if err := s.PutCertificate(ctx, Certificate("1", "d", t0)); !errors.Is(err, storage.ErrConflict) {
			t.Errorf("duplicate serial: %v", err)
		}
		dup := Certificate("2", "d", t0)
		dup.Thumbprint = "TP-1"
		if err := s.PutCertificate(ctx, dup); !errors.Is(err, storage.ErrConflict) {
			t.Errorf("duplicate thumbprint: %v", err)
		}
	})

	t.Run("Revoke", func(t *testing.T) {
		s := newStore(t)
		if err := s.PutCertificate(ctx, Certificate("1", "d", t0)); err != nil {
			t.Fatal(err)
		}
		if err := s.Revoke(ctx, "1", t0.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if err := s.Revoke(ctx, "1", t0.Add(2*time.Hour)); err != nil {
			t.Fatal(err)
		}
		c, _ := s.Certificate(ctx, "1")
		if !c.Revoked || !c.RevokedAt.Equal(t0.Add(time.Hour)) {
			t.Errorf("revoked = %+v", c)
		}
	})

	t.Run("ListFiltersAndPaging", func(t *testing.T) {
		s := newStore(t)
		for i := 1; i <= 4; i++ {
			dev := "DEVICE-01"
			if i > 2 {
				dev = "DEVICE-02"
			}
			c := Certificate(fmt.Sprintf("%d", i), dev, t0.Add(time.Duration(i)*time.Minute))
			if err := s.PutCertificate(ctx, c); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Revoke(ctx, "2", t0); err != nil {
			t.Fatal(err)
		}
		yes, no := true, false
		cases := map[string]struct {
			q    storage.CertificateQuery
			want []string
		}{
			"all":       {q: storage.CertificateQuery{}, want: []string{"1", "2", "3", "4"}},
			"device":    {q: storage.CertificateQuery{DeviceID: "DEVICE-02"}, want: []string{"3", "4"}},
			"revoked":   {q: storage.CertificateQuery{Revoked: &yes}, want: []string{"2"}},
			"unrevoked": {q: storage.CertificateQuery{Revoked: &no}, want: []string{"1", "3", "4"}},
			"expiring":  {q: storage.CertificateQuery{ExpiresBefore: t0.Add(365*24*time.Hour + 150*time.Second)}, want: []string{"1", "2"}},
		}
		for name, tc := range cases {
			res, err := s.ListCertificates(ctx, tc.q, paging.Page{})
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			var got []string
			for _, c := range res.Items {
				got = append(got, c.Serial)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("%s = %v, want %v", name, got, tc.want)
			}
		}
		res, err := s.ListCertificates(ctx, storage.CertificateQuery{}, paging.Page{Limit: 3})
		if err != nil || len(res.Items) != 3 || res.NextCursor == "" {
			t.Fatalf("page 1 = %+v, %v", res, err)
		}
		res, err = s.ListCertificates(ctx, storage.CertificateQuery{}, paging.Page{Limit: 3, Cursor: res.NextCursor})
		if err != nil || len(res.Items) != 1 || res.Items[0].Serial != "4" || res.NextCursor != "" {
			t.Fatalf("page 2 = %+v, %v", res, err)
		}
	})
}

// RunConcurrencySuite creates enrollments and certificates from many
// goroutines and checks nothing is lost.
func RunConcurrencySuite(t *testing.T, newStore Factory) {
	t.Helper()
	ctx := context.Background()
	s := newStore(t)
	const n = 32
	var wg sync.WaitGroup
	errs := make(chan error, 2*n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			serial := fmt.Sprintf("%d", i)
			if err := s.PutCertificate(ctx, Certificate(serial, fmt.Sprintf("DEVICE-%02d", i), t0)); err != nil {
				errs <- err
			}
			if err := s.Create(ctx, Enrollment(i, serial, t0.Add(time.Duration(i)*time.Second))); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	res, err := s.List(ctx, storage.EnrollmentQuery{}, paging.Page{Limit: 1000})
	if err != nil || len(res.Items) != n {
		t.Errorf("enrollments = %d, %v", len(res.Items), err)
	}
	certs, err := s.ListCertificates(ctx, storage.CertificateQuery{}, paging.Page{Limit: 1000})
	if err != nil || len(certs.Items) != n {
		t.Errorf("certificates = %d, %v", len(certs.Items), err)
	}
}
