// Package sort implements the SORT extension (RFC 5256).
//
// SORT adds the SORT command which returns message sequence numbers or UIDs
// in a specified sort order. Unlike client-side sorting, the server performs
// the sort based on criteria such as date, subject, from, size, etc.
package sort

import (
	"fmt"
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Extension implements the SORT extension (RFC 5256).
type Extension struct {
	extension.BaseExtension
}

// New creates a new SORT extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "SORT",
			ExtCapabilities: []imap.Cap{imap.CapSort},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		"SORT": server.CommandHandlerFunc(handleSort),
	}
}

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	return nil
}

// SessionExtension returns the required session extension interface, or nil.
func (e *Extension) SessionExtension() interface{} {
	return (*server.SessionSort)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error {
	return nil
}

// handleSort handles the SORT command.
//
// Command syntax: SORT (sort-criteria) charset search-criteria
// Response:       * SORT num1 num2 ...
func handleSort(ctx *server.CommandContext) error {
	sess, ok := ctx.Session.(server.SessionSort)
	if !ok {
		ctx.Conn.WriteNO(ctx.Tag, "SORT not supported")
		return nil
	}

	dec := ctx.Decoder

	// Parse sort criteria list: (REVERSE? sort-key)+
	var criteria []imap.SortCriterion
	if err := dec.ReadList(func() error {
		atom, err := dec.ReadAtom()
		if err != nil {
			return err
		}

		var criterion imap.SortCriterion
		upper := strings.ToUpper(atom)
		if upper == "REVERSE" {
			criterion.Reverse = true
			if err := dec.ReadSP(); err != nil {
				return err
			}
			keyAtom, err := dec.ReadAtom()
			if err != nil {
				return err
			}
			criterion.Key = imap.SortKey(strings.ToUpper(keyAtom))
		} else {
			criterion.Key = imap.SortKey(upper)
		}
		criteria = append(criteria, criterion)
		return nil
	}); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Invalid sort criteria")
		return nil
	}

	if len(criteria) == 0 {
		ctx.Conn.WriteBAD(ctx.Tag, "Empty sort criteria")
		return nil
	}

	// Read SP and charset
	if err := dec.ReadSP(); err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Expected charset")
		return nil
	}
	_, err := dec.ReadAtom()
	if err != nil {
		ctx.Conn.WriteBAD(ctx.Tag, "Invalid charset")
		return nil
	}

	// The remaining arguments are search criteria. For now, use a basic
	// ALL search criteria since full search criteria parsing is complex.
	searchCriteria := &imap.SearchCriteria{}

	data, err := sess.Sort(ctx.NumKind, criteria, searchCriteria, &imap.SearchOptions{})
	if err != nil {
		ctx.Conn.WriteNO(ctx.Tag, fmt.Sprintf("SORT failed: %v", err))
		return nil
	}

	// Write untagged SORT response: * SORT num1 num2 ...
	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.Star().Atom("SORT")
		for _, num := range data.AllNums {
			enc.SP().Number(num)
		}
		enc.CRLF()
	})

	ctx.Conn.WriteOK(ctx.Tag, "SORT completed")
	return nil
}
