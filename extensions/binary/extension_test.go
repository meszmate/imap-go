package binary

import (
	"bytes"
	"net"
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extensions/condstore"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "BINARY" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "BINARY")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapBinary {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
}

func TestWrapHandler_Append(t *testing.T) {
	ext := New()
	if ext.WrapHandler("APPEND", dummyHandler) == nil {
		t.Error("WrapHandler(APPEND) returned nil")
	}
}

func TestWrapHandler_UnknownCommand(t *testing.T) {
	ext := New()
	for _, name := range []string{"FETCH", "LIST", "STORE", "NOOP"} {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) should return nil", name)
		}
	}
}

func TestSessionExtension(t *testing.T) {
	ext := New()
	sessExt := ext.SessionExtension()
	if sessExt == nil {
		t.Fatal("SessionExtension() returned nil")
	}
	ptr, ok := sessExt.(*SessionBinary)
	if !ok {
		t.Fatalf("expected *SessionBinary, got %T", sessExt)
	}
	if ptr != nil {
		t.Error("expected nil pointer")
	}
}

func TestOnEnabled(t *testing.T) {
	ext := New()
	if err := ext.OnEnabled("test-conn"); err != nil {
		t.Fatalf("OnEnabled returned error: %v", err)
	}
}

func TestCommandHandlers(t *testing.T) {
	ext := New()
	if ext.CommandHandlers() != nil {
		t.Error("expected nil CommandHandlers")
	}
}

func TestParseBinaryPart(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"", nil},
		{"1", []int{1}},
		{"1.2", []int{1, 2}},
		{"1.2.3", []int{1, 2, 3}},
	}

	for _, tc := range tests {
		got := condstore.ParseBinaryPart(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("ParseBinaryPart(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("ParseBinaryPart(%q)[%d] = %d, want %d", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestParseSingleFetchItem_Binary(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("BINARY[1]"))
	options := &imap.FetchOptions{}
	if err := condstore.ParseSingleFetchItem(dec, options); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options.BinarySection) != 1 {
		t.Fatalf("expected 1 BinarySection, got %d", len(options.BinarySection))
	}
	s := options.BinarySection[0]
	if s.Peek {
		t.Error("expected Peek=false")
	}
	if len(s.Part) != 1 || s.Part[0] != 1 {
		t.Errorf("expected Part=[1], got %v", s.Part)
	}
}

func TestParseSingleFetchItem_BinaryPeek(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("BINARY.PEEK[1.2]"))
	options := &imap.FetchOptions{}
	if err := condstore.ParseSingleFetchItem(dec, options); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options.BinarySection) != 1 {
		t.Fatalf("expected 1 BinarySection, got %d", len(options.BinarySection))
	}
	s := options.BinarySection[0]
	if !s.Peek {
		t.Error("expected Peek=true")
	}
	if len(s.Part) != 2 || s.Part[0] != 1 || s.Part[1] != 2 {
		t.Errorf("expected Part=[1,2], got %v", s.Part)
	}
}

func TestParseSingleFetchItem_BinarySize(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("BINARY.SIZE[1]"))
	options := &imap.FetchOptions{}
	if err := condstore.ParseSingleFetchItem(dec, options); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options.BinarySizeSection) != 1 {
		t.Fatalf("expected 1 BinarySizeSection, got %d", len(options.BinarySizeSection))
	}
	part := options.BinarySizeSection[0]
	if len(part) != 1 || part[0] != 1 {
		t.Errorf("expected Part=[1], got %v", part)
	}
}

func TestParseSingleFetchItem_BinaryEmpty(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("BINARY[]"))
	options := &imap.FetchOptions{}
	if err := condstore.ParseSingleFetchItem(dec, options); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options.BinarySection) != 1 {
		t.Fatalf("expected 1 BinarySection, got %d", len(options.BinarySection))
	}
	s := options.BinarySection[0]
	if len(s.Part) != 0 {
		t.Errorf("expected empty Part, got %v", s.Part)
	}
}

func TestParseSingleFetchItem_BinaryMultipleParts(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("BINARY.SIZE[1.2.3]"))
	options := &imap.FetchOptions{}
	if err := condstore.ParseSingleFetchItem(dec, options); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options.BinarySizeSection) != 1 {
		t.Fatalf("expected 1 BinarySizeSection, got %d", len(options.BinarySizeSection))
	}
	part := options.BinarySizeSection[0]
	if len(part) != 3 || part[0] != 1 || part[1] != 2 || part[2] != 3 {
		t.Errorf("expected Part=[1,2,3], got %v", part)
	}
}

