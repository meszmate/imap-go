package commands

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
)

// Login returns a handler for the LOGIN command.
// LOGIN authenticates the user with a username and password.
func Login() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if !ctx.Conn.IsTLS() && !ctx.Server.Options().AllowInsecureAuth {
			return imap.ErrNo("LOGIN disabled without TLS")
		}

		if ctx.Decoder == nil {
			return imap.ErrBad("missing arguments")
		}

		username, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid username")
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("missing password")
		}

		password, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid password")
		}

		if err := ctx.Session.Login(username, password); err != nil {
			return err
		}

		if err := ctx.Conn.SetState(imap.ConnStateAuthenticated); err != nil {
			return err
		}

		ctx.Conn.WriteOK(ctx.Tag, "LOGIN completed")
		return nil
	}
}
