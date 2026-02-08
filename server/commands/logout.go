package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Logout returns a handler for the LOGOUT command.
// LOGOUT informs the server that the client is done with the connection.
func Logout() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		ctx.Conn.WriteBYE("LOGOUT requested")
		if err := ctx.Conn.SetState(imap.ConnStateLogout); err != nil {
			return err
		}
		ctx.Conn.WriteOK(ctx.Tag, "LOGOUT completed")
		return nil
	}
}
