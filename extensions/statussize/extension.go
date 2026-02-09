// Package statussize implements the STATUS=SIZE extension (RFC 8438).
//
// STATUS=SIZE adds the SIZE item to the STATUS command response, allowing
// clients to query the total size (in bytes) of all messages in a mailbox.
// The core STATUS command handler already supports the SIZE item when
// requested -- StatusOptions has a Size bool and StatusData has a Size
// *int64 field. This extension only advertises the capability so that
// clients know the server supports it.
package statussize

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the STATUS=SIZE IMAP extension (RFC 8438).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new STATUS=SIZE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "STATUS=SIZE",
			ExtCapabilities: []imap.Cap{imap.CapStatusSize},
		},
	}
}

// CommandHandlers returns nil because the core STATUS handler already
// supports the SIZE item.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler returns nil because no handler modification is needed.
// The core STATUS handler reads the SIZE item from StatusOptions and
// writes it in StatusData.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns nil because no additional session interface is
// needed. The core StatusOptions.Size and StatusData.Size fields provide
// full support.
func (e *Extension) SessionExtension() interface{} { return nil }

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
