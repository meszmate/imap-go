// Package unselect implements the UNSELECT IMAP extension (RFC 3691).
//
// UNSELECT allows the client to close the current mailbox without
// expunging deleted messages. The command transitions the connection
// from Selected to Authenticated state.
package unselect

import (
	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// Extension implements the UNSELECT IMAP extension (RFC 3691).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new UNSELECT extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "UNSELECT",
			ExtCapabilities: []imap.Cap{imap.CapUnselect},
		},
	}
}

// CommandHandlers returns the UNSELECT command handler.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandUnselect: handleUnselect(),
	}
}

func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns nil because UNSELECT uses the core Session.Unselect() method.
func (e *Extension) SessionExtension() interface{} { return nil }

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleUnselect returns the command handler function for the UNSELECT command.
func handleUnselect() server.CommandHandlerFunc {
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
