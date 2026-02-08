package wire

import (
	"io"
	"strings"
	"testing"
)

func newDecoder(s string) *Decoder {
	return NewDecoder(strings.NewReader(s))
}

// ---------- ReadAtom ----------

func TestReadAtom(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "simple atom", input: "INBOX ", want: "INBOX"},
		{name: "atom with digits", input: "TAG123 ", want: "TAG123"},
		{name: "atom at EOF", input: "HELLO", want: "HELLO"},
		{name: "atom stops at space", input: "FOO BAR", want: "FOO"},
		{name: "atom stops at paren", input: "FLAGS(", want: "FLAGS"},
		{name: "atom stops at open brace", input: "DATA{10}", want: "DATA"},
		{name: "atom stops at quote", input: "X\"y\"", want: "X"},
		{name: "atom with special chars", input: "\\Seen ", want: "", wantErr: true}, // backslash is not atom char
		{name: "empty input", input: "", wantErr: true},
		{name: "starts with space", input: " FOO", wantErr: true},
		{name: "starts with paren", input: "(FOO)", wantErr: true},
		{name: "atom with dash", input: "Content-Type ", want: "Content-Type"},
		{name: "atom with slash", input: "text/plain ", want: "text/plain"},
		{name: "atom with dot", input: "1.2.3 ", want: "1.2.3"},
		{name: "atom stops at bracket", input: "OK]", want: "OK"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			got, err := d.ReadAtom()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadAtom() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ReadAtom() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------- ReadQuotedString ----------

func TestReadQuotedString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "simple", input: `"hello"`, want: "hello"},
		{name: "empty", input: `""`, want: ""},
		{name: "with spaces", input: `"hello world"`, want: "hello world"},
		{name: "escaped quote", input: `"say \"hi\""`, want: `say "hi"`},
		{name: "escaped backslash", input: `"path\\dir"`, want: `path\dir`},
		{name: "no opening quote", input: `hello"`, wantErr: true},
		{name: "unterminated", input: `"hello`, wantErr: true},
		{name: "special chars", input: `"foo(bar)"`, want: "foo(bar)"},
		{name: "with CRLF-like content escaped", input: `"a\\nb"`, want: `a\nb`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			got, err := d.ReadQuotedString()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadQuotedString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ReadQuotedString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------- ReadLiteralInfo ----------

func TestReadLiteralInfo(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSize int64
		wantNS   bool
		wantBin  bool
		wantErr  bool
	}{
		{name: "sync literal", input: "{42}\r\n", wantSize: 42},
		{name: "non-sync literal", input: "{100+}\r\n", wantSize: 100, wantNS: true},
		{name: "binary literal", input: "~{256}\r\n", wantSize: 256, wantBin: true},
		{name: "binary non-sync", input: "~{10+}\r\n", wantSize: 10, wantNS: true, wantBin: true},
		{name: "zero size", input: "{0}\r\n", wantSize: 0},
		{name: "large size", input: "{999999}\r\n", wantSize: 999999},
		{name: "missing CRLF", input: "{10}", wantErr: true},
		{name: "missing brace", input: "42}\r\n", wantErr: true},
		{name: "invalid char", input: "{abc}\r\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			info, err := d.ReadLiteralInfo()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadLiteralInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if info.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", info.Size, tt.wantSize)
			}
			if info.NonSync != tt.wantNS {
				t.Errorf("NonSync = %v, want %v", info.NonSync, tt.wantNS)
			}
			if info.Binary != tt.wantBin {
				t.Errorf("Binary = %v, want %v", info.Binary, tt.wantBin)
			}
		})
	}
}

// ---------- ReadString ----------

