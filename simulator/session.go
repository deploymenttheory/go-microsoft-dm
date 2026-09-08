package simulator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
)

// Errors returned by the DM client.
var (
	// ErrSession reports a server reply the client cannot continue from.
	ErrSession = errors.New("simulator: session error")
	// ErrChallengeLoop reports the server challenging more than twice.
	ErrChallengeLoop = errors.New("simulator: authentication challenge loop")
)

// Device is a software OMA DM client for one enrolled device.
type Device struct {
	// HTTP performs the requests; default http.DefaultClient.
	HTTP *http.Client
	// ManagementURL is the OMA DM endpoint.
	ManagementURL string
	// DeviceID is the OMA DM source.
	DeviceID string
	// Username and Password are the APPSRV credentials for syncml:auth-md5;
	// empty relies on the TLS client certificate.
	Username string
	Password string
	// DevInfo is the ./DevInfo/* map sent in package 1; defaults describe a
	// Windows 11 device.
	DevInfo map[string]string
	// LoginStatus is the package-1 Alert 1224 value (user, others, none);
	// empty omits the alert.
	LoginStatus string
	// SyncType is the AVD synctype (user, device, mixed); empty omits it.
	SyncType string
	// Tree is the fake CSP tree the client answers Get from and Add/Replace
	// writes to.
	Tree map[string]string
	// Namespace is the SyncML namespace to send; empty means 1.2.
	Namespace string
	// MaxObjSize advertises the client's large-object limit in SyncHdr Meta;
	// zero omits it.
	MaxObjSize int64
	// Mode is the ?mode= query value; default Machine.
	Mode string
	// Generics are generic alerts (1226) the client sends in package 1, for
	// example a user-initiated unenroll request.
	Generics []GenericAlert
	// UploadChunkSize, when positive, splits a Get result larger than it
	// into chunks sent across messages with MoreData, exercising the
	// server's large-object reassembly.
	UploadChunkSize int
}

// GenericAlert is a client generic alert (Alert 1226).
type GenericAlert struct {
	Type   string
	Format string
	Mark   string
	Source string
	Data   string
}

// Transcript records what happened in a session, for tests.
type Transcript struct {
	// ServerCommands are the commands the server sent, across all messages,
	// in order.
	ServerCommands []syncml.Command
	// Statuses maps a server command's CmdID to the status the client
	// returned.
	Statuses map[string]syncml.StatusCode
	// Messages is how many request/response round trips occurred.
	Messages int
	// Challenged counts authentication challenges received.
	Challenged int
	// Results holds the LocURIs the client returned Results for.
	Results []string
	// Final reports whether the server's last non-empty message set Final.
	Final bool
	// Ended reports that the session terminated cleanly, either by a Final
	// message with no pending work or by the server's empty 200.
	Ended bool
}

func (c *Device) defaults() {
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}
	if c.DevInfo == nil {
		c.DevInfo = map[string]string{
			"DevId": c.DeviceID, "Man": "VMware, Inc.", "Mod": "VMware7,1", "DmV": "1.3", "Lang": "en-US",
		}
	}
	if c.Tree == nil {
		c.Tree = map[string]string{}
	}
	if c.Tree["./DevDetail/SwV"] == "" {
		c.Tree["./DevDetail/SwV"] = "10.0.26100.1"
	}
	if c.Tree["./DevDetail/LrgObj"] == "" {
		c.Tree["./DevDetail/LrgObj"] = "true"
	}
	if c.Mode == "" {
		c.Mode = "Machine"
	}
}

// sessionState is the mutable state across one session's messages.
type sessionState struct {
	sessionID  string
	clientMsg  int
	nonce      []byte
	authSent   bool
	assembler  syncml.Assembler
	transcript Transcript
	// upload holds the Get-result chunks still to send, first next; uploadGet
	// is the Get they answer and uploadRef its reference for the Results.
	upload    []syncml.Item
	uploadGet syncml.Command
	uploadRef struct{ msgRef, cmdRef string }
}

// RunSession opens a session (client-initiated), answers the server until it
// ends the session, and returns the transcript. sessionID is the SessionID
// to use.
func (c *Device) RunSession(ctx context.Context, sessionID string) (*Transcript, error) {
	c.defaults()
	st := &sessionState{sessionID: sessionID, transcript: Transcript{Statuses: map[string]syncml.StatusCode{}}}
	req := c.packageOne(st)
	const maxMessages = 64
	for st.transcript.Messages < maxMessages {
		resp, err := c.exchange(ctx, req, st)
		if err != nil {
			return &st.transcript, err
		}
		st.transcript.Messages++
		if resp == nil {
			st.transcript.Ended = true
			return &st.transcript, nil // empty 200: session over
		}
		next, done, err := c.answer(resp, st)
		if err != nil {
			return &st.transcript, err
		}
		if done {
			st.transcript.Final = resp.Body.Final
			st.transcript.Ended = true
			return &st.transcript, nil
		}
		req = next
	}
	return &st.transcript, fmt.Errorf("%w: session did not end after %d messages", ErrSession, st.transcript.Messages)
}

// exchange sends one message and returns the decoded reply, or nil for an
// empty 200.
func (c *Device) exchange(ctx context.Context, msg *syncml.Message, st *sessionState) (*syncml.Message, error) {
	body, err := syncml.Encode(msg, syncml.EncodeOptions{})
	if err != nil {
		return nil, err
	}
	url := c.ManagementURL + "?mode=" + c.Mode + "&Platform=WoA"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("simulator: %w", err)
	}
	httpReq.Header.Set("Content-Type", syncml.ContentTypeXML)
	httpReq.Header.Set("User-Agent", "MSFT OMA DM Client/1.2.0.1")
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("simulator: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, syncml.DefaultMaxSize))
	if err != nil {
		return nil, fmt.Errorf("simulator: read response: %w", err)
	}
	for _, te := range resp.TransferEncoding {
		if te == "chunked" {
			return nil, fmt.Errorf("%w: chunked response", ErrProtocol)
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d: %s", ErrSession, resp.StatusCode, bytes.TrimSpace(raw))
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil //nolint:nilnil // empty 200 is a valid end of session
	}
	return syncml.Decode(raw, syncml.DecodeOptions{})
}
