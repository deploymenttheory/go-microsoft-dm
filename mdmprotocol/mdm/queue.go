package mdm

import (
	"context"
	"errors"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/syncml"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
)

// Errors shared by queue implementations.
var (
	// ErrNotFound reports an unknown command, session or device.
	ErrNotFound = errors.New("mdm: not found")
	// ErrConflict reports a duplicate command ID.
	ErrConflict = errors.New("mdm: conflict")
)

// State of a queued command.
type State string

// Command states.
const (
	// StatePending has never been delivered.
	StatePending State = "pending"
	// StateSent was delivered and awaits the client's Status.
	StateSent State = "sent"
	// StateAcknowledged received a 2xx Status; terminal.
	StateAcknowledged State = "acknowledged"
	// StateFailed received a non-2xx Status; terminal.
	StateFailed State = "failed"
	// StateCancelled was withdrawn before completion; terminal.
	StateCancelled State = "cancelled"
)

// Terminal reports whether the state is final.
func (s State) Terminal() bool {
	return s == StateAcknowledged || s == StateFailed || s == StateCancelled
}

// Result is what the client answered for one command: its Status, and for a
// successful Get the Results items. Chunked Results are stored reassembled.
type Result struct {
	Status syncml.StatusCode
	// OriginalError is the msft:originalerror HRESULT on a failing Status.
	OriginalError string
	// Items are the Results items, Source and Data as sent.
	Items []syncml.Item
	// Children hold the per-member statuses of an Atomic or Sequence, in
	// wire order.
	Children []ChildResult
	// ReceivedAt is when the final Status arrived.
	ReceivedAt time.Time
}

// ChildResult is the Status of one member of a group command.
type ChildResult struct {
	Name          string
	Target        string
	Status        syncml.StatusCode
	OriginalError string
	Items         []syncml.Item
}

// QueuedCommand is a command with its delivery state.
type QueuedCommand struct {
	Command
	// Seq is the monotonic per-device sequence the queue assigns; delivery
	// order is Seq order, never wall-clock order (research pitfall
	// "Same-second commands").
	Seq        int64
	State      State
	EnqueuedAt time.Time
	// LastSentAt, SentMsgID and Attempts record the latest delivery.
	LastSentAt time.Time
	SentMsgID  string
	Attempts   int
	// Result is set once the command reached a terminal state through the
	// client.
	Result      *Result
	CompletedAt time.Time
}

// Query filters List. Zero values mean "any".
type Query struct {
	States   []State
	Internal *bool
	Scope    Scope
}

// Delivery records one delivery of a command in a message.
type Delivery struct {
	CommandID string
	MsgID     string
	At        time.Time
}

// CommandQueue persists commands per device. Implementations live in the
// storage tier; storage/storagetest holds the contract suite.
type CommandQueue interface {
	// Enqueue stores a command in StatePending with the next Seq. A Command
	// with an ID that exists for the device is ErrConflict; an empty ID gets
	// one assigned. The returned command carries Seq and ID.
	Enqueue(ctx context.Context, deviceID string, cmd *Command, at time.Time) (*QueuedCommand, error)
	// Deliverable returns the commands to send, in Seq order: pending ones
	// and sent ones whose Status never arrived, up to limit.
	Deliverable(ctx context.Context, deviceID string, limit int) ([]QueuedCommand, error)
	// MarkSent records that the commands went out in a message.
	MarkSent(ctx context.Context, deviceID string, deliveries []Delivery) error
	// StoreResult records the client's answer and moves the command to
	// StateAcknowledged or StateFailed by the status class.
	StoreResult(ctx context.Context, deviceID, commandID string, r Result) error
	// Get returns one command or ErrNotFound.
	Get(ctx context.Context, deviceID, commandID string) (*QueuedCommand, error)
	// List pages through a device's commands in Seq order.
	List(ctx context.Context, deviceID string, q Query, p paging.Page) (paging.Result[QueuedCommand], error)
	// Cancel moves non-terminal commands to StateCancelled and returns how
	// many changed.
	Cancel(ctx context.Context, deviceID string, ids []string, at time.Time) (int, error)
	// Prune deletes terminal commands completed before the time and returns
	// how many.
	Prune(ctx context.Context, before time.Time, limit int) (int, error)
}
