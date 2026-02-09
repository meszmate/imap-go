// Package savedate implements the SAVEDATE extension (RFC 8514).
//
// SAVEDATE provides the date and time a message was saved to a mailbox as a
// FETCH data item. The core FETCH handler already handles parsing and writing
// the SAVEDATE item. This extension advertises the capability and exposes the
// SessionSaveDate interface for backends that want to provide save dates
// through a dedicated method.
package savedate

import (
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// SessionSaveDate is an optional interface for sessions that support
// fetching message save dates (RFC 8514).
type SessionSaveDate interface {
	// FetchSaveDate returns the date and time the message with the given UID
	// was saved to the mailbox. Returns nil if the save date is not available.
	FetchSaveDate(uid imap.UID) (*time.Time, error)
}

// Extension implements the SAVEDATE IMAP extension (RFC 8514).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new SAVEDATE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "SAVEDATE",
			ExtCapabilities: []imap.Cap{imap.CapSaveDate},
		},
	}
}

// CommandHandlers returns nil because the core FETCH handler already
// handles the SAVEDATE data item.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler returns nil because the core FETCH handler already
// handles SAVEDATE parsing and response writing.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionSaveDate interface that sessions may
// implement to provide message save dates through a dedicated method.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionSaveDate)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
