package mdm

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/clock"
	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/schema/csp"
	"github.com/deploymenttheory/go-microsoft-dm/state"
)

// ErrConfig reports an unusable Service configuration.
var ErrConfig = errors.New("mdm: invalid configuration")

// Config configures a Service.
type Config struct {
	// ServerURL is the OMA DM endpoint, written as the Source of every
	// reply. Required.
	ServerURL string
	// Auth resolves devices and verifies credentials. Required.
	Auth Authenticator
	// Queue holds commands per device. Required.
	Queue CommandQueue
	// Sessions keeps session state between messages; default an in-memory
	// store.
	Sessions SessionStore
	// Nonce stores the per-device MD5 nonce with a short life; default an
	// in-memory state.Store.
	Nonce state.Store
	// Hooks receive session events; nil ignores them.
	Hooks Hooks
	// Registry validates commands before they are sent; nil skips schema
	// validation (the builders still apply the structural rules).
	Registry *csp.Registry
	// Clock stamps times; default real time.
	Clock clock.Clock
	// MaxRequestSize bounds a request body; default syncml.DefaultMaxSize.
	MaxRequestSize int
	// NonceTTL is how long an issued MD5 nonce is accepted; default 10
	// minutes.
	NonceTTL time.Duration
	// MaxMessageBytes bounds the reply the engine builds before chunking
	// the remaining commands to the next message; default 512 KiB, the
	// safe size the capture tools do not truncate.
	MaxMessageBytes int
	// AllowBasic permits syncml:auth-basic accounts; MD5 is always allowed.
	AllowBasic bool
	// ProviderID is the DMClient provider the auto-queued ChannelURI read
	// targets; empty skips that read.
	ProviderID string
}

// Service runs OMA DM management sessions.
type Service struct {
	cfg Config
}

// New validates the configuration.
func New(cfg Config) (*Service, error) {
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("%w: ServerURL is required", ErrConfig)
	}
	if cfg.Auth == nil || cfg.Queue == nil {
		return nil, fmt.Errorf("%w: Auth and Queue are required", ErrConfig)
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.Sessions == nil {
		ms := NewMemorySessions()
		ms.Now = cfg.Clock.Now
		cfg.Sessions = ms
	}
	if cfg.Nonce == nil {
		mem := state.NewMemory()
		mem.Now = cfg.Clock.Now
		cfg.Nonce = mem
	}
	if cfg.MaxRequestSize <= 0 {
		cfg.MaxRequestSize = syncml.DefaultMaxSize
	}
	if cfg.NonceTTL <= 0 {
		cfg.NonceTTL = 10 * time.Minute
	}
	if cfg.MaxMessageBytes <= 0 {
		cfg.MaxMessageBytes = 512 << 10
	}
	return &Service{cfg: cfg}, nil
}

// Config returns the effective configuration.
func (s *Service) Config() Config { return s.cfg }

// Handle processes one request and returns the reply body. An empty reply
// (nil, nil) means the session ended and the server sends an empty 200, as
// OMA DM Protocol 8 requires when the client's last package needs no answer.
func (s *Service) Handle(ctx context.Context, t *Transport, body []byte) ([]byte, error) {
	req, err := syncml.Decode(body, syncml.DecodeOptions{MaxSize: s.cfg.MaxRequestSize})
	if err != nil {
		return nil, fmt.Errorf("mdm: decode request: %w", err)
	}
	if err := syncml.Validate(req); err != nil {
		return nil, fmt.Errorf("mdm: invalid request: %w", err)
	}
	deviceID := req.Header.Source.LocURI
	id, err := s.cfg.Auth.Lookup(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("mdm: lookup %q: %w", deviceID, err)
	}
	sess, err := s.session(ctx, req, id, t)
	if err != nil {
		return nil, err
	}
	outcome := s.authenticate(ctx, req, id, sess, t)
	msgID := strconv.Itoa(sess.ServerMsgID + 1)
	resp := syncml.NewResponse(req, s.cfg.ServerURL, msgID)
	resp.Namespace = req.Namespace
	ids := &syncml.CmdIDs{}
	if outcome != authTrusted {
		return s.finish(ctx, sess, req, resp, ids, s.challenge(ctx, req, id, sess, resp, ids, outcome))
	}

	// SyncHdr status, then a status for every command the client sent.
	resp.Body.Commands = append(resp.Body.Commands, syncml.StatusForHeader(req, ids.Next(), syncml.StatusOK))
	if err := s.ackAndFile(ctx, req, sess, resp, ids); err != nil {
		return nil, err
	}

	// A closing session (a user-initiated unenroll) sends only the
	// acknowledgements and ends.
	if sess.Closed {
		resp.Body.Final = true
		return s.finish(ctx, sess, req, resp, ids, nil)
	}
	// A "Next Message" request gets the continuation of the object in flight
	// or the 1222 shape; no new commands.
	if syncml.IsNextMessageRequest(req) && len(req.Body.Statuses()) <= 1 {
		return s.finish(ctx, sess, req, resp, ids, s.continueOrNext(ctx, sess, req, resp, ids))
	}
	return s.finish(ctx, sess, req, resp, ids, s.deliver(ctx, sess, req, resp, ids))
}

