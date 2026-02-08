package wire

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func encoderOutput(fn func(e *Encoder)) string {
	var buf bytes.Buffer
	e := NewEncoder(&buf)
	fn(e)
	_ = e.Flush()
	return buf.String()
}

// ---------- Atom ----------

func TestEncoderAtom(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"OK", "OK"},
		{"INBOX", "INBOX"},
		{"FLAGS", "FLAGS"},
		{"", ""},
	}
	for _, tt := range tests {
		got := encoderOutput(func(e *Encoder) { e.Atom(tt.input) })
		if got != tt.want {
			t.Errorf("Atom(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------- SP ----------

func TestEncoderSP(t *testing.T) {
	got := encoderOutput(func(e *Encoder) { e.SP() })
	if got != " " {
		t.Errorf("SP() = %q, want %q", got, " ")
	}
}

// ---------- CRLF ----------

func TestEncoderCRLF(t *testing.T) {
	got := encoderOutput(func(e *Encoder) { e.CRLF() })
	if got != "\r\n" {
		t.Errorf("CRLF() = %q, want %q", got, "\r\n")
	}
}

// ---------- QuotedString ----------

func TestEncoderQuotedString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "hello", `"hello"`},
		{"empty", "", `""`},
		{"with spaces", "hello world", `"hello world"`},
		{"with quote", `say "hi"`, `"say \"hi\""`},
		{"with backslash", `path\dir`, `"path\\dir"`},
		{"both specials", `a"b\c`, `"a\"b\\c"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) { e.QuotedString(tt.input) })
			if got != tt.want {
				t.Errorf("QuotedString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- String ----------

func TestEncoderString(t *testing.T) {
	tests := []struct {
		name string
		input string
		want  string
	}{
		{"atom", "INBOX", "INBOX"},
		{"needs quoting", "hello world", `"hello world"`},
		{"empty needs quoting", "", `""`},
		{"needs literal (CR)", "line\rone", "{8}\r\nline\rone"},
		{"needs literal (LF)", "line\none", "{8}\r\nline\none"},
		{"needs literal (high byte)", "caf\xc3\xa9", "{5}\r\ncaf\xc3\xa9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) { e.String(tt.input) })
			if got != tt.want {
				t.Errorf("String(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- NString ----------

func TestEncoderNString(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		got := encoderOutput(func(e *Encoder) { e.NString(nil) })
		if got != "NIL" {
			t.Errorf("NString(nil) = %q, want %q", got, "NIL")
		}
	})

	t.Run("non-nil atom", func(t *testing.T) {
		s := "INBOX"
		got := encoderOutput(func(e *Encoder) { e.NString(&s) })
		if got != "INBOX" {
			t.Errorf("NString(%q) = %q, want %q", s, got, "INBOX")
		}
	})

	t.Run("non-nil needs quoting", func(t *testing.T) {
		s := "hello world"
		got := encoderOutput(func(e *Encoder) { e.NString(&s) })
		if got != `"hello world"` {
			t.Errorf("NString(%q) = %q, want %q", s, got, `"hello world"`)
		}
	})

	t.Run("non-nil empty", func(t *testing.T) {
		s := ""
		got := encoderOutput(func(e *Encoder) { e.NString(&s) })
		if got != `""` {
			t.Errorf("NString(%q) = %q, want %q", s, got, `""`)
		}
	})
}

// ---------- Nil ----------

func TestEncoderNil(t *testing.T) {
	got := encoderOutput(func(e *Encoder) { e.Nil() })
	if got != "NIL" {
		t.Errorf("Nil() = %q, want %q", got, "NIL")
	}
}

// ---------- Number ----------

func TestEncoderNumber(t *testing.T) {
	tests := []struct {
		input uint32
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{4294967295, "4294967295"},
	}
	for _, tt := range tests {
		got := encoderOutput(func(e *Encoder) { e.Number(tt.input) })
		if got != tt.want {
			t.Errorf("Number(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------- Number64 ----------

func TestEncoderNumber64(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{0, "0"},
		{9999999999, "9999999999"},
		{18446744073709551615, "18446744073709551615"},
	}
	for _, tt := range tests {
		got := encoderOutput(func(e *Encoder) { e.Number64(tt.input) })
		if got != tt.want {
			t.Errorf("Number64(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------- Literal ----------

func TestEncoderLiteral(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"simple", []byte("hello"), "{5}\r\nhello"},
		{"empty", []byte(""), "{0}\r\n"},
		{"binary data", []byte{0x00, 0x01, 0x02}, "{3}\r\n\x00\x01\x02"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) { e.Literal(tt.input) })
			if got != tt.want {
				t.Errorf("Literal(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- LiteralNonSync ----------

func TestEncoderLiteralNonSync(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"simple", []byte("hello"), "{5+}\r\nhello"},
		{"empty", []byte(""), "{0+}\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) { e.LiteralNonSync(tt.input) })
			if got != tt.want {
				t.Errorf("LiteralNonSync(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- List ----------

func TestEncoderList(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"empty list", nil, "()"},
		{"empty slice", []string{}, "()"},
		{"single item", []string{"INBOX"}, "(INBOX)"},
		{"multiple items", []string{"A", "B", "C"}, "(A B C)"},
		{"items needing quoting", []string{"hello world", "INBOX"}, `("hello world" INBOX)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) { e.List(tt.input) })
			if got != tt.want {
				t.Errorf("List(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- Flags ----------

func TestEncoderFlags(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"no flags", nil, "()"},
		{"standard flags", []string{"\\Seen", "\\Answered"}, `("\\Seen" "\\Answered")`},
		{"single flag", []string{"\\Flagged"}, `("\\Flagged")`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) { e.Flags(tt.input) })
			if got != tt.want {
				t.Errorf("Flags(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- Date ----------

func TestEncoderDate(t *testing.T) {
	tm := time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)
	got := encoderOutput(func(e *Encoder) { e.Date(tm) })
	want := `"15-Mar-2024"`
	if got != want {
		t.Errorf("Date() = %q, want %q", got, want)
	}

	// Single digit day
	tm2 := time.Date(2024, time.January, 5, 0, 0, 0, 0, time.UTC)
	got2 := encoderOutput(func(e *Encoder) { e.Date(tm2) })
	want2 := `"05-Jan-2024"`
	if got2 != want2 {
		t.Errorf("Date() = %q, want %q", got2, want2)
	}
}

// ---------- DateTime ----------

func TestEncoderDateTime(t *testing.T) {
	loc := time.FixedZone("EST", -5*3600)
	tm := time.Date(2024, time.March, 15, 14, 30, 45, 0, loc)
	got := encoderOutput(func(e *Encoder) { e.DateTime(tm) })
	want := `"15-Mar-2024 14:30:45 -0500"`
	if got != want {
		t.Errorf("DateTime() = %q, want %q", got, want)
	}

	// UTC
	tmUTC := time.Date(2024, time.December, 25, 0, 0, 0, 0, time.UTC)
	gotUTC := encoderOutput(func(e *Encoder) { e.DateTime(tmUTC) })
	wantUTC := `"25-Dec-2024 00:00:00 +0000"`
	if gotUTC != wantUTC {
		t.Errorf("DateTime(UTC) = %q, want %q", gotUTC, wantUTC)
	}
}

// ---------- Tag ----------

func TestEncoderTag(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"A001", "A001"},
		{"TAG1", "TAG1"},
		{"*", "*"},
	}
	for _, tt := range tests {
		got := encoderOutput(func(e *Encoder) { e.Tag(tt.input) })
		if got != tt.want {
			t.Errorf("Tag(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------- Star ----------

func TestEncoderStar(t *testing.T) {
	got := encoderOutput(func(e *Encoder) { e.Star() })
	if got != "* " {
		t.Errorf("Star() = %q, want %q", got, "* ")
	}
}

// ---------- Plus ----------

func TestEncoderPlus(t *testing.T) {
	got := encoderOutput(func(e *Encoder) { e.Plus() })
	if got != "+ " {
		t.Errorf("Plus() = %q, want %q", got, "+ ")
	}
}

// ---------- StatusResponse ----------

func TestEncoderStatusResponse(t *testing.T) {
	tests := []struct {
		name       string
		tag        string
		status     string
		code       string
		text       string
		want       string
	}{
		{
			name:   "untagged OK",
			tag:    "*",
			status: "OK",
			code:   "",
			text:   "server ready",
			want:   "* OK server ready\r\n",
		},
		{
			name:   "tagged OK",
			tag:    "A001",
			status: "OK",
			code:   "",
			text:   "completed",
			want:   "A001 OK completed\r\n",
		},
		{
			name:   "tagged OK with code",
			tag:    "A001",
			status: "OK",
			code:   "READ-WRITE",
			text:   "SELECT completed",
			want:   "A001 OK [READ-WRITE] SELECT completed\r\n",
		},
		{
			name:   "tagged NO",
			tag:    "A002",
			status: "NO",
			code:   "",
			text:   "command failed",
			want:   "A002 NO command failed\r\n",
		},
		{
			name:   "tagged BAD",
			tag:    "A003",
			status: "BAD",
			code:   "",
			text:   "syntax error",
			want:   "A003 BAD syntax error\r\n",
		},
		{
			name:   "empty tag (untagged)",
			tag:    "",
			status: "BYE",
			code:   "",
			text:   "server shutting down",
			want:   "* BYE server shutting down\r\n",
		},
		{
			name:   "with code no text",
			tag:    "A001",
			status: "OK",
			code:   "READ-ONLY",
			text:   "",
			want:   "A001 OK [READ-ONLY]\r\n",
		},
		{
			name:   "no code no text",
			tag:    "A001",
			status: "OK",
			code:   "",
			text:   "",
			want:   "A001 OK\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) {
				e.StatusResponse(tt.tag, tt.status, tt.code, tt.text)
			})
			if got != tt.want {
				t.Errorf("StatusResponse(%q, %q, %q, %q) = %q, want %q",
					tt.tag, tt.status, tt.code, tt.text, got, tt.want)
			}
		})
	}
}

// ---------- NumResponse ----------

func TestEncoderNumResponse(t *testing.T) {
	tests := []struct {
		num  uint32
		name string
		want string
	}{
		{5, "EXISTS", "* 5 EXISTS\r\n"},
		{3, "RECENT", "* 3 RECENT\r\n"},
		{0, "EXPUNGE", "* 0 EXPUNGE\r\n"},
		{100, "FETCH", "* 100 FETCH\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) { e.NumResponse(tt.num, tt.name) })
			if got != tt.want {
				t.Errorf("NumResponse(%d, %q) = %q, want %q", tt.num, tt.name, got, tt.want)
			}
		})
	}
}

// ---------- ContinuationRequest ----------

func TestEncoderContinuationRequest(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"with text", "ready for literal", "+ ready for literal\r\n"},
		{"empty text", "", "+ \r\n"},
		{"base64 challenge", "AAAAAA==", "+ AAAAAA==\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) { e.ContinuationRequest(tt.text) })
			if got != tt.want {
				t.Errorf("ContinuationRequest(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

// ---------- MailboxName ----------

func TestEncoderMailboxName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"INBOX canonical", "INBOX", "INBOX"},
		{"inbox lowercase", "inbox", "INBOX"},
		{"Inbox mixed", "Inbox", "INBOX"},
		{"other mailbox atom", "Drafts", "Drafts"},
		{"other mailbox needs quoting", "Sent Items", `"Sent Items"`},
		{"nested mailbox", "INBOX.Subfolder", "INBOX.Subfolder"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) { e.MailboxName(tt.input) })
			if got != tt.want {
				t.Errorf("MailboxName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- ResponseCode ----------

func TestEncoderResponseCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		args []interface{}
		want string
	}{
		{"simple code", "UIDVALIDITY", []interface{}{12345}, "[UIDVALIDITY 12345] "},
		{"no args", "READ-WRITE", nil, "[READ-WRITE] "},
		{"multiple args", "APPENDUID", []interface{}{100, 200}, "[APPENDUID 100 200] "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encoderOutput(func(e *Encoder) { e.ResponseCode(tt.code, tt.args...) })
			if got != tt.want {
				t.Errorf("ResponseCode(%q, %v) = %q, want %q", tt.code, tt.args, got, tt.want)
			}
		})
	}
}

// ---------- BeginList / EndList ----------

func TestEncoderBeginEndList(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		got := encoderOutput(func(e *Encoder) { e.BeginList().EndList() })
		if got != "()" {
			t.Errorf("BeginList().EndList() = %q, want %q", got, "()")
		}
	})

	t.Run("list with content", func(t *testing.T) {
		got := encoderOutput(func(e *Encoder) {
			e.BeginList().Atom("A").SP().Atom("B").EndList()
		})
		if got != "(A B)" {
			t.Errorf("BeginList()...EndList() = %q, want %q", got, "(A B)")
		}
	})

	t.Run("nested lists", func(t *testing.T) {
		got := encoderOutput(func(e *Encoder) {
			e.BeginList().
				Atom("BODY").SP().
				BeginList().Atom("TEXT").SP().Atom("PLAIN").EndList().
				EndList()
		})
		if got != "(BODY (TEXT PLAIN))" {
			t.Errorf("nested lists = %q, want %q", got, "(BODY (TEXT PLAIN))")
		}
	})
}

