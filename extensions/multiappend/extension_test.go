package multiappend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/imaptest/mock"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// multiAppendMockSession embeds mock.Session and implements SessionMultiAppend.
type multiAppendMockSession struct {
	mock.Session
	appendMultiFunc func(mailbox string, messages []MultiAppendMessage) ([]*imap.AppendData, error)
}

func (m *multiAppendMockSession) AppendMulti(mailbox string, messages []MultiAppendMessage) ([]*imap.AppendData, error) {
	if m.appendMultiFunc != nil {
		return m.appendMultiFunc(mailbox, messages)
	}
	return nil, nil
}

var _ SessionMultiAppend = (*multiAppendMockSession)(nil)

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

// newTestCtx creates a CommandContext with a response-draining goroutine.
// Use for tests that don't need to inspect server output.
func newTestCtx(t *testing.T, args string, sess server.Session) *server.CommandContext {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	// Drain responses from server
	go func() {
		io.Copy(io.Discard, clientConn)
	}()

	var dec *wire.Decoder
	if args != "" {
		dec = wire.NewDecoder(strings.NewReader(args))
	}

	return &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "APPEND",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}
}

// newPipeCtx creates a CommandContext backed by a net.Pipe.
// The clientConn can be used to write input data (literal bodies) and also has
// a goroutine draining server responses to prevent deadlocks.
// For tests that need to inspect output, use newPipeCtxWithOutput instead.
func newPipeCtx(t *testing.T, args string, sess server.Session) (*server.CommandContext, net.Conn) {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	var dec *wire.Decoder
	if args != "" {
		dec = wire.NewDecoder(strings.NewReader(args))
	}

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "APPEND",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, clientConn
}

// newPipeCtxWithOutput creates a CommandContext backed by a net.Pipe.
// Returns ctx, clientConn for writing, outBuf for captured output, and done channel.
// Close serverConn (via ctx.Conn.Close()) and wait on done to get complete output.
func newPipeCtxWithOutput(t *testing.T, args string, sess server.Session) (*server.CommandContext, net.Conn, *bytes.Buffer, chan struct{}) {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	var outBuf bytes.Buffer
	done := make(chan struct{})
	// This goroutine captures output. Note: with net.Pipe(), reads from clientConn
	// get data written to serverConn (server responses). Writes to clientConn go to
	// serverConn's reader (server's decoder).
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
	if args != "" {
		dec = wire.NewDecoder(strings.NewReader(args))
	}

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "APPEND",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, clientConn, &outBuf, done
}

// --- Basic extension tests ---

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "MULTIAPPEND" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "MULTIAPPEND")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapMultiAppend {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
}

func TestCommandHandlers(t *testing.T) {
	ext := New()
	if ext.CommandHandlers() != nil {
		t.Error("expected nil CommandHandlers")
	}
}

func TestSessionExtension(t *testing.T) {
	ext := New()
	sessExt := ext.SessionExtension()
	if sessExt == nil {
		t.Fatal("SessionExtension() returned nil")
	}
	ptr, ok := sessExt.(*SessionMultiAppend)
	if !ok {
		t.Fatalf("expected *SessionMultiAppend, got %T", sessExt)
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

func TestWrapHandler_InvalidHandler(t *testing.T) {
	ext := New()
	if ext.WrapHandler("APPEND", "not a handler") != nil {
		t.Error("WrapHandler with invalid handler should return nil")
	}
}

func TestWrapHandler_CommandHandler(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("APPEND", dummyHandler)
	if h == nil {
		t.Fatal("WrapHandler(APPEND) returned nil")
	}
	if _, ok := h.(server.CommandHandlerFunc); !ok {
		t.Errorf("expected server.CommandHandlerFunc, got %T", h)
	}
}

// --- Nil decoder delegates to original ---

func TestNilDecoder_Delegates(t *testing.T) {
	ext := New()
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	h := ext.WrapHandler("APPEND", original).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "", &mock.Session{})
	ctx.Decoder = nil

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called with nil decoder")
	}
}

// --- Single message APPEND tests ---