// session finds or opens the session for the request, enforcing the MsgID
// sequence and that a new session begins with package 1.
func (s *Service) session(ctx context.Context, req *syncml.Message, id *Identity, t *Transport) (*Session, error) {
	key := SessionKey(req.Header.Source.LocURI, req.Header.SessionID)
	clientMsg, _ := strconv.Atoi(req.Header.MsgID)
	sess, err := s.cfg.Sessions.GetSession(ctx, key)
	// Windows restarts session numbering after reenrollment. A session from
	// the old certificate must not carry authentication or MsgID state forward.
	if err == nil && sess.EnrollmentKey != id.EnrollmentKey {
		err = ErrNotFound
	}
	if errors.Is(err, ErrNotFound) {
		if clientMsg != 1 {
			return nil, fmt.Errorf("%w: new session %q opened with MsgID %s, not 1", syncml.ErrInvalid, key, req.Header.MsgID)
		}
		if !syncml.IsPackageOne(req) {
			return nil, fmt.Errorf("%w: first message of session %q is not package 1", syncml.ErrInvalid, key)
		}
		now := s.cfg.Clock.Now()
		sess = &Session{
			Key: key, DeviceID: req.Header.Source.LocURI, SessionID: req.Header.SessionID,
			EnrollmentKey: id.EnrollmentKey, Namespace: req.Namespace,
			Sent: map[string]string{}, Children: map[string]string{}, StartedAt: now, LastSeen: now,
		}
	} else if err != nil {
		return nil, fmt.Errorf("mdm: session: %w", err)
	} else {
		if clientMsg != sess.ClientMsgID+1 {
			return nil, fmt.Errorf("%w: session %q expected MsgID %d, got %s", syncml.ErrInvalid, key, sess.ClientMsgID+1, req.Header.MsgID)
		}
	}
	sess.ClientMsgID = clientMsg
	sess.LastSeen = s.cfg.Clock.Now()
	if req.Header.Meta != nil {
		if req.Header.Meta.MaxMsgSize > 0 {
			sess.MaxMsgSize = req.Header.Meta.MaxMsgSize
		}
		if req.Header.Meta.MaxObjSize > 0 {
			sess.MaxObjSize = req.Header.Meta.MaxObjSize
		}
	}
	if t != nil {
		sess.Facts.UserAgentOrigin = firstNonEmpty(t.UserAgentOrigin, sess.Facts.UserAgentOrigin)
	}
	return sess, nil
}

// authenticate decides whether the message may be acted on: a trusted
// certificate authenticates the whole session; otherwise the SyncHdr Cred is
// verified against the issued nonce.
func (s *Service) authenticate(ctx context.Context, req *syncml.Message, id *Identity, sess *Session, t *Transport) authOutcome {
	if sess.Authenticated {
		return authTrusted
	}
	if t != nil && (id.AuthType == AuthCertificate || len(t.Certificates) > 0) {
		if s.cfg.Auth.TrustCertificate(ctx, id, t.Certificates) || certTrusts(id, t.Certificates) {
			sess.Authenticated, sess.AuthType = true, string(AuthCertificate)
			return authTrusted
		}
	}
	if id.AuthType == AuthBasic && !s.cfg.AllowBasic {
		return authChallenge
	}
	nonce := s.currentNonce(ctx, id.DeviceID)
	if req.Header.Cred == nil {
		return authChallenge
	}
	if verifyCredential(id, req.Header.Cred, nonce) {
		sess.Authenticated, sess.AuthType = true, string(id.AuthType)
		return authTrusted
	}
	return authRejected
}

// challenge builds the 407 or 401 response: a status on the SyncHdr carrying
// a Chal with a fresh nonce, and nothing else.
func (s *Service) challenge(ctx context.Context, req *syncml.Message, id *Identity, sess *Session, resp *syncml.Message, ids *syncml.CmdIDs, outcome authOutcome) error {
	code := syncml.StatusAuthenticationRequired
	if outcome == authRejected {
		code = syncml.StatusInvalidCredentials
	}
	st := syncml.StatusForHeader(req, ids.Next(), code)
	if id.AuthType == AuthBasic {
		st.Chal = syncml.NewBasicChal()
	} else {
		nonce, err := s.issueNonce(ctx, id.DeviceID)
		if err != nil {
			return err
		}
		st.Chal = syncml.NewMD5Chal(nonce)
	}
	resp.Body.Commands = append(resp.Body.Commands, st)
	sess.Challenged++
	return nil
}

// ackAndFile writes a Status for each command the client sent and files the
// client's Status and Results against the queued commands they answer.
func (s *Service) ackAndFile(ctx context.Context, req *syncml.Message, sess *Session, resp *syncml.Message, ids *syncml.CmdIDs) error {
	for _, c := range req.Body.Commands {
		switch cmd := c.(type) {
		case *syncml.Status:
			// The client's status on our SyncHdr and commands; not acked.
		case *syncml.Results:
			if err := s.fileResults(sess, cmd); err != nil {
				return err
			}
		case *syncml.Alert:
			if err := s.handleAlert(ctx, sess, cmd, resp, ids); err != nil {
				return err
			}
		default:
			// Replace (DevInfo in package 1), and anything else the client
			// sends, is acknowledged.
			resp.Body.Commands = append(resp.Body.Commands, syncml.StatusFor(req, c, ids.Next(), syncml.StatusOK))
		}
	}
	for _, st := range req.Body.Statuses() {
		s.fileStatus(sess, st)
	}
	if syncml.IsPackageOne(req) && !sess.gotPackageOne {
		if err := s.recordPackageOne(ctx, sess, req); err != nil {
			return err
		}
	}
	return nil
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
