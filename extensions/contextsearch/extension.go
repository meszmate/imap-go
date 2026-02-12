// Package contextsearch implements the CONTEXT=SEARCH extension (RFC 5267).
//
// CONTEXT=SEARCH adds the CONTEXT and UPDATE return options to SEARCH
// commands, allowing clients to maintain live search results that are
// automatically updated as the mailbox changes. It also adds the
// CANCELUPDATE command to stop receiving updates, and ADDTO/REMOVEFROM
// ESEARCH response data for incremental result updates.
package contextsearch

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/extensions/esearch"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionContext is the session interface for CONTEXT=SEARCH support.
// Implementations provide persistent search contexts that deliver
// UPDATE notifications when the result set changes.
type SessionContext interface {
	SearchContext(tag string, kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
	CancelSearchContext(tags []string) error
}

// Extension implements the CONTEXT=SEARCH IMAP extension (RFC 5267).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new CONTEXT=SEARCH extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "CONTEXT=SEARCH",
			ExtCapabilities: []imap.Cap{imap.CapContextSearch},
			ExtDependencies: []string{"ESEARCH"},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		"CANCELUPDATE": server.CommandHandlerFunc(handleCancelUpdate),
	}
}

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
			return handleContextSearch(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionContext)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }

// handleCancelUpdate handles the CANCELUPDATE command.
func handleCancelUpdate(ctx *server.CommandContext) error {
	if ctx.State() != imap.ConnStateSelected {
		return imap.ErrBad("CANCELUPDATE requires selected state")
	}

	dec := ctx.Decoder
	if dec == nil {
		return imap.ErrBad("missing CANCELUPDATE arguments")
	}

	var tags []string
	for {
		tag, err := dec.ReadQuotedString()
		if err != nil {
			if len(tags) == 0 {
				return imap.ErrBad("missing search context tag")
			}
			break
		}
		tags = append(tags, tag)
		if err := dec.ReadSP(); err != nil {
			break
		}
	}

	sess, ok := ctx.Session.(SessionContext)
	if !ok {
		return imap.ErrNo("CANCELUPDATE not supported")
	}

	if err := sess.CancelSearchContext(tags); err != nil {
		return err
	}

	ctx.Conn.WriteOK(ctx.Tag, "CANCELUPDATE completed")
	return nil
}

// handleContextSearch wraps the SEARCH command to support CONTEXT and UPDATE return options.
func handleContextSearch(ctx *server.CommandContext, originalHandler server.CommandHandlerFunc) error {
	if ctx.Decoder == nil {
		return imap.ErrBad("missing search criteria")
	}

	dec := ctx.Decoder
	criteria := &imap.SearchCriteria{}
	options := &imap.SearchOptions{}

	// Peek to check if first token is "RETURN"
	first, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("missing search criteria")
	}

	if !strings.EqualFold(first, "RETURN") {
		// No RETURN — delegate to original handler (ESEARCH's wrapper)
		// Parse the first criterion and remaining criteria, then route to session
		if err := esearch.ParseSearchCriterion(first, dec, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
		if err := dec.ReadSP(); err == nil {
			if err := esearch.ParseSearchCriteria(dec, criteria); err != nil {
				return imap.ErrBad("invalid search criteria: " + err.Error())
			}
		}

		data, err := ctx.Session.Search(ctx.NumKind, criteria, options)
		if err != nil {
			return err
		}

		writeTraditionalSearchResponse(ctx, data)
		ctx.Conn.WriteOK(ctx.Tag, "SEARCH completed")
		return nil
	}

	// Parse RETURN options
	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing RETURN options")
	}
	hasContext, hasUpdate, err := parseReturnOptions(dec, options)
	if err != nil {
		return err
	}

	// Parse search criteria
	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing search criteria after RETURN")
	}
	if err := esearch.ParseSearchCriteria(dec, criteria); err != nil {
		return imap.ErrBad("invalid search criteria: " + err.Error())
	}

	// Route to session
	var data *imap.SearchData
	if hasContext || hasUpdate {
		// Try SessionContext first
		if sess, ok := ctx.Session.(SessionContext); ok {
			data, err = sess.SearchContext(ctx.Tag, ctx.NumKind, criteria, options)
		} else {
			// No SessionContext support — send NOUPDATE and fall back
			writeNoUpdateResponse(ctx)
			options.ReturnContext = false
			options.ReturnUpdate = false
			if hasAnyStandardReturnOption(options) {
				if sess, ok := ctx.Session.(esearch.SessionESearch); ok {
					data, err = sess.SearchExtended(ctx.NumKind, criteria, options)
				} else {
					data, err = ctx.Session.Search(ctx.NumKind, criteria, options)
				}
			} else {
				data, err = ctx.Session.Search(ctx.NumKind, criteria, options)
			}
		}
	} else {
		// No CONTEXT/UPDATE — standard ESEARCH behavior
		if hasAnyStandardReturnOption(options) {
			if sess, ok := ctx.Session.(esearch.SessionESearch); ok {
				data, err = sess.SearchExtended(ctx.NumKind, criteria, options)
			} else {
				data, err = ctx.Session.Search(ctx.NumKind, criteria, options)
			}
		} else {
			data, err = ctx.Session.Search(ctx.NumKind, criteria, options)
		}
	}
	if err != nil {
		return err
	}

	// Write response
	enc := ctx.Conn.Encoder()
	if hasAnyReturnOption(options) {
		writeContextESearchResponse(enc, ctx, data, options)
	} else {
		writeTraditionalSearchResponse(ctx, data)
	}

	ctx.Conn.WriteOK(ctx.Tag, "SEARCH completed")
	return nil
}

