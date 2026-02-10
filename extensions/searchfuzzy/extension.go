// Package searchfuzzy implements the SEARCH=FUZZY extension (RFC 6203).
//
// SEARCH=FUZZY adds support for fuzzy matching in SEARCH commands.
// When enabled, the server may use approximate matching for search
// criteria. The FUZZY keyword can prefix individual search criteria
// to indicate that fuzzy matching is acceptable.
package searchfuzzy

import (
	"fmt"
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/extensions/esearch"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionSearchFuzzy is the session interface for SEARCH=FUZZY support.
// Implementations provide fuzzy (approximate) matching for search criteria.
type SessionSearchFuzzy interface {
	SearchFuzzy(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
}

// Extension implements the SEARCH=FUZZY IMAP extension (RFC 6203).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new SEARCH=FUZZY extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "SEARCH=FUZZY",
			ExtCapabilities: []imap.Cap{imap.CapSearchFuzzy},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps an existing command handler.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} {
	h, ok := handler.(server.CommandHandlerFunc)
	if !ok {
		ch, ok2 := handler.(server.CommandHandler)
		if !ok2 {
			return nil
		}
		h = ch.Handle
	}

	switch name {
	case "SEARCH":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleFuzzySearch(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionSearchFuzzy)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }

// handleFuzzySearch wraps the SEARCH command to parse FUZZY modifiers.
func handleFuzzySearch(ctx *server.CommandContext, _ server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing search criteria")
	}

	dec := ctx.Decoder
	criteria := &imap.SearchCriteria{}
	options := &imap.SearchOptions{}
	hasReturn := false

	first, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("missing search criteria")
	}

	if strings.EqualFold(first, "RETURN") {
		hasReturn = true
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing RETURN options")
		}
		if err := parseReturnOptions(dec, options); err != nil {
			return err
		}
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing search criteria after RETURN")
		}
		if err := parseFuzzySearchCriteria(dec, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
	} else {
		if err := parseFuzzySearchCriterion(first, dec, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
		if err := dec.ReadSP(); err == nil {
			if err := parseFuzzySearchCriteria(dec, criteria); err != nil {
				return imap.ErrBad("invalid search criteria: " + err.Error())
			}
		}
	}

	// Route to session
	var data *imap.SearchData
	if criteria.Fuzzy {
		if sess, ok := ctx.Session.(SessionSearchFuzzy); ok {
			data, err = sess.SearchFuzzy(ctx.NumKind, criteria, options)
		} else if hasReturn {
			if sess, ok := ctx.Session.(esearch.SessionESearch); ok {
				data, err = sess.SearchExtended(ctx.NumKind, criteria, options)
			} else {
				data, err = ctx.Session.Search(ctx.NumKind, criteria, options)
			}
		} else {
			data, err = ctx.Session.Search(ctx.NumKind, criteria, options)
		}
	} else if hasReturn {
		if sess, ok := ctx.Session.(esearch.SessionESearch); ok {
			data, err = sess.SearchExtended(ctx.NumKind, criteria, options)
		} else {
			data, err = ctx.Session.Search(ctx.NumKind, criteria, options)
		}
	} else {
		data, err = ctx.Session.Search(ctx.NumKind, criteria, options)
	}
	if err != nil {
		return err
	}

	// Write response
	enc := ctx.Conn.Encoder()
	if hasReturn && hasAnyReturnOption(options) {
		writeESearchResponse(enc, ctx, data, options)
	} else {
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("SEARCH")
			if ctx.NumKind == server.NumKindUID {
				for _, uid := range data.AllUIDs {
					e.SP().Number(uint32(uid))
				}
			} else {
				for _, num := range data.AllSeqNums {
					e.SP().Number(num)
				}
			}
			if data.ModSeq > 0 {
				e.SP().BeginList().Atom("MODSEQ").SP().Number64(data.ModSeq).EndList()
			}
			e.CRLF()
		})
	}

	ctx.Conn.WriteOK(ctx.Tag, "SEARCH completed")
	return nil
}