// ---------- Raw / RawString ----------

func TestEncoderRaw(t *testing.T) {
	got := encoderOutput(func(e *Encoder) { e.Raw([]byte("raw bytes")) })
	if got != "raw bytes" {
		t.Errorf("Raw() = %q, want %q", got, "raw bytes")
	}
}

func TestEncoderRawString(t *testing.T) {
	got := encoderOutput(func(e *Encoder) { e.RawString("raw string") })
	if got != "raw string" {
		t.Errorf("RawString() = %q, want %q", got, "raw string")
	}
}

// ---------- AString ----------

func TestEncoderAString(t *testing.T) {
	// AString delegates to String
	got := encoderOutput(func(e *Encoder) { e.AString("INBOX") })
	if got != "INBOX" {
		t.Errorf("AString(INBOX) = %q, want %q", got, "INBOX")
	}

	got2 := encoderOutput(func(e *Encoder) { e.AString("hello world") })
	if got2 != `"hello world"` {
		t.Errorf("AString(hello world) = %q, want %q", got2, `"hello world"`)
	}
}

// ---------- BeginResponse ----------

func TestEncoderBeginResponse(t *testing.T) {
	got := encoderOutput(func(e *Encoder) { e.BeginResponse("FLAGS") })
	if got != "* FLAGS " {
		t.Errorf("BeginResponse(FLAGS) = %q, want %q", got, "* FLAGS ")
	}
}

