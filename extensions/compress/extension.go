// Package compress implements the COMPRESS=DEFLATE extension (RFC 4978).
//
// COMPRESS allows the client and server to negotiate DEFLATE compression
// for the IMAP connection. After successful negotiation, all data sent
// in both directions is compressed using the DEFLATE algorithm, which
// can significantly reduce bandwidth usage.
//
// This is a capability-only registration; the full COMPRESS command
// implementation (which requires wrapping the connection's reader/writer
// with compress/flate) will be added in a future update.
package compress

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
)

// Extension implements the COMPRESS=DEFLATE extension (RFC 4978).
type Extension struct {
	extension.BaseExtension
}

// New creates a new COMPRESS=DEFLATE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "COMPRESS",
			ExtCapabilities: []imap.Cap{imap.CapCompressDeflate},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// A full implementation would add the COMPRESS command handler here that
// negotiates DEFLATE compression on the connection.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns the required session extension interface, or nil.
func (e *Extension) SessionExtension() interface{} {
	return nil
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}
