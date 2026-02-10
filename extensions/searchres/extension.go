// Package searchres implements the SEARCHRES extension (RFC 5182).
//
// SEARCHRES provides the ability to save search results and reference
// them later using the $ marker in subsequent commands. When a client
// sends SEARCH RETURN (SAVE ...) criteria, the server saves the result
// set. The $ marker can then replace a sequence/UID set in FETCH, STORE,
// COPY, MOVE, or as a criterion in SEARCH.
package searchres

import (
	"fmt"
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/extensions/condstore"
	"github.com/meszmate/imap-go/extensions/esearch"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionSearchRes is the session interface for SEARCHRES support.
// Implementations manage saved search result sets that can be
// referenced by subsequent commands using the $ marker.
type SessionSearchRes interface {
	SaveSearchResult(data *imap.SearchData) error
	GetSearchResult() (*imap.SeqSet, error)
}

// Extension implements the SEARCHRES IMAP extension (RFC 5182).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new SEARCHRES extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "SEARCHRES",
			ExtCapabilities: []imap.Cap{imap.CapSearchRes},
			ExtDependencies: []string{"ESEARCH"},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} { return nil }

// WrapHandler wraps an existing command handler to add SEARCHRES support.
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
			return handleSearchRes(ctx, h)
		})
	case "FETCH":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleDollarFetch(ctx, h)
		})
	case "STORE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleDollarStore(ctx, h)
		})
	case "COPY":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleDollarCopy(ctx, h)
		})
	case "MOVE":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleDollarMove(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionSearchRes)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }

