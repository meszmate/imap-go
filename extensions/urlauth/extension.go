// Package urlauth implements the URLAUTH IMAP extension (RFC 4467).
//
// URLAUTH provides URL-based message access authorization. It allows
// clients to generate authorized URLs for accessing message content
// and to fetch content using those URLs.
package urlauth

import (
	"fmt"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// URLAuthRequest represents a request to generate an authorized URL.
type URLAuthRequest struct {
	// URL is the IMAP URL to authorize.
	URL string
	// Mechanism is the authorization mechanism to use.
	Mechanism string
}

// URLFetchResponse represents the result of fetching a URL.
type URLFetchResponse struct {
	// URL is the IMAP URL that was fetched.
	URL string
	// Data is the content retrieved from the URL.
	Data []byte
}

// SessionURLAuth is an optional interface for sessions that support
// the URLAUTH commands.
type SessionURLAuth interface {
	// GenURLAuth generates authorized URLs for the given requests.
	GenURLAuth(urls []URLAuthRequest) ([]string, error)

	// ResetKey resets the URL authorization key for the given mailbox
	// and mechanisms.
	ResetKey(mailbox string, mechanisms []string) error

	// URLFetch fetches the content at the given authorized URLs.
	URLFetch(urls []string) ([]URLFetchResponse, error)
}

// Extension implements the URLAUTH IMAP extension (RFC 4467).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new URLAUTH extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "URLAUTH",
			ExtCapabilities: []imap.Cap{imap.CapURLAuth},
		},
	}
}

// CommandHandlers returns the URLAUTH command handlers.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		imap.CommandGenURLAuth: server.CommandHandlerFunc(handleGenURLAuth),
		imap.CommandResetKey:   server.CommandHandlerFunc(handleResetKey),
		imap.CommandURLFetch:   server.CommandHandlerFunc(handleURLFetch),
	}
}

func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionURLAuth interface that sessions
// must implement to support the URLAUTH commands.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionURLAuth)(nil)
}

func (e *Extension) OnEnabled(connID string) error { return nil }

// handleGenURLAuth handles the GENURLAUTH command.
//
// Command syntax: GENURLAUTH url mechanism [url mechanism ...]
// Response:       * GENURLAUTH url1 url2 ...
func handleGenURLAuth(ctx *server.CommandContext) error {
	state := ctx.Conn.State()
	if state != imap.ConnStateAuthenticated && state != imap.ConnStateSelected {
		ctx.Conn.WriteBAD(ctx.Tag, "GENURLAUTH not allowed in current state")
		return nil
	}

	sess, ok := ctx.Session.(SessionURLAuth)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "GENURLAUTH not supported")
		return nil
	}

	if ctx.Decoder == nil {
		ctx.Conn.WriteBAD(ctx.Tag, "missing arguments")
		return nil
	}

	// Read url/mechanism pairs
	var requests []URLAuthRequest
	for {
		url, err := ctx.Decoder.ReadAString()
		if err != nil {
			break
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "expected mechanism after URL")
			return nil
		}

		mechanism, err := ctx.Decoder.ReadAtom()
		if err != nil {
			ctx.Conn.WriteBAD(ctx.Tag, "invalid mechanism")
			return nil
		}

		requests = append(requests, URLAuthRequest{URL: url, Mechanism: mechanism})

		if err := ctx.Decoder.ReadSP(); err != nil {
			break
		}
	}

	if len(requests) == 0 {
		ctx.Conn.WriteBAD(ctx.Tag, "missing URL and mechanism arguments")
		return nil
	}

	urls, err := sess.GenURLAuth(requests)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("GENURLAUTH failed: %v", err))
		return nil
	}

	// Write GENURLAUTH response: * GENURLAUTH url1 url2 ...
	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("GENURLAUTH")
		for _, u := range urls {
			enc.SP().AString(u)
		}
		enc.CRLF()
	})

	ctx.Conn.WriteOK(ctx.Tag, "GENURLAUTH completed")
	return nil
}

// handleResetKey handles the RESETKEY command.
//
// Command syntax: RESETKEY [mailbox [mechanism ...]]
func handleResetKey(ctx *server.CommandContext) error {
	state := ctx.Conn.State()
	if state != imap.ConnStateAuthenticated && state != imap.ConnStateSelected {
		ctx.Conn.WriteBAD(ctx.Tag, "RESETKEY not allowed in current state")
		return nil
	}

	sess, ok := ctx.Session.(SessionURLAuth)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "RESETKEY not supported")
		return nil
	}

	var mailbox string
	var mechanisms []string

	if ctx.Decoder != nil {
		mb, err := ctx.Decoder.ReadAString()
		if err == nil {
			mailbox = mb

			// Read optional mechanism list
			for {
				if err := ctx.Decoder.ReadSP(); err != nil {
					break
				}
				mech, err := ctx.Decoder.ReadAtom()
				if err != nil {
					break
				}
				mechanisms = append(mechanisms, mech)
			}
		}
	}

	if err := sess.ResetKey(mailbox, mechanisms); err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("RESETKEY failed: %v", err))
		return nil
	}

	ctx.Conn.WriteOK(ctx.Tag, "RESETKEY completed")
	return nil
}

// handleURLFetch handles the URLFETCH command.
//
// Command syntax: URLFETCH url [url ...]
// Response:       * URLFETCH url data [url data ...]
func handleURLFetch(ctx *server.CommandContext) error {
	state := ctx.Conn.State()
	if state != imap.ConnStateAuthenticated && state != imap.ConnStateSelected {
		ctx.Conn.WriteBAD(ctx.Tag, "URLFETCH not allowed in current state")
		return nil
	}

	sess, ok := ctx.Session.(SessionURLAuth)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "URLFETCH not supported")
		return nil
	}

	if ctx.Decoder == nil {
		ctx.Conn.WriteBAD(ctx.Tag, "missing URL arguments")
		return nil
	}

	// Read URL list
	var urls []string
	for {
		url, err := ctx.Decoder.ReadAString()
		if err != nil {
			break
		}
		urls = append(urls, url)

		if err := ctx.Decoder.ReadSP(); err != nil {
			break
		}
	}

	if len(urls) == 0 {
		ctx.Conn.WriteBAD(ctx.Tag, "missing URL arguments")
		return nil
	}

	responses, err := sess.URLFetch(urls)
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("URLFETCH failed: %v", err))
		return nil
	}

	// Write URLFETCH response: * URLFETCH url data [url data ...]
	for _, resp := range responses {
		ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
			enc.Star().Atom("URLFETCH").SP().AString(resp.URL).SP()
			if resp.Data != nil {
				enc.Literal(resp.Data)
			} else {
				enc.Nil()
			}
			enc.CRLF()
		})
	}

	ctx.Conn.WriteOK(ctx.Tag, "URLFETCH completed")
	return nil
}
