package mdm

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/state"
)

func TestStatusFor(t *testing.T) {
	t.Parallel()
	cases := map[error]int{
		ErrUnenrolled:        http.StatusForbidden,
		syncml.ErrSyntax:     http.StatusBadRequest,
		syncml.ErrInvalid:    http.StatusBadRequest,
		syncml.ErrTooLarge:   http.StatusBadRequest,
		syncml.ErrNamespace:  http.StatusBadRequest,
		errors.New("other"):  http.StatusInternalServerError,
	}
	for err, want := range cases {
		if got := statusFor(err); got != want {
			t.Errorf("statusFor(%v) = %d, want %d", err, got, want)
		}
	}
}

func TestSessionNowDefault(t *testing.T) {
	t.Parallel()
	m := NewMemorySessions()
	m.TTL = 0 // exercises the default-hour branch
	s := &Session{Key: "k", LastSeen: time.Now()}
	if err := m.PutSession(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetSession(context.Background(), "k"); err != nil {
		t.Errorf("default now: %v", err)
	}
}

// failingNonce is a state.Store whose Update always fails.
type failingNonce struct{ state.Store }

func (failingNonce) Update(context.Context, []string, func(state.Tx) error) error {
	return errors.New("nonce store down")
}

func TestIssueNonceFailure(t *testing.T) {
	t.Parallel()
	svc, err := New(Config{
		ServerURL: "https://x/svc", Auth: nil, Queue: nil,
	})
	_ = err
	_ = svc
	// Build a service with a failing nonce store directly.
	s := &Service{cfg: Config{Clock: clock.NewFake(t0), NonceTTL: time.Minute, Nonce: failingNonce{Store: state.NewMemory()}}}
	if _, err := s.issueNonce(context.Background(), "DEVICE-01"); !errors.Is(err, ErrAuth) {
		t.Errorf("issueNonce error = %v", err)
	}
}

func TestCurrentNonceExpired(t *testing.T) {
	t.Parallel()
	mem := state.NewMemory()
	clk := clock.NewFake(t0)
	mem.Now = clk.Now
	s := &Service{cfg: Config{Clock: clk, Nonce: mem}}
	// No record.
	if s.currentNonce(context.Background(), "d") != nil {
		t.Error("nonce without a record")
	}
	// An expired record.
	key := nonceKey("d")
	_ = mem.Update(context.Background(), []string{key}, func(tx state.Tx) error {
		return tx.Put(context.Background(), state.Record{Key: key, Value: []byte{1}, ExpiresAt: t0.Add(-time.Minute)})
	})
	if s.currentNonce(context.Background(), "d") != nil {
		t.Error("expired nonce returned")
	}
	// A live record.
	_ = mem.Update(context.Background(), []string{key}, func(tx state.Tx) error {
		return tx.Put(context.Background(), state.Record{Key: key, Value: []byte{9}, ExpiresAt: t0.Add(time.Hour)})
	})
	if got := s.currentNonce(context.Background(), "d"); len(got) != 1 || got[0] != 9 {
		t.Errorf("live nonce = %v", got)
	}
}

func TestNonceKeySanitizes(t *testing.T) {
	t.Parallel()
	k := nonceKey("a b\t\x00")
	if k != "mdm/nonce/a_b__" {
		t.Errorf("nonceKey = %q", k)
	}
}

func TestFmtNonceErr(t *testing.T) {
	t.Parallel()
	if err := fmtNonceErr(errors.New("x")); !errors.Is(err, ErrAuth) {
		t.Errorf("fmtNonceErr = %v", err)
	}
}
