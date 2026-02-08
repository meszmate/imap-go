package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Expunge returns a handler for the EXPUNGE command.
// EXPUNGE permanently removes all messages that have the \Deleted flag set.
func Expunge() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		// For UID EXPUNGE, parse the UID set
		var uids *imap.UIDSet
		if ctx.NumKind == server.NumKindUID && ctx.Decoder != nil {
			uidStr, err := ctx.Decoder.ReadAtom()
			if err != nil {
				return imap.ErrBad("invalid UID set")
			}
			uidSet, err := imap.ParseUIDSet(uidStr)
			if err != nil {
				return imap.ErrBad("invalid UID set")
			}
			uids = uidSet
		}

		w := server.NewExpungeWriter(ctx.Conn.Encoder())
		if err := ctx.Session.Expunge(w, uids); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "EXPUNGE completed")
		return nil
	}
}
