package sqlstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
)

func TestPushChannelLifecycle(t *testing.T) {
	t.Parallel()
	runPushChannelLifecycle(t, newSQLite(t))
}

func runPushChannelLifecycle(t *testing.T, s *sqlstore.Store) {
	t.Helper()
	ctx := context.Background()
	at := time.Now().UTC()
	a, b, status := "https://a.notify.windows.com/old", "https://b.notify.windows.com/new", "0"
	if err := s.ObservePush(ctx, "serial1", &a, &status, at); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkPushDead(ctx, "serial1", a); err != nil {
		t.Fatal(err)
	}
	if err := s.ObservePush(ctx, "serial1", &a, nil, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	c, err := s.PushChannel(ctx, "serial1")
	if err != nil || !c.Dead || !c.FirstSeen.Equal(at) || c.Status != "0" {
		t.Fatalf("same URI revived or age reset: %+v, %v", c, err)
	}
	if err := s.ObservePush(ctx, "serial1", &b, nil, at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkPushDead(ctx, "serial1", a); err != nil {
		t.Fatal(err)
	}
	c, err = s.PushChannel(ctx, "serial1")
	if err != nil || c.Dead || c.URI != b || !c.FirstSeen.Equal(at.Add(2*time.Hour)) {
		t.Fatalf("renewal lost: %+v, %v", c, err)
	}
	if err := s.ObservePush(ctx, "serial1", &a, nil, at); err != nil {
		t.Fatal(err)
	}
	c, _ = s.PushChannel(ctx, "serial1")
	if c.URI != b {
		t.Fatal("old observation replaced renewal")
	}
	if _, err := s.PushChannel(ctx, "serial2"); !errors.Is(err, sqlstore.ErrNotFound) {
		t.Fatal("reenrollment inherited channel")
	}
	empty := ""
	if err := s.ObservePush(ctx, "serial1", &empty, nil, at.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	c, _ = s.PushChannel(ctx, "serial1")
	if c.URI != "" {
		t.Fatal("empty channel not cleared")
	}
	lower, upper := "https://cloud.notify.windows.com/token", "https://cloud.notify.windows.com/TOKEN"
	if err := s.ObservePush(ctx, "case", &lower, nil, at); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkPushDead(ctx, "case", lower); err != nil {
		t.Fatal(err)
	}
	if err := s.ObservePush(ctx, "case", &upper, nil, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkPushDead(ctx, "case", lower); err != nil {
		t.Fatal(err)
	}
	c, err = s.PushChannel(ctx, "case")
	if err != nil || c.Dead || !c.FirstSeen.Equal(at.Add(time.Hour)) {
		t.Fatal("database collation conflated distinct channel tokens", err)
	}
}
