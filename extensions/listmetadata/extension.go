// Package listmetadata implements the LIST-METADATA IMAP extension (RFC 9590).
//
// LIST-METADATA adds the METADATA return option to the LIST command,
// allowing clients to retrieve mailbox metadata annotations for each listed
// mailbox in a single operation. The core LIST command already handles the
// METADATA return option via ListOptions.ReturnMetadata; this extension
// advertises the capability and exposes a session interface.
package listmetadata

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionListMetadata is an optional interface for sessions that support
// the LIST-METADATA extension. Backends implementing this interface can
// return METADATA annotations alongside LIST responses.
type SessionListMetadata interface {
	ListMetadata(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error
}

// Extension implements the LIST-METADATA IMAP extension (RFC 9590).
// LIST-METADATA adds the METADATA return option to the LIST command,
// allowing clients to retrieve mailbox metadata annotations for each
// listed mailbox. The core LIST command already handles this via
// ListOptions; this extension advertises the capability and exposes a
// session interface.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new LIST-METADATA extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "LIST-METADATA",
			ExtCapabilities: []imap.Cap{imap.CapListMetadata},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionListMetadata interface that sessions
// may implement to support METADATA return options in LIST.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionListMetadata)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }
