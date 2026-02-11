// Package utf8accept implements the UTF8=ACCEPT extension (RFC 6855).
//
// UTF8=ACCEPT allows the IMAP server to accept and send UTF-8 encoded
// strings in mailbox names, message headers, and other protocol elements.
// It is activated via the ENABLE command (ENABLE UTF8=ACCEPT), after which
// the server operates in UTF-8 mode for the connection.
//
// This extension wraps ENABLE to notify the backend session when UTF-8 mode
// is activated, and wraps APPEND to handle the UTF8 (~{N+}) literal syntax.
package utf8accept

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

// SessionUTF8Accept is the session interface for UTF8=ACCEPT support.
// Backends implement this to be notified when a client enables UTF-8 mode,
// allowing the session to adjust its behavior for UTF-8 encoded data.
type SessionUTF8Accept interface {
	// EnableUTF8 is called when a client enables UTF-8 mode via
	// ENABLE UTF8=ACCEPT. After this call, the session should accept
	// and produce UTF-8 encoded strings in mailbox names, headers,
	// and other protocol elements.
	EnableUTF8() error
}

// Extension implements the UTF8=ACCEPT extension (RFC 6855).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new UTF8=ACCEPT extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "UTF8=ACCEPT",
			ExtCapabilities: []imap.Cap{imap.CapUTF8Accept},
		},
	}
}

// CommandHandlers returns new command handlers to register.
// UTF8=ACCEPT is activated via the ENABLE command, which is handled
// separately, so no new command handlers are needed.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return nil
}

// WrapHandler wraps ENABLE and APPEND command handlers.
// ENABLE is wrapped to notify the backend session when UTF-8 mode is activated.
// APPEND is wrapped to handle the UTF8 (~{N+}) literal syntax per RFC 6855.
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
	case "ENABLE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUTF8Enable(ctx, h)
		})
	case "APPEND":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleUTF8Append(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns a typed nil pointer to SessionUTF8Accept,
// indicating that sessions should implement this interface for full
// UTF8=ACCEPT support.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionUTF8Accept)(nil)
}

// OnEnabled is called when a client enables UTF8=ACCEPT via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// handleUTF8Enable wraps the ENABLE command to detect UTF8=ACCEPT activation
// and notify the backend session.
func handleUTF8Enable(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	// Run the original ENABLE handler first — it adds caps to Conn.Enabled()
	// and writes the * ENABLED response + tagged OK.
	if err := original(ctx); err != nil {
		return err
	}

	// Check if UTF8=ACCEPT was just enabled
	if !ctx.Conn.Enabled().Has(imap.CapUTF8Accept) {
		return nil
	}

	// Notify the session if it implements SessionUTF8Accept
	if sess, ok := ctx.Session.(SessionUTF8Accept); ok {
		return sess.EnableUTF8()
	}

	return nil
}

// handleUTF8Append wraps the APPEND command to handle the UTF8 (~{N+}) literal
// syntax per RFC 6855 section 3.
func handleUTF8Append(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
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

		b, err = dec.PeekByte()
		if err != nil {
			return imap.ErrBad("unexpected end of command")
		}
	}

	// If we see a literal start ({N} or ~{N}), this is a standard/binary APPEND.
	if b == '{' || b == '~' {
		return handleStandardAppend(ctx, dec, mailbox, options)
	}

	// Read the next atom to see if it is UTF8
	atom, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("invalid APPEND arguments")
	}

	if !strings.EqualFold(atom, "UTF8") {
		return imap.ErrBad("expected literal or UTF8")
	}

	// UTF8 APPEND — check that UTF8=ACCEPT is enabled
	if !ctx.Conn.Enabled().Has(imap.CapUTF8Accept) {
		return imap.ErrBad("UTF8=ACCEPT not enabled")
	}

	// Read SP before the parenthesized literal
	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing UTF8 literal")
	}

	// Read opening parenthesis
	if err := dec.ExpectByte('('); err != nil {
		return imap.ErrBad("expected '(' after UTF8")
	}

	// Read the literal size from inside the parentheses.
	// The ~{N+} literal8 syntax is required by RFC 6855 for UTF8 APPEND;
	// the ~ prefix is part of the literal8 format, not a binary indicator.
	litSize, _, err := readLiteralSize(dec)
	if err != nil {
		return imap.ErrBad(fmt.Sprintf("invalid UTF8 literal: %v", err))
	}

	options.UTF8 = true

	// Read the literal body from the connection's main decoder
	connDec := ctx.Conn.Decoder()
	literalReader := imap.LiteralReader{
		Reader: connDec.ReadLiteral(litSize),
		Size:   litSize,
	}

	data, err := ctx.Session.Append(mailbox, literalReader, options)
	if err != nil {
		_, _ = io.Copy(io.Discard, literalReader.Reader)
		return err
	}
	_, _ = io.Copy(io.Discard, literalReader.Reader)

	// Read closing parenthesis from connection decoder
	if pErr := connDec.ExpectByte(')'); pErr != nil {
		return imap.ErrBad("expected ')' after UTF8 literal")
	}

	writeAppendOK(ctx, data)
	return nil
}

// handleStandardAppend handles a standard (non-UTF8) APPEND after the mailbox,
// flags, and date have already been parsed from the arg decoder.
func handleStandardAppend(ctx *server.CommandContext, dec *wire.Decoder, mailbox string, options *imap.AppendOptions) error {
	litSize, isBinary, err := readLiteralSize(dec)
	if err != nil {
		return imap.ErrBad(fmt.Sprintf("invalid literal: %v", err))
	}

	if isBinary {
		options.Binary = true
	}

	connDec := ctx.Conn.Decoder()
	literalReader := imap.LiteralReader{
		Reader: connDec.ReadLiteral(litSize),
		Size:   litSize,
	}

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
		ctx.Conn.WriteOKCode(ctx.Tag,
			fmt.Sprintf("APPENDUID %d %d", data.UIDValidity, uint32(data.UID)),
			"APPEND completed")
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
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q", s)
}
