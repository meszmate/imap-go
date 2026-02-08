package literalplus

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the LITERAL+ IMAP extension (RFC 7888).
// LITERAL+ allows the client to send literal data without waiting for
// a continuation request from the server. The handling of non-synchronizing
// literals is performed at the wire protocol layer; this extension only
// advertises the capability.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new LITERAL+ extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "LITERAL+",
			ExtCapabilities: []imap.Cap{imap.CapLiteralPlus},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{}                            { return nil }
func (e *Extension) OnEnabled(connID string) error                            { return nil }
