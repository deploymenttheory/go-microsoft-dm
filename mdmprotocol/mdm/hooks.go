package mdm

import "context"

// Event is something a device reported that the engine routes to a Hook
// rather than acting on itself.
type Event struct {
	// DeviceID and SessionID identify the session.
	DeviceID  string
	SessionID string
	// Alert is the client event (1224) or generic alert (1226) code.
	Alert int
	// Type is the Meta/Type of the alert item.
	Type string
	// Data is the alert item's data.
	Data string
	// Mark is the Meta/Mark importance for a generic alert, or "".
	Mark string
	// Source is the Item Source LocURI for a generic alert, or "".
	Source string
}

// Hooks receive out-of-band session events. Every method is optional; a nil
// Hooks ignores all of them. A hook's error fails the message, so a hook
// that only observes should return nil.
type Hooks interface {
	// PackageOne is called once per session when the device sends package 1,
	// with the facts it reported.
	PackageOne(ctx context.Context, deviceID, sessionID string, facts Facts) error
	// GenericAlert is called for each 1226 generic alert. The
	// unenrollment-request alert ends the session (the engine unenrolls
	// after the hook returns).
	GenericAlert(ctx context.Context, e Event) error
	// ClientEvent is called for each 1224 client event that the engine does
	// not consume itself (LoginStatus, SyncType and DevicePrepSync become
	// Facts and are not delivered here).
	ClientEvent(ctx context.Context, e Event) error
	// Unenrolled is called when a session ends the enrollment (a
	// user-initiated unenroll alert).
	Unenrolled(ctx context.Context, deviceID string) error
}

// NopHooks implements Hooks with no behaviour, for embedding.
type NopHooks struct{}

// PackageOne implements Hooks.
func (NopHooks) PackageOne(context.Context, string, string, Facts) error { return nil }

// GenericAlert implements Hooks.
func (NopHooks) GenericAlert(context.Context, Event) error { return nil }

// ClientEvent implements Hooks.
func (NopHooks) ClientEvent(context.Context, Event) error { return nil }

// Unenrolled implements Hooks.
func (NopHooks) Unenrolled(context.Context, string) error { return nil }
