package move

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Extension implements the MOVE IMAP extension (RFC 6851).
// MOVE atomically moves messages from the selected mailbox to a
// destination mailbox, combining the effects of UID COPY, UID STORE
// +FLAGS (\Deleted), and UID EXPUNGE into a single operation.
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new MOVE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "MOVE",
			ExtCapabilities: []imap.Cap{imap.CapMove},
		},
	}
}

// CommandHandlers returns the MOVE command handler.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandMove: handleMove(),
	}
}

func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionMove interface that sessions must
// implement to support the MOVE command.
func (e *Extension) SessionExtension() interface{} {
	return (*server.SessionMove)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleMove returns the command handler function for the MOVE command.
func handleMove() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing arguments")
		}

		// Read the message set (sequence set or UID set)
		setStr, err := ctx.Decoder.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid message set")
		}

		var numSet imap.NumSet
		if ctx.NumKind == server.NumKindUID {
			uidSet, err := imap.ParseUIDSet(setStr)
			if err != nil {
				return imap.ErrBad("invalid UID set")
			}
			numSet = uidSet
		} else {
			seqSet, err := imap.ParseSeqSet(setStr)
			if err != nil {
				return imap.ErrBad("invalid sequence set")
			}
			numSet = seqSet
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("missing destination mailbox")
		}

		dest, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid destination mailbox")
		}

		// The session must implement SessionMove
		sessMove, ok := ctx.Session.(server.SessionMove)
		if !ok {
			return imap.ErrNo("MOVE not supported")
		}

		w := server.NewMoveWriter(ctx.Conn.Encoder())
		if err := sessMove.Move(w, numSet, dest); err != nil {
			return err
		}

		// Write tagged OK response
		// The COPYUID response code is part of the OK for MOVE, similar to COPY
		ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
			enc.StatusResponse(ctx.Tag, "OK", "", "MOVE completed")
		})

		return nil
	}
}
