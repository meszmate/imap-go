package utf8accept

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/imaptest/mock"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// utf8MockSession embeds mock.Session and implements SessionUTF8Accept.
type utf8MockSession struct {
	mock.Session
	enableUTF8Called bool
	enableUTF8Err   error
}

func (m *utf8MockSession) EnableUTF8() error {
	m.enableUTF8Called = true
	return m.enableUTF8Err
}

var _ SessionUTF8Accept = (*utf8MockSession)(nil)

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func newTestCtx(t *testing.T, name, args string, sess server.Session) *server.CommandContext {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	// Drain server output to prevent blocking.
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := clientConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	var dec *wire.Decoder
	if args != "" {
		dec = wire.NewDecoder(strings.NewReader(args))
	}

	return &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    name,
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}
}

// newTestCtxWithConn creates a CommandContext where:
// - the arg decoder reads from argLine (the remainder of the command line)
// - the connection's underlying wire has wireData (literal body + trailing bytes)
func newTestCtxWithConn(t *testing.T, name, argLine, wireData string, sess server.Session) (*server.CommandContext, *bytes.Buffer, chan struct{}) {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	// Write wireData to the client side so the server's decoder can read it.
	go func() {
		_, _ = clientConn.Write([]byte(wireData))
	}()

	// Capture server output.
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

	var dec *wire.Decoder
	if argLine != "" {
		dec = wire.NewDecoder(strings.NewReader(argLine))
	}

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    name,
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, &outBuf, done
}

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "UTF8=ACCEPT" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "UTF8=ACCEPT")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapUTF8Accept {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
}

func TestWrapHandler_Commands(t *testing.T) {
	ext := New()
	for _, name := range []string{"ENABLE", "APPEND"} {
		if ext.WrapHandler(name, dummyHandler) == nil {
			t.Errorf("WrapHandler(%q) returned nil, want non-nil", name)
		}
	}
	for _, name := range []string{"FETCH", "SELECT", "SEARCH", "STORE", "LIST"} {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) returned non-nil, want nil", name)
		}
	}
}

func TestSessionExtension(t *testing.T) {
	ext := New()
	se := ext.SessionExtension()
	if se == nil {
		t.Fatal("SessionExtension() returned nil")
	}
	ptr, ok := se.(*SessionUTF8Accept)
	if !ok {
		t.Fatalf("expected *SessionUTF8Accept, got %T", se)
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

// --- ENABLE tests ---

func TestEnable_UTF8Accept(t *testing.T) {
	ext := New()

	sess := &utf8MockSession{}
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		// Simulate what the real ENABLE handler does: add cap to Enabled set.
		ctx.Conn.Enabled().Add(imap.CapUTF8Accept)
		return nil
	})

	h := ext.WrapHandler("ENABLE", original).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "ENABLE", "UTF8=ACCEPT", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original ENABLE handler should have been called")
	}
	if !sess.enableUTF8Called {
		t.Error("EnableUTF8 should have been called on session")
	}
}

func TestEnable_UTF8Accept_NoSessionInterface(t *testing.T) {
	ext := New()

	// Use a plain mock.Session that doesn't implement SessionUTF8Accept.
	sess := &mock.Session{}
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		ctx.Conn.Enabled().Add(imap.CapUTF8Accept)
		return nil
	})

	h := ext.WrapHandler("ENABLE", original).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "ENABLE", "UTF8=ACCEPT", sess)

	// Should not error even though session doesn't implement SessionUTF8Accept.
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnable_OtherCap(t *testing.T) {
	ext := New()

	sess := &utf8MockSession{}
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		// Simulate enabling a different capability.
		ctx.Conn.Enabled().Add(imap.CapCondStore)
		return nil
	})

	h := ext.WrapHandler("ENABLE", original).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "ENABLE", "CONDSTORE", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.enableUTF8Called {
		t.Error("EnableUTF8 should NOT have been called for non-UTF8 capability")
	}
}

