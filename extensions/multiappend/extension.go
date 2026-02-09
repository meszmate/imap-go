// Package multiappend implements the MULTIAPPEND extension (RFC 3502).
//
// MULTIAPPEND extends the APPEND command to allow appending multiple messages
// to a mailbox in a single command. This is more efficient than issuing
// separate APPEND commands because the server can treat the entire operation
// atomically -- either all messages are appended or none are.
//
// The command format is:
//
//	tag APPEND mailbox [flags] [date-time] {literal} [flags] [date-time] {literal} ...
//
// This extension wraps the APPEND handler to detect additional messages after
// the first one and delegates to SessionMultiAppend for atomic multi-message
// appending.
package multiappend

import (
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// MultiAppendMessage represents a single message in a MULTIAPPEND operation.
type MultiAppendMessage struct {
	// Flags is the list of flags to set on the message.
	Flags []imap.Flag
	// InternalDate is the internal date to set on the message.
	InternalDate time.Time
	// Literal is the message content reader.
	Literal imap.LiteralReader
}

// SessionMultiAppend is an optional interface for sessions that support
// the MULTIAPPEND extension (RFC 3502).
type SessionMultiAppend interface {
	// AppendMulti atomically appends multiple messages to a mailbox.
	// Either all messages are appended or none are.
	AppendMulti(mailbox string, messages []MultiAppendMessage) ([]*imap.AppendData, error)
}

// Extension implements the MULTIAPPEND IMAP extension (RFC 3502).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new MULTIAPPEND extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "MULTIAPPEND",
			ExtCapabilities: []imap.Cap{imap.CapMultiAppend},
		},
	}
}

// CommandHandlers returns nil because MULTIAPPEND modifies the existing APPEND
// command rather than adding new commands.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler returns nil for now. The multi-message APPEND behavior is
// exposed through the SessionMultiAppend interface. A full wire-level
// implementation would wrap the APPEND handler to detect and collect
// additional messages after the first literal, then call SessionMultiAppend.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionMultiAppend interface that sessions may
// implement to support atomic multi-message APPEND operations.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionMultiAppend)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
