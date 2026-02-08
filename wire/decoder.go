// Package wire provides the IMAP wire protocol encoder and decoder.
//
// This package implements a streaming parser and encoder for the IMAP protocol
// as defined in RFC 9051 (IMAP4rev2) and RFC 3501 (IMAP4rev1).
package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Decoder reads and parses IMAP protocol data from an io.Reader.
type Decoder struct {
	r *bufio.Reader

	// ContinuationRequest is called when the decoder needs to send a
	// continuation request for non-synchronizing literals.
	ContinuationRequest func() error
}

// NewDecoder creates a new Decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReaderSize(r, 4096)
	}
	return &Decoder{r: br}
}

// ReadLine reads a complete IMAP line (terminated by CRLF).
func (d *Decoder) ReadLine() (string, error) {
	var line []byte
	for {
		part, isPrefix, err := d.r.ReadLine()
		if err != nil {
			return "", err
		}
		line = append(line, part...)
		if !isPrefix {
			break
		}
	}
	return string(line), nil
}

// ReadAtom reads an atom (a sequence of non-special characters).
func (d *Decoder) ReadAtom() (string, error) {
	var buf bytes.Buffer
	for {
		b, err := d.r.Peek(1)
		if err != nil {
			if err == io.EOF && buf.Len() > 0 {
				return buf.String(), nil
			}
			return "", err
		}
		if isAtomChar(b[0]) {
			ch, err := d.r.ReadByte()
			if err != nil {
				return "", err
			}
			buf.WriteByte(ch)
		} else {
			break
		}
	}
	if buf.Len() == 0 {
		return "", fmt.Errorf("imap: expected atom")
	}
	return buf.String(), nil
}

// ReadQuotedString reads a quoted string.
func (d *Decoder) ReadQuotedString() (string, error) {
	b, err := d.r.ReadByte()
	if err != nil {
		return "", err
	}
	if b != '"' {
		return "", fmt.Errorf("imap: expected '\"', got %q", b)
	}

	var buf bytes.Buffer
	for {
		ch, err := d.r.ReadByte()
		if err != nil {
			return "", err
		}
		if ch == '"' {
			return buf.String(), nil
		}
		if ch == '\\' {
			// Escaped character
			escaped, err := d.r.ReadByte()
			if err != nil {
				return "", err
			}
			buf.WriteByte(escaped)
		} else {
			buf.WriteByte(ch)
		}
	}
}

// LiteralInfo contains information about a literal.
type LiteralInfo struct {
	Size         int64
	NonSync      bool // {n+} literal
	Binary       bool // ~{n} literal
}

// ReadLiteralInfo reads a literal header like {42}, {42+}, or ~{42}.
func (d *Decoder) ReadLiteralInfo() (*LiteralInfo, error) {
	info := &LiteralInfo{}

	b, err := d.r.ReadByte()
	if err != nil {
		return nil, err
	}

	if b == '~' {
		info.Binary = true
		b, err = d.r.ReadByte()
		if err != nil {
			return nil, err
		}
	}

	if b != '{' {
		return nil, fmt.Errorf("imap: expected '{', got %q", b)
	}

	var numStr bytes.Buffer
	for {
		ch, err := d.r.ReadByte()
		if err != nil {
			return nil, err
		}
		if ch == '+' {
			info.NonSync = true
		} else if ch == '}' {
			break
		} else if ch >= '0' && ch <= '9' {
			numStr.WriteByte(ch)
		} else {
			return nil, fmt.Errorf("imap: unexpected character in literal: %q", ch)
		}
	}

	size, err := strconv.ParseInt(numStr.String(), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("imap: invalid literal size: %w", err)
	}
	info.Size = size

	// Read the trailing CRLF after the literal header
	if err := d.ReadCRLF(); err != nil {
		return nil, fmt.Errorf("imap: expected CRLF after literal: %w", err)
	}

	return info, nil
}

// ReadLiteral reads a literal value from the stream after the header has been parsed.
func (d *Decoder) ReadLiteral(size int64) io.Reader {
	return io.LimitReader(d.r, size)
}

// ReadString reads either a quoted string, literal, or NIL.
func (d *Decoder) ReadString() (string, error) {
	b, err := d.r.Peek(1)
	if err != nil {
		return "", err
	}

	switch b[0] {
	case '"':
		return d.ReadQuotedString()
	case '{', '~':
		info, err := d.ReadLiteralInfo()
		if err != nil {
			return "", err
		}
		data := make([]byte, info.Size)
		_, err = io.ReadFull(d.r, data)
		if err != nil {
			return "", err
		}
		return string(data), nil
	default:
		return d.ReadAtom()
	}
}

