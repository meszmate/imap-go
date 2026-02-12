// Package esearch implements the ESEARCH extension (RFC 4731).
//
// ESEARCH extends the SEARCH command with additional return options
// (MIN, MAX, ALL, COUNT, SAVE) and a new ESEARCH response format.
// The core SEARCH handler already supports SearchOptions with these
// return options; this extension advertises the capability and exposes
// a session interface for extended search operations.
package esearch

import (
	"fmt"
	"strings"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionESearch is the session interface for ESEARCH support.
// Implementations provide extended search with return options such as
// MIN, MAX, ALL, COUNT, and SAVE.
type SessionESearch interface {
	SearchExtended(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
}

// Extension implements the ESEARCH IMAP extension (RFC 4731).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new ESEARCH extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "ESEARCH",
			ExtCapabilities: []imap.Cap{imap.CapESearch},
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
			return handleESearch(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionESearch)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }

// handleESearch wraps the SEARCH command to parse RETURN options and write ESEARCH responses.
func handleESearch(ctx *server.CommandContext, _ server.CommandHandlerFunc) error {
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
		// Parse SP then parenthesized list of return options
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing RETURN options")
		}
		if err := parseReturnOptions(dec, options); err != nil {
			return err
		}
		// Parse SP then search criteria
		if err := dec.ReadSP(); err != nil {
			return imap.ErrBad("missing search criteria after RETURN")
		}
		if err := ParseSearchCriteria(dec, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
	} else {
		// First atom is a search criterion, not RETURN
		if err := ParseSearchCriterion(first, dec, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
		// Try to read more criteria
		if err := dec.ReadSP(); err == nil {
			if err := ParseSearchCriteria(dec, criteria); err != nil {
				return imap.ErrBad("invalid search criteria: " + err.Error())
			}
		}
	}

	// Route to session
	var data *imap.SearchData
	if hasReturn {
		if sess, ok := ctx.Session.(SessionESearch); ok {
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
		// Empty RETURN () — no options
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
		// MODSEQ emitted regardless of RETURN when present
		if data.ModSeq > 0 {
			e.SP().Atom("MODSEQ").SP().Number64(data.ModSeq)
		}
		e.CRLF()
	})
}

// ParseSearchCriteria reads search criteria from the decoder in a loop.
func ParseSearchCriteria(dec *wire.Decoder, criteria *imap.SearchCriteria) error {
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

		if err := ParseSearchCriterion(key, dec, criteria); err != nil {
			return err
		}

		if err := dec.ReadSP(); err != nil {
			return nil
		}
	}
}

