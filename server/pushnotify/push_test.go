package pushnotify_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/enroll"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/msplatformservices/wns"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/dmclient"
	"github.com/deploymenttheory/go-microsoft-dm/server/pushnotify"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

type sendFunc func(context.Context, string, wns.Notification) (wns.Result, error)

func (f sendFunc) Send(c context.Context, u string, n wns.Notification) (wns.Result, error) {
	return f(c, u, n)
}

func setup(t *testing.T) (*sqlstore.Store, *pushnotify.Service, time.Time) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	s, err := sqlstore.Open(ctx, sqlstore.SQLite, filepath.Join(t.TempDir(), "push.db"), sqlstore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Create(ctx, &storage.Enrollment{Serial: "1", Thumbprint: "thumb", DeviceID: "d", EnrollmentType: enroll.EnrollmentTypeFull, EnrolledAt: now}); err != nil {
		t.Fatal(err)
	}
	return s, &pushnotify.Service{Store: s, Clock: clock.NewFake(now), Sender: sendFunc(func(_ context.Context, _ string, n wns.Notification) (wns.Result, error) {
		if len(n.Body) == 0 || n.TTLSeconds == nil {
			t.Error("missing MDM wake payload/cache lifetime")
		}
		return wns.Result{HTTPStatus: 200, Outcome: wns.Accepted}, nil
	})}, now
}