func TestEnable_OriginalError(t *testing.T) {
	ext := New()

	sess := &utf8MockSession{}
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return imap.ErrBad("test error")
	})

	h := ext.WrapHandler("ENABLE", original).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "ENABLE", "UTF8=ACCEPT", sess)

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error from original handler")
	}

	if sess.enableUTF8Called {
		t.Error("EnableUTF8 should NOT have been called when original handler fails")
	}
}

// --- APPEND tests ---

func TestAppend_StandardLiteral_Passthrough(t *testing.T) {
	ext := New()

	var gotMailbox string
	var gotOptions *imap.AppendOptions
	var gotData []byte
	sess := &utf8MockSession{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		gotMailbox = mailbox
		gotOptions = options
		gotData, _ = io.ReadAll(r.Reader)
		return &imap.AppendData{UIDValidity: 1, UID: 42}, nil
	}

	literalBody := "Hello World"
	// Arg line: INBOX {11+}
	// Wire: literal body
	argLine := "INBOX {11+}"
	wireData := literalBody

	ctx, _, done := newTestCtxWithConn(t, "APPEND", argLine, wireData, sess)

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if gotMailbox != "INBOX" {
		t.Errorf("mailbox = %q, want %q", gotMailbox, "INBOX")
	}
	if gotOptions == nil {
		t.Fatal("options should not be nil")
	}
	if gotOptions.UTF8 {
		t.Error("UTF8 should be false for standard literal")
	}
	if gotOptions.Binary {
		t.Error("Binary should be false for standard literal")
	}
	if string(gotData) != literalBody {
		t.Errorf("data = %q, want %q", string(gotData), literalBody)
	}
}

func TestAppend_BinaryLiteral_Passthrough(t *testing.T) {
	ext := New()

	var gotOptions *imap.AppendOptions
	sess := &utf8MockSession{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		gotOptions = options
		_, _ = io.ReadAll(r.Reader)
		return &imap.AppendData{}, nil
	}

	literalBody := "binary data!"
	argLine := "INBOX ~{12+}"
	wireData := literalBody

	ctx, _, done := newTestCtxWithConn(t, "APPEND", argLine, wireData, sess)

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if gotOptions == nil {
		t.Fatal("options should not be nil")
	}
	if gotOptions.UTF8 {
		t.Error("UTF8 should be false for binary literal")
	}
	if !gotOptions.Binary {
		t.Error("Binary should be true for ~{N} literal")
	}
}

func TestAppend_UTF8Literal(t *testing.T) {
	ext := New()

	var gotMailbox string
	var gotOptions *imap.AppendOptions
	var gotData []byte
	sess := &utf8MockSession{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		gotMailbox = mailbox
		gotOptions = options
		gotData, _ = io.ReadAll(r.Reader)
		return &imap.AppendData{UIDValidity: 1, UID: 100}, nil
	}

	literalBody := "Hello UTF-8!"
	// Arg line: INBOX UTF8 (~{12+}
	// Wire: literal body + closing paren
	argLine := "INBOX UTF8 (~{12+}"
	wireData := literalBody + ")"

	ctx, _, done := newTestCtxWithConn(t, "APPEND", argLine, wireData, sess)
	// Enable UTF8=ACCEPT on the connection.
	ctx.Conn.Enabled().Add(imap.CapUTF8Accept)

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if gotMailbox != "INBOX" {
		t.Errorf("mailbox = %q, want %q", gotMailbox, "INBOX")
	}
	if gotOptions == nil {
		t.Fatal("options should not be nil")
	}
	if !gotOptions.UTF8 {
		t.Error("UTF8 should be true for UTF8 literal")
	}
	// Note: ~{N+} is literal8 syntax required by RFC 6855, not a binary indicator
	if string(gotData) != literalBody {
		t.Errorf("data = %q, want %q", string(gotData), literalBody)
	}
}

