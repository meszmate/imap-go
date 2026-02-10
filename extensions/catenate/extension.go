// Package catenate implements the CATENATE extension (RFC 4469).
//
// CATENATE modifies the APPEND command to accept a CATENATE(...) construct
// instead of a single literal. This allows clients to compose a message
// from multiple parts (URLs referencing existing messages and text literals)
// without downloading and re-uploading entire messages.
//
// The command format when using CATENATE is:
//
//	tag APPEND mailbox [flags] [date-time] CATENATE (part1 part2 ...)
//
// where each part is either:
//
//	URL "url-string" - a reference to an existing message section
//	TEXT {literal-size} <literal-data> - inline text content
package catenate

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

// CatenatePart represents a single part in a CATENATE APPEND command.
type CatenatePart struct {
	// Type is "URL" or "TEXT".
	Type string
	// URL is the IMAP URL for URL parts.
	URL string
	// Text is the literal content for TEXT parts.
	Text []byte
}

// SessionCatenate is an optional interface for sessions that support
// the CATENATE extension (RFC 4469).
type SessionCatenate interface {
	// AppendCatenate appends a message composed from multiple parts to a mailbox.
	AppendCatenate(mailbox string, parts []CatenatePart, options *imap.AppendOptions) (*imap.AppendData, error)
}

// Extension implements the CATENATE IMAP extension (RFC 4469).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new CATENATE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "CATENATE",
			ExtCapabilities: []imap.Cap{imap.CapCatenate},
		},
	}
}

// CommandHandlers returns nil because CATENATE modifies the existing APPEND
// command rather than adding new commands.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps the APPEND command handler to support CATENATE syntax.
// When the APPEND arguments contain a CATENATE keyword instead of a literal,
// the wrapper parses the catenate parts and delegates to SessionCatenate.
// Otherwise, it falls through to the original APPEND handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	switch name {
	case "APPEND":
		h, ok := handler.(server.CommandHandlerFunc)
		if !ok {
			if ch, ok2 := handler.(server.CommandHandler); ok2 {
				h = ch.Handle
			} else {
				return nil
			}
		}
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleCatenateAppend(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the SessionCatenate interface that sessions must
// implement to support the CATENATE command.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionCatenate)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }

// handleCatenateAppend intercepts the APPEND command to check for CATENATE
// syntax. If the keyword after mailbox/flags/date is CATENATE, it parses
// the parts and calls SessionCatenate. Otherwise it handles the standard
// APPEND since we already consumed arguments from the decoder.
func handleCatenateAppend(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return original(ctx)
	}

	dec := ctx.Decoder

	// Read the mailbox name
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

		b, err = dec.PeekByte()
		if err != nil {
			return imap.ErrBad("unexpected end of command")
		}
	}

	// If we see a literal start, this is a standard APPEND.
	// Handle it ourselves since we already consumed the mailbox/flags/date.
	if b == '{' || b == '~' {
		return handleStandardAppend(ctx, dec, mailbox, options)
	}

	// Read the next atom to see if it is CATENATE
	atom, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("invalid APPEND arguments")
	}

	if !strings.EqualFold(atom, "CATENATE") {
		return imap.ErrBad("expected literal or CATENATE")
	}

	// This is a CATENATE APPEND
	sess, ok := ctx.Session.(SessionCatenate)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "CATENATE not supported")
		return nil
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing CATENATE parts")
	}

	// Parse the list of catenate parts.
	parts, err := parseCatenateParts(dec, ctx.Conn)
	if err != nil {
		return imap.ErrBad(fmt.Sprintf("invalid CATENATE parts: %v", err))
	}

	data, err := sess.AppendCatenate(mailbox, parts, options)
	if err != nil {
		return err
	}

	// Write tagged OK response
	if data != nil && data.UIDValidity > 0 && data.UID > 0 {
		ctx.Conn.WriteOKCode(ctx.Tag,
			fmt.Sprintf("APPENDUID %d %d", data.UIDValidity, uint32(data.UID)),
			"APPEND completed")
	} else {
		ctx.Conn.WriteOK(ctx.Tag, "APPEND completed")
	}

	return nil
}

// handleStandardAppend handles a standard (non-CATENATE) APPEND after
// the mailbox, flags, and date have already been parsed from the arg decoder.
func handleStandardAppend(ctx *server.CommandContext, dec *wire.Decoder, mailbox string, options *imap.AppendOptions) error {
	// Read the literal size from the arg decoder. The arg decoder has the
	// {N}, {N+}, or ~{N} at the end of the line without a trailing CRLF.
	litSize, isBinary, err := readLiteralSizeBinary(dec)
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

	data, err := ctx.Session.Append(mailbox, literalReader, options)
	if err != nil {
		// Drain remaining literal data
		_, _ = io.Copy(io.Discard, literalReader.Reader)
		return err
	}

	// Drain remaining literal data
	_, _ = io.Copy(io.Discard, literalReader.Reader)

	// Write tagged OK response
	if data != nil && data.UIDValidity > 0 && data.UID > 0 {
		ctx.Conn.WriteOKCode(ctx.Tag,
			fmt.Sprintf("APPENDUID %d %d", data.UIDValidity, uint32(data.UID)),
			"APPEND completed")
	} else {
		ctx.Conn.WriteOK(ctx.Tag, "APPEND completed")
	}

	return nil
}