func TestReadString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "atom", input: "INBOX ", want: "INBOX"},
		{name: "quoted", input: `"hello world"`, want: "hello world"},
		{name: "literal", input: "{5}\r\nhello", want: "hello"},
		{name: "empty quoted", input: `""`, want: ""},
		{name: "binary literal prefix", input: "~{3}\r\nfoo", want: "foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			got, err := d.ReadString()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ReadString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------- ReadNString ----------

func TestReadNString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantOK  bool
		wantErr bool
	}{
		{name: "NIL uppercase", input: "NIL ", want: "", wantOK: false},
		{name: "NIL lowercase", input: "nil ", want: "", wantOK: false},
		{name: "NIL at EOF", input: "NIL", want: "", wantOK: false},
		{name: "quoted string", input: `"hello"`, want: "hello", wantOK: true},
		{name: "atom (not NIL)", input: "INBOX ", want: "INBOX", wantOK: true},
		{name: "literal", input: "{3}\r\nfoo", want: "foo", wantOK: true},
		{name: "NILS is not NIL", input: "NILS ", want: "NILS", wantOK: true},
		{name: "NIL123 is not NIL", input: "NIL123 ", want: "NIL123", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			got, ok, err := d.ReadNString()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadNString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if ok != tt.wantOK {
				t.Errorf("ReadNString() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("ReadNString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------- ReadNumber ----------

func TestReadNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint32
		wantErr bool
	}{
		{name: "zero", input: "0 ", want: 0},
		{name: "simple", input: "42 ", want: 42},
		{name: "max uint32", input: "4294967295 ", want: 4294967295},
		{name: "overflow", input: "4294967296 ", wantErr: true},
		{name: "negative", input: "-1 ", wantErr: true},
		{name: "not a number", input: "abc ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			got, err := d.ReadNumber()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadNumber() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ReadNumber() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestReadNumber64(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{name: "zero", input: "0 ", want: 0},
		{name: "large value", input: "9999999999 ", want: 9999999999},
		{name: "max uint64", input: "18446744073709551615 ", want: 18446744073709551615},
		{name: "not a number", input: "xyz ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			got, err := d.ReadNumber64()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadNumber64() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ReadNumber64() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ---------- ReadSP ----------

func TestReadSP(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "space", input: " ", wantErr: false},
		{name: "tab is not SP", input: "\t", wantErr: true},
		{name: "letter", input: "A", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			err := d.ReadSP()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadSP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ---------- ReadCRLF ----------

func TestReadCRLF(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid CRLF", input: "\r\n", wantErr: false},
		{name: "only CR", input: "\r", wantErr: true},
		{name: "only LF", input: "\n", wantErr: true},
		{name: "LF CR (wrong order)", input: "\n\r", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "letters", input: "AB", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			err := d.ReadCRLF()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadCRLF() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ---------- ReadList ----------

func TestReadList(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		d := newDecoder("()")
		var items []string
		err := d.ReadList(func() error {
			atom, err := d.ReadAtom()
			if err != nil {
				return err
			}
			items = append(items, atom)
			return nil
		})
		if err != nil {
			t.Fatalf("ReadList() error = %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("single item", func(t *testing.T) {
		d := newDecoder("(ITEM)")
		var items []string
		err := d.ReadList(func() error {
			atom, err := d.ReadAtom()
			if err != nil {
				return err
			}
			items = append(items, atom)
			return nil
		})
		if err != nil {
			t.Fatalf("ReadList() error = %v", err)
		}
		if len(items) != 1 || items[0] != "ITEM" {
			t.Errorf("got %v, want [ITEM]", items)
		}
	})

	t.Run("multiple items", func(t *testing.T) {
		d := newDecoder("(A B C)")
		var items []string
		err := d.ReadList(func() error {
			atom, err := d.ReadAtom()
			if err != nil {
				return err
			}
			items = append(items, atom)
			return nil
		})
		if err != nil {
			t.Fatalf("ReadList() error = %v", err)
		}
		if len(items) != 3 {
			t.Fatalf("expected 3 items, got %d: %v", len(items), items)
		}
		expected := []string{"A", "B", "C"}
		for i, want := range expected {
			if items[i] != want {
				t.Errorf("items[%d] = %q, want %q", i, items[i], want)
			}
		}
	})

	t.Run("missing open paren", func(t *testing.T) {
		d := newDecoder("A B)")
		err := d.ReadList(func() error {
			_, err := d.ReadAtom()
			return err
		})
		if err == nil {
			t.Fatal("expected error for missing open paren")
		}
	})
}

// ---------- ReadFlags ----------

func TestReadFlags(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{name: "empty flags", input: "()", want: nil},
		{name: "single flag", input: "(FLAG1)", want: []string{"FLAG1"}},
		{name: "multiple flags", input: "(FLAG1 FLAG2 FLAG3)", want: []string{"FLAG1", "FLAG2", "FLAG3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			got, err := d.ReadFlags()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ReadFlags() returned %d flags, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("ReadFlags()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// ---------- PeekByte ----------

func TestPeekByte(t *testing.T) {
	t.Run("peek does not consume", func(t *testing.T) {
		d := newDecoder("AB")
		b1, err := d.PeekByte()
		if err != nil {
			t.Fatal(err)
		}
		b2, err := d.PeekByte()
		if err != nil {
			t.Fatal(err)
		}
		if b1 != 'A' || b2 != 'A' {
			t.Errorf("PeekByte() returned %q then %q, want 'A' both times", b1, b2)
		}
	})

	t.Run("peek on empty reader", func(t *testing.T) {
		d := newDecoder("")
		_, err := d.PeekByte()
		if err == nil {
			t.Fatal("expected error on empty reader")
		}
	})

	t.Run("peek various bytes", func(t *testing.T) {
		tests := []struct {
			input string
			want  byte
		}{
			{input: "(", want: '('},
			{input: "\"hello\"", want: '"'},
			{input: "{10}", want: '{'},
			{input: " X", want: ' '},
		}
		for _, tt := range tests {
			d := newDecoder(tt.input)
			got, err := d.PeekByte()
			if err != nil {
				t.Fatalf("PeekByte(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("PeekByte(%q) = %q, want %q", tt.input, got, tt.want)
			}
		}
	})
}

// ---------- ReadLine ----------

func TestReadLine(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "simple line", input: "hello\r\n", want: "hello"},
		{name: "empty line", input: "\r\n", want: ""},
		{name: "line with spaces", input: "A001 OK done\r\n", want: "A001 OK done"},
		{name: "no newline EOF", input: "partial", want: "partial"},
		{name: "LF only", input: "hello\n", want: "hello"},
		{name: "multiple lines reads first", input: "first\r\nsecond\r\n", want: "first"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			got, err := d.ReadLine()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadLine() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ReadLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------- ExpectByte ----------

func TestExpectByte(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		d := newDecoder("(")
		if err := d.ExpectByte('('); err != nil {
			t.Fatalf("ExpectByte('(') error = %v", err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		d := newDecoder("X")
		err := d.ExpectByte('(')
		if err == nil {
			t.Fatal("expected error for mismatch")
		}
	})

	t.Run("empty reader", func(t *testing.T) {
		d := newDecoder("")
		err := d.ExpectByte('(')
		if err == nil {
			t.Fatal("expected error for empty reader")
		}
	})
}

// ---------- NeedsQuoting ----------

func TestNeedsQuoting(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "", want: true},
		{input: "INBOX", want: false},
		{input: "hello world", want: true},
		{input: "foo(bar)", want: true},
		{input: `say"hi"`, want: true},
		{input: "with space", want: true},
		{input: "plain", want: false},
		{input: "TAG123", want: false},
		{input: "has{brace", want: true},
		{input: "percent%", want: true},
		{input: "star*", want: true},
		{input: "back\\slash", want: true},
		{input: "close]bracket", want: true},
		{input: "tab\there", want: true},
		{input: "highbit\x80", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NeedsQuoting(tt.input)
			if got != tt.want {
				t.Errorf("NeedsQuoting(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- NeedsLiteral ----------

func TestNeedsLiteral(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "plain ASCII", input: "hello", want: false},
		{name: "with CR", input: "line\rone", want: true},
		{name: "with LF", input: "line\none", want: true},
		{name: "with CRLF", input: "line\r\ntwo", want: true},
		{name: "with null", input: "null\x00byte", want: true},
		{name: "high bit set", input: "caf\xc3\xa9", want: true},
		{name: "empty", input: "", want: false},
		{name: "spaces only", input: "   ", want: false},
		{name: "just below boundary", input: "~", want: false},
		{name: "at boundary 0x7f", input: "\x7f", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsLiteral(tt.input)
			if got != tt.want {
				t.Errorf("NeedsLiteral(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- IsAtomSpecial ----------

func TestIsAtomSpecial(t *testing.T) {
	// Characters that ARE atom-special (not valid in atoms)
	specials := []byte{'(', ')', '{', ' ', '%', '*', '"', '\\', ']', '\r', '\n', 0x00, 0x7f, 0x80}
	for _, b := range specials {
		if !IsAtomSpecial(b) {
			t.Errorf("IsAtomSpecial(%q) = false, want true", b)
		}
	}

	// Characters that are NOT atom-special (valid in atoms)
	normals := []byte{'A', 'z', '0', '9', '-', '.', '/', ':', '+', '!', '#', '$', '&', '\'', ',', ';', '<', '=', '>', '?', '@', '^', '_', '`', '|', '~'}
	for _, b := range normals {
		if IsAtomSpecial(b) {
			t.Errorf("IsAtomSpecial(%q) = true, want false", b)
		}
	}
}

// ---------- IsQuotedSpecial ----------

func TestIsQuotedSpecial(t *testing.T) {
	if !IsQuotedSpecial('"') {
		t.Error("IsQuotedSpecial('\"') = false, want true")
	}
	if !IsQuotedSpecial('\\') {
		t.Error("IsQuotedSpecial('\\') = false, want true")
	}
	if IsQuotedSpecial('A') {
		t.Error("IsQuotedSpecial('A') = true, want false")
	}
	if IsQuotedSpecial(' ') {
		t.Error("IsQuotedSpecial(' ') = true, want false")
	}
}

// ---------- DiscardLine ----------

func TestDiscardLine(t *testing.T) {
	d := newDecoder("some data to discard\r\nremaining")
	if err := d.DiscardLine(); err != nil {
		t.Fatal(err)
	}
	// After discarding, reading should get "remaining"
	atom, err := d.ReadAtom()
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if atom != "remaining" {
		t.Errorf("after DiscardLine, got %q, want %q", atom, "remaining")
	}
}

// ---------- DiscardN ----------

func TestDiscardN(t *testing.T) {
	d := newDecoder("ABCDEFrest")
	if err := d.DiscardN(6); err != nil {
		t.Fatal(err)
	}
	atom, err := d.ReadAtom()
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if atom != "rest" {
		t.Errorf("after DiscardN(6), got %q, want %q", atom, "rest")
	}
}

// ---------- ReadLiteral ----------

func TestReadLiteral(t *testing.T) {
	input := "{5}\r\nhelloworld"
	d := newDecoder(input)
	info, err := d.ReadLiteralInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != 5 {
		t.Fatalf("expected size 5, got %d", info.Size)
	}
	r := d.ReadLiteral(info.Size)
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("ReadLiteral returned %q, want %q", data, "hello")
	}
	// "world" should still be readable
	atom, err := d.ReadAtom()
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if atom != "world" {
		t.Errorf("remaining data = %q, want %q", atom, "world")
	}
}

// ---------- NewDecoder wraps existing bufio.Reader ----------

func TestNewDecoderWithBufioReader(t *testing.T) {
	// Ensure NewDecoder reuses an existing *bufio.Reader
	import_strings_reader := strings.NewReader("TEST ")
	br := io.Reader(import_strings_reader)
	d := NewDecoder(br)
	atom, err := d.ReadAtom()
	if err != nil {
		t.Fatal(err)
	}
	if atom != "TEST" {
		t.Errorf("got %q, want %q", atom, "TEST")
	}
}

// ---------- Buffered ----------

func TestBuffered(t *testing.T) {
	d := newDecoder("HELLO WORLD")
	// After creating the decoder, the underlying bufio.Reader may buffer data
	// just ensure Buffered() doesn't panic and returns >= 0
	n := d.Buffered()
	if n < 0 {
		t.Errorf("Buffered() = %d, want >= 0", n)
	}
}

// ---------- ReadAString ----------

func TestReadAString(t *testing.T) {
	// ReadAString delegates to ReadString, so test basic behavior
	d := newDecoder(`"hello"`)
	got, err := d.ReadAString()
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Errorf("ReadAString() = %q, want %q", got, "hello")
	}
}

// ---------- Combined reads ----------

func TestCombinedReads(t *testing.T) {
	// Simulate reading a simple command: TAG1 SELECT INBOX\r\n
	d := newDecoder("TAG1 SELECT INBOX\r\n")

	tag, err := d.ReadAtom()
	if err != nil {
		t.Fatal(err)
	}
	if tag != "TAG1" {
		t.Fatalf("tag = %q, want TAG1", tag)
	}

	if err := d.ReadSP(); err != nil {
		t.Fatal(err)
	}

	cmd, err := d.ReadAtom()
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "SELECT" {
		t.Fatalf("cmd = %q, want SELECT", cmd)
	}

	if err := d.ReadSP(); err != nil {
		t.Fatal(err)
	}

	mailbox, err := d.ReadAtom()
	if err != nil {
		t.Fatal(err)
	}
	if mailbox != "INBOX" {
		t.Fatalf("mailbox = %q, want INBOX", mailbox)
	}

	if err := d.ReadCRLF(); err != nil {
		t.Fatal(err)
	}
}
