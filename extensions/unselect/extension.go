package unselect

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the UNSELECT IMAP extension (RFC 3691).
// UNSELECT allows the client to close the current mailbox without
// expunging deleted messages. The command handling is built into the
// core server; this extension only advertises the capability.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new UNSELECT extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "UNSELECT",
			ExtCapabilities: []imap.Cap{imap.CapUnselect},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{}                            { return nil }
func (e *Extension) OnEnabled(connID string) error                            { return nil }
