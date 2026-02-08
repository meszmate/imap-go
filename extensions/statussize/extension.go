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

func (e *Extension) CommandHandlers() map[string]interface{} { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{} { return nil }
func (e *Extension) OnEnabled(connID string) error { return nil }
