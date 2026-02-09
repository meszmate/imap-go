// Package language implements the LANGUAGE IMAP extension (RFC 5255).
//
// LANGUAGE allows the client to request that the server use a specific
// language for human-readable text in responses, or to query which
// languages the server supports.
package language

import (
	"fmt"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionLanguage is an optional interface for sessions that support
// the LANGUAGE command.
type SessionLanguage interface {
	// Language negotiates the language for the session.
	// If tags is empty, the server should return the list of available languages.
	// Returns the selected language tag, the list of available tags, and any error.
	Language(tags []string) (string, []string, error)
}

// Extension implements the LANGUAGE IMAP extension (RFC 5255).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new LANGUAGE extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "LANGUAGE",
			ExtCapabilities: []imap.Cap{imap.CapLanguage},
		},
	}
}

// CommandHandlers returns the LANGUAGE command handler.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandLanguage: server.CommandHandlerFunc(handleLanguage),
	}
}

func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionLanguage interface that sessions
// must implement to support the LANGUAGE command.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionLanguage)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleLanguage handles the LANGUAGE command.
func handleLanguage(ctx *server.CommandContext) error {
	state := ctx.Conn.State()
	if state != imap.ConnStateAuthenticated && state != imap.ConnStateSelected {
		ctx.Conn.WriteBAD(ctx.Tag, "LANGUAGE not allowed in current state")
		return nil
	}

	sess, ok := ctx.Session.(SessionLanguage)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "LANGUAGE not supported")
		return nil
	}

	// Read optional language tags
	var tags []string
	if ctx.Decoder != nil {
		for {
			tag, err := ctx.Decoder.ReadAtom()
			if err != nil {
				break
			}
			tags = append(tags, tag)

			if err := ctx.Decoder.ReadSP(); err != nil {
				break
			}
		}
	}

	selected, available, err := sess.Language(tags)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("LANGUAGE failed: %v", err))
		return nil
	}

	// Write available languages if returned
	if len(available) > 0 {
		ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
			enc.Star().Atom("LANGUAGE").SP().BeginList()
			for i, lang := range available {
				if i > 0 {
					enc.SP()
				}
				enc.AString(lang)
			}
			enc.EndList().CRLF()
		})
	}

	_ = selected

	ctx.Conn.WriteOK(ctx.Tag, "LANGUAGE completed")
	return nil
}
