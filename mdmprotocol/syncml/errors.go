package syncml

import (
	"errors"
	"fmt"
)

var (
	// ErrSyntax wraps every malformed-document error from Decode.
	ErrSyntax = errors.New("syncml: malformed message")
	// ErrNamespace reports a root namespace other than SYNCML:SYNCML1.2 or 1.1.
	ErrNamespace = errors.New("syncml: unsupported namespace")
	// ErrUnsupportedCommand reports an OMA command Windows does not implement
	// (Copy, Map, Move, Put, Search, Sync).
	ErrUnsupportedCommand = errors.New("syncml: command not supported by Windows")
	// ErrUnknownElement reports an element the DTD does not allow at that position.
	ErrUnknownElement = errors.New("syncml: unknown element")
	// ErrFinalNotLast reports a Final element followed by another element.
	ErrFinalNotLast = errors.New("syncml: Final must be the last element of SyncBody")
	// ErrTooLarge reports a document over the decoder's size limit.
	ErrTooLarge = errors.New("syncml: message exceeds size limit")
	// ErrInvalid wraps every Validate failure.
	ErrInvalid = errors.New("syncml: invalid message")
	// ErrChunk wraps large-object assembly failures.
	ErrChunk = errors.New("syncml: large object")
	// ErrDataConflict reports a Data with both Value and XML set.
	ErrDataConflict = errors.New("syncml: Data has both Value and XML")
)

// SyntaxError describes where a document broke a rule. It unwraps to ErrSyntax.
type SyntaxError struct {
	Element string
	Msg     string
	Err     error
}

func (e *SyntaxError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("syncml: %s: %s: %v", e.Element, e.Msg, e.Err)
	}
	return fmt.Sprintf("syncml: %s: %s", e.Element, e.Msg)
}

func (e *SyntaxError) Unwrap() []error {
	if e.Err != nil {
		return []error{ErrSyntax, e.Err}
	}
	return []error{ErrSyntax}
}

// ValidationError describes one rule a message breaks. It unwraps to
// ErrInvalid and to the rule's sentinel.
type ValidationError struct {
	// Rule is the sentinel for the broken rule (for example ErrCmdID).
	Rule error
	// Path locates the offending element: "SyncHdr/MsgID", "Atomic[10]/Get[3]".
	Path string
	Msg  string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("syncml: %s: %s", e.Path, e.Msg)
}

func (e *ValidationError) Unwrap() []error { return []error{ErrInvalid, e.Rule} }

// Validation rule sentinels.
var (
	ErrVersion              = errors.New("version")
	ErrSessionID            = errors.New("session id")
	ErrMsgID                = errors.New("message id")
	ErrRouting              = errors.New("routing")
	ErrEmptyBody            = errors.New("empty body")
	ErrCmdID                = errors.New("command id")
	ErrDuplicateCmdID       = errors.New("duplicate command id")
	ErrItems                = errors.New("items")
	ErrLocURI               = errors.New("locuri")
	ErrAlertCode            = errors.New("alert code")
	ErrStatusCode           = errors.New("status code")
	ErrStatusRef            = errors.New("status reference")
	ErrFormat               = errors.New("format")
	ErrAtomicNested         = errors.New("atomic: nested Atomic")
	ErrAtomicGet            = errors.New("atomic: Get inside Atomic")
	ErrAtomicAddThenReplace = errors.New("atomic: Add then Replace on one node")
	ErrAuth                 = errors.New("auth")
)
