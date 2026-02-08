package binary

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the BINARY IMAP extension (RFC 3516).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new BINARY extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "BINARY",
			ExtCapabilities: []imap.Cap{imap.CapBinary},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{} { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{} { return nil }
func (e *Extension) OnEnabled(connID string) error { return nil }
