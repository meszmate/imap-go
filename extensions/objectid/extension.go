// Package objectid implements the OBJECTID extension (RFC 8474).
//
// OBJECTID provides unique identifiers for messages (EMAILID, THREADID) and
// mailboxes (MAILBOXID). The core FETCH handler handles parsing and writing
// EMAILID and THREADID data items. The core STATUS handler supports MAILBOXID
// as a STATUS item. The core SELECT/EXAMINE handler and extension SELECT
// handlers (CONDSTORE, QRESYNC) emit MAILBOXID as a response code when
// present in SelectData. This extension advertises the capability and exposes
// the SessionObjectID interface for backends.
package objectid

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// SessionObjectID is an optional interface for sessions that support
// the OBJECTID extension (RFC 8474).
type SessionObjectID interface {
	// ObjectIDs returns the EMAILID and THREADID for the message with the given UID.
	ObjectIDs(uid imap.UID) (emailID string, threadID string, err error)
}

// Extension implements the OBJECTID IMAP extension (RFC 8474).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new OBJECTID extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "OBJECTID",
			ExtCapabilities: []imap.Cap{imap.CapObjectID},
		},
	}
}

// CommandHandlers returns nil because the core FETCH and STATUS handlers
// already handle EMAILID, THREADID, and MAILBOXID data items.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler returns nil because the core and extension SELECT/EXAMINE
// handlers already emit MAILBOXID, and FETCH/STATUS handle EMAILID/THREADID/MAILBOXID.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionObjectID interface that sessions may
// implement to provide EMAILID and THREADID through a dedicated method.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionObjectID)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