// handleSearchRes wraps the SEARCH command to support $ in criteria and RETURN (SAVE).
func handleSearchRes(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing search criteria")
	}

	dec := ctx.Decoder
	criteria := &imap.SearchCriteria{}
	options := &imap.SearchOptions{}
	hasReturn := false

	// Peek to check if first token is "RETURN"
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
		if err := parseSearchCriteriaWithDollar(ctx, dec, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
	} else {
		// First atom is a search criterion, not RETURN
		if err := parseSearchCriterionWithDollar(ctx, first, dec, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
		// Try to read more criteria
		if err := dec.ReadSP(); err == nil {
			if err := parseSearchCriteriaWithDollar(ctx, dec, criteria); err != nil {
				return imap.ErrBad("invalid search criteria: " + err.Error())
			}
		}
	}

	// Route to session
	var data *imap.SearchData
	if hasReturn {
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

	// If SAVE was requested and session supports it, save the result
	if options.ReturnSave {
		if sess, ok := ctx.Session.(SessionSearchRes); ok {
			if err := sess.SaveSearchResult(data); err != nil {
				return err
			}
		}
	}

	// Write response
	enc := ctx.Conn.Encoder()
	if hasReturn && hasAnyReturnOption(options) {
		writeESearchResponse(enc, ctx, data, options)
	} else {
		// Traditional SEARCH response
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

// parseSearchCriteriaWithDollar reads search criteria with $ support.
func parseSearchCriteriaWithDollar(ctx *server.CommandContext, dec *wire.Decoder, criteria *imap.SearchCriteria) error {
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

		if err := parseSearchCriterionWithDollar(ctx, key, dec, criteria); err != nil {
			return err
		}

		if err := dec.ReadSP(); err != nil {
			return nil
		}
	}
}

// parseSearchCriterionWithDollar handles a single criterion key, intercepting $.
func parseSearchCriterionWithDollar(ctx *server.CommandContext, key string, dec *wire.Decoder, criteria *imap.SearchCriteria) error {
	if key == "$" {
		sess, ok := ctx.Session.(SessionSearchRes)
		if !ok {
			return fmt.Errorf("no saved search result")
		}
		savedSet, err := sess.GetSearchResult()
		if err != nil {
			return err
		}
		if savedSet != nil {
			criteria.SeqNum = savedSet
		}
		return nil
	}
	return esearch.ParseSearchCriterion(key, dec, criteria)
}

// resolveDollar resolves the $ marker to a saved search result set.
func resolveDollar(ctx *server.CommandContext) (imap.NumSet, error) {
	sess, ok := ctx.Session.(SessionSearchRes)
	if !ok {
		return nil, imap.ErrBad("no saved search result")
	}
	savedSet, err := sess.GetSearchResult()
	if err != nil {
		return nil, err
	}
	if savedSet == nil || len(savedSet.Set) == 0 {
		return nil, imap.ErrBad("no saved search result")
	}
	if ctx.NumKind == server.NumKindUID {
		return &imap.UIDSet{Set: savedSet.Set}, nil
	}
	return savedSet, nil
}

// isDollarCommand checks if the decoder's next byte is '$' and peeks to
// confirm it's followed by SP (i.e., it's a standalone "$" token).
func isDollarCommand(dec *wire.Decoder) bool {
	if dec == nil {
		return false
	}
	b, err := dec.PeekByte()
	if err != nil {
		return false
	}
	return b == '$'
}

// handleDollarFetch wraps FETCH to support $ as the sequence set.
func handleDollarFetch(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if !isDollarCommand(ctx.Decoder) {
		return original(ctx)
	}

	dec := ctx.Decoder

	// Read "$"
	atom, err := dec.ReadAtom()
	if err != nil || atom != "$" {
		return original(ctx)
	}

	numSet, err := resolveDollar(ctx)
	if err != nil {
		return err
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing fetch items")
	}

	// Parse fetch items using condstore's exported parser
	options, err := condstore.ParseFetchItems(dec)
	if err != nil {
		return imap.ErrBad("invalid fetch items: " + err.Error())
	}

	// Check for (CHANGEDSINCE <modseq>) modifier after fetch items
	if err := dec.ReadSP(); err == nil {
		b, err := dec.PeekByte()
		if err == nil && b == '(' {
			if err := dec.ExpectByte('('); err != nil {
				return imap.ErrBad("invalid modifier")
			}
			modAtom, err := dec.ReadAtom()
			if err != nil {
				return imap.ErrBad("invalid modifier name")
			}
			if !strings.EqualFold(modAtom, "CHANGEDSINCE") {
				return imap.ErrBad("unknown fetch modifier: " + modAtom)
			}
			if err := dec.ReadSP(); err != nil {
				return imap.ErrBad("missing CHANGEDSINCE value")
			}
			modseq, err := dec.ReadNumber64()
			if err != nil {
				return imap.ErrBad("invalid CHANGEDSINCE value")
			}
			if err := dec.ExpectByte(')'); err != nil {
				return imap.ErrBad("missing closing paren for modifier")
			}
			options.ChangedSince = modseq
			options.ModSeq = true
		}
	}

	if ctx.NumKind == server.NumKindUID {
		options.UID = true
	}

	w := server.NewFetchWriter(ctx.Conn.Encoder())
	if err := ctx.Session.Fetch(w, numSet, options); err != nil {
		return err
	}

	ctx.Conn.WriteOK(ctx.Tag, "FETCH completed")
	return nil
}

// handleDollarStore wraps STORE to support $ as the sequence set.
func handleDollarStore(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if !isDollarCommand(ctx.Decoder) {
		return original(ctx)
	}

	dec := ctx.Decoder

	// Read "$"
	atom, err := dec.ReadAtom()
	if err != nil || atom != "$" {
		return original(ctx)
	}

	numSet, err := resolveDollar(ctx)
	if err != nil {
		return err
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing store action")
	}

	// Check for (UNCHANGEDSINCE <modseq>) modifier
	storeOptions := &imap.StoreOptions{}
	b, err := dec.PeekByte()
	if err != nil {
		return imap.ErrBad("unexpected end of command")
	}

	if b == '(' {
		if err := dec.ExpectByte('('); err != nil {
			return imap.ErrBad("invalid modifier")
		}
		modAtom, err := dec.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid modifier name")
		}
		if !strings.EqualFold(modAtom, "UNCHANGEDSINCE") {
			return imap.ErrBad("unknown store modifier: " + modAtom)
		}
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing UNCHANGEDSINCE value")
		}
		modseq, err := dec.ReadNumber64()
		if err != nil {
			return imap.ErrBad("invalid UNCHANGEDSINCE value")
		}
		if err := dec.ExpectByte(')'); err != nil {
			return imap.ErrBad("missing closing paren for modifier")
		}
		storeOptions.UnchangedSince = modseq

		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing store action")
		}
	}

	// Read store action
	actionStr, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("invalid store action")
	}

	storeFlags := &imap.StoreFlags{}
	upper := strings.ToUpper(actionStr)

	switch {
	case strings.HasPrefix(upper, "+FLAGS"):
		storeFlags.Action = imap.StoreFlagsAdd
	case strings.HasPrefix(upper, "-FLAGS"):
		storeFlags.Action = imap.StoreFlagsDel
	case strings.HasPrefix(upper, "FLAGS"):
		storeFlags.Action = imap.StoreFlagsSet
	default:
		return imap.ErrBad("invalid store action: " + actionStr)
	}

	if strings.HasSuffix(upper, ".SILENT") {
		storeFlags.Silent = true
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing flags")
	}

	flagStrs, err := dec.ReadFlags()
	if err != nil {
		return imap.ErrBad("invalid flags")
	}

	for _, f := range flagStrs {
		storeFlags.Flags = append(storeFlags.Flags, imap.Flag(f))
	}

	w := server.NewFetchWriter(ctx.Conn.Encoder())
	if err := ctx.Session.Store(w, numSet, storeFlags, storeOptions); err != nil {
		return err
	}

	ctx.Conn.WriteOK(ctx.Tag, "STORE completed")
	return nil
}

