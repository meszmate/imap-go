package commands

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Status returns a handler for the STATUS command.
// STATUS requests the status of the indicated mailbox.
func Status() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing arguments")
		}

		mailbox, err := ctx.Decoder.ReadAString()
		if err != nil {
			return imap.ErrBad("invalid mailbox name")
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("missing status items")
		}

		// Parse status items list
		options := &imap.StatusOptions{}
		if err := ctx.Decoder.ReadList(func() error {
			item, err := ctx.Decoder.ReadAtom()
			if err != nil {
				return err
			}
			switch strings.ToUpper(item) {
			case "MESSAGES":
				options.NumMessages = true
			case "UIDNEXT":
				options.UIDNext = true
			case "UIDVALIDITY":
				options.UIDValidity = true
			case "UNSEEN":
				options.NumUnseen = true
			case "RECENT":
				options.NumRecent = true
			case "SIZE":
				options.Size = true
			case "APPENDLIMIT":
				options.AppendLimit = true
			case "DELETED":
				options.NumDeleted = true
			case "HIGHESTMODSEQ":
				options.HighestModSeq = true
			case "MAILBOXID":
				options.MailboxID = true
			}
			return nil
		}); err != nil {
			return imap.ErrBad("invalid status items list")
		}

		data, err := ctx.Session.Status(mailbox, options)
		if err != nil {
			return err
		}

		// Write STATUS response
		enc := ctx.Conn.Encoder()
		enc.Encode(func(e *wire.Encoder) {
			e.Star().Atom("STATUS").SP().MailboxName(data.Mailbox).SP().BeginList()

			first := true
			sp := func() {
				if !first {
					e.SP()
				}
				first = false
			}

			if data.NumMessages != nil {
				sp()
				e.Atom("MESSAGES").SP().Number(*data.NumMessages)
			}
			if data.UIDNext != nil {
				sp()
				e.Atom("UIDNEXT").SP().Number(*data.UIDNext)
			}
			if data.UIDValidity != nil {
				sp()
				e.Atom("UIDVALIDITY").SP().Number(*data.UIDValidity)
			}
			if data.NumUnseen != nil {
				sp()
				e.Atom("UNSEEN").SP().Number(*data.NumUnseen)
			}
			if data.NumRecent != nil {
				sp()
				e.Atom("RECENT").SP().Number(*data.NumRecent)
			}
			if data.Size != nil {
				sp()
				e.Atom("SIZE").SP().Number64(uint64(*data.Size))
			}
			if data.AppendLimit != nil {
				sp()
				e.Atom("APPENDLIMIT").SP().Number(*data.AppendLimit)
			}
			if data.NumDeleted != nil {
				sp()
				e.Atom("DELETED").SP().Number(*data.NumDeleted)
			}
			if data.HighestModSeq != nil {
				sp()
				e.Atom("HIGHESTMODSEQ").SP().Number64(*data.HighestModSeq)
			}
			if data.MailboxID != "" {
				sp()
				e.Atom("MAILBOXID").SP().BeginList().AString(data.MailboxID).EndList()
			}

			e.EndList().CRLF()
		})

		ctx.Conn.WriteOK(ctx.Tag, "STATUS completed")
		return nil
	}
}
