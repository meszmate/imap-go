// Package multisearch implements the MULTISEARCH extension (RFC 7377).
//
// MULTISEARCH adds the MULTISEARCH capability and a new ESEARCH command
// that allows searching across multiple mailboxes simultaneously. Each
// matching mailbox produces a separate ESEARCH response line with MAILBOX
// and UIDVALIDITY fields. Results are always UIDs.
package multisearch

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/extensions/esearch"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// MultiSearchSource specifies the source mailboxes for a multi-mailbox search.
type MultiSearchSource struct {
	Filter    string   // "mailboxes", "subtree", or "subtree-one"
	Mailboxes []string
}

// SessionMultiSearch is an optional interface for sessions that support
// the MULTISEARCH extension (RFC 7377).
type SessionMultiSearch interface {
	// MultiSearch performs a search across multiple mailboxes.
	// Results are always UIDs regardless of the NumKind.
	MultiSearch(kind server.NumKind, source *MultiSearchSource, criteria *imap.SearchCriteria, options *imap.SearchOptions) ([]imap.MultiSearchResult, error)
}

// Extension implements the MULTISEARCH IMAP extension (RFC 7377).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new MULTISEARCH extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "MULTISEARCH",
			ExtCapabilities: []imap.Cap{imap.CapMultiSearch},
			ExtDependencies: []string{"ESEARCH"},
		},
	}
}

// CommandHandlers returns new command handlers to register.
func (e *Extension) CommandHandlers() map[string]interface{} {
	return map[string]interface{}{
		"ESEARCH": server.CommandHandlerFunc(handleMultiSearch),
	}
}

// WrapHandler returns nil because MULTISEARCH does not wrap existing commands.
func (e *Extension) WrapHandler(name string, handler interface{}) interface{} { return nil }

// SessionExtension returns the SessionMultiSearch interface that sessions may
// implement to support multi-mailbox search operations.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionMultiSearch)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }

