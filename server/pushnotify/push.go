package pushnotify

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/msplatformservices/wns"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/dmclient"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

// ErrUnavailable means polling must supply a usable channel before another push.
var ErrUnavailable = errors.New("pushnotify: no usable channel; wait for device check-in")

// Store holds enrollment-bound channels and server events.
type Store interface {
	storage.EnrollmentStore
	PushChannel(context.Context, string) (sqlstore.PushChannel, error)
	ObservePush(context.Context, string, *string, *string, time.Time) error
	MarkPushDead(context.Context, string, string) error
	LogEvent(context.Context, string, string, any) error
}

// Sender is the WNS delivery seam used by the real sender and simulation tests.
type Sender interface {
	Send(context.Context, string, wns.Notification) (wns.Result, error)
}

// Service wakes a device; it never treats WNS acceptance as device acknowledgement.
type Service struct {
	Store  Store
	Sender Sender
	Clock  clock.Clock
}

func (s *Service) now() time.Time {
	if s.Clock == nil {
		return time.Now()
	}
	return s.Clock.Now()
}

// Wake sends a non-empty raw body. Channel age is a conservative estimate from
// first observation, since DMClient does not report the actual creation time.
func (s *Service) Wake(ctx context.Context, deviceID string) (wns.Result, error) {
	if s.Store == nil || s.Sender == nil {
		return wns.Result{}, ErrUnavailable
	}
	e, err := s.Store.Get(ctx, deviceID)
	if err != nil {
		return wns.Result{}, err
	}
	if e.State != storage.StateActive {
		return wns.Result{}, ErrUnavailable
	}
	c, err := s.Store.PushChannel(ctx, e.Serial)
	if errors.Is(err, sqlstore.ErrNotFound) {
		return wns.Result{}, ErrUnavailable
	}
	if err != nil {
		return wns.Result{}, err
	}
	if c.URI == "" || c.Dead || c.FirstSeen.IsZero() || s.now().Sub(c.FirstSeen) >= 30*24*time.Hour {
		return wns.Result{}, ErrUnavailable
	}
	ttl := 300
	r, sendErr := s.Sender.Send(ctx, c.URI, wns.Notification{Body: []byte{0}, TTLSeconds: &ttl})
	if r.Outcome == wns.DeadChannel {
		if err := s.Store.MarkPushDead(ctx, e.Serial, c.URI); err != nil {
			return r, errors.Join(sendErr, err)
		}
	}
	// No channel URI, OAuth token or remote error body enters the event log.
	if err := s.Store.LogEvent(ctx, deviceID, "push", map[string]any{"result": r, "failed": sendErr != nil, "renewal_due": s.now().Sub(c.FirstSeen) >= 15*24*time.Hour}); err != nil {
		return r, errors.Join(sendErr, err)
	}
	return r, sendErr
}

// MissingCheckIn is a server-side alert, independent of the Windows event log.
type MissingCheckIn struct {
	DeviceID     string
	LastActivity time.Time
}

// CheckIns reports and logs active enrollments older than the caller's threshold.
// Callers schedule this operation; there is no hidden polling goroutine.
func (s *Service) CheckIns(ctx context.Context, maxAge time.Duration) ([]MissingCheckIn, error) {
	if maxAge <= 0 || s.Store == nil {
		return nil, fmt.Errorf("pushnotify: positive check-in threshold and store required")
	}
	var out []MissingCheckIn
	page := paging.Page{Limit: paging.MaxPageSize}
	for {
		res, err := s.Store.List(ctx, storage.EnrollmentQuery{}, page)
		if err != nil {
			return nil, err
		}
		for _, e := range res.Items {
			at := e.LastSeenAt
			if at.IsZero() {
				at = e.EnrolledAt
			}
			if e.State != storage.StateActive || s.now().Sub(at) <= maxAge {
				continue
			}
			alert := MissingCheckIn{DeviceID: e.DeviceID, LastActivity: at}
			if err := s.Store.LogEvent(ctx, e.DeviceID, "missing_checkin", alert); err != nil {
				return nil, err
			}
			out = append(out, alert)
		}
		if res.NextCursor == "" {
			return out, nil
		}
		page.Cursor = res.NextCursor
	}
}

// TrackingQueue observes only successful, correlated Get results from management.
// All other queue behavior belongs to the underlying command queue.
type TrackingQueue struct {
	mdm.CommandQueue
	Store      Store
	ProviderID string
}

// StoreResult records channel/status observations before marking a command complete,
// so a storage failure remains retryable. Repeated observations are idempotent.
func (q *TrackingQueue) StoreResult(ctx context.Context, deviceID, commandID string, r mdm.Result) error {
	if r.Status == syncml.StatusOK && len(r.Items) > 0 {
		cmd, err := q.CommandQueue.Get(ctx, deviceID, commandID)
		if err != nil {
			return err
		}
		get, ok := cmd.Body.(*syncml.Get)
		if ok {
			for _, requested := range get.Items {
				for _, item := range r.Items {
					if item.LocURI() != requested.Target || item.Data == nil {
						continue
					}
					var uri, status *string
					value := item.Data.Text()
					switch requested.Target {
					case dmclient.DeviceProviderPushChannelURI(q.ProviderID):
						if value != "" && wns.ValidateChannel(value) != nil {
							continue
						}
						uri = &value
					case dmclient.DeviceProviderPushStatus(q.ProviderID):
						status = &value
					default:
						continue
					}
					e, err := q.Store.Get(ctx, deviceID)
					if err != nil {
						return err
					}
					if e.State != storage.StateActive {
						continue
					}
					if err := q.Store.ObservePush(ctx, e.Serial, uri, status, r.ReceivedAt); err != nil {
						return err
					}
				}
			}
		}
	}
	return q.CommandQueue.StoreResult(ctx, deviceID, commandID, r)
}
