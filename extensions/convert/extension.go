// Package convert implements the CONVERT IMAP extension (RFC 5259).
//
// CONVERT allows clients to request on-the-fly conversion of message
// content from one media type to another. The server performs the
// conversion and returns the converted data.
package convert

import (
	"fmt"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionConvert is an optional interface for sessions that support
// the CONVERT command.
type SessionConvert interface {
	// Convert converts the content of the specified message section
	// to the given media type with optional parameters.
	Convert(seqNum uint32, section string, mediaType string, params map[string]string) ([]byte, error)
}

// Extension implements the CONVERT IMAP extension (RFC 5259).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new CONVERT extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "CONVERT",
			ExtCapabilities: []imap.Cap{imap.CapConvert},
		},
	}
}

// CommandHandlers returns the CONVERT command handler.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandConvert: server.CommandHandlerFunc(handleConvert),
	}
}

func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionConvert interface that sessions
// must implement to support the CONVERT command.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionConvert)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleConvert handles the CONVERT command.
//
// Command syntax: CONVERT seqnum section media-type [(param value) ...]
// Response:       * CONVERTED section literal-data
func handleConvert(ctx *server.CommandContext) error {
	state := ctx.Conn.State()
	if state != imap.ConnStateSelected {
		ctx.Conn.WriteBAD(ctx.Tag, "CONVERT requires selected state")
		return nil
	}

	sess, ok := ctx.Session.(SessionConvert)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "CONVERT not supported")
		return nil
	}

	if ctx.Decoder == nil {
		ctx.Conn.WriteBAD(ctx.Tag, "missing arguments")
		return nil
	}

	// Read sequence number
	seqNum, err := ctx.Decoder.ReadNumber()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "invalid sequence number")
		return nil
	}

	if err := ctx.Decoder.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "missing section")
		return nil
	}

	// Read section string
	section, err := ctx.Decoder.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "invalid section")
		return nil
	}

	if err := ctx.Decoder.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "missing media type")
		return nil
	}

	// Read media type
	mediaType, err := ctx.Decoder.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "invalid media type")
		return nil
	}

	// Read optional parameters: (param value) ...
	params := make(map[string]string)
	for {
		if err := ctx.Decoder.ReadSP(); err != nil {
			break
		}

		b, err := ctx.Decoder.PeekByte()
		if err != nil {
			break
		}

		if b != '(' {
			break
		}

		// Read parameter pair
		if err := ctx.Decoder.ExpectByte('('); err != nil {
			break
		}

		paramName, err := ctx.Decoder.ReadAtom()
		if err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "invalid parameter name")
			return nil
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "missing parameter value")
			return nil
		}

		paramValue, err := ctx.Decoder.ReadAString()
		if err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "invalid parameter value")
			return nil
		}

		if err := ctx.Decoder.ExpectByte(')'); err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "expected closing parenthesis")
			return nil
		}

		params[paramName] = paramValue
	}

	data, err := sess.Convert(seqNum, section, mediaType, params)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("CONVERT failed: %v", err))
		return nil
	}

	// Write CONVERTED response: * CONVERTED section literal-data
	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("CONVERTED").SP().AString(section).SP().Literal(data).CRLF()
	})

	ctx.Conn.WriteOK(ctx.Tag, "CONVERT completed")
	return nil
}
