package saslir

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the SASL-IR IMAP extension (RFC 4959).
// SASL-IR (SASL Initial Response) allows the client to include an initial
// authentication response in the AUTHENTICATE command, eliminating one
// round-trip during SASL authentication. The handling is performed in the
// AUTHENTICATE command; this extension only advertises the capability.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new SASL-IR extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "SASL-IR",
			ExtCapabilities: []imap.Cap{imap.CapSASLIR},
		},
	}
}

func (e *Extension) CommandHandlers() map[string]interface{}                  { return nil }
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }
func (e *Extension) SessionExtension() interface{}                            { return nil }
func (e *Extension) OnEnabled(connID string) error                            { return nil }
