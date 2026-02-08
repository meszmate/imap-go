package id

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Extension implements the ID IMAP extension (RFC 2971).
// ID allows the client and server to exchange identification information
// such as software name, version, and other details.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new ID extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "ID",
			ExtCapabilities: []imap.Cap{imap.CapID},
		},
	}
}

// CommandHandlers returns the ID command handler.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		"ID": handleID(),
	}
}

func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionID interface that sessions must
// implement to support the ID command.
func (e *Extension) SessionExtension() interface{} {
	return (*server.SessionID)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleID returns the command handler function for the ID command.
func handleID() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		// The session must implement SessionID
		sessID, ok := ctx.Session.(server.SessionID)
		if !ok {
			return imap.ErrNo("ID not supported")
		}

		// Parse client ID data: either NIL or a parenthesized list of
		// key-value pairs.
		clientID, err := readIDData(ctx.Decoder)
		if err != nil {
			return imap.ErrBad("invalid ID arguments")
		}

		serverID, err := sessID.ID(clientID)
		if err != nil {
			return err
		}

		// Write untagged ID response
		ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
			enc.Star().Atom("ID").SP()
			writeIDData(enc, serverID)
			enc.CRLF()
		})

		ctx.Conn.WriteOK(ctx.Tag, "ID completed")
		return nil
	}
}

// readIDData reads ID data from the decoder. The data is either NIL or
// a parenthesized list of string key-value pairs.
func readIDData(dec *wire.Decoder) (imap.IDData, error) {
	if dec == nil {
		return nil, nil
	}

	// Check for NIL
	b, err := dec.PeekByte()
	if err != nil {
		// No arguments means NIL
		return nil, nil
	}

	if b == 'N' || b == 'n' {
		// Read NIL atom
		atom, err := dec.ReadAtom()
		if err != nil {
			return nil, err
		}
		if atom == "NIL" || atom == "nil" || atom == "Nil" {
			return nil, nil
		}
		return nil, imap.ErrBad("expected NIL or list")
	}

	// Read parenthesized list of key-value pairs
	data := make(imap.IDData)
	err = dec.ReadList(func() error {
		key, err := dec.ReadString()
		if err != nil {
			return err
		}
		if err := dec.ReadSP(); err != nil {
			return err
		}
		val, ok, err := dec.ReadNString()
		if err != nil {
			return err
		}
		if ok {
			data[key] = &val
		} else {
			data[key] = nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return data, nil
}

// writeIDData writes ID data to the encoder. Writes NIL if data is nil,
// otherwise writes a parenthesized list of key-value pairs.
func writeIDData(enc *wire.Encoder, data *imap.IDData) {
	if data == nil || len(*data) == 0 {
		enc.Nil()
		return
	}

	enc.BeginList()
	first := true
	for key, val := range *data {
		if !first {
			enc.SP()
		}
		enc.QuotedString(key)
		enc.SP()
		enc.NString(val)
		first = false
	}
	enc.EndList()
}
