package commands

import (
	"strings"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// Fetch returns a handler for the FETCH command.
// FETCH retrieves data associated with a message in the mailbox.
func Fetch() server.CommandHandlerFunc {
	return func(ctx *server.CommandContext) error {
		if ctx.Decoder == nil {
			return imap.ErrBad("missing arguments")
		}

		// Read sequence set
		seqSetStr, err := ctx.Decoder.ReadAtom()
		if err != nil {
			return imap.ErrBad("invalid sequence set")
		}

		var numSet imap.NumSet
		if ctx.NumKind == server.NumKindUID {
			uidSet, err := imap.ParseUIDSet(seqSetStr)
			if err != nil {
				return imap.ErrBad("invalid UID set")
			}
			numSet = uidSet
		} else {
			seqSet, err := imap.ParseSeqSet(seqSetStr)
			if err != nil {
				return imap.ErrBad("invalid sequence set")
			}
			numSet = seqSet
		}

		if err := ctx.Decoder.ReadSP(); err != nil {
			return imap.ErrBad("missing fetch items")
		}

		// Parse fetch items
		options, err := parseFetchItems(ctx.Decoder)
		if err != nil {
			return imap.ErrBad("invalid fetch items: " + err.Error())
		}

		// Always include UID for UID FETCH
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
}

func parseFetchItems(dec *wire.Decoder) (*imap.FetchOptions, error) {
	options := &imap.FetchOptions{}

	b, err := dec.PeekByte()
	if err != nil {
		return nil, err
	}

	if b == '(' {
		// Parenthesized list of items
		if err := dec.ReadList(func() error {
			return parseSingleFetchItem(dec, options)
		}); err != nil {
			return nil, err
		}
	} else {
		// Single item or macro
		if err := parseSingleFetchItem(dec, options); err != nil {
			return nil, err
		}
	}

	return options, nil
}

func parseSingleFetchItem(dec *wire.Decoder, options *imap.FetchOptions) error {
	item, err := dec.ReadAtom()
	if err != nil {
		return err
	}

	switch strings.ToUpper(item) {
	case "ALL":
		options.Flags = true
		options.InternalDate = true
		options.RFC822Size = true
		options.Envelope = true
	case "FAST":
		options.Flags = true
		options.InternalDate = true
		options.RFC822Size = true
	case "FULL":
		options.Flags = true
		options.InternalDate = true
		options.RFC822Size = true
		options.Envelope = true
		options.BodyStructure = true
	case "ENVELOPE":
		options.Envelope = true
	case "FLAGS":
		options.Flags = true
	case "INTERNALDATE":
		options.InternalDate = true
	case "RFC822.SIZE":
		options.RFC822Size = true
	case "UID":
		options.UID = true
	case "BODYSTRUCTURE":
		options.BodyStructure = true
	case "MODSEQ":
		options.ModSeq = true
	case "PREVIEW":
		options.Preview = true
	case "EMAILID":
		options.EmailID = true
	case "THREADID":
		options.ThreadID = true
	case "BODY.PEEK", "BODY":
		peek := strings.ToUpper(item) == "BODY.PEEK"
		// Check if there's a section specification
		b, err := dec.PeekByte()
		if err == nil && b == '[' {
			section, err := parseFetchBodySection(dec, peek)
			if err != nil {
				return err
			}
			options.BodySection = append(options.BodySection, section)
		} else if !peek {
			// bare BODY means BODYSTRUCTURE
			options.BodyStructure = true
		}
	case "RFC822":
		// RFC822 is equivalent to BODY[]
		options.BodySection = append(options.BodySection, &imap.FetchItemBodySection{})
	case "RFC822.HEADER":
		options.BodySection = append(options.BodySection, &imap.FetchItemBodySection{
			Specifier: "HEADER",
			Peek:      true,
		})
	case "RFC822.TEXT":
		options.BodySection = append(options.BodySection, &imap.FetchItemBodySection{
			Specifier: "TEXT",
		})
	}

	return nil
}

func parseFetchBodySection(dec *wire.Decoder, peek bool) (*imap.FetchItemBodySection, error) {
	section := &imap.FetchItemBodySection{
		Peek: peek,
	}

	// Read '['
	if err := dec.ExpectByte('['); err != nil {
		return nil, err
	}

	// Read section content until ']'
	b, err := dec.PeekByte()
	if err != nil {
		return nil, err
	}

	if b != ']' {
		// Read the section specifier
		spec, err := dec.ReadAtom()
		if err != nil {
			return nil, err
		}

		upper := strings.ToUpper(spec)
		switch {
		case upper == "HEADER":
			section.Specifier = "HEADER"
		case upper == "TEXT":
			section.Specifier = "TEXT"
		case upper == "MIME":
			section.Specifier = "MIME"
		case strings.HasPrefix(upper, "HEADER.FIELDS.NOT"):
			section.Specifier = "HEADER.FIELDS.NOT"
			section.NotFields = true
			if err := dec.ReadSP(); err != nil {
				return nil, err
			}
			fields, err := readFieldList(dec)
			if err != nil {
				return nil, err
			}
			section.Fields = fields
		case strings.HasPrefix(upper, "HEADER.FIELDS"):
			section.Specifier = "HEADER.FIELDS"
			if err := dec.ReadSP(); err != nil {
				return nil, err
			}
			fields, err := readFieldList(dec)
			if err != nil {
				return nil, err
			}
			section.Fields = fields
		}
	}

	// Read ']'
	if err := dec.ExpectByte(']'); err != nil {
		return nil, err
	}

	// Check for partial <offset.count>
	b, err = dec.PeekByte()
	if err == nil && b == '<' {
		// Simplified: skip partial parsing for now, consume until '>'
		for {
			ch, err := dec.PeekByte()
			if err != nil {
				break
			}
			if ch == '>' {
				_ = dec.ExpectByte('>')
				break
			}
			_, _ = dec.ReadAtom()
		}
	}

	return section, nil
}

func readFieldList(dec *wire.Decoder) ([]string, error) {
	var fields []string
	if err := dec.ReadList(func() error {
		field, err := dec.ReadAString()
		if err != nil {
			return err
		}
		fields = append(fields, field)
		return nil
	}); err != nil {
		return nil, err
	}
	return fields, nil
}
