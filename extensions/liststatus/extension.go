// Package liststatus implements the LIST-STATUS IMAP extension (RFC 5819).
//
// LIST-STATUS allows the client to request STATUS data for each mailbox
// returned by a LIST command, reducing the number of round-trips needed
// to gather mailbox information. The core LIST command already handles the
// STATUS return option via ListOptions.ReturnStatus; this extension
// advertises the capability and exposes a session interface.
// It depends on the LIST-EXTENDED extension (RFC 5258).
package liststatus

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionListStatus is an optional interface for sessions that support
// the LIST-STATUS extension. Backends implementing this interface can
// return STATUS data alongside LIST responses.
type SessionListStatus interface {
	ListStatus(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error
}

// Extension implements the LIST-STATUS IMAP extension (RFC 5819).
// LIST-STATUS allows the client to request STATUS data for each mailbox
// returned by a LIST command, reducing the number of round-trips needed
// to gather mailbox information. The core LIST command already handles the
// STATUS return option; this extension advertises the capability and
// exposes a session interface. It depends on the LIST-EXTENDED extension.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new LIST-STATUS extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "LIST-STATUS",
			ExtCapabilities: []imap.Cap{imap.CapListStatus},
			ExtDependencies: []string{"LIST-EXTENDED"},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionListStatus interface that sessions
// may implement to support STATUS return options in LIST.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionListStatus)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }
