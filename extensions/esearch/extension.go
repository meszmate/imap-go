package esearch

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the ESEARCH IMAP extension (RFC 4731).
// ESEARCH extends the SEARCH command with additional return options
// (MIN, MAX, ALL, COUNT) and a new ESEARCH response format. The SEARCH
// command already handles ESEARCH options; this extension only advertises
// the capability.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new ESEARCH extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "ESEARCH",
			ExtCapabilities: []imap.Cap{imap.CapESearch},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{}                            { return nil }
func (e *Extension) OnEnabled(connID string) error                            { return nil }