// parseReturnOptions parses a parenthesized list of RETURN options including UPDATE and CONTEXT.
func parseReturnOptions(dec *wire.Decoder, options *imap.SearchOptions) (hasContext, hasUpdate bool, err error) {
	if err := dec.ExpectByte('('); err != nil {
		return false, false, imap.ErrBad("expected '(' for RETURN options")
	}

	b, err := dec.PeekByte()
	if err != nil {
		return false, false, imap.ErrBad("unexpected end in RETURN options")
	}
	if b == ')' {
		if err := dec.ExpectByte(')'); err != nil {
			return false, false, imap.ErrBad("expected ')' for RETURN options")
		}
		return false, false, nil
	}

	for {
		atom, err := dec.ReadAtom()
		if err != nil {
			return false, false, imap.ErrBad("invalid RETURN option")
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
		case "UPDATE":
			options.ReturnUpdate = true
			hasUpdate = true
		case "CONTEXT":
			options.ReturnContext = true
			hasContext = true
		default:
			return false, false, imap.ErrBad("unknown RETURN option: " + atom)
		}

		b, err := dec.PeekByte()
		if err != nil {
			return false, false, imap.ErrBad("unexpected end in RETURN options")
		}
		if b == ')' {
			if err := dec.ExpectByte(')'); err != nil {
				return false, false, imap.ErrBad("expected ')' for RETURN options")
			}
			return hasContext, hasUpdate, nil
		}
		if err := dec.ReadSP(); err != nil {
			return false, false, imap.ErrBad("expected SP between RETURN options")
		}
	}
}

// hasAnyReturnOption returns true if any RETURN option is set (including CONTEXT/UPDATE).
func hasAnyReturnOption(options *imap.SearchOptions) bool {
	return options.ReturnMin || options.ReturnMax || options.ReturnAll ||
		options.ReturnCount || options.ReturnSave ||
		options.ReturnContext || options.ReturnUpdate
}

// hasAnyStandardReturnOption returns true if any standard ESEARCH RETURN option is set.
func hasAnyStandardReturnOption(options *imap.SearchOptions) bool {
	return options.ReturnMin || options.ReturnMax || options.ReturnAll ||
		options.ReturnCount || options.ReturnSave
}

// writeNoUpdateResponse writes a NOUPDATE untagged NO response.
func writeNoUpdateResponse(ctx *server.CommandContext) {
	enc := ctx.Conn.Encoder()
	enc.Encode(func(e *wire.Encoder) {
		e.StatusResponse("*", "NO", `NOUPDATE "`+ctx.Tag+`"`, "search context not supported")
	})
}

// writeContextESearchResponse writes an ESEARCH response with optional ADDTO/REMOVEFROM.
func writeContextESearchResponse(enc *server.ResponseEncoder, ctx *server.CommandContext, data *imap.SearchData, options *imap.SearchOptions) {
	// Write ADDTO notifications
	for _, update := range data.AddTo {
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("ESEARCH").SP()
			e.BeginList().Atom("TAG").SP().QuotedString(ctx.Tag).EndList()
			if ctx.NumKind == server.NumKindUID {
				e.SP().Atom("UID")
			}
			e.SP().Atom("ADDTO").SP()
			e.BeginList().Number(update.Position).SP().Atom(update.SeqSet.String()).EndList()
			e.CRLF()
		})
	}

	// Write REMOVEFROM notifications
	for _, update := range data.RemoveFrom {
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("ESEARCH").SP()
			e.BeginList().Atom("TAG").SP().QuotedString(ctx.Tag).EndList()
			if ctx.NumKind == server.NumKindUID {
				e.SP().Atom("UID")
			}
			e.SP().Atom("REMOVEFROM").SP()
			e.BeginList().Number(update.Position).SP().Atom(update.SeqSet.String()).EndList()
			e.CRLF()
		})
	}

	// Write main ESEARCH response
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

// writeTraditionalSearchResponse writes a traditional * SEARCH response.
func writeTraditionalSearchResponse(ctx *server.CommandContext, data *imap.SearchData) {
	enc := ctx.Conn.Encoder()
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