// ParseSearchCriterion handles a single already-read criterion key.
func ParseSearchCriterion(key string, dec *wire.Decoder, criteria *imap.SearchCriteria) error {
	switch strings.ToUpper(key) {
	case "ALL":
		// Match all messages (no-op for criteria)
	case "ANSWERED":
		criteria.Flag = append(criteria.Flag, imap.FlagAnswered)
	case "DELETED":
		criteria.Flag = append(criteria.Flag, imap.FlagDeleted)
	case "DRAFT":
		criteria.Flag = append(criteria.Flag, imap.FlagDraft)
	case "FLAGGED":
		criteria.Flag = append(criteria.Flag, imap.FlagFlagged)
	case "SEEN":
		criteria.Flag = append(criteria.Flag, imap.FlagSeen)
	case "RECENT":
		criteria.Flag = append(criteria.Flag, imap.FlagRecent)
	case "UNANSWERED":
		criteria.NotFlag = append(criteria.NotFlag, imap.FlagAnswered)
	case "UNDELETED":
		criteria.NotFlag = append(criteria.NotFlag, imap.FlagDeleted)
	case "UNDRAFT":
		criteria.NotFlag = append(criteria.NotFlag, imap.FlagDraft)
	case "UNFLAGGED":
		criteria.NotFlag = append(criteria.NotFlag, imap.FlagFlagged)
	case "UNSEEN":
		criteria.NotFlag = append(criteria.NotFlag, imap.FlagSeen)
	case "NEW":
		criteria.Flag = append(criteria.Flag, imap.FlagRecent)
		criteria.NotFlag = append(criteria.NotFlag, imap.FlagSeen)
	case "OLD":
		criteria.NotFlag = append(criteria.NotFlag, imap.FlagRecent)
	case "KEYWORD":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		kw, err := dec.ReadAtom()
		if err != nil {
			return err
		}
		criteria.Flag = append(criteria.Flag, imap.Flag(kw))
	case "UNKEYWORD":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		kw, err := dec.ReadAtom()
		if err != nil {
			return err
		}
		criteria.NotFlag = append(criteria.NotFlag, imap.Flag(kw))
	case "LARGER":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		n, err := dec.ReadNumber64()
		if err != nil {
			return err
		}
		criteria.Larger = int64(n)
	case "SMALLER":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		n, err := dec.ReadNumber64()
		if err != nil {
			return err
		}
		criteria.Smaller = int64(n)
	case "BODY":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAString()
		if err != nil {
			return err
		}
		criteria.Body = append(criteria.Body, s)
	case "TEXT":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAString()
		if err != nil {
			return err
		}
		criteria.Text = append(criteria.Text, s)
	case "SUBJECT":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAString()
		if err != nil {
			return err
		}
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key: "Subject", Value: s,
		})
	case "FROM":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAString()
		if err != nil {
			return err
		}
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key: "From", Value: s,
		})
	case "TO":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAString()
		if err != nil {
			return err
		}
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key: "To", Value: s,
		})
	case "CC":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAString()
		if err != nil {
			return err
		}
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key: "Cc", Value: s,
		})
	case "BCC":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAString()
		if err != nil {
			return err
		}
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key: "Bcc", Value: s,
		})
	case "HEADER":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		hdrName, err := dec.ReadAString()
		if err != nil {
			return err
		}
		if err := dec.ReadSP(); err != nil {
			return err
		}
		hdrValue, err := dec.ReadAString()
		if err != nil {
			return err
		}
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key: hdrName, Value: hdrValue,
		})
	case "UID":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAtom()
		if err != nil {
			return err
		}
		uidSet, err := imap.ParseUIDSet(s)
		if err != nil {
			return err
		}
		criteria.UID = uidSet
	case "MODSEQ":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		modseqCrit := &imap.SearchCriteriaModSeq{}
		b, err := dec.PeekByte()
		if err != nil {
			return err
		}
		if b == '"' {
			entryName, err := dec.ReadQuotedString()
			if err != nil {
				return err
			}
			modseqCrit.MetadataName = entryName
			if err := dec.ReadSP(); err != nil {
				return err
			}
			entryType, err := dec.ReadAtom()
			if err != nil {
				return err
			}
			modseqCrit.MetadataType = strings.ToLower(entryType)
			if err := dec.ReadSP(); err != nil {
				return err
			}
		}
		n, err := dec.ReadNumber64()
		if err != nil {
			return err
		}
		modseqCrit.ModSeq = n
		criteria.ModSeq = modseqCrit
	case "SAVEDBEFORE":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAString()
		if err != nil {
			return err
		}
		t, err := time.Parse("2-Jan-2006", s)
		if err != nil {
			return fmt.Errorf("invalid SAVEDBEFORE date: %w", err)
		}
		criteria.SavedBefore = t
	case "SAVEDSINCE":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAString()
		if err != nil {
			return err
		}
		t, err := time.Parse("2-Jan-2006", s)
		if err != nil {
			return fmt.Errorf("invalid SAVEDSINCE date: %w", err)
		}
		criteria.SavedSince = t
	case "SAVEDON":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		s, err := dec.ReadAString()
		if err != nil {
			return err
		}
		t, err := time.Parse("2-Jan-2006", s)
		if err != nil {
			return fmt.Errorf("invalid SAVEDON date: %w", err)
		}
		criteria.SavedOn = t
	case "NOT":
		if err := dec.ReadSP(); err != nil {
			return err
		}
		sub := &imap.SearchCriteria{}
		if err := ParseSearchCriteria(dec, sub); err != nil {
			return err
		}
		criteria.Not = append(criteria.Not, *sub)
	default:
		// Try to parse as a sequence set
		seqSet, err := imap.ParseSeqSet(key)
		if err == nil {
			criteria.SeqNum = seqSet
		}
	}
	return nil
}
