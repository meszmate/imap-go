// Package partial implements the PARTIAL extension (RFC 9394).
//
// PARTIAL adds support for requesting partial search and sort results,
// allowing clients to paginate through large result sets. The RETURN
// (PARTIAL offset:count) option returns a subset of matching UIDs in
// an ESEARCH response format. Negative offsets count from the end.
package partial

import (
	"fmt"
	"strconv"
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/extensions/esearch"
	"github.com/meszmate/imap-go/extensions/esort"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionPartial is the session interface for PARTIAL support.
// Implementations provide paginated search results via the PARTIAL
// return option.
type SessionPartial interface {
	SearchPartial(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
}

// Extension implements the PARTIAL IMAP extension (RFC 9394).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new PARTIAL extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "PARTIAL",
			ExtCapabilities: []imap.Cap{imap.CapPartial},
			ExtDependencies: []string{"ESEARCH"},
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
			return handlePartialSearch(ctx, h)
		})
	case "SORT":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handlePartialSort(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionPartial)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }

// handlePartialSearch wraps the SEARCH command to parse RETURN options with PARTIAL support.
func handlePartialSearch(ctx *server.CommandContext, _ server.CommandHandlerFunc) error {
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
		if err := esearch.ParseSearchCriteria(dec, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
	} else {
		if err := esearch.ParseSearchCriterion(first, dec, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
		if err := dec.ReadSP(); err == nil {
			if err := esearch.ParseSearchCriteria(dec, criteria); err != nil {
				return imap.ErrBad("invalid search criteria: " + err.Error())
			}
		}
	}

	// Route to session
	var data *imap.SearchData
	if hasReturn {
		if options.ReturnPartial != nil {
			if sess, ok := ctx.Session.(SessionPartial); ok {
				data, err = sess.SearchPartial(ctx.NumKind, criteria, options)
			} else if sess, ok := ctx.Session.(esearch.SessionESearch); ok {
				data, err = sess.SearchExtended(ctx.NumKind, criteria, options)
			} else {
				data, err = ctx.Session.Search(ctx.NumKind, criteria, options)
			}
		} else {
			if sess, ok := ctx.Session.(esearch.SessionESearch); ok {
				data, err = sess.SearchExtended(ctx.NumKind, criteria, options)
			} else {
				data, err = ctx.Session.Search(ctx.NumKind, criteria, options)
			}
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

// handlePartialSort wraps the SORT command to add RETURN (PARTIAL) support.
func handlePartialSort(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return original(ctx)
	}

	dec := ctx.Decoder

	// Peek first byte: if '(' this is a standard SORT with no RETURN
	b, err := dec.PeekByte()
	if err != nil {
		return original(ctx)
	}
	if b == '(' {
		return original(ctx)
	}

	// Read atom, expect "RETURN"
	atom, err := dec.ReadAtom()
	if err != nil {
		return original(ctx)
	}
	if !strings.EqualFold(atom, "RETURN") {
		return imap.ErrBad("expected RETURN or sort criteria list")
	}

	// Parse return options with PARTIAL support
	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing RETURN options")
	}
	options := &imap.SearchOptions{}
	if err := parseReturnOptions(dec, options); err != nil {
		return err
	}

	// Read SP then sort criteria list
	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing sort criteria")
	}
	sortCriteria, err := parseSortCriteria(dec)
	if err != nil {
		return imap.ErrBad("invalid sort criteria: " + err.Error())
	}
	if len(sortCriteria) == 0 {
		return imap.ErrBad("empty sort criteria")
	}

	// Read SP then charset
	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("expected charset")
	}
	if _, err := dec.ReadAtom(); err != nil {
		return imap.ErrBad("invalid charset")
	}

	// Read SP then search criteria
	searchCriteria := &imap.SearchCriteria{}
	if err := dec.ReadSP(); err == nil {
		if err := esearch.ParseSearchCriteria(dec, searchCriteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
	}

	// Route to session — requires SessionESort for ESEARCH-style response
	sess, ok := ctx.Session.(esort.SessionESort)
	if !ok {
		return imap.ErrNo("SORT with RETURN not supported")
	}

	data, err := sess.SortExtended(ctx.NumKind, sortCriteria, searchCriteria, options)
	if err != nil {
		return err
	}

	// Write ESEARCH response
	writeESearchResponse(ctx.Conn.Encoder(), ctx, data, options)

	ctx.Conn.WriteOK(ctx.Tag, "SORT completed")
	return nil
}

// parseReturnOptions parses a parenthesized list of RETURN options with PARTIAL support.
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
		case "PARTIAL":
			if err := dec.ReadSP(); err != nil {
				return imap.ErrBad("missing PARTIAL range")
			}
			rangeAtom, err := dec.ReadAtom()
			if err != nil {
				return imap.ErrBad("invalid PARTIAL range")
			}
			offset, count, err := parsePartialRange(rangeAtom)
			if err != nil {
				return imap.ErrBad("invalid PARTIAL range: " + err.Error())
			}
			options.ReturnPartial = &imap.SearchReturnPartial{Offset: offset, Count: count}
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

// parsePartialRange parses a PARTIAL range like "1:100" or "-1:100".
func parsePartialRange(s string) (int32, uint32, error) {
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return 0, 0, fmt.Errorf("missing ':' separator")
	}
	offsetStr := s[:idx]
	countStr := s[idx+1:]

	offset64, err := strconv.ParseInt(offsetStr, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid offset: %s", offsetStr)
	}
	if offset64 == 0 {
		return 0, 0, fmt.Errorf("offset must not be zero")
	}

	count64, err := strconv.ParseUint(countStr, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid count: %s", countStr)
	}
	if count64 == 0 {
		return 0, 0, fmt.Errorf("count must be positive")
	}

	return int32(offset64), uint32(count64), nil
}

