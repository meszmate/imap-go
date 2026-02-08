package server

import (
	"context"
	"sync"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/wire"
)

// NumKind indicates whether a command uses sequence numbers or UIDs.
type NumKind = imap.NumKind

const (
	NumKindSeq = imap.NumKindSeq
	NumKindUID = imap.NumKindUID
)

// CommandHandler handles an IMAP command.
type CommandHandler interface {
	Handle(ctx *CommandContext) error
}

// CommandHandlerFunc is a function that implements CommandHandler.
type CommandHandlerFunc func(ctx *CommandContext) error

// Handle implements CommandHandler.
func (f CommandHandlerFunc) Handle(ctx *CommandContext) error {
	return f(ctx)
}

// CommandContext provides context for handling a single IMAP command.
type CommandContext struct {
	// Context is the Go context for this command (with timeout, cancellation, tracing).
	Context context.Context

	// Tag is the command tag.
	Tag string

	// Name is the command name (uppercase).
	Name string

	// NumKind indicates if this is a UID command (NumKindUID) or sequence command (NumKindSeq).
	NumKind NumKind

	// Conn is the connection this command was received on.
	Conn *Conn

	// Session is the backend session for this connection.
	Session Session

	// Server is the server instance.
	Server *Server

	// Decoder is the wire decoder for reading command arguments.
	Decoder *wire.Decoder

	// values stores middleware-injected values for the duration of this command.
	mu     sync.RWMutex
	values map[string]interface{}
}

// SetValue stores a value in the command context (for middleware data passing).
func (ctx *CommandContext) SetValue(key string, value interface{}) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if ctx.values == nil {
		ctx.values = make(map[string]interface{})
	}
	ctx.values[key] = value
}

// Value retrieves a value from the command context.
func (ctx *CommandContext) Value(key string) (interface{}, bool) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	v, ok := ctx.values[key]
	return v, ok
}

// State returns the current connection state.
func (ctx *CommandContext) State() imap.ConnState {
	return ctx.Conn.State()
}