// parseCatenateParts reads the CATENATE parts list. It begins reading from
// the argument decoder (argDec) for the opening paren and initial items.
// When a TEXT literal is encountered, the literal header {N} is parsed from
// the arg decoder and the literal body is read from the connection decoder.
// After reading a literal body, subsequent items and the closing paren are
// read from the connection decoder since they arrive on subsequent lines.
func parseCatenateParts(argDec *wire.Decoder, conn *server.Conn) ([]CatenatePart, error) {
	var parts []CatenatePart

	// The current decoder starts as the arg decoder but switches to the
	// connection decoder after reading a TEXT literal (since subsequent
	// data arrives on the wire after the literal body).
	dec := argDec

	// Read opening paren
	if err := dec.ExpectByte('('); err != nil {
		return nil, fmt.Errorf("expected opening paren: %w", err)
	}

	first := true
	for {
		b, err := dec.PeekByte()
		if err != nil {
			// If the arg decoder is exhausted, switch to conn decoder
			if dec != conn.Decoder() {
				dec = conn.Decoder()
				b, err = dec.PeekByte()
				if err != nil {
					return nil, fmt.Errorf("unexpected end of CATENATE parts: %w", err)
				}
			} else {
				return nil, fmt.Errorf("unexpected end of CATENATE parts: %w", err)
			}
		}

		if b == ')' {
			_ = dec.ExpectByte(')')
			break
		}

		if !first {
			if err := dec.ReadSP(); err != nil {
				return nil, fmt.Errorf("expected SP between CATENATE parts: %w", err)
			}
		}
		first = false

		atom, err := dec.ReadAtom()
		if err != nil {
			return nil, fmt.Errorf("expected URL or TEXT: %w", err)
		}

		switch strings.ToUpper(atom) {
		case "URL":
			if err := dec.ReadSP(); err != nil {
				return nil, fmt.Errorf("expected SP after URL: %w", err)
			}
			url, err := dec.ReadAString()
			if err != nil {
				return nil, fmt.Errorf("invalid URL: %w", err)
			}
			parts = append(parts, CatenatePart{
				Type: "URL",
				URL:  url,
			})

		case "TEXT":
			if err := dec.ReadSP(); err != nil {
				return nil, fmt.Errorf("expected SP after TEXT: %w", err)
			}
			// Read the literal size from the current decoder.
			// If on the arg decoder, the {N} is at the end of the line
			// without CRLF. Read it using readLiteralSize which consumes
			// remaining bytes.
			var litSize int64
			if dec == argDec {
				litSize, err = readLiteralSize(dec)
				if err != nil {
					return nil, fmt.Errorf("invalid TEXT literal: %w", err)
				}
			} else {
				// On the connection decoder, use ReadLiteralInfo which
				// expects {N}\r\n.
				litInfo, err := dec.ReadLiteralInfo()
				if err != nil {
					return nil, fmt.Errorf("invalid TEXT literal: %w", err)
				}
				litSize = litInfo.Size
			}

			// Read literal body from the connection decoder
			connDec := conn.Decoder()
			data, err := io.ReadAll(io.LimitReader(connDec.ReadLiteral(litSize), litSize))
			if err != nil {
				return nil, fmt.Errorf("error reading TEXT literal: %w", err)
			}
			parts = append(parts, CatenatePart{
				Type: "TEXT",
				Text: data,
			})

			// After reading a literal body, subsequent data comes from
			// the connection decoder.
			dec = connDec

		default:
			return nil, fmt.Errorf("unknown CATENATE part type: %s", atom)
		}
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("empty CATENATE parts list")
	}

	return parts, nil
}

// readLiteralSize reads a literal size specification like {42} or {42+}
// from the decoder, without expecting a trailing CRLF. This is used when
// reading from the arg decoder which is built from an already-parsed line.
func readLiteralSize(dec *wire.Decoder) (int64, error) {
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

	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return 0, fmt.Errorf("expected literal, got %q", s)
	}

	inner := s[1 : len(s)-1]
	inner = strings.TrimSuffix(inner, "+")

	size, err := strconv.ParseInt(inner, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid literal size %q: %w", inner, err)
	}

	return size, nil
}

// readLiteralSizeBinary is like readLiteralSize but also detects ~{N} binary
// literals (RFC 3516). Returns the size, whether it's binary, and any error.
func readLiteralSizeBinary(dec *wire.Decoder) (int64, bool, error) {
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
func parseDate(s string) (t time.Time, err error) {
	layouts := []string{
		imap.InternalDateLayout,
		"2-Jan-2006 15:04:05 -0700",
	}
	for _, layout := range layouts {
		if t, err = time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return t, fmt.Errorf("cannot parse date %q", s)
}
