// Package specialuse implements the SPECIAL-USE IMAP extension (RFC 6154).
//
// SPECIAL-USE allows the server to advertise special-use attributes
// (such as \Drafts, \Sent, \Trash, \Junk, \All, \Archive, \Flagged)
// on mailboxes via LIST responses, and allows clients to create mailboxes
// with specific special-use attributes. The core LIST command already
// handles selection and return of special-use attributes via ListOptions,
// and the core CREATE command already passes CreateOptions with SpecialUse;
// this extension advertises the SPECIAL-USE and CREATE-SPECIAL-USE
// capabilities and exposes a session interface.
package specialuse

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionSpecialUse is an optional interface for sessions that support
// the SPECIAL-USE extension. Backends implementing this interface can
// handle special-use attribute filtering in LIST and creation of
// mailboxes with special-use attributes.
type SessionSpecialUse interface {
	// ListSpecialUse lists mailboxes with special-use attribute handling.
	ListSpecialUse(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error

	// CreateSpecialUse creates a mailbox with a special-use attribute.
	CreateSpecialUse(mailbox string, options *imap.CreateOptions) error
}

// Extension implements the SPECIAL-USE IMAP extension (RFC 6154).
// SPECIAL-USE allows the server to advertise special-use attributes
// on mailboxes via LIST responses and supports creating mailboxes with
// special-use attributes. The core commands already handle these via
// ListOptions and CreateOptions; this extension advertises the capabilities
// and exposes a session interface.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new SPECIAL-USE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "SPECIAL-USE",
			ExtCapabilities: []imap.Cap{imap.CapSpecialUse, imap.CapCreateSpecialUse},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionSpecialUse interface that sessions
// may implement to support special-use mailbox attributes.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionSpecialUse)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }
