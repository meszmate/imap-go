package objectid

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the OBJECTID IMAP extension (RFC 8474).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new OBJECTID extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "OBJECTID",
			ExtCapabilities: []imap.Cap{imap.CapObjectID},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{} { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{} { return nil }
func (e *Extension) OnEnabled(connID string) error { return nil }