func TestSingleAppend_LiteralOnly(t *testing.T) {
	ext := New()
	var gotMailbox string
	var gotOptions *imap.AppendOptions
	var gotBody []byte

	sess := &mock.Session{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		gotMailbox = mailbox
		gotOptions = options
		gotBody, _ = io.ReadAll(r.Reader)
		return &imap.AppendData{UIDValidity: 1, UID: 10}, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	msgContent := "Hello, World!"
	args := fmt.Sprintf("INBOX {%d}", len(msgContent))

	ctx, clientConn := newPipeCtx(t, args, sess)

	// Write literal body + CRLF, then drain responses
	go func() {
		clientConn.Write([]byte(msgContent + "\r\n"))
		// Drain server responses to prevent deadlock
		io.Copy(io.Discard, clientConn)
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMailbox != "INBOX" {
		t.Errorf("mailbox = %q, want %q", gotMailbox, "INBOX")
	}
	if len(gotOptions.Flags) != 0 {
		t.Errorf("expected no flags, got %v", gotOptions.Flags)
	}
	if !gotOptions.InternalDate.IsZero() {
		t.Errorf("expected zero date, got %v", gotOptions.InternalDate)
	}
	if string(gotBody) != msgContent {
		t.Errorf("body = %q, want %q", string(gotBody), msgContent)
	}
}

func TestSingleAppend_WithFlagsAndDate(t *testing.T) {
	ext := New()
	var gotFlags []imap.Flag
	var gotDate time.Time
	var gotBody []byte

	sess := &mock.Session{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		gotFlags = options.Flags
		gotDate = options.InternalDate
		gotBody, _ = io.ReadAll(r.Reader)
		return &imap.AppendData{UIDValidity: 1, UID: 10}, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	msgContent := "Test message"
	args := fmt.Sprintf(`INBOX (Seen Flagged) "02-Jan-2006 15:04:05 -0700" {%d}`, len(msgContent))

	ctx, clientConn := newPipeCtx(t, args, sess)

	go func() {
		clientConn.Write([]byte(msgContent + "\r\n"))
		io.Copy(io.Discard, clientConn)
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotFlags) != 2 {
		t.Fatalf("expected 2 flags, got %d", len(gotFlags))
	}
	if gotFlags[0] != "Seen" || gotFlags[1] != "Flagged" {
		t.Errorf("flags = %v, want [Seen Flagged]", gotFlags)
	}
	if gotDate.IsZero() {
		t.Error("expected non-zero date")
	}
	if string(gotBody) != msgContent {
		t.Errorf("body = %q, want %q", string(gotBody), msgContent)
	}
}

func TestSingleAppend_WithFlagsOnly(t *testing.T) {
	ext := New()
	var gotFlags []imap.Flag

	sess := &mock.Session{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		gotFlags = options.Flags
		io.ReadAll(r.Reader)
		return &imap.AppendData{UIDValidity: 1, UID: 10}, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	args := `INBOX (Draft) {4}`

	ctx, clientConn := newPipeCtx(t, args, sess)

	go func() {
		clientConn.Write([]byte("test\r\n"))
		io.Copy(io.Discard, clientConn)
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotFlags) != 1 || gotFlags[0] != "Draft" {
		t.Errorf("flags = %v, want [Draft]", gotFlags)
	}
}

func TestSingleAppend_APPENDUID_Response(t *testing.T) {
	ext := New()

	sess := &mock.Session{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		io.ReadAll(r.Reader)
		return &imap.AppendData{UIDValidity: 42, UID: 100}, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)

	msg := "test"
	args := fmt.Sprintf("INBOX {%d}", len(msg))
	ctx, clientConn, outBuf, done := newPipeCtxWithOutput(t, args, sess)

	go func() {
		clientConn.Write([]byte(msg + "\r\n"))
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "APPENDUID 42 100") {
		t.Errorf("response should contain APPENDUID 42 100, got: %s", output)
	}
}

func TestSingleAppend_NoAPPENDUID(t *testing.T) {
	ext := New()

	sess := &mock.Session{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		io.ReadAll(r.Reader)
		return &imap.AppendData{}, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)

	msg := "test"
	args := fmt.Sprintf("INBOX {%d}", len(msg))
	ctx, clientConn, outBuf, done := newPipeCtxWithOutput(t, args, sess)

	go func() {
		clientConn.Write([]byte(msg + "\r\n"))
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if strings.Contains(output, "APPENDUID") {
		t.Errorf("response should NOT contain APPENDUID, got: %s", output)
	}
	if !strings.Contains(output, "OK") {
		t.Errorf("response should contain OK, got: %s", output)
	}
}

// --- Multi-message APPEND tests ---

func TestMultiAppend_TwoMessages(t *testing.T) {
	ext := New()
	var gotMailbox string
	var gotMessages []MultiAppendMessage

	sess := &multiAppendMockSession{}
	sess.appendMultiFunc = func(mailbox string, messages []MultiAppendMessage) ([]*imap.AppendData, error) {
		gotMailbox = mailbox
		gotMessages = messages
		return []*imap.AppendData{
			{UIDValidity: 1, UID: 10},
			{UIDValidity: 1, UID: 11},
		}, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)

	msg1 := "First message"
	msg2 := "Second message"
	args := fmt.Sprintf("INBOX (Seen) {%d}", len(msg1))

	ctx, clientConn := newPipeCtx(t, args, sess)

	go func() {
		clientConn.Write([]byte(msg1))
		fmt.Fprintf(clientConn, " (Flagged) {%d+}\r\n", len(msg2))
		clientConn.Write([]byte(msg2 + "\r\n"))
		io.Copy(io.Discard, clientConn)
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMailbox != "INBOX" {
		t.Errorf("mailbox = %q, want %q", gotMailbox, "INBOX")
	}
	if len(gotMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(gotMessages))
	}

	// Check first message
	body1, _ := io.ReadAll(gotMessages[0].Literal.Reader)
	if string(body1) != msg1 {
		t.Errorf("msg1 body = %q, want %q", string(body1), msg1)
	}
	if len(gotMessages[0].Flags) != 1 || gotMessages[0].Flags[0] != "Seen" {
		t.Errorf("msg1 flags = %v, want [Seen]", gotMessages[0].Flags)
	}

	// Check second message
	body2, _ := io.ReadAll(gotMessages[1].Literal.Reader)
	if string(body2) != msg2 {
		t.Errorf("msg2 body = %q, want %q", string(body2), msg2)
	}
	if len(gotMessages[1].Flags) != 1 || gotMessages[1].Flags[0] != "Flagged" {
		t.Errorf("msg2 flags = %v, want [Flagged]", gotMessages[1].Flags)
	}
}

func TestMultiAppend_ThreeMessages(t *testing.T) {
	ext := New()
	var gotMessages []MultiAppendMessage

	sess := &multiAppendMockSession{}
	sess.appendMultiFunc = func(mailbox string, messages []MultiAppendMessage) ([]*imap.AppendData, error) {
		gotMessages = messages
		results := make([]*imap.AppendData, len(messages))
		for i := range messages {
			results[i] = &imap.AppendData{UIDValidity: 1, UID: imap.UID(10 + i)}
		}
		return results, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)

	msg1 := "First"
	msg2 := "Second"
	msg3 := "Third"
	args := fmt.Sprintf("INBOX {%d}", len(msg1))

	ctx, clientConn := newPipeCtx(t, args, sess)

	go func() {
		clientConn.Write([]byte(msg1))
		fmt.Fprintf(clientConn, " {%d+}\r\n", len(msg2))
		clientConn.Write([]byte(msg2))
		fmt.Fprintf(clientConn, " {%d+}\r\n", len(msg3))
		clientConn.Write([]byte(msg3 + "\r\n"))
		io.Copy(io.Discard, clientConn)
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotMessages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(gotMessages))
	}

	bodies := make([]string, len(gotMessages))
	for i, m := range gotMessages {
		b, _ := io.ReadAll(m.Literal.Reader)
		bodies[i] = string(b)
	}
	if bodies[0] != msg1 || bodies[1] != msg2 || bodies[2] != msg3 {
		t.Errorf("bodies = %v, want [%q, %q, %q]", bodies, msg1, msg2, msg3)
	}
}

func TestMultiAppend_APPENDUID_Response(t *testing.T) {
	ext := New()

	sess := &multiAppendMockSession{}
	sess.appendMultiFunc = func(mailbox string, messages []MultiAppendMessage) ([]*imap.AppendData, error) {
		return []*imap.AppendData{
			{UIDValidity: 42, UID: 100},
			{UIDValidity: 42, UID: 101},
		}, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)

	msg1 := "First"
	msg2 := "Second"
	args := fmt.Sprintf("INBOX {%d}", len(msg1))

	ctx, clientConn, outBuf, done := newPipeCtxWithOutput(t, args, sess)

	go func() {
		clientConn.Write([]byte(msg1))
		fmt.Fprintf(clientConn, " {%d+}\r\n", len(msg2))
		clientConn.Write([]byte(msg2 + "\r\n"))
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "APPENDUID 42 100,101") {
		t.Errorf("response should contain APPENDUID 42 100,101, got: %s", output)
	}
}

// --- No SessionMultiAppend interface ---

func TestMultiAppend_NoSessionInterface(t *testing.T) {
	ext := New()

	sess := &mock.Session{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		io.ReadAll(r.Reader)
		return &imap.AppendData{}, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)

	msg1 := "First"
	msg2 := "Second"
	args := fmt.Sprintf("INBOX {%d}", len(msg1))

	ctx, clientConn, outBuf, done := newPipeCtxWithOutput(t, args, sess)

	go func() {
		clientConn.Write([]byte(msg1))
		fmt.Fprintf(clientConn, " {%d+}\r\n", len(msg2))
		clientConn.Write([]byte(msg2 + "\r\n"))
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "NO") {
		t.Errorf("response should contain NO, got: %s", output)
	}
	if !strings.Contains(output, "MULTIAPPEND not supported") {
		t.Errorf("response should mention MULTIAPPEND not supported, got: %s", output)
	}
}

// --- Error propagation ---

func TestSingleAppend_SessionError(t *testing.T) {
	ext := New()

	sess := &mock.Session{}
	sess.AppendFunc = func(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
		io.ReadAll(r.Reader)
		return nil, imap.ErrNo("mailbox full")
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)
	msg := "test"
	args := fmt.Sprintf("INBOX {%d}", len(msg))

	ctx, clientConn := newPipeCtx(t, args, sess)

	go func() {
		clientConn.Write([]byte(msg + "\r\n"))
		io.Copy(io.Discard, clientConn)
	}()

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error from session")
	}
	if !strings.Contains(err.Error(), "mailbox full") {
		t.Errorf("error = %q, want to contain 'mailbox full'", err.Error())
	}
}

func TestMultiAppend_AppendMultiError(t *testing.T) {
	ext := New()

	sess := &multiAppendMockSession{}
	sess.appendMultiFunc = func(mailbox string, messages []MultiAppendMessage) ([]*imap.AppendData, error) {
		return nil, imap.ErrNo("atomic append failed")
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)

	msg1 := "First"
	msg2 := "Second"
	args := fmt.Sprintf("INBOX {%d}", len(msg1))

	ctx, clientConn := newPipeCtx(t, args, sess)

	go func() {
		clientConn.Write([]byte(msg1))
		fmt.Fprintf(clientConn, " {%d+}\r\n", len(msg2))
		clientConn.Write([]byte(msg2 + "\r\n"))
		io.Copy(io.Discard, clientConn)
	}()

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error from AppendMulti")
	}
	if !strings.Contains(err.Error(), "atomic append failed") {
		t.Errorf("error = %q, want to contain 'atomic append failed'", err.Error())
	}
}

// --- readLiteralSize helper tests ---

func TestReadLiteralSize_Standard(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("{42}"))
	size, err := readLiteralSize(dec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 42 {
		t.Errorf("size = %d, want 42", size)
	}
}

func TestReadLiteralSize_NonSync(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("{42+}"))
	size, err := readLiteralSize(dec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 42 {
		t.Errorf("size = %d, want 42", size)
	}
}

func TestReadLiteralSize_Invalid(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("hello"))
	_, err := readLiteralSize(dec)
	if err == nil {
		t.Fatal("expected error for invalid literal")
	}
}

func TestReadLiteralSize_Empty(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader(""))
	_, err := readLiteralSize(dec)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

// --- parseDate helper tests ---

func TestParseDate_Standard(t *testing.T) {
	_, err := parseDate("02-Jan-2006 15:04:05 -0700")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDate_SingleDigitDay(t *testing.T) {
	_, err := parseDate("2-Jan-2006 15:04:05 -0700")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDate_RFC822Z(t *testing.T) {
	_, err := parseDate("02 Jan 06 15:04 -0700")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDate_Invalid(t *testing.T) {
	_, err := parseDate("not a date")
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

// --- writeMultiAppendOK tests ---

func TestWriteMultiAppendOK_AllValid(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	conn := server.NewTestConn(serverConn, nil)

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

	ctx := &server.CommandContext{
		Tag:  "A001",
		Conn: conn,
	}

	results := []*imap.AppendData{
		{UIDValidity: 42, UID: 100},
		{UIDValidity: 42, UID: 101},
		{UIDValidity: 42, UID: 102},
	}

	writeMultiAppendOK(ctx, results)
	serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "APPENDUID 42 100,101,102") {
		t.Errorf("response should contain APPENDUID 42 100,101,102, got: %s", output)
	}
}

func TestWriteMultiAppendOK_NilResults(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	conn := server.NewTestConn(serverConn, nil)

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

	ctx := &server.CommandContext{
		Tag:  "A001",
		Conn: conn,
	}

	writeMultiAppendOK(ctx, nil)
	serverConn.Close()
	<-done

	output := outBuf.String()
	if strings.Contains(output, "APPENDUID") {
		t.Errorf("response should NOT contain APPENDUID, got: %s", output)
	}
	if !strings.Contains(output, "OK") {
		t.Errorf("response should contain OK, got: %s", output)
	}
}

func TestWriteMultiAppendOK_ZeroUID(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	conn := server.NewTestConn(serverConn, nil)

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

	ctx := &server.CommandContext{
		Tag:  "A001",
		Conn: conn,
	}

	results := []*imap.AppendData{
		{UIDValidity: 42, UID: 100},
		{UIDValidity: 42, UID: 0},
	}

	writeMultiAppendOK(ctx, results)
	serverConn.Close()
	<-done

	output := outBuf.String()
	if strings.Contains(output, "APPENDUID") {
		t.Errorf("response should NOT contain APPENDUID when UID is 0, got: %s", output)
	}
}

// --- Multi-message with date ---

func TestMultiAppend_WithDate(t *testing.T) {
	ext := New()
	var gotMessages []MultiAppendMessage

	sess := &multiAppendMockSession{}
	sess.appendMultiFunc = func(mailbox string, messages []MultiAppendMessage) ([]*imap.AppendData, error) {
		gotMessages = messages
		results := make([]*imap.AppendData, len(messages))
		for i := range messages {
			results[i] = &imap.AppendData{UIDValidity: 1, UID: imap.UID(10 + i)}
		}
		return results, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)

	msg1 := "First"
	msg2 := "Second"
	args := fmt.Sprintf(`INBOX (Seen) "02-Jan-2006 15:04:05 -0700" {%d}`, len(msg1))

	ctx, clientConn := newPipeCtx(t, args, sess)

	go func() {
		clientConn.Write([]byte(msg1))
		fmt.Fprintf(clientConn, " (Flagged) \"02-Jan-2006 15:04:05 -0700\" {%d+}\r\n", len(msg2))
		clientConn.Write([]byte(msg2 + "\r\n"))
		io.Copy(io.Discard, clientConn)
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(gotMessages))
	}

	if len(gotMessages[0].Flags) != 1 || gotMessages[0].Flags[0] != "Seen" {
		t.Errorf("msg1 flags = %v, want [Seen]", gotMessages[0].Flags)
	}
	if gotMessages[0].InternalDate.IsZero() {
		t.Error("msg1 should have non-zero date")
	}

	if len(gotMessages[1].Flags) != 1 || gotMessages[1].Flags[0] != "Flagged" {
		t.Errorf("msg2 flags = %v, want [Flagged]", gotMessages[1].Flags)
	}
	if gotMessages[1].InternalDate.IsZero() {
		t.Error("msg2 should have non-zero date")
	}
}

// --- Multi-message without flags/date ---

func TestMultiAppend_NoFlagsNoDate(t *testing.T) {
	ext := New()
	var gotMessages []MultiAppendMessage

	sess := &multiAppendMockSession{}
	sess.appendMultiFunc = func(mailbox string, messages []MultiAppendMessage) ([]*imap.AppendData, error) {
		gotMessages = messages
		results := make([]*imap.AppendData, len(messages))
		for i := range messages {
			results[i] = &imap.AppendData{UIDValidity: 1, UID: imap.UID(10 + i)}
		}
		return results, nil
	}

	h := ext.WrapHandler("APPEND", dummyHandler).(server.CommandHandlerFunc)

	msg1 := "First"
	msg2 := "Second"
	args := fmt.Sprintf("INBOX {%d}", len(msg1))

	ctx, clientConn := newPipeCtx(t, args, sess)

	go func() {
		clientConn.Write([]byte(msg1))
		fmt.Fprintf(clientConn, " {%d+}\r\n", len(msg2))
		clientConn.Write([]byte(msg2 + "\r\n"))
		io.Copy(io.Discard, clientConn)
	}()

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(gotMessages))
	}

	for i, m := range gotMessages {
		if len(m.Flags) != 0 {
			t.Errorf("msg%d should have no flags, got %v", i+1, m.Flags)
		}
		if !m.InternalDate.IsZero() {
			t.Errorf("msg%d should have zero date, got %v", i+1, m.InternalDate)
		}
	}
}
