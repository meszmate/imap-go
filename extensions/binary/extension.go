// Package binary implements the BINARY extension (RFC 3516).
//
// BINARY extends IMAP with the ability to fetch and store message content
// in its original binary form, without requiring base64 or quoted-printable
// encoding. It adds the BINARY[] fetch item for retrieving decoded binary
// content and supports binary literal notation (~{n}) for uploads.
//
// This extension exposes the session interface for backends that want to
// provide binary content access and advertises the BINARY capability.
package binary

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// SessionBinary is the session interface for BINARY support.
// Backends implement this to provide binary content fetch and append
// operations without transfer encoding.
type SessionBinary interface {
	// FetchBinary retrieves the binary content of a message section.
	// The section parameter specifies which part of the message to fetch
	// (e.g., "1", "1.2", "TEXT"). The returned bytes are the raw decoded
	// content without any transfer encoding.
	FetchBinary(uid imap.UID, section string) ([]byte, error)

	// AppendBinary appends a message with binary content to a mailbox.
	// The data parameter contains the raw binary message content using
	// the ~{n} binary literal notation.
	AppendBinary(mailbox string, data []byte, options *imap.AppendOptions) (*imap.AppendData, error)
}

// Extension implements the BINARY extension (RFC 3516).
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

// CommandHandlers returns new command handlers to register.
// BINARY modifies existing FETCH and APPEND commands rather than adding
// new ones, so no new command handlers are needed.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps an existing command handler.
// Binary content handling is managed through the session interface,
// so no wrapping is needed.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns a typed nil pointer to SessionBinary, indicating
// that sessions should implement this interface for full BINARY support.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionBinary)(nil)
}

// OnEnabled is called when a client enables BINARY via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}
