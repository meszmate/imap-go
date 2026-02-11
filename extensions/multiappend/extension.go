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
	"bytes"
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

// WrapHandler wraps the APPEND command handler to detect and collect
// additional messages after the first literal, then call SessionMultiAppend
// for atomic multi-message appending.
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
			return handleMultiAppend(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the SessionMultiAppend interface that sessions may
// implement to support atomic multi-message APPEND operations.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionMultiAppend)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }

// handleMultiAppend wraps APPEND to detect multi-message appends and route
// to SessionMultiAppend.AppendMulti() when available.
func handleMultiAppend(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return original(ctx)
	}

	dec := ctx.Decoder

	// Read mailbox name
	mailbox, err := dec.ReadAString()
	if err != nil {
		return imap.ErrBad("invalid mailbox name")
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing message data")
	}

	// Parse first message metadata (flags, date) from arg decoder
	flags, internalDate, err := readMessageMeta(dec)
	if err != nil {
		return err
	}

	// Read first literal size from arg decoder
	litSize, err := readLiteralSize(dec)
	if err != nil {
		return imap.ErrBad(fmt.Sprintf("invalid literal: %v", err))
	}

	// Read first literal body from connection decoder
	connDec := ctx.Conn.Decoder()
	var firstBody bytes.Buffer
	if _, err := io.Copy(&firstBody, io.LimitReader(connDec.ReadLiteral(litSize), litSize)); err != nil {
		return imap.ErrBad(fmt.Sprintf("error reading literal: %v", err))
	}

	// Check if there are more messages by peeking for SP
	hasMore := false
	if b, err := connDec.PeekByte(); err == nil && b == ' ' {
		hasMore = true
	}

	if !hasMore {
		// Single message — call standard Session.Append()
		options := &imap.AppendOptions{
			Flags:        flags,
			InternalDate: internalDate,
		}
		literalReader := imap.LiteralReader{
			Reader: bytes.NewReader(firstBody.Bytes()),
			Size:   int64(firstBody.Len()),
		}

		data, err := ctx.Session.Append(mailbox, literalReader, options)
		if err != nil {
			return err
		}
		writeAppendOK(ctx, data)
		return nil
	}

	// Multiple messages — collect all
	messages := []MultiAppendMessage{
		{
			Flags:        flags,
			InternalDate: internalDate,
			Literal: imap.LiteralReader{
				Reader: bytes.NewReader(firstBody.Bytes()),
				Size:   int64(firstBody.Len()),
			},
		},
	}

	for hasMore {
		// Read SP separator
		if err := connDec.ReadSP(); err != nil {
			return imap.ErrBad("expected SP between messages")
		}

		// Parse subsequent message metadata from conn decoder
		msgFlags, msgDate, err := readMessageMeta(connDec)
		if err != nil {
			return err
		}

		// Read literal info from conn decoder ({N+}\r\n)
		litInfo, err := connDec.ReadLiteralInfo()
		if err != nil {
			return imap.ErrBad(fmt.Sprintf("invalid literal: %v", err))
		}

		// Read literal body
		var body bytes.Buffer
		if _, err := io.Copy(&body, io.LimitReader(connDec.ReadLiteral(litInfo.Size), litInfo.Size)); err != nil {
			return imap.ErrBad(fmt.Sprintf("error reading literal: %v", err))
		}

		messages = append(messages, MultiAppendMessage{
			Flags:        msgFlags,
			InternalDate: msgDate,
			Literal: imap.LiteralReader{
				Reader: bytes.NewReader(body.Bytes()),
				Size:   int64(body.Len()),
			},
		})

		// Check for another message
		hasMore = false
		if b, err := connDec.PeekByte(); err == nil && b == ' ' {
			hasMore = true
		}
	}

	// Check session implements SessionMultiAppend
	sess, ok := ctx.Session.(SessionMultiAppend)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "MULTIAPPEND not supported by backend")
		return nil
	}

	results, err := sess.AppendMulti(mailbox, messages)
	if err != nil {
		return err
	}

	writeMultiAppendOK(ctx, results)
	return nil
}

// readMessageMeta reads optional [flags] [date] from a decoder.
// Returns the flags, internal date, and any error.
func readMessageMeta(dec *wire.Decoder) ([]imap.Flag, time.Time, error) {
	var flags []imap.Flag
	var internalDate time.Time

	b, err := dec.PeekByte()
	if err != nil {
		return nil, time.Time{}, imap.ErrBad("unexpected end of command")
	}

	// Check for optional flags
	if b == '(' {
		flagStrs, err := dec.ReadFlags()
		if err != nil {
			return nil, time.Time{}, imap.ErrBad("invalid flags")
		}
		for _, f := range flagStrs {
			flags = append(flags, imap.Flag(f))
		}

		if err := dec.ReadSP(); err != nil {
			return nil, time.Time{}, imap.ErrBad("missing message data")
		}

		b, err = dec.PeekByte()
		if err != nil {
			return nil, time.Time{}, imap.ErrBad("unexpected end of command")
		}
	}

	// Check for optional date-time
	if b == '"' {
		dateStr, err := dec.ReadQuotedString()
		if err != nil {
			return nil, time.Time{}, imap.ErrBad("invalid date-time")
		}

		t, parseErr := parseDate(dateStr)
		if parseErr != nil {
			return nil, time.Time{}, imap.ErrBad("invalid date-time format")
		}
		internalDate = t

		if err := dec.ReadSP(); err != nil {
			return nil, time.Time{}, imap.ErrBad("missing message data")
		}
	}

	return flags, internalDate, nil
}

// readLiteralSize reads a literal size specification like {42}, {42+} from
// the arg decoder. Returns the size and any error.
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

// writeAppendOK writes the tagged OK response for a single-message APPEND,
// optionally with APPENDUID.
func writeAppendOK(ctx *server.CommandContext, data *imap.AppendData) {
	if data != nil && data.UIDValidity > 0 && data.UID > 0 {
		ctx.Conn.WriteOKCode(ctx.Tag,
			fmt.Sprintf("APPENDUID %d %d", data.UIDValidity, uint32(data.UID)),
			"APPEND completed")
	} else {
		ctx.Conn.WriteOK(ctx.Tag, "APPEND completed")
	}
}

// writeMultiAppendOK writes the tagged OK response for a multi-message APPEND,
// with APPENDUID containing a comma-separated list of UIDs.
func writeMultiAppendOK(ctx *server.CommandContext, results []*imap.AppendData) {
	if len(results) > 0 && results[0] != nil && results[0].UIDValidity > 0 {
		var uids []string
		allValid := true
		for _, r := range results {
			if r == nil || r.UID == 0 {
				allValid = false
				break
			}
			uids = append(uids, fmt.Sprintf("%d", uint32(r.UID)))
		}
		if allValid {
			ctx.Conn.WriteOKCode(ctx.Tag,
				fmt.Sprintf("APPENDUID %d %s", results[0].UIDValidity, strings.Join(uids, ",")),
				"APPEND completed")
			return
		}
	}
	ctx.Conn.WriteOK(ctx.Tag, "APPEND completed")
}
