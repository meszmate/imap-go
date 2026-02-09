// Package filters implements the FILTERS IMAP extension (RFC 5466).
//
// FILTERS provides server-side message filtering capabilities. It allows
// clients to get and set named filter criteria that the server can apply
// to incoming messages.
package filters

import (
	"fmt"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionFilters is an optional interface for sessions that support
// the FILTERS commands.
type SessionFilters interface {
	// GetFilter retrieves the filter criteria for the given filter name.
	GetFilter(name string) (string, error)

	// SetFilter sets the filter criteria for the given filter name.
	SetFilter(name string, criteria string) error
}

// Extension implements the FILTERS IMAP extension (RFC 5466).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new FILTERS extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "FILTERS",
			ExtCapabilities: []imap.Cap{imap.CapFilters},
		},
	}
}

// CommandHandlers returns the FILTERS command handlers.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandGetFilter: server.CommandHandlerFunc(handleGetFilter),
		imap.CommandSetFilter: server.CommandHandlerFunc(handleSetFilter),
	}
}

func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionFilters interface that sessions
// must implement to support the FILTERS commands.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionFilters)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleGetFilter handles the GETFILTER command.
//
// Command syntax: GETFILTER name
// Response:       * FILTER name criteria
func handleGetFilter(ctx *server.CommandContext) error {
	state := ctx.Conn.State()
	if state != imap.ConnStateAuthenticated && state != imap.ConnStateSelected {
		ctx.Conn.WriteBAD(ctx.Tag, "GETFILTER not allowed in current state")
		return nil
	}

	sess, ok := ctx.Session.(SessionFilters)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "GETFILTER not supported")
		return nil
	}

	if ctx.Decoder == nil {
		ctx.Conn.WriteBAD(ctx.Tag, "missing filter name")
		return nil
	}

	name, err := ctx.Decoder.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "invalid filter name")
		return nil
	}

	criteria, err := sess.GetFilter(name)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("GETFILTER failed: %v", err))
		return nil
	}

	// Write FILTER response: * FILTER name criteria
	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("FILTER").SP().AString(name).SP().AString(criteria).CRLF()
	})

	ctx.Conn.WriteOK(ctx.Tag, "GETFILTER completed")
	return nil
}

// handleSetFilter handles the SETFILTER command.
//
// Command syntax: SETFILTER name criteria
func handleSetFilter(ctx *server.CommandContext) error {
	state := ctx.Conn.State()
	if state != imap.ConnStateAuthenticated && state != imap.ConnStateSelected {
		ctx.Conn.WriteBAD(ctx.Tag, "SETFILTER not allowed in current state")
		return nil
	}

	sess, ok := ctx.Session.(SessionFilters)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "SETFILTER not supported")
		return nil
	}

	if ctx.Decoder == nil {
		ctx.Conn.WriteBAD(ctx.Tag, "missing arguments")
		return nil
	}

	name, err := ctx.Decoder.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "invalid filter name")
		return nil
	}

	if err := ctx.Decoder.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "missing filter criteria")
		return nil
	}

	criteria, err := ctx.Decoder.ReadAString()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "invalid filter criteria")
		return nil
	}

	if err := sess.SetFilter(name, criteria); err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("SETFILTER failed: %v", err))
		return nil
	}

	ctx.Conn.WriteOK(ctx.Tag, "SETFILTER completed")
	return nil
}