// handleMultiSearch handles the ESEARCH command (RFC 7377).
func handleMultiSearch(ctx *server.CommandContext) error {
	// State check: requires authenticated or selected
	state := ctx.State()
	if state != imap.ConnStateAuthenticated && state != imap.ConnStateSelected {
		return imap.ErrBad("ESEARCH requires authenticated or selected state")
	}

	dec := ctx.Decoder
	if dec == nil {
		return imap.ErrBad("missing ESEARCH arguments")
	}

	// Parse "IN" keyword
	inKeyword, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("missing IN keyword")
	}
	if !strings.EqualFold(inKeyword, "IN") {
		return imap.ErrBad("expected IN keyword, got " + inKeyword)
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing source specification after IN")
	}

	// Parse source-mbox: ( filter-type SP mailbox-or-list )
	if err := dec.ExpectByte('('); err != nil {
		return imap.ErrBad("expected '(' for source specification")
	}

	filterType, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("missing filter type")
	}
	filterLower := strings.ToLower(filterType)
	if filterLower != "mailboxes" && filterLower != "subtree" && filterLower != "subtree-one" {
		return imap.ErrBad("unknown filter type: " + filterType)
	}

	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing mailbox after filter type")
	}

	// Parse mailbox-or-list: either a parenthesized list or a single mailbox
	source := &MultiSearchSource{Filter: filterLower}
	b, err := dec.PeekByte()
	if err != nil {
		return imap.ErrBad("unexpected end in source specification")
	}
	if b == '(' {
		// Parenthesized list of mailboxes
		if err := dec.ReadList(func() error {
			mbox, err := dec.ReadAString()
			if err != nil {
				return err
			}
			source.Mailboxes = append(source.Mailboxes, mbox)
			return nil
		}); err != nil {
			return imap.ErrBad("invalid mailbox list: " + err.Error())
		}
	} else {
		// Single mailbox
		mbox, err := dec.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox name: " + err.Error())
		}
		source.Mailboxes = []string{mbox}
	}

	// Close source-mbox paren
	if err := dec.ExpectByte(')'); err != nil {
		return imap.ErrBad("expected ')' for source specification")
	}

	options := &imap.SearchOptions{}
	criteria := &imap.SearchCriteria{}

	// Parse remaining arguments: optional RETURN, optional CHARSET, then search criteria
	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing search criteria")
	}

	// Read next atom to check for RETURN or CHARSET or search criterion
	atom, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("missing search arguments")
	}

	if strings.EqualFold(atom, "RETURN") {
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing RETURN options")
		}
		if err := parseReturnOptions(dec, options); err != nil {
			return err
		}
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing search criteria after RETURN")
		}
		// Read next atom for CHARSET check or search criterion
		atom, err = dec.ReadAtom()
		if err != nil {
			return imap.ErrBad("missing search criteria")
		}
	}

	if strings.EqualFold(atom, "CHARSET") {
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing charset name")
		}
		// Consume charset name (pass through to backend)
		if _, err := dec.ReadAString(); err != nil {
			return imap.ErrBad("invalid charset name")
		}
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing search criteria after CHARSET")
		}
		// Read first search criterion
		atom, err = dec.ReadAtom()
		if err != nil {
			return imap.ErrBad("missing search criteria")
		}
	}

	// Parse the first search criterion we already read
	if err := esearch.ParseSearchCriterion(atom, dec, criteria); err != nil {
		return imap.ErrBad("invalid search criteria: " + err.Error())
	}
	// Parse remaining search criteria
	if err := dec.ReadSP(); err == nil {
		if err := esearch.ParseSearchCriteria(dec, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
	}

	// Route to session
	sess, ok := ctx.Session.(SessionMultiSearch)
	if !ok {
		return imap.ErrNo("MULTISEARCH not supported")
	}

	results, err := sess.MultiSearch(ctx.NumKind, source, criteria, options)
	if err != nil {
		return err
	}

	// Write per-mailbox ESEARCH responses
	writeMultiSearchResponse(ctx, results, options)

	ctx.Conn.WriteOK(ctx.Tag, "ESEARCH completed")
	return nil
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

// writeMultiSearchResponse writes one ESEARCH response per mailbox result.
func writeMultiSearchResponse(ctx *server.CommandContext, results []imap.MultiSearchResult, options *imap.SearchOptions) {
	enc := ctx.Conn.Encoder()
	for _, result := range results {
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("ESEARCH").SP()
			// TAG correlator
			e.BeginList().Atom("TAG").SP().QuotedString(ctx.Tag).EndList()
			// MAILBOX and UIDVALIDITY (RFC 7377)
			e.SP().Atom("MAILBOX").SP().MailboxName(result.Mailbox)
			e.SP().Atom("UIDVALIDITY").SP().Number(result.UIDValidity)
			// Always UID (RFC 7377: results are always UIDs)
			e.SP().Atom("UID")
			// Result items
			if result.Data != nil {
				hasResults := result.Data.Min > 0 || result.Data.Max > 0 || result.Data.All != nil || result.Data.Count > 0
				if hasResults {
					if options.ReturnMin && result.Data.Min > 0 {
						e.SP().Atom("MIN").SP().Number(result.Data.Min)
					}
					if options.ReturnMax && result.Data.Max > 0 {
						e.SP().Atom("MAX").SP().Number(result.Data.Max)
					}
					if options.ReturnAll && result.Data.All != nil {
						e.SP().Atom("ALL").SP().Atom(result.Data.All.String())
					}
					if options.ReturnCount {
						e.SP().Atom("COUNT").SP().Number(result.Data.Count)
					}
				}
				if result.Data.ModSeq > 0 {
					e.SP().Atom("MODSEQ").SP().Number64(result.Data.ModSeq)
				}
			}
			e.CRLF()
		})
	}
}
