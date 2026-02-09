// Package notify implements the NOTIFY extension (RFC 5465).
//
// NOTIFY allows a client to request that the server send unsolicited
// notifications about changes to specified mailboxes. This enables
// real-time monitoring of multiple mailboxes without requiring separate
// IDLE connections for each one.
package notify

import (
	"fmt"
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
)

// NotifyEvent represents a notification event type.
type NotifyEvent struct {
	// EventType is the type of event (e.g., "MessageNew", "MessageExpunge",
	// "FlagChange", etc.).
	EventType string
}

// NotifySpec represents a notification specification for a mailbox or
// mailbox filter.
type NotifySpec struct {
	// Mailbox is the mailbox name, or a special keyword such as "SELECTED",
	// "INBOXES", "PERSONAL", "SUBSCRIBED", etc.
	Mailbox string
	// Events is the list of events to monitor for this mailbox.
	Events []NotifyEvent
}

// SessionNotify is an optional interface for sessions that support
// the NOTIFY command.
type SessionNotify interface {
	// Notify sets up notifications for the given specifications.
	Notify(specs []NotifySpec) error

	// CancelNotify cancels all active notifications.
	CancelNotify() error
}

// Extension implements the NOTIFY extension (RFC 5465).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new NOTIFY extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "NOTIFY",
			ExtCapabilities: []imap.Cap{imap.CapNotify},
		},
	}
}

// CommandHandlers returns the NOTIFY command handler.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandNotify: server.CommandHandlerFunc(handleNotify),
	}
}

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns the SessionNotify interface that sessions
// must implement to support the NOTIFY command.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionNotify)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// handleNotify handles the NOTIFY command.
//
// Command syntax:
//
//	NOTIFY SET spec [spec ...]
//	NOTIFY NONE
//
// Each spec is: mailbox-filter (events)
// mailbox-filter can be: SELECTED, INBOXES, PERSONAL, SUBSCRIBED, or a mailbox name
func handleNotify(ctx *server.CommandContext) error {
	state := ctx.Conn.State()
	if state != imap.ConnStateAuthenticated && state != imap.ConnStateSelected {
		ctx.Conn.WriteBAD(ctx.Tag, "NOTIFY not allowed in current state")
		return nil
	}

	sess, ok := ctx.Session.(SessionNotify)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "NOTIFY not supported")
		return nil
	}

	if ctx.Decoder == nil {
		ctx.Conn.WriteBAD(ctx.Tag, "missing subcommand")
		return nil
	}

	// Read subcommand: SET or NONE
	subcommand, err := ctx.Decoder.ReadAtom()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "invalid subcommand")
		return nil
	}

	switch strings.ToUpper(subcommand) {
	case "NONE":
		if err := sess.CancelNotify(); err != nil {
			ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("NOTIFY NONE failed: %v", err))
			return nil
		}

		ctx.Conn.WriteOK(ctx.Tag, "NOTIFY completed")
		return nil

	case "SET":
		// Parse notification specifications
		var specs []NotifySpec

		for {
			if err := ctx.Decoder.ReadSP(); err != nil {
				break
			}

			// Read mailbox filter (atom or quoted string)
			mailbox, err := ctx.Decoder.ReadAString()
			if err != nil {
				break
			}

			spec := NotifySpec{Mailbox: mailbox}

			// Try to read events list in parentheses
			if err := ctx.Decoder.ReadSP(); err == nil {
				b, err := ctx.Decoder.PeekByte()
				if err == nil && b == '(' {
					// Read events list
					var events []NotifyEvent
					listErr := ctx.Decoder.ReadList(func() error {
						eventType, err := ctx.Decoder.ReadAtom()
						if err != nil {
							return err
						}
						events = append(events, NotifyEvent{EventType: eventType})
						return nil
					})
					if listErr != nil {
						ctx.Conn.WriteBAD(ctx.Tag, "invalid event list")
						return nil
					}
					spec.Events = events
				}
			}

			specs = append(specs, spec)
		}

		if len(specs) == 0 {
			ctx.Conn.WriteBAD(ctx.Tag, "missing notification specifications")
			return nil
		}

		if err := sess.Notify(specs); err != nil {
			ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("NOTIFY SET failed: %v", err))
			return nil
		}

		ctx.Conn.WriteOK(ctx.Tag, "NOTIFY completed")
		return nil

	default:
		ctx.Conn.WriteBAD(ctx.Tag, fmt.Sprintf("unknown NOTIFY subcommand: %s", subcommand))
		return nil
	}
}