// hasAnyReturnOption returns true if any RETURN option is set.
func hasAnyReturnOption(options *imap.SearchOptions) bool {
	return options.ReturnMin || options.ReturnMax || options.ReturnAll ||
		options.ReturnCount || options.ReturnSave || options.ReturnPartial != nil
}

// writeESearchResponse writes an ESEARCH untagged response with PARTIAL support.
func writeESearchResponse(enc *server.ResponseEncoder, ctx *server.CommandContext, data *imap.SearchData, options *imap.SearchOptions) {
	enc.Encode(func(e *wire.Encoder) {
		e.Star().Atom("ESEARCH").SP()
		// TAG correlator
		e.BeginList().Atom("TAG").SP().QuotedString(ctx.Tag).EndList()
		// UID flag
		if ctx.NumKind == server.NumKindUID {
			e.SP().Atom("UID")
		}
		// Result items — only when there are matches
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
		// PARTIAL item
		if options.ReturnPartial != nil {
			e.SP().Atom("PARTIAL").SP().BeginList()
			e.Atom(fmt.Sprintf("%d:%d", options.ReturnPartial.Offset, options.ReturnPartial.Count))
			if data.Partial != nil {
				e.SP().Number(data.Partial.Total)
				if len(data.Partial.UIDs) > 0 {
					e.SP()
					uidSet := &imap.UIDSet{}
					uidSet.AddNum(data.Partial.UIDs...)
					e.Atom(uidSet.String())
				}
			} else {
				e.SP().Number(0)
			}
			e.EndList()
		}
		// MODSEQ emitted regardless of RETURN when present
		if data.ModSeq > 0 {
			e.SP().Atom("MODSEQ").SP().Number64(data.ModSeq)
		}
		e.CRLF()
	})
}

// parseSortCriteria reads a parenthesized list of sort criteria.
func parseSortCriteria(dec *wire.Decoder) ([]imap.SortCriterion, error) {
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
		return nil, err
	}
	return criteria, nil
}
