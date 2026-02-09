// Package listextended implements the LIST-EXTENDED IMAP extension (RFC 5258).
//
// LIST-EXTENDED adds selection and return options to the LIST command,
// enabling features like subscribed-only listing, children info, and
// special-use attribute filtering. The core LIST command handler already
// parses and passes ListOptions to Session.List(); this extension advertises
// the capability and exposes a session interface for backends that want to
// handle extended LIST specifically.
package listextended

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionListExtended is an optional interface for sessions that support
// the LIST-EXTENDED extension. Backends implementing this interface can
// handle selection options (SUBSCRIBED, REMOTE, RECURSIVEMATCH) and
// return options (SUBSCRIBED, CHILDREN) in the LIST command.
type SessionListExtended interface {
	ListExtended(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error
}

// Extension implements the LIST-EXTENDED IMAP extension (RFC 5258).
// LIST-EXTENDED adds selection and return options to the LIST command,
// enabling features like subscribed-only listing, children info, and
// special-use attribute filtering. The core LIST command already handles
// these options via ListOptions; this extension advertises the capability
// and exposes a session interface.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new LIST-EXTENDED extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "LIST-EXTENDED",
			ExtCapabilities: []imap.Cap{imap.CapListExtended},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionListExtended interface that sessions
// may implement to support extended LIST operations.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionListExtended)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }
