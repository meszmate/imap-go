// Package metadata implements the METADATA extension (RFC 5464).
//
// METADATA allows clients to get and set per-mailbox and server-level
// annotations (metadata entries). Each entry has a name and an optional
// string value. Setting a value to nil removes the entry.
package metadata

import (
	"fmt"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionMetadata is an optional interface for sessions that support METADATA commands.
type SessionMetadata interface {
	// GetMetadata returns metadata entries for the given mailbox (or server if empty).
	GetMetadata(mailbox string, entries []string, options *imap.MetadataOptions) (*imap.MetadataData, error)

	// SetMetadata sets metadata entries for the given mailbox (or server if empty).
	SetMetadata(mailbox string, entries []imap.MetadataEntry) error
}

// Extension implements the METADATA extension (RFC 5464).
type Extension struct {
	extension.BaseExtension
}

// New creates a new METADATA extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName: "METADATA",
			ExtCapabilities: []imap.Cap{
				imap.CapMetadata,
				imap.CapMetadataServer,
			},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		"GETMETADATA": server.CommandHandlerFunc(handleGetMetadata),
		"SETMETADATA": server.CommandHandlerFunc(handleSetMetadata),
	}
}

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns the required session extension interface, or nil.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionMetadata)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// handleGetMetadata handles the GETMETADATA command.
//
// Command syntax: GETMETADATA [options] mailbox entry-names
// Options may include (MAXSIZE n) (DEPTH "0"|"1"|"infinity")
// Response:       * METADATA mailbox (entry1 value1 entry2 value2 ...)
func handleGetMetadata(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(SessionMetadata)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "GETMETADATA not supported")
		return nil
	}

	dec := ctx.Decoder

	// Peek to see if options are present (starts with '(')
	options := &imap.MetadataOptions{}
	b, err := dec.PeekByte()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected arguments")
		return nil
	}

	if b == '(' {
		// Parse options: (MAXSIZE n) or (DEPTH "0"|"1"|"infinity") or both
		if err := dec.ReadList(func() error {
			optName, err := dec.ReadAtom()
			if err != nil {
				return err
			}
			if err := dec.ReadSP(); err != nil {
				return err
			}
			switch optName {
			case "MAXSIZE":
				n, err := dec.ReadNumber64()
				if err != nil {
					return err
				}
				maxSize := int64(n)
				options.MaxSize = &maxSize
			case "DEPTH":
				depth, err := dec.ReadAString()
				if err != nil {
					return err
				}
				options.Depth = depth
			default:
				return fmt.Errorf("unknown GETMETADATA option: %s", optName)
			}
			return nil
		}); err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, fmt.Sprintf("Invalid options: %v", err))
			return nil
		}

		if err := dec.ReadSP(); err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "Expected mailbox name")
			return nil
		}
	}

	// Read mailbox name
	mailbox, err := dec.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected mailbox name")
		return nil
	}

	if err := dec.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected entry names")
		return nil
	}

	// Read entry names - can be a single entry or a parenthesized list
	var entries []string
	b, err = dec.PeekByte()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected entry names")
		return nil
	}

	if b == '(' {
		if err := dec.ReadList(func() error {
			entry, err := dec.ReadAString()
			if err != nil {
				return err
			}
			entries = append(entries, entry)
			return nil
		}); err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "Invalid entry list")
			return nil
		}
	} else {
		entry, err := dec.ReadAString()
		if err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "Expected entry name")
			return nil
		}
		entries = append(entries, entry)
	}

	data, err := sess.GetMetadata(mailbox, entries, options)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("GETMETADATA failed: %v", err))
		return nil
	}

	// Write METADATA response: * METADATA mailbox (entry1 value1 entry2 value2 ...)
	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("METADATA").SP()
		if data.Mailbox == "" {
			enc.QuotedString("")
		} else {
			enc.MailboxName(data.Mailbox)
		}
		enc.SP().BeginList()
		first := true
		for name, value := range data.Entries {
			if !first {
				enc.SP()
			}
			first = false
			enc.AString(name).SP()
			if value == nil {
				enc.Nil()
			} else {
				enc.String(*value)
			}
		}
		enc.EndList().CRLF()
	})

	ctx.Conn.WriteOK(ctx.Tag, "GETMETADATA completed")
	return nil
}

// handleSetMetadata handles the SETMETADATA command.
//
// Command syntax: SETMETADATA mailbox (entry1 value1 entry2 value2 ...)
// Values may be NIL to remove an entry.
func handleSetMetadata(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(SessionMetadata)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "SETMETADATA not supported")
		return nil
	}

	dec := ctx.Decoder

	mailbox, err := dec.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected mailbox name")
		return nil
	}

	if err := dec.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected entry list")
		return nil
	}

	// Parse entries: (name value name value ...)
	var entries []imap.MetadataEntry
	if err := dec.ReadList(func() error {
		name, err := dec.ReadAString()
		if err != nil {
			return err
		}
		if err := dec.ReadSP(); err != nil {
			return err
		}

		// Value can be NIL or a string
		valueStr, ok, err := dec.ReadNString()
		if err != nil {
			return err
		}

		entry := imap.MetadataEntry{Name: name}
		if ok {
			entry.Value = &valueStr
		}
		entries = append(entries, entry)
		return nil
	}); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Invalid entry list")
		return nil
	}

	if err := sess.SetMetadata(mailbox, entries); err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("SETMETADATA failed: %v", err))
		return nil
	}

	ctx.Conn.WriteOK(ctx.Tag, "SETMETADATA completed")
	return nil
}