// parseFuzzySearchCriteria reads search criteria from the decoder, handling FUZZY modifiers.
func parseFuzzySearchCriteria(dec *wire.Decoder, criteria *imap.SearchCriteria) error {
	for {
		b, err := dec.PeekByte()
		if err != nil {
			return nil
		}
		if b == ')' {
			return nil
		}

		key, err := dec.ReadAtom()
		if err != nil {
			return nil
		}

		if err := parseFuzzySearchCriterion(key, dec, criteria); err != nil {
			return err
		}

		if err := dec.ReadSP(); err != nil {
			return nil
		}
	}
}

// parseFuzzySearchCriterion handles a single already-read criterion key, intercepting FUZZY.
func parseFuzzySearchCriterion(key string, dec *wire.Decoder, criteria *imap.SearchCriteria) error {
	if strings.EqualFold(key, "FUZZY") {
		criteria.Fuzzy = true
		if err := dec.ReadSP(); err != nil {
			return fmt.Errorf("expected search key after FUZZY")
		}
		next, err := dec.ReadAtom()
		if err != nil {
			return fmt.Errorf("expected search key after FUZZY")
		}
		return esearch.ParseSearchCriterion(next, dec, criteria)
	}
	return esearch.ParseSearchCriterion(key, dec, criteria)
}

// parseReturnOptions parses a parenthesized list of RETURN options.
func parseReturnOptions(dec *wire.Decoder, options *imap.SearchOptions) error {
	if err := dec.ExpectByte('('); err != nil {
		return imap.ErrBad("expected '(' for RETURN options")
	}

	b, err := dec.PeekByte()
	if err != nil {
		return imap.ErrBad("unexpected end in RETURN options")
	}
	if b == ')' {
		if err := dec.ExpectByte(')'); err != nil {
			return imap.ErrBad("expected ')' for RETURN options")
		}
		return nil
	}

	for {
		atom, err := dec.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid RETURN option")
		}
		switch strings.ToUpper(atom) {
		case "MIN":
			options.ReturnMin = true
		case "MAX":
			options.ReturnMax = true
		case "ALL":
			options.ReturnAll = true
		case "COUNT":
			options.ReturnCount = true
		case "SAVE":
			options.ReturnSave = true
		default:
			return imap.ErrBad("unknown RETURN option: " + atom)
		}

		b, err := dec.PeekByte()
		if err != nil {
			return imap.ErrBad("unexpected end in RETURN options")
		}
		if b == ')' {
			if err := dec.ExpectByte(')'); err != nil {
				return imap.ErrBad("expected ')' for RETURN options")
			}
			return nil
		}
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("expected SP between RETURN options")
		}
	}
}

// hasAnyReturnOption returns true if any RETURN option is set.
func hasAnyReturnOption(options *imap.SearchOptions) bool {
	return options.ReturnMin || options.ReturnMax || options.ReturnAll || options.ReturnCount || options.ReturnSave
}

// writeESearchResponse writes an ESEARCH untagged response.
func writeESearchResponse(enc *server.ResponseEncoder, ctx *server.CommandContext, data *imap.SearchData, options *imap.SearchOptions) {
	enc.Encode(func(e *wire.Encoder) {
		e.Star().Atom("ESEARCH").SP()
		e.BeginList().Atom("TAG").SP().QuotedString(ctx.Tag).EndList()
		if ctx.NumKind == server.NumKindUID {
			e.SP().Atom("UID")
		}
		hasResults := data.Min > 0 || data.Max > 0 || data.All != nil || data.Count > 0
		if hasResults {
			if options.ReturnMin && data.Min > 0 {
				e.SP().Atom("MIN").SP().Number(data.Min)
			}
			if options.ReturnMax && data.Max > 0 {
				e.SP().Atom("MAX").SP().Number(data.Max)
			}
			if options.ReturnAll && data.All != nil {
				e.SP().Atom("ALL").SP().Atom(data.All.String())
			}
			if options.ReturnCount {
				e.SP().Atom("COUNT").SP().Number(data.Count)
			}
		}
		if data.ModSeq > 0 {
			e.SP().Atom("MODSEQ").SP().Number64(data.ModSeq)
		}
		e.CRLF()
	})
}
