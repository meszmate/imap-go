package listextended

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the LIST-EXTENDED IMAP extension (RFC 5258).
// LIST-EXTENDED adds selection and return options to the LIST command,
// enabling features like subscribed-only listing, children info, and
// special-use attribute filtering. The LIST command already handles
// these options; this extension only advertises the capability.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new LIST-EXTENDED extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "LIST-EXTENDED",
			ExtCapabilities: []imap.Cap{imap.CapListExtended},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{}                            { return nil }
func (e *Extension) OnEnabled(connID string) error                            { return nil }
