// Package compress implements the COMPRESS=DEFLATE extension (RFC 4978).
//
// COMPRESS allows the client and server to negotiate DEFLATE compression
// for the IMAP connection. After successful negotiation, all data sent
// in both directions is compressed using the DEFLATE algorithm, which
// can significantly reduce bandwidth usage.
package compress

import (
	"fmt"
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionCompress is an optional interface for sessions that support
// the COMPRESS command.
type SessionCompress interface {
	// Compress activates compression using the specified algorithm.
	// The session is responsible for wrapping the connection's reader/writer.
	Compress(algorithm string) error
}

// Extension implements the COMPRESS=DEFLATE extension (RFC 4978).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new COMPRESS=DEFLATE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "COMPRESS",
			ExtCapabilities: []imap.Cap{imap.CapCompressDeflate},
		},
	}
}

// CommandHandlers returns the COMPRESS command handler.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandCompress: server.CommandHandlerFunc(handleCompress),
	}
}

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns the SessionCompress interface that sessions
// must implement to support the COMPRESS command.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionCompress)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// handleCompress handles the COMPRESS command.
func handleCompress(ctx *server.CommandContext) error {
	state := ctx.Conn.State()
	if state != imap.ConnStateAuthenticated && state != imap.ConnStateSelected {
		ctx.Conn.WriteBAD(ctx.Tag, "COMPRESS not allowed in current state")
		return nil
	}

	sess, ok := ctx.Session.(SessionCompress)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "COMPRESS not supported")
		return nil
	}

	if ctx.Decoder == nil {
		ctx.Conn.WriteBAD(ctx.Tag, "missing algorithm argument")
		return nil
	}

	algorithm, err := ctx.Decoder.ReadAtom()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "invalid algorithm")
		return nil
	}

	if !strings.EqualFold(algorithm, "DEFLATE") {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("unsupported compression algorithm: %s", algorithm))
		return nil
	}

	if err := sess.Compress(algorithm); err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("COMPRESS failed: %v", err))
		return nil
	}

	ctx.Conn.WriteOK(ctx.Tag, "COMPRESS completed")
	return nil
}