func TestParseSingleFetchItem_BodyBracket(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("BODY[HEADER]"))
	options := &imap.FetchOptions{}
	if err := condstore.ParseSingleFetchItem(dec, options); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options.BodySection) != 1 {
		t.Fatalf("expected 1 BodySection, got %d", len(options.BodySection))
	}
	s := options.BodySection[0]
	if s.Specifier != "HEADER" {
		t.Errorf("expected Specifier=HEADER, got %q", s.Specifier)
	}
	if s.Peek {
		t.Error("expected Peek=false")
	}
}

func TestParseSingleFetchItem_BodyPeekBracket(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("BODY.PEEK[TEXT]"))
	options := &imap.FetchOptions{}
	if err := condstore.ParseSingleFetchItem(dec, options); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options.BodySection) != 1 {
		t.Fatalf("expected 1 BodySection, got %d", len(options.BodySection))
	}
	s := options.BodySection[0]
	if s.Specifier != "TEXT" {
		t.Errorf("expected Specifier=TEXT, got %q", s.Specifier)
	}
	if !s.Peek {
		t.Error("expected Peek=true")
	}
}

func TestParseSingleFetchItem_BodyEmpty(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("BODY[]"))
	options := &imap.FetchOptions{}
	if err := condstore.ParseSingleFetchItem(dec, options); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options.BodySection) != 1 {
		t.Fatalf("expected 1 BodySection, got %d", len(options.BodySection))
	}
	s := options.BodySection[0]
	if s.Specifier != "" {
		t.Errorf("expected empty Specifier, got %q", s.Specifier)
	}
}

func TestReadLiteralSize_Binary(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("~{42}"))
	size, binary, err := readLiteralSize(dec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 42 {
		t.Errorf("size = %d, want 42", size)
	}
	if !binary {
		t.Error("expected binary=true")
	}
}

func TestReadLiteralSize_NonBinary(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("{100}"))
	size, binary, err := readLiteralSize(dec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 100 {
		t.Errorf("size = %d, want 100", size)
	}
	if binary {
		t.Error("expected binary=false")
	}
}

func TestReadLiteralSize_NonSync(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("{50+}"))
	size, binary, err := readLiteralSize(dec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 50 {
		t.Errorf("size = %d, want 50", size)
	}
	if binary {
		t.Error("expected binary=false")
	}
}

func TestReadLiteralSize_BinaryNonSync(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("~{50+}"))
	size, binary, err := readLiteralSize(dec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 50 {
		t.Errorf("size = %d, want 50", size)
	}
	if !binary {
		t.Error("expected binary=true")
	}
}

func TestWriteFetchData_BinarySize(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	conn := server.NewTestConn(serverConn, nil)
	w := server.NewFetchWriter(conn.Encoder())

	var outBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 8192)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				outBuf.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	data := &imap.FetchMessageData{
		SeqNum: 1,
		UID:    42,
		BinarySizeSection: []imap.BinarySizeData{
			{Part: []int{1}, Size: 1024},
		},
	}

	w.WriteFetchData(data)
	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "BINARY.SIZE[1]") {
		t.Errorf("response should contain BINARY.SIZE[1], got: %s", output)
	}
	if !strings.Contains(output, "1024") {
		t.Errorf("response should contain 1024, got: %s", output)
	}
}

func TestWriteFetchData_BinarySection(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	conn := server.NewTestConn(serverConn, nil)
	w := server.NewFetchWriter(conn.Encoder())

	var outBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 8192)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				outBuf.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	binaryContent := []byte("hello binary")
	section := &imap.FetchItemBinarySection{Part: []int{1, 2}}
	data := &imap.FetchMessageData{
		SeqNum: 1,
		UID:    42,
		BinarySection: map[*imap.FetchItemBinarySection]imap.SectionReader{
			section: {
				Reader: bytes.NewReader(binaryContent),
				Size:   int64(len(binaryContent)),
			},
		},
	}

	w.WriteFetchData(data)
	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "BINARY[1.2]") {
		t.Errorf("response should contain BINARY[1.2], got: %s", output)
	}
	if !strings.Contains(output, "~{12}") {
		t.Errorf("response should contain ~{12} binary literal, got: %s", output)
	}
	if !strings.Contains(output, "hello binary") {
		t.Errorf("response should contain binary content, got: %s", output)
	}
}

func TestWriteFetchData_BinarySizeMultipleParts(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()

	conn := server.NewTestConn(serverConn, nil)
	w := server.NewFetchWriter(conn.Encoder())

	var outBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 8192)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				outBuf.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	data := &imap.FetchMessageData{
		SeqNum: 1,
		BinarySizeSection: []imap.BinarySizeData{
			{Part: []int{1, 2, 3}, Size: 512},
		},
	}

	w.WriteFetchData(data)
	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "BINARY.SIZE[1.2.3]") {
		t.Errorf("response should contain BINARY.SIZE[1.2.3], got: %s", output)
	}
	if !strings.Contains(output, "512") {
		t.Errorf("response should contain 512, got: %s", output)
	}
}
