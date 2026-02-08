package specialuse

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the SPECIAL-USE IMAP extension (RFC 6154).
// SPECIAL-USE allows the server to advertise special-use attributes
// (such as \Drafts, \Sent, \Trash, etc.) on mailboxes via LIST responses.
// The LIST command already handles returning these attributes; this extension
// only advertises the SPECIAL-USE and CREATE-SPECIAL-USE capabilities.
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
func (e *Extension) SessionExtension() interface{}                            { return nil }
func (e *Extension) OnEnabled(connID string) error                            { return nil }
