package inprogress

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the INPROGRESS IMAP extension (RFC 9585).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new INPROGRESS extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "INPROGRESS",
			ExtCapabilities: []imap.Cap{imap.CapInProgress},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{} { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{} { return nil }
func (e *Extension) OnEnabled(connID string) error { return nil }
