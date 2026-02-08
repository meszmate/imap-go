// Package thread implements the THREAD extension (RFC 5256).
//
// THREAD adds the THREAD command which returns message threading information.
// Two algorithms are supported: ORDEREDSUBJECT (simple subject-based threading)
// and REFERENCES (using the References and In-Reply-To headers).
package thread

import (
	"fmt"
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Extension implements the THREAD extension (RFC 5256).
type Extension struct {
	extension.BaseExtension
}

// New creates a new THREAD extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName: "THREAD",
			ExtCapabilities: []imap.Cap{
				imap.CapThreadOrderedSubject,
				imap.CapThreadReferences,
			},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		"THREAD": server.CommandHandlerFunc(handleThread),
	}
}

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns the required session extension interface, or nil.
func (e *Extension) SessionExtension() interface{} {
	return (*server.SessionThread)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// handleThread handles the THREAD command.
//
// Command syntax: THREAD algorithm charset search-criteria
// Response:       * THREAD (thread1)(thread2)...
func handleThread(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(server.SessionThread)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "THREAD not supported")
		return nil
	}

	dec := ctx.Decoder

	// Read threading algorithm
	algoStr, err := dec.ReadAtom()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected threading algorithm")
		return nil
	}

	algorithm := imap.ThreadAlgorithm(strings.ToUpper(algoStr))
	switch algorithm {
	case imap.ThreadAlgorithmOrderedSubject, imap.ThreadAlgorithmReferences:
		// valid
	default:
		ctx.Conn.WriteBAD(ctx.Tag, fmt.Sprintf("Unknown threading algorithm: %s", algoStr))
		return nil
	}

	// Read SP and charset
	if err := dec.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected charset")
		return nil
	}
	_, err = dec.ReadAtom()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Invalid charset")
		return nil
	}

	// The remaining arguments are search criteria. For now, use a basic
	// ALL search criteria since full search criteria parsing is complex.
	searchCriteria := &imap.SearchCriteria{}

	data, err := sess.Thread(ctx.NumKind, algorithm, searchCriteria, &imap.SearchOptions{})
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("THREAD failed: %v", err))
		return nil
	}

	// Write untagged THREAD response: * THREAD (thread1)(thread2)...
	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("THREAD")
		for _, t := range data.Threads {
			enc.SP()
			writeThread(enc, &t)
		}
		enc.CRLF()
	})

	ctx.Conn.WriteOK(ctx.Tag, "THREAD completed")
	return nil
}

// writeThread recursively writes a thread structure.
// Thread format: (num (child1)(child2)...)
func writeThread(enc *wire.Encoder, t *imap.Thread) {
	enc.BeginList().Number(t.Num)
	for i := range t.Children {
		enc.SP()
		writeThread(enc, &t.Children[i])
	}
	enc.EndList()
}
