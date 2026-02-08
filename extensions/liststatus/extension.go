package liststatus

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the LIST-STATUS IMAP extension (RFC 5819).
// LIST-STATUS allows the client to request STATUS data for each mailbox
// returned by a LIST command, reducing the number of round-trips needed
// to gather mailbox information. The LIST command already handles the
// STATUS return option; this extension only advertises the capability.
// It depends on the LIST-EXTENDED extension (RFC 5258).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new LIST-STATUS extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "LIST-STATUS",
			ExtCapabilities: []imap.Cap{imap.CapListStatus},
			ExtDependencies: []string{"LIST-EXTENDED"},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{}                            { return nil }
func (e *Extension) OnEnabled(connID string) error                            { return nil }
