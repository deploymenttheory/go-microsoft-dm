package mdm

import (
	"context"
	"sync"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// Facts is what a device told the server about itself.
type Facts struct {
	// DevInfo holds the ./DevInfo/* leaves from package 1 (DevId, Man, Mod,
	// DmV, Lang).
	DevInfo map[string]string
	// LoginStatus is user, others or none (Alert 1224 LoginStatus).
	LoginStatus string
	// SyncType is user, device or mixed (Alert 1224 synctype, AVD); empty
	// when the client did not say, which means mixed.
	SyncType string
	// DevicePrepSync is the device preparation state (Alert 1224).
	DevicePrepSync string
	// UserAgentOrigin is the MS-MDM 3.1.7.2 header value when sent.
	UserAgentOrigin string
}

// Outgoing is a large object the server is sending across messages.
type Outgoing struct {
	CommandID string
	// Chunks are the items still to send, first next.
	Chunks []syncml.Item
	// Template is the command shape the chunks travel in.
	Name string
}

// Session is the state of one OMA DM session between two messages.
type Session struct {
	// Key is DeviceID + "/" + SessionID.
	Key       string
	DeviceID  string
	SessionID string
	// EnrollmentKey is the identity the Authenticator resolved (the
	// certificate serial).
	EnrollmentKey string
	// ClientMsgID is the last MsgID received; ServerMsgID the last sent.
	ClientMsgID int
	ServerMsgID int
	// Authenticated is set once the client's credentials were accepted (or
	// its certificate trusted).
	Authenticated bool
	// AuthType is the Cred type in use, kept for the whole session.
	AuthType string
	// Challenged counts challenges sent, to stop a loop.
	Challenged int
	// ServerChallenged is set when the client asked the server to
	// authenticate; the next message carries a Cred.
	ServerNonce []byte
	Facts       Facts
	// gotPackageOne is set once the package-1 facts have been recorded, so
	// the hook fires once per session.
	gotPackageOne bool //nolint:unused // read in service.go within the package
	// Sent maps "MsgID/CmdID" of every delivered command (and every chunk)
	// to the command ID, so Status and Results can be filed.
	Sent map[string]string
	// Children maps "MsgID/CmdID" of a group member to its parent command.
	Children map[string]string
	// Outgoing is the large object in flight, if any.
	Outgoing *Outgoing
	// Assembler reassembles a large object the client uploads.
	Assembler *syncml.Assembler
	// AssemblingCommand is the command whose Results are being reassembled.
	AssemblingCommand string
	// results accumulates per-command answers across a message.
	results map[string]*pendingResult //nolint:unused // used within the package
	// MaxMsgSize and MaxObjSize are the client's limits from SyncHdr Meta.
	MaxMsgSize int64
	MaxObjSize int64
	// Namespace is the SyncML namespace the client used.
	Namespace string
	StartedAt time.Time
	LastSeen  time.Time
	// Closed is set when the session is ending.
	Closed bool
	// unenroll is set when a user-initiated unenroll alert arrived, so the
	// engine unenrolls after the reply is built.
	unenroll bool //nolint:unused // read in service.go within the package
}

// SessionKey builds the key of a session.
func SessionKey(deviceID, sessionID string) string { return deviceID + "/" + sessionID }

// SessionStore keeps sessions between messages.
type SessionStore interface {
	// GetSession returns the session or ErrNotFound.
	GetSession(ctx context.Context, key string) (*Session, error)
	// PutSession stores or replaces the session.
	PutSession(ctx context.Context, s *Session) error
	// DeleteSession removes it; unknown keys are not an error.
	DeleteSession(ctx context.Context, key string) error
}

// MemorySessions is a SessionStore in memory with a time-to-live.
type MemorySessions struct {
	mu       sync.Mutex
	sessions map[string]*Session
	// TTL discards sessions not seen for this long; zero means one hour.
	TTL time.Duration
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// NewMemorySessions returns an empty store.
func NewMemorySessions() *MemorySessions {
	return &MemorySessions{sessions: map[string]*Session{}, TTL: time.Hour}
}

func (m *MemorySessions) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// GetSession implements SessionStore.
func (m *MemorySessions) GetSession(_ context.Context, key string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[key]
	if !ok {
		return nil, ErrNotFound
	}
	ttl := m.TTL
	if ttl == 0 {
		ttl = time.Hour
	}
	if m.now().Sub(s.LastSeen) > ttl {
		delete(m.sessions, key)
		return nil, ErrNotFound
	}
	c := *s
	return &c, nil
}

// PutSession implements SessionStore.
func (m *MemorySessions) PutSession(_ context.Context, s *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := *s
	m.sessions[s.Key] = &c
	return nil
}

// DeleteSession implements SessionStore.
func (m *MemorySessions) DeleteSession(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, key)
	return nil
}

// Len reports how many sessions are held.
func (m *MemorySessions) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}
