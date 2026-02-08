package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Close returns a handler for the CLOSE command.
// CLOSE closes the current mailbox, permanently removing all messages
// with the \Deleted flag set, and returns to the authenticated state.
func Close() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		// CLOSE silently expunges, then unselects
		w := server.NewExpungeWriter(ctx.Conn.Encoder())
		// CLOSE does not send expunge responses, but we still need to
		// tell the backend to expunge. The backend handles this via Expunge.
		// Per RFC 3501, CLOSE does not send untagged EXPUNGE responses.
		// We pass a no-op writer or just call expunge and ignore responses.
		_ = ctx.Session.Expunge(w, nil)

		if err := ctx.Session.Unselect(); err != nil {
			return err
		}

		ctx.Conn.SetMailbox("", false)
		if err := ctx.Conn.SetState(imap.ConnStateAuthenticated); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "CLOSE completed")
		return nil
	}
}

// Unselect returns a handler for the UNSELECT command (RFC 3691).
// UNSELECT closes the current mailbox without expunging.
func Unselect() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if err := ctx.Session.Unselect(); err != nil {
			return err
		}

		ctx.Conn.SetMailbox("", false)
		if err := ctx.Conn.SetState(imap.ConnStateAuthenticated); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "UNSELECT completed")
		return nil
	}
}
