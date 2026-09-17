package sqlstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/storagetest"
)

func TestAgentTokenFollowsActiveEnrollment(t *testing.T) {
	ctx := context.Background()
	s := newSQLite(t)
	first := storagetest.Enrollment(1, "agent-first", time.Now())
	if err := s.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	token, err := s.IssueAgentToken(ctx, first.DeviceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 43 {
		t.Fatalf("token length = %d", len(token))
	}
	if id, err := s.AgentWake(ctx, first.DeviceID, token); err != nil || id != "" {
		t.Fatalf("empty queue: %q, %v", id, err)
	}
	cmd, err := mdm.NewGet([]string{"./DevInfo/Man"})
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.Queue().Enqueue(ctx, first.DeviceID, cmd, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if id, err := s.AgentWake(ctx, first.DeviceID, token); err != nil || id != q.ID {
		t.Fatalf("pending wake: %q, %v", id, err)
	}
	if _, err := s.AgentWake(ctx, first.DeviceID, "wrong"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("wrong token: %v", err)
	}
	second := storagetest.Enrollment(1, "agent-second", time.Now().Add(time.Second))
	if err := s.Create(ctx, second); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AgentWake(ctx, first.DeviceID, token); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("old enrollment token: %v", err)
	}
	newToken, err := s.IssueAgentToken(ctx, first.DeviceID)
	if err != nil || newToken == token {
		t.Fatalf("new token: %v", err)
	}
	if _, err := s.AgentWake(ctx, first.DeviceID, token); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("rotated token: %v", err)
	}
}

func TestAgentTokenMissingDevice(t *testing.T) {
	s := newSQLite(t)
	if _, err := s.IssueAgentToken(context.Background(), "absent"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal(err)
	}
}