// ---------- LiteralWriter (encoder method) ----------

func TestEncoderLiteralWriter(t *testing.T) {
	t.Run("sync literal writer", func(t *testing.T) {
		var buf bytes.Buffer
		e := NewEncoder(&buf)
		w := e.LiteralWriter(5, false)
		_, err := w.Write([]byte("hello"))
		if err != nil {
			t.Fatal(err)
		}
		_ = e.Flush()
		got := buf.String()
		if !strings.HasPrefix(got, "{5}\r\n") {
			t.Errorf("LiteralWriter header wrong: %q", got)
		}
		if !strings.HasSuffix(got, "hello") {
			t.Errorf("LiteralWriter data wrong: %q", got)
		}
	})

	t.Run("non-sync literal writer", func(t *testing.T) {
		var buf bytes.Buffer
		e := NewEncoder(&buf)
		w := e.LiteralWriter(3, true)
		_, err := w.Write([]byte("abc"))
		if err != nil {
			t.Fatal(err)
		}
		_ = e.Flush()
		got := buf.String()
		if !strings.HasPrefix(got, "{3+}\r\n") {
			t.Errorf("LiteralWriter non-sync header wrong: %q", got)
		}
	})
}

// ---------- Fluent API chain ----------

func TestEncoderFluentChaining(t *testing.T) {
	got := encoderOutput(func(e *Encoder) {
		e.Tag("A001").SP().Atom("OK").SP().Atom("done").CRLF()
	})
	want := "A001 OK done\r\n"
	if got != want {
		t.Errorf("fluent chain = %q, want %q", got, want)
	}
}

