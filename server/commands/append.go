package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Append returns a handler for the APPEND command.
// APPEND appends a message to the specified mailbox.
//
// The command format is:
//
//	tag APPEND mailbox [flags] [date-time] {literal-size}
//	<literal data>
func Append() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing arguments")
		}

		mailbox, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox name")
		}

		options := &imap.AppendOptions{}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("missing message data")
		}

		// Check if we have flags (starts with '(')
		b, err := ctx.Decoder.PeekByte()
		if err != nil {
			return imap.ErrBad("unexpected end of command")
		}

		if b == '(' {
			flagStrs, err := ctx.Decoder.ReadFlags()
			if err != nil {
				return imap.ErrBad("invalid flags")
			}
			for _, f := range flagStrs {
				options.Flags = append(options.Flags, imap.Flag(f))
			}

			if err := ctx.Decoder.ReadSP(); err != nil {
				return imap.ErrBad("missing message data")
			}

			b, err = ctx.Decoder.PeekByte()
			if err != nil {
				return imap.ErrBad("unexpected end of command")
			}
		}

		// Check if we have a date-time (starts with '"')
		if b == '"' {
			dateStr, err := ctx.Decoder.ReadQuotedString()
			if err != nil {
				return imap.ErrBad("invalid date-time")
			}

			t, err := time.Parse(imap.InternalDateLayout, dateStr)
			if err != nil {
				// Try alternative layouts
				layouts := []string{
					"2-Jan-2006 15:04:05 -0700",
					time.RFC822Z,
				}
				var parsed bool
				for _, layout := range layouts {
					if t, err = time.Parse(layout, dateStr); err == nil {
						parsed = true
						break
					}
				}
				if !parsed {
					return imap.ErrBad("invalid date-time format")
				}
			}
			options.InternalDate = t

			if err := ctx.Decoder.ReadSP(); err != nil {
				return imap.ErrBad("missing message data")
			}
		}

		// Parse literal size from the rest of the arguments.
		// The literal info looks like {42} or {42+} at the end of the line.
		// Since the arg decoder is built from the line remainder (after CRLF
		// stripping), we parse the literal header here and then read the
		// actual data from the connection's main decoder.
		litSize, err := readLiteralSize(ctx.Decoder)
		if err != nil {
			return imap.ErrBad(fmt.Sprintf("invalid literal: %v", err))
		}

		// Read the literal body from the connection's main decoder
		connDec := ctx.Conn.Decoder()
		literalReader := imap.LiteralReader{
			Reader: connDec.ReadLiteral(litSize),
			Size:   litSize,
		}

		data, err := ctx.Session.Append(mailbox, literalReader, options)
		if err != nil {
			// Drain any remaining literal data
			_, _ = io.Copy(io.Discard, literalReader.Reader)
			return err
		}

		// Drain any remaining literal data
		_, _ = io.Copy(io.Discard, literalReader.Reader)

		// Write tagged OK, optionally with APPENDUID response code
		if data != nil && data.UIDValidity > 0 && data.UID > 0 {
			enc := ctx.Conn.Encoder()
			enc.Encode(func(e *wire.Encoder) {
				code := fmt.Sprintf("APPENDUID %d %d", data.UIDValidity, uint32(data.UID))
				e.StatusResponse(ctx.Tag, "OK", code, "APPEND completed")
			})
		} else {
			ctx.Conn.WriteOK(ctx.Tag, "APPEND completed")
		}

		return nil
	}
}

// readLiteralSize reads a literal size specification like {42} or {42+}
// from the decoder, without expecting a trailing CRLF (since the arg
// decoder is built from an already-parsed line).
func readLiteralSize(dec *wire.Decoder) (int64, error) {
	// Read remaining content as a string to parse the literal spec
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

	// Expect format: {number} or {number+}
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return 0, fmt.Errorf("expected literal, got %q", s)
	}

	inner := s[1 : len(s)-1]
	inner = strings.TrimSuffix(inner, "+") // Handle non-synchronizing literal

	size, err := strconv.ParseInt(inner, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid literal size %q: %w", inner, err)
	}

	return size, nil
}
