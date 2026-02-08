package wire

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Encoder writes IMAP protocol data to an io.Writer.
// It provides a fluent API for building IMAP responses and commands.
type Encoder struct {
	w *bufio.Writer
}

// NewEncoder creates a new Encoder writing to w.
func NewEncoder(w io.Writer) *Encoder {
	bw, ok := w.(*bufio.Writer)
	if !ok {
		bw = bufio.NewWriterSize(w, 4096)
	}
	return &Encoder{w: bw}
}

// Flush flushes buffered data to the underlying writer.
func (e *Encoder) Flush() error {
	return e.w.Flush()
}

// Raw writes raw bytes to the output.
func (e *Encoder) Raw(data []byte) *Encoder {
	_, _ = e.w.Write(data)
	return e
}

// RawString writes a raw string to the output.
func (e *Encoder) RawString(s string) *Encoder {
	_, _ = e.w.WriteString(s)
	return e
}

// Atom writes an atom.
func (e *Encoder) Atom(s string) *Encoder {
	_, _ = e.w.WriteString(s)
	return e
}

// SP writes a space.
func (e *Encoder) SP() *Encoder {
	_ = e.w.WriteByte(' ')
	return e
}

// CRLF writes a CRLF.
func (e *Encoder) CRLF() *Encoder {
	_, _ = e.w.WriteString("\r\n")
	return e
}

// QuotedString writes a quoted string, escaping special characters.
func (e *Encoder) QuotedString(s string) *Encoder {
	_ = e.w.WriteByte('"')
	for i := 0; i < len(s); i++ {
		if IsQuotedSpecial(s[i]) {
			_ = e.w.WriteByte('\\')
		}
		_ = e.w.WriteByte(s[i])
	}
	_ = e.w.WriteByte('"')
	return e
}

// String writes a string, using the best encoding (atom, quoted, or literal).
func (e *Encoder) String(s string) *Encoder {
	if NeedsLiteral(s) {
		return e.Literal([]byte(s))
	}
	if NeedsQuoting(s) {
		return e.QuotedString(s)
	}
	return e.Atom(s)
}

// AString writes an astring (atom or string).
func (e *Encoder) AString(s string) *Encoder {
	return e.String(s)
}

// NString writes a nstring (NIL or string).
func (e *Encoder) NString(s *string) *Encoder {
	if s == nil {
		return e.Nil()
	}
	return e.String(*s)
}

// Nil writes NIL.
func (e *Encoder) Nil() *Encoder {
	_, _ = e.w.WriteString("NIL")
	return e
}

// Number writes an unsigned 32-bit number.
func (e *Encoder) Number(n uint32) *Encoder {
	_, _ = e.w.WriteString(strconv.FormatUint(uint64(n), 10))
	return e
}

// Number64 writes an unsigned 64-bit number.
func (e *Encoder) Number64(n uint64) *Encoder {
	_, _ = e.w.WriteString(strconv.FormatUint(n, 10))
	return e
}

// Literal writes a literal string {n}\r\n<data>.
func (e *Encoder) Literal(data []byte) *Encoder {
	_ = e.w.WriteByte('{')
	_, _ = e.w.WriteString(strconv.Itoa(len(data)))
	_ = e.w.WriteByte('}')
	_, _ = e.w.WriteString("\r\n")
	_, _ = e.w.Write(data)
	return e
}

// LiteralNonSync writes a non-synchronizing literal {n+}\r\n<data>.
func (e *Encoder) LiteralNonSync(data []byte) *Encoder {
	_ = e.w.WriteByte('{')
	_, _ = e.w.WriteString(strconv.Itoa(len(data)))
	_, _ = e.w.WriteString("+}")
	_, _ = e.w.WriteString("\r\n")
	_, _ = e.w.Write(data)
	return e
}

