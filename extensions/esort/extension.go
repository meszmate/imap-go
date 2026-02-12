// Package esort implements the ESORT extension (RFC 5267).
//
// ESORT extends the SORT command with ESEARCH-style return options
// (MIN, MAX, ALL, COUNT, PARTIAL), returning results in the ESEARCH
// response format instead of the traditional SORT response. This
// extension advertises the ESORT and CONTEXT=SORT capabilities and
// exposes a session interface for extended sort operations.
package esort

import (
	"fmt"
	"strings"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extension"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// SessionESort is the session interface for ESORT support.
// Implementations provide extended sort with ESEARCH-style return
// options, returning SearchData instead of SortData.
type SessionESort interface {
	SortExtended(kind server.NumKind, criteria []imap.SortCriterion, searchCriteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error)
}

// Extension implements the ESORT IMAP extension (RFC 5267).
type Extension struct {
	extension.BaseExtension
}

var _ extension.ServerExtension = (*Extension)(nil)

// New creates a new ESORT extension.
func New() *Extension {
	return &Extension{
		BaseExtension: extension.BaseExtension{
			ExtName:         "ESORT",
			ExtCapabilities: []imap.Cap{imap.CapESort, imap.CapContextSort},
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
	case "SORT":
		return server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
			return handleESort(ctx, h)
		})
	}
	return nil
}

// SessionExtension returns the required session extension interface.
func (e *Extension) SessionExtension() interface{} {
	return (*SessionESort)(nil)
}

// OnEnabled is called when a client enables this extension via ENABLE.
func (e *Extension) OnEnabled(connID string) error { return nil }

// handleESort wraps the SORT command to parse RETURN options and write ESEARCH responses.
func handleESort(ctx *server.CommandContext, originalHandler server.CommandHandlerFunc) error {
	dec := ctx.Decoder
	if dec == nil {
		return imap.ErrBad("missing sort arguments")
	}

	// SORT grammar: SORT (sort-criteria) charset search-criteria
	// ESORT grammar: SORT RETURN (options) (sort-criteria) charset search-criteria
	// Peek first byte: '(' means no RETURN, delegate to original handler.
	b, err := dec.PeekByte()
	if err != nil {
		return imap.ErrBad("missing sort arguments")
	}
	if b == '(' {
		// No RETURN keyword — delegate to original SORT handler
		return originalHandler(ctx)
	}

	// Read the atom — should be "RETURN"
	first, err := dec.ReadAtom()
	if err != nil {
		return imap.ErrBad("missing sort arguments")
	}
	if !strings.EqualFold(first, "RETURN") {
		return imap.ErrBad("expected RETURN or sort criteria list")
	}

	options := &imap.SearchOptions{}
	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing RETURN options")
	}
	if err := parseReturnOptions(dec, options); err != nil {
		return err
	}
	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("missing sort criteria after RETURN")
	}

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
		return imap.ErrBad("invalid sort criteria")
	}

	if len(criteria) == 0 {
		return imap.ErrBad("empty sort criteria")
	}

	// Read SP and charset (consumed but ignored)
	if err := dec.ReadSP(); err != nil {
		return imap.ErrBad("expected charset")
	}
	if _, err := dec.ReadAtom(); err != nil {
		return imap.ErrBad("invalid charset")
	}

	// Parse search criteria
	searchCriteria := &imap.SearchCriteria{}
	if err := dec.ReadSP(); err == nil {
		if err := parseSearchCriteria(dec, searchCriteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}
	}

	// Route to session
	var data *imap.SearchData
	if sess, ok := ctx.Session.(SessionESort); ok {
		data, err = sess.SortExtended(ctx.NumKind, criteria, searchCriteria, options)
	} else if sess, ok := ctx.Session.(server.SessionSort); ok {
		sortData, sortErr := sess.Sort(ctx.NumKind, criteria, searchCriteria, options)
		if sortErr != nil {
			return sortErr
		}
		data = sortDataToSearchData(sortData)
		err = nil
	} else {
		ctx.Conn.WriteNO(ctx.Tag, "SORT not supported")
		return nil
	}
	if err != nil {
		return err
	}

	// Write response
	enc := ctx.Conn.Encoder()
	if hasAnyReturnOption(options) {
		writeESearchResponse(enc, ctx, data, options)
	} else {
		// RETURN () with no options — write traditional SORT response
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("SORT")
			if data != nil {
				for _, num := range data.AllSeqNums {
					e.SP().Number(num)
				}
			}
			e.CRLF()
		})
	}

	ctx.Conn.WriteOK(ctx.Tag, "SORT completed")
	return nil
}

// sortDataToSearchData converts SortData to SearchData for ESEARCH response.
func sortDataToSearchData(sd *imap.SortData) *imap.SearchData {
	if sd == nil {
		return &imap.SearchData{}
	}
	data := &imap.SearchData{
		AllSeqNums: sd.AllNums,
	}
	if len(sd.AllNums) > 0 {
		data.Count = uint32(len(sd.AllNums))
		// Find min and max
		min := sd.AllNums[0]
		max := sd.AllNums[0]
		for _, n := range sd.AllNums[1:] {
			if n < min {
				min = n
			}
			if n > max {
				max = n
			}
		}
		data.Min = min
		data.Max = max
		// Build ALL as a SeqSet
		seqStr := fmt.Sprintf("%d", sd.AllNums[0])
		for _, n := range sd.AllNums[1:] {
			seqStr += fmt.Sprintf(",%d", n)
		}
		seqSet, _ := imap.ParseSeqSet(seqStr)
		data.All = seqSet
	}
	return data
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

// parseSearchCriteria reads search criteria from the decoder in a loop.
func parseSearchCriteria(dec *wire.Decoder, criteria *imap.SearchCriteria) error {
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

		if err := parseSearchCriterion(key, dec, criteria); err != nil {
			return err
		}

		if err := dec.ReadSP(); err != nil {
			return nil
		}
	}
}

// parseSearchCriterion handles a single already-read criterion key.
func parseSearchCriterion(key string, dec *wire.Decoder, criteria *imap.SearchCriteria) error {
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
		if err := parseSearchCriteria(dec, sub); err != nil {
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
