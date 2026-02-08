package server

import (
	"context"
	"fmt"
	"strings"
	"sync"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/state"
	"github.com/meszmate/imap-go/wire"
)

// Dispatcher manages command handler registration and dispatch.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string]CommandHandler
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string]CommandHandler),
	}
}

// Register registers a handler for a command name.
func (d *Dispatcher) Register(name string, handler CommandHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[strings.ToUpper(name)] = handler
}

// RegisterFunc registers a handler function for a command name.
func (d *Dispatcher) RegisterFunc(name string, fn CommandHandlerFunc) {
	d.Register(name, fn)
}

// Get returns the handler for a command, or nil if not registered.
func (d *Dispatcher) Get(name string) CommandHandler {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.handlers[strings.ToUpper(name)]
}

// Wrap wraps an existing handler with a wrapper function.
// If no handler is registered, this is a no-op.
func (d *Dispatcher) Wrap(name string, wrapper func(CommandHandler) CommandHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	upper := strings.ToUpper(name)
	if h, ok := d.handlers[upper]; ok {
		d.handlers[upper] = wrapper(h)
	}
}

// Names returns all registered command names.
func (d *Dispatcher) Names() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	names := make([]string, 0, len(d.handlers))
	for name := range d.handlers {
		names = append(names, name)
	}
	return names
}

// dispatch dispatches a command to its handler.
func (srv *Server) dispatch(c *Conn, tag, name, rest string) error {
	upper := strings.ToUpper(name)

	// Check for UID prefix
	numKind := NumKindSeq
	if upper == "UID" {
		numKind = NumKindUID
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			c.WriteBAD(tag, "missing command after UID")
			return nil
		}
		upper = strings.ToUpper(parts[0])
		if len(parts) > 1 {
			rest = parts[1]
		} else {
			rest = ""
		}
	}

	// Check command is allowed in current state
	allowed := state.CommandAllowedStates(upper)
	if allowed == nil {
		// Extension command - check the dispatcher
		handler := srv.dispatcher.Get(upper)
		if handler == nil {
			c.WriteBAD(tag, fmt.Sprintf("unknown command %s", upper))
			return nil
		}
		// Extension commands are handled via dispatcher, state checking is handler's responsibility
	} else {
		if err := c.state.RequireState(allowed...); err != nil {
			c.WriteBAD(tag, err.Error())
			return nil
		}
	}

	handler := srv.dispatcher.Get(upper)
	if handler == nil {
		c.WriteBAD(tag, fmt.Sprintf("command %s not implemented", upper))
		return nil
	}

	// Build decoder for the rest of the line
	var dec *wire.Decoder
	if rest != "" {
		dec = wire.NewDecoder(strings.NewReader(rest))
	}

	ctx := &CommandContext{
		Context: context.Background(),
		Tag:     tag,
		Name:    upper,
		NumKind: numKind,
		Conn:    c,
		Session: c.session,
		Server:  srv,
		Decoder: dec,
	}

	err := handler.Handle(ctx)
	if err != nil {
		// Check if it's an IMAP error
		if imapErr, ok := err.(*imap.IMAPError); ok {
			switch imapErr.Type {
			case imap.StatusResponseTypeNO:
				c.encoder.Encode(func(enc *wire.Encoder) {
					code := ""
					if imapErr.Code != "" {
						code = string(imapErr.Code)
					}
					enc.StatusResponse(tag, "NO", code, imapErr.Text)
				})
			case imap.StatusResponseTypeBAD:
				c.encoder.Encode(func(enc *wire.Encoder) {
					code := ""
					if imapErr.Code != "" {
						code = string(imapErr.Code)
					}
					enc.StatusResponse(tag, "BAD", code, imapErr.Text)
				})
			case imap.StatusResponseTypeBYE:
				c.WriteBYE(imapErr.Text)
				return fmt.Errorf("BYE: %s", imapErr.Text)
			default:
				c.WriteNO(tag, err.Error())
			}
		} else {
			c.logger.Error("command handler error", "command", upper, "error", err)
			c.WriteNO(tag, "internal server error")
		}
	}

	return nil
}

// parseLine parses a command line into tag, command name, and remaining arguments.
func parseLine(line string) (tag, name, rest string, err error) {
	if line == "" {
		return "", "", "", fmt.Errorf("empty command")
	}

	// Find tag
	spaceIdx := strings.IndexByte(line, ' ')
	if spaceIdx < 0 {
		return "", "", "", fmt.Errorf("missing command name")
	}
	tag = line[:spaceIdx]
	remaining := line[spaceIdx+1:]

	// Find command name
	spaceIdx = strings.IndexByte(remaining, ' ')
	if spaceIdx < 0 {
		name = remaining
		rest = ""
	} else {
		name = remaining[:spaceIdx]
		rest = remaining[spaceIdx+1:]
	}

	if tag == "" {
		return "", "", "", fmt.Errorf("empty tag")
	}
	if name == "" {
		return "", "", "", fmt.Errorf("empty command name")
	}

	return tag, name, rest, nil
}
