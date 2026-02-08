package children

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the CHILDREN IMAP extension (RFC 3348).
// CHILDREN allows the server to indicate whether a mailbox has child
// mailboxes via \HasChildren and \HasNoChildren attributes in LIST
// responses. The LIST command already handles these attributes; this
// extension only advertises the capability.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new CHILDREN extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "CHILDREN",
			ExtCapabilities: []imap.Cap{imap.CapChildren},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{}                            { return nil }
func (e *Extension) OnEnabled(connID string) error                            { return nil }
