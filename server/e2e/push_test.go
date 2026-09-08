//go:build e2e

package e2e_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/msplatformservices/wns"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp/dmclient"
	"github.com/deploymenttheory/go-microsoft-dm/server/internal/app"
	"github.com/deploymenttheory/go-microsoft-dm/server/pushnotify"
)

type pushToken struct{}

func (pushToken) Token(context.Context, bool) (string, error) { return "e2e-token", nil }

type pushTransport func(*http.Request) (*http.Response, error)

func (f pushTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPushWakesSessionAndTracksRenewal(t *testing.T) {
	h := newHarness(t, func(c *app.Config) { c.Push.PFN = "Example.Conformance_12345" })
	ctx := context.Background()
	enrollment, dev := enrollDevice(t, h)
	if enrollment.DMClient.PushPFN != "Example.Conformance_12345" {
		t.Fatal("simulation enrollment lost Push/PFN")
	}
	channelNode := dmclient.DeviceProviderPushChannelURI("e2e")
	statusNode := dmclient.DeviceProviderPushStatus("e2e")
	oldURI, newURI := "https://cloud.notify.windows.com/old", "https://cloud.notify.windows.com/new"
	dev.Tree = map[string]string{channelNode: oldURI, statusNode: "0", "./DevInfo/Man": "Push simulator"}
	if _, err := dev.RunSession(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	serial := enrollment.Certificate.SerialNumber.String()
	first, err := h.app.Store.PushChannel(ctx, serial)
	if err != nil || first.URI != oldURI || first.Status != "0" {
		t.Fatalf("initial channel=%+v err=%v", first, err)
	}
	queued, _ := mdm.NewGet([]string{"./DevInfo/Man"})
	command, err := h.app.Store.Queue().Enqueue(ctx, deviceID, queued, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	dev.Tree[channelNode] = newURI
	var sessionErr error
	calls := 0
	dead := false
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer e2e-token" {
			t.Error("missing auth")
		}
		if dead {
			w.WriteHeader(410)
			return
		}
		// Simulate reception by the device, not mere WNS acceptance.
		_, sessionErr = dev.RunSession(r.Context(), "2")
		w.Header().Set("X-WNS-Status", "received")
		w.WriteHeader(200)
	}))
	defer cloud.Close()
	u, _ := url.Parse(cloud.URL)
	client, err := wns.New(wns.Config{Tokens: pushToken{}, HTTP: &http.Client{Transport: pushTransport(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = u.Scheme, u.Host
		return http.DefaultTransport.RoundTrip(r)
	})}})
	if err != nil {
		t.Fatal(err)
	}
	h.app.Push.Sender = client
	if _, err := h.app.Push.Wake(ctx, deviceID); err != nil || sessionErr != nil {
		t.Fatalf("push=%v session=%v", err, sessionErr)
	}
	result, err := h.app.Store.Queue().Get(ctx, deviceID, command.ID)
	if err != nil || result.State != mdm.StateAcknowledged || result.Result == nil || len(result.Result.Items) != 1 {
		t.Fatalf("push did not complete queued Get: %+v %v", result, err)
	}
	renewed, err := h.app.Store.PushChannel(ctx, serial)
	if err != nil || renewed.URI != newURI || !renewed.FirstSeen.After(first.FirstSeen) {
		t.Fatalf("per-session renewal missed: %+v %v", renewed, err)
	}
	e, err := h.app.Store.Get(ctx, deviceID)
	if err != nil || e.LastSeenAt.IsZero() {
		t.Fatal("authenticated check-in not recorded")
	}
	dead = true
	if _, err := h.app.Push.Wake(ctx, deviceID); err == nil {
		t.Fatal("dead channel accepted")
	}
	if _, err := h.app.Push.Wake(ctx, deviceID); !errors.Is(err, pushnotify.ErrUnavailable) || calls != 2 {
		t.Fatalf("dead channel retried: %v calls=%d", err, calls)
	}
	if _, err := dev.RunSession(ctx, "3"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.app.Push.Wake(ctx, deviceID); !errors.Is(err, pushnotify.ErrUnavailable) {
		t.Fatal("same URI revived after poll")
	}
	dev.Tree[channelNode] = "https://cloud.notify.windows.com/third"
	if _, err := dev.RunSession(ctx, "4"); err != nil {
		t.Fatal(err)
	}
	renewed, _ = h.app.Store.PushChannel(ctx, serial)
	if renewed.Dead {
		t.Fatal("renewed channel remains dead")
	}
	// Missed-check-in alerts use server activity, independent of Windows logs.
	h.app.Push.Clock = clock.NewFake(e.LastSeenAt.Add(48 * time.Hour))
	alerts, err := h.app.Push.CheckIns(ctx, 24*time.Hour)
	if err != nil || len(alerts) != 1 {
		t.Fatalf("missing check-in alerts=%v %v", alerts, err)
	}
	h.app.Push.Clock = clock.NewFake(renewed.FirstSeen.Add(31 * 24 * time.Hour))
	if _, err := h.app.Push.Wake(ctx, deviceID); !errors.Is(err, pushnotify.ErrUnavailable) {
		t.Fatal("aged channel used")
	}
}