func TestWakeAvailabilityAndEvidence(t *testing.T) {
	t.Parallel()
	s, p, now := setup(t)
	ctx := context.Background()
	if _, err := p.Wake(ctx, "d"); !errors.Is(err, pushnotify.ErrUnavailable) {
		t.Fatal(err)
	}
	uri, status := "https://cloud.notify.windows.com/private-channel-token", "0"
	if err := s.ObservePush(ctx, "1", &uri, &status, now); err != nil {
		t.Fatal(err)
	}
	p.Clock = clock.NewFake(now.Add(16 * 24 * time.Hour))
	if _, err := p.Wake(ctx, "d"); err != nil {
		t.Fatal(err)
	}
	events, err := s.Events(ctx, "d", 0)
	if err != nil || len(events) != 1 || strings.Contains(string(events[0].Detail), "private-channel-token") || !strings.Contains(string(events[0].Detail), `"renewal_due":true`) {
		t.Fatal("unsafe or missing delivery evidence", err)
	}
	p.Sender = sendFunc(func(context.Context, string, wns.Notification) (wns.Result, error) {
		r := wns.Result{HTTPStatus: 404, Outcome: wns.DeadChannel}
		return r, &wns.ResponseError{Result: r}
	})
	if _, err := p.Wake(ctx, "d"); err == nil {
		t.Fatal("404 accepted")
	}
	if _, err := p.Wake(ctx, "d"); !errors.Is(err, pushnotify.ErrUnavailable) {
		t.Fatal("dead channel not suppressed")
	}
	// A new enrollment gets its own channel state, even for the same device.
	if err := s.Create(ctx, &storage.Enrollment{Serial: "2", Thumbprint: "new", DeviceID: "d", EnrollmentType: enroll.EnrollmentTypeFull, EnrolledAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Wake(ctx, "d"); !errors.Is(err, pushnotify.ErrUnavailable) {
		t.Fatal("new enrollment reused old channel")
	}
	if _, err := (&pushnotify.Service{}).Wake(ctx, "d"); !errors.Is(err, pushnotify.ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestTrackingOnlyAcceptsCorrelatedSuccessfulGets(t *testing.T) {
	t.Parallel()
	s, _, now := setup(t)
	ctx := context.Background()
	q := &pushnotify.TrackingQueue{CommandQueue: s.Queue(), Store: s, ProviderID: "p"}
	uri := dmclient.DeviceProviderPushChannelURI("p")
	for _, tc := range []struct {
		requested, returned, value string
		code                       syncml.StatusCode
	}{
		{uri, uri, "https://evil.test/steal", syncml.StatusOK},
		{uri, "./DevInfo/Man", "https://cloud.notify.windows.com/forged", syncml.StatusOK},
		{uri, uri, "https://cloud.notify.windows.com/failed", syncml.StatusNotFound},
		{"./DevInfo/Man", uri, "https://cloud.notify.windows.com/unsolicited", syncml.StatusOK},
	} {
		cmd, _ := mdm.NewGet([]string{tc.requested})
		queued, err := q.Enqueue(ctx, "d", cmd, now)
		if err != nil {
			t.Fatal(err)
		}
		err = q.StoreResult(ctx, "d", queued.ID, mdm.Result{Status: tc.code, ReceivedAt: now, Items: []syncml.Item{{Source: tc.returned, Data: &syncml.Data{Value: tc.value}}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.PushChannel(ctx, "1"); !errors.Is(err, sqlstore.ErrNotFound) {
			t.Fatal("untrusted result stored")
		}
	}
	for _, node := range []string{uri, dmclient.DeviceProviderPushStatus("p")} {
		value := "https://cloud.notify.windows.com/valid"
		if node != uri {
			value = "0"
		}
		cmd, _ := mdm.NewGet([]string{node})
		queued, err := q.Enqueue(ctx, "d", cmd, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := q.StoreResult(ctx, "d", queued.ID, mdm.Result{Status: syncml.StatusOK, ReceivedAt: now, Items: []syncml.Item{{Source: node, Data: &syncml.Data{Value: value}}}}); err != nil {
			t.Fatal(err)
		}
	}
	c, err := s.PushChannel(ctx, "1")
	if err != nil || c.URI == "" || c.Status != "0" {
		t.Fatal("valid push observations not persisted", err)
	}
}

func TestMissingCheckInsUseEnrollmentThenAuthenticatedActivity(t *testing.T) {
	t.Parallel()
	s, p, now := setup(t)
	ctx := context.Background()
	p.Clock = clock.NewFake(now.Add(48 * time.Hour))
	if _, err := p.CheckIns(ctx, 0); err == nil {
		t.Fatal("nonpositive threshold")
	}
	alerts, err := p.CheckIns(ctx, 24*time.Hour)
	if err != nil || len(alerts) != 1 {
		t.Fatalf("alerts=%v %v", alerts, err)
	}
	if err := s.TouchLastSeen(ctx, "1", now.Add(47*time.Hour)); err != nil {
		t.Fatal(err)
	}
	alerts, err = p.CheckIns(ctx, 24*time.Hour)
	if err != nil || len(alerts) != 0 {
		t.Fatalf("fresh check-in alerted: %v %v", alerts, err)
	}
}

var errStore = errors.New("store unavailable")

type failingStore struct {
	*sqlstore.Store
	fail string
}

func (s failingStore) Get(ctx context.Context, id string) (*storage.Enrollment, error) {
	if s.fail == "inactive" {
		return &storage.Enrollment{Serial: "1", State: storage.StateUnenrolled}, nil
	}
	if s.fail == "get" {
		return nil, errStore
	}
	return s.Store.Get(ctx, id)
}
func (s failingStore) PushChannel(ctx context.Context, id string) (sqlstore.PushChannel, error) {
	if s.fail == "channel" {
		return sqlstore.PushChannel{}, errStore
	}
	return s.Store.PushChannel(ctx, id)
}
func (s failingStore) MarkPushDead(ctx context.Context, id, uri string) error {
	if s.fail == "dead" {
		return errStore
	}
	return s.Store.MarkPushDead(ctx, id, uri)
}
func (s failingStore) ObservePush(ctx context.Context, id string, uri, status *string, at time.Time) error {
	if s.fail == "observe" {
		return errStore
	}
	return s.Store.ObservePush(ctx, id, uri, status, at)
}
func (s failingStore) LogEvent(ctx context.Context, id, kind string, detail any) error {
	if s.fail == "log" {
		return errStore
	}
	return s.Store.LogEvent(ctx, id, kind, detail)
}
func (s failingStore) List(ctx context.Context, q storage.EnrollmentQuery, p paging.Page) (paging.Result[storage.Enrollment], error) {
	if s.fail == "list" {
		return paging.Result[storage.Enrollment]{}, errStore
	}
	return s.Store.List(ctx, q, p)
}

func TestStorageFailuresRemainVisible(t *testing.T) {
	t.Parallel()
	s, p, now := setup(t)
	ctx := context.Background()
	uri := "https://cloud.notify.windows.com/test"
	if err := s.ObservePush(ctx, "1", &uri, nil, now); err != nil {
		t.Fatal(err)
	}
	p.Clock = nil // default clock also records a delivery without hidden configuration.
	if _, err := p.Wake(ctx, "d"); err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"get", "channel", "log", "dead"} {
		p.Store = failingStore{Store: s, fail: failure}
		if failure == "dead" {
			p.Sender = sendFunc(func(context.Context, string, wns.Notification) (wns.Result, error) {
				r := wns.Result{Outcome: wns.DeadChannel, HTTPStatus: 410}
				return r, &wns.ResponseError{Result: r}
			})
		}
		if _, err := p.Wake(ctx, "d"); !errors.Is(err, errStore) {
			t.Fatalf("wake %s: %v", failure, err)
		}
	}
	for _, failure := range []string{"list", "log"} {
		p.Store = failingStore{Store: s, fail: failure}
		p.Clock = clock.NewFake(now.Add(48 * time.Hour))
		if _, err := p.CheckIns(ctx, time.Hour); !errors.Is(err, errStore) {
			t.Fatalf("checkins %s: %v", failure, err)
		}
	}
	for _, failure := range []string{"get", "observe"} {
		q := &pushnotify.TrackingQueue{CommandQueue: s.Queue(), Store: failingStore{Store: s, fail: failure}, ProviderID: "p"}
		node := dmclient.DeviceProviderPushChannelURI("p")
		cmd, _ := mdm.NewGet([]string{node})
		queued, err := q.Enqueue(ctx, "d", cmd, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := q.StoreResult(ctx, "d", queued.ID, mdm.Result{Status: syncml.StatusOK, ReceivedAt: now, Items: []syncml.Item{{Source: node, Data: &syncml.Data{Value: uri}}}}); !errors.Is(err, errStore) {
			t.Fatalf("tracking %s: %v", failure, err)
		}
		stored, _ := s.Queue().Get(ctx, "d", queued.ID)
		if stored.State.Terminal() {
			t.Fatal("failed observation incorrectly completed queue command")
		}
	}
	q := &pushnotify.TrackingQueue{CommandQueue: s.Queue(), Store: s, ProviderID: "p"}
	if err := q.StoreResult(ctx, "d", "missing", mdm.Result{Status: syncml.StatusOK, Items: []syncml.Item{{Source: uri}}}); !errors.Is(err, mdm.ErrNotFound) {
		t.Fatal(err)
	}
	p.Store = failingStore{Store: s, fail: "inactive"}
	if _, err := p.Wake(ctx, "d"); !errors.Is(err, pushnotify.ErrUnavailable) {
		t.Fatal("inactive enrollment pushed")
	}
	q.Store = p.Store
	node := dmclient.DeviceProviderPushChannelURI("p")
	cmd, _ := mdm.NewGet([]string{node})
	queued, err := q.Enqueue(ctx, "d", cmd, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.StoreResult(ctx, "d", queued.ID, mdm.Result{Status: syncml.StatusOK, ReceivedAt: now.Add(time.Hour), Items: []syncml.Item{{Source: node, Data: &syncml.Data{Value: "https://cloud.notify.windows.com/inactive"}}}}); err != nil {
		t.Fatal(err)
	}
	channel, _ := s.PushChannel(ctx, "1")
	if channel.URI != uri {
		t.Fatal("inactive enrollment changed channel")
	}
}