// handleDollarCopy wraps COPY to support $ as the sequence set.
func handleDollarCopy(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if !isDollarCommand(ctx.Decoder) {
		return original(ctx)
	}

	dec := ctx.Decoder

	// Read "$"
	atom, err := dec.ReadAtom()
	if err != nil || atom != "$" {
		return original(ctx)
	}

	numSet, err := resolveDollar(ctx)
	if err != nil {
		return err
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing destination mailbox")
	}

	dest, err := dec.ReadAString()
	if err != nil {
		return imap.ErrBad("invalid destination mailbox")
	}

	data, err := ctx.Session.Copy(numSet, dest)
	if err != nil {
		return err
	}

	if data != nil && data.UIDValidity > 0 {
		enc := ctx.Conn.Encoder()
		enc.Encode(func(e *wire.Encoder) {
			code := fmt.Sprintf("COPYUID %d %s %s",
				data.UIDValidity,
				data.SourceUIDs.String(),
				data.DestUIDs.String())
			e.StatusResponse(ctx.Tag, "OK", code, "COPY completed")
		})
	} else {
		ctx.Conn.WriteOK(ctx.Tag, "COPY completed")
	}

	return nil
}

// handleDollarMove wraps MOVE to support $ as the sequence set.
func handleDollarMove(ctx *server.CommandContext, original server.CommandHandlerFunc) error {
	if !isDollarCommand(ctx.Decoder) {
		return original(ctx)
	}

	dec := ctx.Decoder

	// Read "$"
	atom, err := dec.ReadAtom()
	if err != nil || atom != "$" {
		return original(ctx)
	}

	numSet, err := resolveDollar(ctx)
	if err != nil {
		return err
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing destination mailbox")
	}

	dest, err := dec.ReadAString()
	if err != nil {
		return imap.ErrBad("invalid destination mailbox")
	}

	sessMove, ok := ctx.Session.(server.SessionMove)
	if !ok {
		return imap.ErrNo("MOVE not supported")
	}

	w := server.NewMoveWriter(ctx.Conn.Encoder())
	if err := sessMove.Move(w, numSet, dest); err != nil {
		return err
	}

	ctx.Conn.Encoder().Encode(func(enc *wire.Encoder) {
		enc.StatusResponse(ctx.Tag, "OK", "", "MOVE completed")
	})

	return nil
}

// parseReturnOptions parses a parenthesized list of RETURN options.
// This is a local copy of esearch's private parseReturnOptions.
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
// This is a local copy of esearch's private writeESearchResponse.
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
