// Package preview implements the PREVIEW extension (RFC 8970).
//
// PREVIEW provides a short text preview of a message's content as a FETCH
// data item. The core FETCH handler and writers.go already handle parsing
// the PREVIEW item and writing it in responses. This extension advertises
// the capability and exposes the SessionPreview interface for backends that
// want to provide previews through a dedicated method.
package preview

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// SessionPreview is an optional interface for sessions that support
// fetching message previews (RFC 8970).
type SessionPreview interface {
	// FetchPreview returns a short text preview for the message with the given UID.
	FetchPreview(uid imap.UID) (string, error)
}

// Extension implements the PREVIEW IMAP extension (RFC 8970).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new PREVIEW extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "PREVIEW",
			ExtCapabilities: []imap.Cap{imap.CapPreview},
		},
	}
}

// CommandHandlers returns nil because the core FETCH handler already
// handles the PREVIEW data item.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler returns nil because the core FETCH handler already
// handles PREVIEW parsing and response writing.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionPreview interface that sessions may
// implement to provide message previews through a dedicated method.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionPreview)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }
