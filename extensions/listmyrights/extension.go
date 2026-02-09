// Package listmyrights implements the LIST-MYRIGHTS IMAP extension (RFC 8440).
//
// LIST-MYRIGHTS adds the MYRIGHTS return option to the LIST command,
// allowing clients to retrieve their access rights for each listed mailbox
// in a single operation. The core LIST command already handles the MYRIGHTS
// return option via ListOptions.ReturnMyRights; this extension advertises
// the capability and exposes a session interface.
package listmyrights

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionListMyRights is an optional interface for sessions that support
// the LIST-MYRIGHTS extension. Backends implementing this interface can
// return MYRIGHTS data alongside LIST responses.
type SessionListMyRights interface {
	ListMyRights(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error
}

// Extension implements the LIST-MYRIGHTS IMAP extension (RFC 8440).
// LIST-MYRIGHTS adds the MYRIGHTS return option to the LIST command,
// allowing clients to retrieve their access rights for each listed
// mailbox. The core LIST command already handles this via ListOptions;
// this extension advertises the capability and exposes a session interface.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new LIST-MYRIGHTS extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "LIST-MYRIGHTS",
			ExtCapabilities: []imap.Cap{imap.CapListMyRights},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionListMyRights interface that sessions
// may implement to support MYRIGHTS return options in LIST.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionListMyRights)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }
