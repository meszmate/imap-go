// Package unauthenticate implements the UNAUTHENTICATE IMAP extension (RFC 8437).
//
// UNAUTHENTICATE allows the client to return to the not-authenticated
// state, enabling re-authentication on the same connection without
// disconnecting.
package unauthenticate

import (
	"fmt"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// SessionUnauthenticate is an optional interface for sessions that support
// the UNAUTHENTICATE command.
type SessionUnauthenticate interface {
	// Unauthenticate resets the session to the not-authenticated state.
	Unauthenticate() error
}

// Extension implements the UNAUTHENTICATE IMAP extension (RFC 8437).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new UNAUTHENTICATE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "UNAUTHENTICATE",
			ExtCapabilities: []imap.Cap{imap.CapUnauthenticate},
		},
	}
}

// CommandHandlers returns the UNAUTHENTICATE command handler.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandUnauthenticate: server.CommandHandlerFunc(handleUnauthenticate),
	}
}

func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionUnauthenticate interface that sessions
// must implement to support the UNAUTHENTICATE command.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionUnauthenticate)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleUnauthenticate handles the UNAUTHENTICATE command.
func handleUnauthenticate(ctx *server.CommandContext) error {
	state := ctx.Conn.State()
	if state != imap.ConnStateAuthenticated && state != imap.ConnStateSelected {
		ctx.Conn.WriteBAD(ctx.Tag, "UNAUTHENTICATE not allowed in current state")
		return nil
	}

	sess, ok := ctx.Session.(SessionUnauthenticate)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "UNAUTHENTICATE not supported")
		return nil
	}

	if err := sess.Unauthenticate(); err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("UNAUTHENTICATE failed: %v", err))
		return nil
	}

	if err := ctx.Conn.SetState(imap.ConnStateNotAuthenticated); err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("UNAUTHENTICATE failed: %v", err))
		return nil
	}

	ctx.Conn.WriteOK(ctx.Tag, "UNAUTHENTICATE completed")
	return nil
}