// LiteralWriter returns a writer for streaming literal data.
func (e *Encoder) LiteralWriter(size int64, nonSync bool) io.Writer {
	_ = e.w.WriteByte('{')
	_, _ = e.w.WriteString(strconv.FormatInt(size, 10))
	if nonSync {
		_ = e.w.WriteByte('+')
	}
	_ = e.w.WriteByte('}')
	_, _ = e.w.WriteString("\r\n")
	_ = e.w.Flush()
	return e.w
}

// BeginList writes an opening parenthesis.
func (e *Encoder) BeginList() *Encoder {
	_ = e.w.WriteByte('(')
	return e
}

// EndList writes a closing parenthesis.
func (e *Encoder) EndList() *Encoder {
	_ = e.w.WriteByte(')')
	return e
}

// List writes a parenthesized list of strings.
func (e *Encoder) List(items []string) *Encoder {
	_ = e.w.WriteByte('(')
	for i, item := range items {
		if i > 0 {
			_ = e.w.WriteByte(' ')
		}
		e.String(item)
	}
	_ = e.w.WriteByte(')')
	return e
}

// Flags writes a parenthesized list of flags.
func (e *Encoder) Flags(flags []string) *Encoder {
	return e.List(flags)
}

// Date writes a date in DD-Mon-YYYY format.
func (e *Encoder) Date(t time.Time) *Encoder {
	return e.QuotedString(t.Format("02-Jan-2006"))
}

// DateTime writes a date-time in DD-Mon-YYYY HH:MM:SS +ZZZZ format.
func (e *Encoder) DateTime(t time.Time) *Encoder {
	return e.QuotedString(t.Format("02-Jan-2006 15:04:05 -0700"))
}

// Tag writes a command tag.
func (e *Encoder) Tag(tag string) *Encoder {
	_, _ = e.w.WriteString(tag)
	return e
}

// Star writes the untagged response prefix "* ".
func (e *Encoder) Star() *Encoder {
	_, _ = e.w.WriteString("* ")
	return e
}

// Plus writes the continuation request prefix "+ ".
func (e *Encoder) Plus() *Encoder {
	_, _ = e.w.WriteString("+ ")
	return e
}

// StatusResponse writes a status response (OK, NO, BAD, BYE, PREAUTH).
func (e *Encoder) StatusResponse(tag, status, code, text string) *Encoder {
	if tag == "" || tag == "*" {
		e.Star()
	} else {
		e.Tag(tag).SP()
	}
	e.Atom(status)
	if code != "" {
		_, _ = e.w.WriteString(" [")
		_, _ = e.w.WriteString(code)
		_ = e.w.WriteByte(']')
	}
	if text != "" {
		e.SP()
		_, _ = e.w.WriteString(text)
	}
	return e.CRLF()
}

// BeginResponse starts an untagged response with the given name.
func (e *Encoder) BeginResponse(name string) *Encoder {
	return e.Star().Atom(name).SP()
}

// NumResponse writes an untagged numeric response (e.g., "* 5 EXISTS").
func (e *Encoder) NumResponse(num uint32, name string) *Encoder {
	return e.Star().Number(num).SP().Atom(name).CRLF()
}

// ContinuationRequest writes a continuation request.
func (e *Encoder) ContinuationRequest(text string) *Encoder {
	e.Plus()
	if text != "" {
		_, _ = e.w.WriteString(text)
	}
	return e.CRLF()
}

// MailboxName writes a mailbox name, quoting if needed.
func (e *Encoder) MailboxName(name string) *Encoder {
	if strings.EqualFold(name, "INBOX") {
		return e.Atom("INBOX")
	}
	return e.AString(name)
}

// ResponseCode writes a response code in brackets.
func (e *Encoder) ResponseCode(code string, args ...interface{}) *Encoder {
	_, _ = e.w.WriteString("[")
	_, _ = e.w.WriteString(code)
	for _, arg := range args {
		_ = e.w.WriteByte(' ')
		fmt.Fprint(e.w, arg)
	}
	_, _ = e.w.WriteString("] ")
	return e
}

// Writer returns the underlying buffered writer.
func (e *Encoder) Writer() *bufio.Writer {
	return e.w
}
