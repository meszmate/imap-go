package commands

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Search returns a handler for the SEARCH command.
// SEARCH searches the mailbox for messages that match the given criteria.
func Search() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing search criteria")
		}

		criteria := &imap.SearchCriteria{}
		options := &imap.SearchOptions{}

		// Parse search criteria from the decoder
		if err := parseSearchCriteria(ctx.Decoder, criteria); err != nil {
			return imap.ErrBad("invalid search criteria: " + err.Error())
		}

		data, err := ctx.Session.Search(ctx.NumKind, criteria, options)
		if err != nil {
			return err
		}

		// Write SEARCH response
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
			e.CRLF()
		})

		ctx.Conn.WriteOK(ctx.Tag, "SEARCH completed")
		return nil
	}
}

func parseSearchCriteria(dec *wire.Decoder, criteria *imap.SearchCriteria) error {
	for {
		b, err := dec.PeekByte()
		if err != nil {
			// End of input
			return nil
		}
		if b == ')' {
			return nil
		}

		key, err := dec.ReadAtom()
		if err != nil {
			return nil // End of arguments
		}

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
			// Otherwise ignore unknown criteria
		}

		// Try to read SP between criteria, but don't fail if at end
		if err := dec.ReadSP(); err != nil {
			return nil
		}
	}
}