func TestAppend_UTF8NotEnabled(t *testing.T) {
	ext := New()

	sess := &utf8MockSession{}

	// UTF8 literal but UTF8=ACCEPT is NOT enabled.
	argLine := "INBOX UTF8 (~{5+}"
	wireData := "hello)"

	ctx, _, done := newTestCtxWithConn(t, "APPEND", argLine, wireData, sess)
	// Do NOT enable UTF8=ACCEPT.

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	err := h.Handle(ctx)

	_ = ctx.Conn.Close()
	<-done

	if err == nil {
		t.Fatal("expected BAD error when UTF8=ACCEPT not enabled")
	}
	if !strings.Contains(err.Error(), "UTF8=ACCEPT not enabled") {
		t.Errorf("error = %q, want to contain 'UTF8=ACCEPT not enabled'", err.Error())
	}
}

func TestAppend_UTF8WithFlags(t *testing.T) {
	ext := New()

	var gotOptions *imap.AppendOptions
	sess := &utf8MockSession{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		gotOptions = options
		_, _ = io.ReadAll(r.Reader)
		return &imap.AppendData{}, nil
	}

	literalBody := "flagged"
	argLine := "INBOX (Seen Flagged) UTF8 (~{7+}"
	wireData := literalBody + ")"

	ctx, _, done := newTestCtxWithConn(t, "APPEND", argLine, wireData, sess)
	ctx.Conn.Enabled().Add(imap.CapUTF8Accept)

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if gotOptions == nil {
		t.Fatal("options should not be nil")
	}
	if !gotOptions.UTF8 {
		t.Error("UTF8 should be true")
	}
	if len(gotOptions.Flags) != 2 {
		t.Fatalf("expected 2 flags, got %d", len(gotOptions.Flags))
	}
	if gotOptions.Flags[0] != "Seen" || gotOptions.Flags[1] != "Flagged" {
		t.Errorf("flags = %v, want [Seen Flagged]", gotOptions.Flags)
	}
}

func TestAppend_UTF8WithFlagsAndDate(t *testing.T) {
	ext := New()

	var gotOptions *imap.AppendOptions
	sess := &utf8MockSession{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		gotOptions = options
		_, _ = io.ReadAll(r.Reader)
		return &imap.AppendData{}, nil
	}

	literalBody := "dated"
	argLine := `INBOX (Seen) "25-Jan-2024 12:00:00 +0000" UTF8 (~{5+}`
	wireData := literalBody + ")"

	ctx, _, done := newTestCtxWithConn(t, "APPEND", argLine, wireData, sess)
	ctx.Conn.Enabled().Add(imap.CapUTF8Accept)

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if gotOptions == nil {
		t.Fatal("options should not be nil")
	}
	if !gotOptions.UTF8 {
		t.Error("UTF8 should be true")
	}
	if len(gotOptions.Flags) != 1 || gotOptions.Flags[0] != "Seen" {
		t.Errorf("flags = %v, want [Seen]", gotOptions.Flags)
	}
	if gotOptions.InternalDate.IsZero() {
		t.Error("InternalDate should be set")
	}
}

func TestAppend_UTF8Response(t *testing.T) {
	ext := New()

	sess := &utf8MockSession{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		_, _ = io.ReadAll(r.Reader)
		return &imap.AppendData{UIDValidity: 42, UID: 100}, nil
	}

	literalBody := "test"
	argLine := "INBOX UTF8 (~{4+}"
	wireData := literalBody + ")"

	ctx, outBuf, done := newTestCtxWithConn(t, "APPEND", argLine, wireData, sess)
	ctx.Conn.Enabled().Add(imap.CapUTF8Accept)

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "APPENDUID 42 100") {
		t.Errorf("response should contain APPENDUID 42 100, got: %s", output)
	}
	if !strings.Contains(output, "OK") {
		t.Errorf("response should contain OK, got: %s", output)
	}
}

func TestAppend_NilDecoder(t *testing.T) {
	ext := New()

	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	h := ext.WrapHandler("APPEND", original).(server.CommandHandlerFunc)

	sess := &utf8MockSession{}
	ctx := newTestCtx(t, "APPEND", "", sess)
	ctx.Decoder = nil

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called when decoder is nil")
	}
}

// --- readLiteralSize tests ---

func TestReadLiteralSize_Standard(t *testing.T) {
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
