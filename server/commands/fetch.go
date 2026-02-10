package commands

import (
	"strconv"
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

	upper := strings.ToUpper(item)
	switch {
	case upper == "ALL":
		options.Flags = true
		options.InternalDate = true
		options.RFC822Size = true
		options.Envelope = true
	case upper == "FAST":
		options.Flags = true
		options.InternalDate = true
		options.RFC822Size = true
	case upper == "FULL":
		options.Flags = true
		options.InternalDate = true
		options.RFC822Size = true
		options.Envelope = true
		options.BodyStructure = true
	case upper == "ENVELOPE":
		options.Envelope = true
	case upper == "FLAGS":
		options.Flags = true
	case upper == "INTERNALDATE":
		options.InternalDate = true
	case upper == "RFC822.SIZE":
		options.RFC822Size = true
	case upper == "UID":
		options.UID = true
	case upper == "BODYSTRUCTURE":
		options.BodyStructure = true
	case upper == "MODSEQ":
		options.ModSeq = true
	case upper == "PREVIEW":
		options.Preview = true
	case upper == "EMAILID":
		options.EmailID = true
	case upper == "THREADID":
		options.ThreadID = true

	// BODY with bracket embedded in atom ([ is an atom char)
	case strings.HasPrefix(upper, "BODY.PEEK["):
		section, err := parseFetchBodySectionFromAtom(dec, item, true)
		if err != nil {
			return err
		}
		options.BodySection = append(options.BodySection, section)
	case strings.HasPrefix(upper, "BODY["):
		section, err := parseFetchBodySectionFromAtom(dec, item, false)
		if err != nil {
			return err
		}
		options.BodySection = append(options.BodySection, section)
	case upper == "BODY.PEEK":
		b, err := dec.PeekByte()
		if err == nil && b == '[' {
			section, err := parseFetchBodySection(dec, true)
			if err != nil {
				return err
			}
			options.BodySection = append(options.BodySection, section)
		}
	case upper == "BODY":
		b, err := dec.PeekByte()
		if err == nil && b == '[' {
			section, err := parseFetchBodySection(dec, false)
			if err != nil {
				return err
			}
			options.BodySection = append(options.BodySection, section)
		} else {
			options.BodyStructure = true
		}

	// BINARY items (RFC 3516)
	case strings.HasPrefix(upper, "BINARY.SIZE["):
		part := parseBinaryPart(item[len("BINARY.SIZE["):])
		if err := dec.ExpectByte(']'); err != nil {
			return err
		}
		options.BinarySizeSection = append(options.BinarySizeSection, part)
	case strings.HasPrefix(upper, "BINARY.PEEK["):
		section := parseBinaryItemFromAtom(item, "BINARY.PEEK[", true)
		if err := dec.ExpectByte(']'); err != nil {
			return err
		}
		section.Partial = consumePartial(dec)
		options.BinarySection = append(options.BinarySection, section)
	case strings.HasPrefix(upper, "BINARY["):
		section := parseBinaryItemFromAtom(item, "BINARY[", false)
		if err := dec.ExpectByte(']'); err != nil {
			return err
		}
		section.Partial = consumePartial(dec)
		options.BinarySection = append(options.BinarySection, section)

	case upper == "RFC822":
		options.BodySection = append(options.BodySection, &imap.FetchItemBodySection{})
	case upper == "RFC822.HEADER":
		options.BodySection = append(options.BodySection, &imap.FetchItemBodySection{
			Specifier: "HEADER",
			Peek:      true,
		})
	case upper == "RFC822.TEXT":
		options.BodySection = append(options.BodySection, &imap.FetchItemBodySection{
			Specifier: "TEXT",
		})
	}

	return nil
}

// parseFetchBodySectionFromAtom handles BODY[section or BODY.PEEK[section
// where [ was included in the atom by ReadAtom.
func parseFetchBodySectionFromAtom(dec *wire.Decoder, item string, peek bool) (*imap.FetchItemBodySection, error) {
	bracketIdx := strings.IndexByte(item, '[')
	sectionStr := strings.ToUpper(item[bracketIdx+1:])

	section := &imap.FetchItemBodySection{Peek: peek}

	switch {
	case sectionStr == "":
		// empty section — will read ] next
	case sectionStr == "HEADER":
		section.Specifier = "HEADER"
	case sectionStr == "TEXT":
		section.Specifier = "TEXT"
	case sectionStr == "MIME":
		section.Specifier = "MIME"
	case strings.HasPrefix(sectionStr, "HEADER.FIELDS.NOT"):
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
	case strings.HasPrefix(sectionStr, "HEADER.FIELDS"):
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

	if err := dec.ExpectByte(']'); err != nil {
		return nil, err
	}

	// Check for partial <offset.count>
	b, err := dec.PeekByte()
	if err == nil && b == '<' {
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

// parseBinaryPart parses a MIME part string like "1.2" into []int{1, 2}.
func parseBinaryPart(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ".")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		result = append(result, n)
	}
	return result
}

// parseBinaryItemFromAtom builds a FetchItemBinarySection from the atom string.
func parseBinaryItemFromAtom(item, prefix string, peek bool) *imap.FetchItemBinarySection {
	sectionStr := item[len(prefix):]
	return &imap.FetchItemBinarySection{
		Part: parseBinaryPart(sectionStr),
		Peek: peek,
	}
}

// consumePartial consumes a <offset.count> partial specifier if present.
func consumePartial(dec *wire.Decoder) *imap.SectionPartial {
	b, err := dec.PeekByte()
	if err != nil || b != '<' {
		return nil
	}
	_ = dec.ExpectByte('<')
	atom, err := dec.ReadAtom()
	if err != nil {
		return nil
	}
	_ = dec.ExpectByte('>')

	parts := strings.SplitN(atom, ".", 2)
	if len(parts) != 2 {
		return nil
	}
	offset, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil
	}
	count, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil
	}
	return &imap.SectionPartial{Offset: offset, Count: count}
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
