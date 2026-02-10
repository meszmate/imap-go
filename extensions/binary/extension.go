// Package binary implements the BINARY extension (RFC 3516).
//
// BINARY extends IMAP with the ability to fetch and store message content
// in its original binary form, without requiring base64 or quoted-printable
// encoding. It adds the BINARY[] fetch item for retrieving decoded binary
// content and supports binary literal notation (~{n}) for uploads.
//
// Binary fetch items (BINARY[], BINARY.PEEK[], BINARY.SIZE[]) are parsed
// by the core FETCH command handler through FetchOptions.BinarySection and
// FetchOptions.BinarySizeSection fields. This extension wraps APPEND to
// detect ~{N} binary literals and route to SessionBinary.AppendBinary().
package binary

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionBinary is the session interface for BINARY support.
// Backends implement this to handle binary content append operations
// using the ~{n} binary literal notation.
type SessionBinary interface {
	// AppendBinary appends a message with binary content to a mailbox.
	// Called when the client uses ~{N} binary literal notation.
	AppendBinary(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error)
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

// WrapHandler wraps the APPEND command handler to detect binary literals
// and route to SessionBinary.AppendBinary() when the session supports it.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	h, ok := handler.(server.CommandHandlerFunc)
	if !ok {
		ch, ok2 := handler.(server.CommandHandler)
		if !ok2 {
			return nil
		}
		h = ch.Handle
	}

	switch name {
	case "APPEND":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleBinaryAppend(ctx, h)
		})
	}
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

// handleBinaryAppend wraps APPEND to detect ~{N} binary literals and route
// to SessionBinary.AppendBinary() when available.
func handleBinaryAppend(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return original(ctx)
	}

	dec := ctx.Decoder

	// Read mailbox name
	mailbox, err := dec.ReadAString()
	if err != nil {
		return imap.ErrBad("invalid mailbox name")
	}

	options := &imap.AppendOptions{}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing message data")
	}

	// Check for optional flags
	b, err := dec.PeekByte()
	if err != nil {
		return imap.ErrBad("unexpected end of command")
	}

	if b == '(' {
		flagStrs, err := dec.ReadFlags()
		if err != nil {
			return imap.ErrBad("invalid flags")
		}
		for _, f := range flagStrs {
			options.Flags = append(options.Flags, imap.Flag(f))
		}

		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing message data")
		}

		b, err = dec.PeekByte()
		if err != nil {
			return imap.ErrBad("unexpected end of command")
		}
	}

	// Check for optional date-time
	if b == '"' {
		dateStr, err := dec.ReadQuotedString()
		if err != nil {
			return imap.ErrBad("invalid date-time")
		}

		t, parseErr := parseDate(dateStr)
		if parseErr != nil {
			return imap.ErrBad("invalid date-time format")
		}
		options.InternalDate = t

		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing message data")
		}
	}

	// Read literal size — detect ~{N} binary literal
	litSize, isBinary, err := readLiteralSize(dec)
	if err != nil {
		return imap.ErrBad(fmt.Sprintf("invalid literal: %v", err))
	}

	if isBinary {
		options.Binary = true
	}

	// Read the literal body from the connection's main decoder
	connDec := ctx.Conn.Decoder()
	literalReader := imap.LiteralReader{
		Reader: connDec.ReadLiteral(litSize),
		Size:   litSize,
	}

	// If binary and session supports SessionBinary, use AppendBinary
	if isBinary {
		if sess, ok := ctx.Session.(SessionBinary); ok {
			data, err := sess.AppendBinary(mailbox, literalReader, options)
			if err != nil {
				_, _ = io.Copy(io.Discard, literalReader.Reader)
				return err
			}
			_, _ = io.Copy(io.Discard, literalReader.Reader)
			writeAppendOK(ctx, data)
			return nil
		}
	}

	// Fall back to standard Append
	data, err := ctx.Session.Append(mailbox, literalReader, options)
	if err != nil {
		_, _ = io.Copy(io.Discard, literalReader.Reader)
		return err
	}
	_, _ = io.Copy(io.Discard, literalReader.Reader)
	writeAppendOK(ctx, data)
	return nil
}

// writeAppendOK writes the tagged OK response, optionally with APPENDUID.
func writeAppendOK(ctx *server.CommandContext, data *imap.AppendData) {
	if data != nil && data.UIDValidity > 0 && data.UID > 0 {
		enc := ctx.Conn.Encoder()
		enc.Encode(func(e *wire.Encoder) {
			code := fmt.Sprintf("APPENDUID %d %d", data.UIDValidity, uint32(data.UID))
			e.StatusResponse(ctx.Tag, "OK", code, "APPEND completed")
		})
	} else {
		ctx.Conn.WriteOK(ctx.Tag, "APPEND completed")
	}
}

// readLiteralSize reads a literal size specification like {42}, {42+}, or ~{42}
// from the decoder. Returns the size, whether it's a binary literal, and any error.
func readLiteralSize(dec *wire.Decoder) (int64, bool, error) {
	var sb strings.Builder
	for {
		b, err := dec.PeekByte()
		if err != nil {
			break
		}
		if err := dec.ExpectByte(b); err != nil {
			break
		}
		sb.WriteByte(b)
	}

	s := strings.TrimSpace(sb.String())

	binary := false
	if strings.HasPrefix(s, "~") {
		binary = true
		s = s[1:]
	}

	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return 0, false, fmt.Errorf("expected literal, got %q", s)
	}

	inner := s[1 : len(s)-1]
	inner = strings.TrimSuffix(inner, "+")

	size, err := strconv.ParseInt(inner, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid literal size %q: %w", inner, err)
	}

	return size, binary, nil
}

// parseDate attempts to parse an IMAP internal date string.
func parseDate(s string) (time.Time, error) {
	layouts := []string{
		imap.InternalDateLayout,
		"2-Jan-2006 15:04:05 -0700",
		time.RFC822Z,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q", s)
}