// ReadAString reads an astring (atom or string).
func (d *Decoder) ReadAString() (string, error) {
	return d.ReadString()
}

// ReadNString reads a nstring (NIL or string). Returns empty string and false for NIL.
func (d *Decoder) ReadNString() (string, bool, error) {
	b, err := d.r.Peek(3)
	if err != nil && len(b) == 0 {
		return "", false, err
	}

	if len(b) >= 3 && strings.EqualFold(string(b), "NIL") {
		// Check that the next char after NIL is a delimiter
		next, err := d.r.Peek(4)
		if err == io.EOF || (len(next) >= 4 && !isAtomChar(next[3])) || len(next) == 3 {
			// Consume NIL
			buf := make([]byte, 3)
			_, _ = d.r.Read(buf)
			return "", false, nil
		}
	}

	s, err := d.ReadString()
	if err != nil {
		return "", false, err
	}
	return s, true, nil
}

// ReadNumber reads an unsigned number.
func (d *Decoder) ReadNumber() (uint32, error) {
	atom, err := d.ReadAtom()
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(atom, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("imap: invalid number %q: %w", atom, err)
	}
	return uint32(n), nil
}

// ReadNumber64 reads a 64-bit unsigned number.
func (d *Decoder) ReadNumber64() (uint64, error) {
	atom, err := d.ReadAtom()
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(atom, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("imap: invalid number %q: %w", atom, err)
	}
	return n, nil
}

// ReadSP reads a single space character.
func (d *Decoder) ReadSP() error {
	b, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	if b != ' ' {
		return fmt.Errorf("imap: expected SP, got %q", b)
	}
	return nil
}

// ReadCRLF reads a CRLF (carriage return + line feed).
func (d *Decoder) ReadCRLF() error {
	b1, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	b2, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	if b1 != '\r' || b2 != '\n' {
		return fmt.Errorf("imap: expected CRLF, got %q%q", b1, b2)
	}
	return nil
}

// ExpectByte reads a byte and returns an error if it doesn't match.
func (d *Decoder) ExpectByte(expected byte) error {
	b, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	if b != expected {
		return fmt.Errorf("imap: expected %q, got %q", expected, b)
	}
	return nil
}

// PeekByte returns the next byte without consuming it.
func (d *Decoder) PeekByte() (byte, error) {
	b, err := d.r.Peek(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

// ReadList reads a parenthesized list and calls fn for each element.
func (d *Decoder) ReadList(fn func() error) error {
	if err := d.ExpectByte('('); err != nil {
		return err
	}

	first := true
	for {
		b, err := d.PeekByte()
		if err != nil {
			return err
		}
		if b == ')' {
			_, _ = d.r.ReadByte()
			return nil
		}
		if !first {
			if err := d.ReadSP(); err != nil {
				return err
			}
		}
		if err := fn(); err != nil {
			return err
		}
		first = false
	}
}

// ReadFlags reads a parenthesized list of flags.
func (d *Decoder) ReadFlags() ([]string, error) {
	var flags []string
	err := d.ReadList(func() error {
		flag, err := d.ReadAtom()
		if err != nil {
			return err
		}
		flags = append(flags, flag)
		return nil
	})
	return flags, err
}

// DiscardLine discards the rest of the current line.
func (d *Decoder) DiscardLine() error {
	_, err := d.r.ReadBytes('\n')
	return err
}

// DiscardN discards n bytes.
func (d *Decoder) DiscardN(n int64) error {
	_, err := io.CopyN(io.Discard, d.r, n)
	return err
}

// Buffered returns the number of bytes buffered.
func (d *Decoder) Buffered() int {
	return d.r.Buffered()
}

// isAtomChar returns true if the byte is a valid atom character.
// Atom characters are any CHAR except atom-specials.
func isAtomChar(b byte) bool {
	if b < 0x20 || b > 0x7e {
		return false
	}
	switch b {
	case '(', ')', '{', ' ', '%', '*', '"', '\\', ']':
		return false
	}
	return true
}

// IsAtomSpecial returns true if the byte is an atom-special character.
func IsAtomSpecial(b byte) bool {
	return !isAtomChar(b)
}

// IsQuotedSpecial returns true if the byte needs escaping in a quoted string.
func IsQuotedSpecial(b byte) bool {
	return b == '"' || b == '\\'
}

// NeedsQuoting returns true if the string needs to be quoted for IMAP.
func NeedsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for i := 0; i < len(s); i++ {
		if !isAtomChar(s[i]) {
			return true
		}
	}
	return false
}

// NeedsLiteral returns true if the string must be sent as a literal.
func NeedsLiteral(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '\r' || b == '\n' || b == 0 {
			return true
		}
		if b > 0x7e {
			return true
		}
	}
	return false
}