// ---------- NewEncoder with existing bufio.Writer ----------

func TestNewEncoderWithBufioWriter(t *testing.T) {
	var buf bytes.Buffer
	// Passing a raw writer should work
	e := NewEncoder(&buf)
	e.Atom("TEST")
	_ = e.Flush()
	if buf.String() != "TEST" {
		t.Errorf("got %q, want %q", buf.String(), "TEST")
	}
}

// ---------- Writer ----------

func TestEncoderWriter(t *testing.T) {
	var buf bytes.Buffer
	e := NewEncoder(&buf)
	w := e.Writer()
	if w == nil {
		t.Fatal("Writer() returned nil")
	}
	_, _ = w.WriteString("direct")
	_ = w.Flush()
	if buf.String() != "direct" {
		t.Errorf("Writer().WriteString() = %q, want %q", buf.String(), "direct")
	}
}

// ---------- Flush ----------

func TestEncoderFlush(t *testing.T) {
	var buf bytes.Buffer
	e := NewEncoder(&buf)
	e.Atom("DATA")
	// Before flush, data might be buffered
	err := e.Flush()
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if buf.String() != "DATA" {
		t.Errorf("after Flush() = %q, want %q", buf.String(), "DATA")
	}
}

// ---------- Complex IMAP response ----------

func TestEncoderComplexResponse(t *testing.T) {
	got := encoderOutput(func(e *Encoder) {
		// * 1 FETCH (FLAGS (\Seen) BODY[HEADER] {11}\r\nhello world)
		e.Star().Number(1).SP().Atom("FETCH").SP().
			BeginList().
			Atom("FLAGS").SP().
			BeginList().Atom("\\Seen").EndList().SP().
			Atom("BODY[HEADER]").SP().
			Literal([]byte("hello world")).
			EndList().CRLF()
	})
	want := "* 1 FETCH (FLAGS (\\Seen) BODY[HEADER] {11}\r\nhello world)\r\n"
	if got != want {
		t.Errorf("complex response:\ngot  %q\nwant %q", got, want)
	}
}
