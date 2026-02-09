// Package replace implements the REPLACE IMAP extension (RFC 8508).
//
// REPLACE atomically replaces a message in a mailbox. It combines the
// operations of APPEND and STORE +FLAGS (\Deleted) and EXPUNGE into a
// single atomic operation, ensuring that the old message is removed and
// the new message is added without race conditions.
package replace

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

// SessionReplace is an optional interface for sessions that support
// the REPLACE command.
type SessionReplace interface {
	// Replace atomically replaces a message identified by numSet in the
	// specified mailbox with the new message data.
	Replace(numSet imap.NumSet, mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error)
}

// Extension implements the REPLACE IMAP extension (RFC 8508).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new REPLACE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "REPLACE",
			ExtCapabilities: []imap.Cap{imap.CapReplace},
		},
	}
}

// CommandHandlers returns the REPLACE command handler.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandReplace: handleReplace(),
	}
}

func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionReplace interface that sessions
// must implement to support the REPLACE command.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionReplace)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleReplace returns the command handler function for the REPLACE command.
func handleReplace() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			ctx.Conn.WriteBAD(ctx.Tag, "missing arguments")
			return nil
		}

		sess, ok := ctx.Session.(SessionReplace)
		if !ok {
			ctx.Conn.WriteNO(ctx.Tag, "REPLACE not supported")
			return nil
		}

		// Read the message set (sequence set or UID set)
		setStr, err := ctx.Decoder.ReadAtom()
		if err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "invalid message set")
			return nil
		}

		var numSet imap.NumSet
		if ctx.NumKind == server.NumKindUID {
			uidSet, err := imap.ParseUIDSet(setStr)
			if err != nil {
				ctx.Conn.WriteBAD(ctx.Tag, "invalid UID set")
				return nil
			}
			numSet = uidSet
		} else {
			seqSet, err := imap.ParseSeqSet(setStr)
			if err != nil {
				ctx.Conn.WriteBAD(ctx.Tag, "invalid sequence set")
				return nil
			}
			numSet = seqSet
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "missing destination mailbox")
			return nil
		}

		// Read mailbox name
		mailbox, err := ctx.Decoder.ReadAString()
		if err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "invalid mailbox name")
			return nil
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "missing message data")
			return nil
		}

		options := &imap.AppendOptions{}

		// Check if we have flags (starts with '(')
		b, err := ctx.Decoder.PeekByte()
		if err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "unexpected end of command")
			return nil
		}

		if b == '(' {
			flagStrs, err := ctx.Decoder.ReadFlags()
			if err != nil {
				ctx.Conn.WriteBAD(ctx.Tag, "invalid flags")
				return nil
			}
			for _, f := range flagStrs {
				options.Flags = append(options.Flags, imap.Flag(f))
			}

			if err := ctx.Decoder.ReadSP(); err != nil {
				ctx.Conn.WriteBAD(ctx.Tag, "missing message data")
				return nil
			}

			b, err = ctx.Decoder.PeekByte()
			if err != nil {
				ctx.Conn.WriteBAD(ctx.Tag, "unexpected end of command")
				return nil
			}
		}

		// Check if we have a date-time (starts with '"')
		if b == '"' {
			dateStr, err := ctx.Decoder.ReadQuotedString()
			if err != nil {
				ctx.Conn.WriteBAD(ctx.Tag, "invalid date-time")
				return nil
			}

			t, err := time.Parse(imap.InternalDateLayout, dateStr)
			if err != nil {
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
					ctx.Conn.WriteBAD(ctx.Tag, "invalid date-time format")
					return nil
				}
			}
			options.InternalDate = t

			if err := ctx.Decoder.ReadSP(); err != nil {
				ctx.Conn.WriteBAD(ctx.Tag, "missing message data")
				return nil
			}
		}

		// Parse literal size
		litSize, err := readLiteralSize(ctx.Decoder)
		if err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, fmt.Sprintf("invalid literal: %v", err))
			return nil
		}

		// Read the literal body from the connection's main decoder
		connDec := ctx.Conn.Decoder()
		literalReader := imap.LiteralReader{
			Reader: connDec.ReadLiteral(litSize),
			Size:   litSize,
		}

		data, err := sess.Replace(numSet, mailbox, literalReader, options)
		if err != nil {
			_, _ = io.Copy(io.Discard, literalReader.Reader)
			ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("REPLACE failed: %v", err))
			return nil
		}

		// Drain any remaining literal data
		_, _ = io.Copy(io.Discard, literalReader.Reader)

		// Write tagged OK, optionally with APPENDUID response code
		if data != nil && data.UIDValidity > 0 && data.UID > 0 {
			enc := ctx.Conn.Encoder()
			enc.Encode(func(e *wire.Encoder) {
				code := fmt.Sprintf("APPENDUID %d %d", data.UIDValidity, uint32(data.UID))
				e.StatusResponse(ctx.Tag, "OK", code, "REPLACE completed")
			})
		} else {
			ctx.Conn.WriteOK(ctx.Tag, "REPLACE completed")
		}

		return nil
	}
}

// readLiteralSize reads a literal size specification like {42} or {42+}
// from the decoder.
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
