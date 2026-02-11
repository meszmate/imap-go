package uidplus

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/imaptest/mock"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// uidplusMockSession embeds mock.Session and adds CopyUIDs and ExpungeUIDs.
type uidplusMockSession struct {
	mock.Session
	copyUIDsCalled    bool
	copyUIDsNumSet    imap.NumSet
	copyUIDsDest      string
	copyUIDsResult    *imap.CopyData
	copyUIDsErr       error
	expungeUIDsCalled bool
	expungeUIDsUIDs   *imap.UIDSet
	expungeUIDsErr    error
}

func (m *uidplusMockSession) CopyUIDs(numSet imap.NumSet, dest string) (*imap.CopyData, error) {
	m.copyUIDsCalled = true
	m.copyUIDsNumSet = numSet
	m.copyUIDsDest = dest
	return m.copyUIDsResult, m.copyUIDsErr
}

func (m *uidplusMockSession) ExpungeUIDs(w *server.ExpungeWriter, uids *imap.UIDSet) error {
	m.expungeUIDsCalled = true
	m.expungeUIDsUIDs = uids
	return m.expungeUIDsErr
}

var _ SessionUIDPlus = (*uidplusMockSession)(nil)

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func newTestCommandContext(t *testing.T, args string, sess server.Session) *server.CommandContext {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

	// Drain output to prevent blocking
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
		Name:    "TEST",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}
}

// newTestCommandContextCapture creates a command context that captures output.
func newTestCommandContextCapture(t *testing.T, args string, sess server.Session) (*server.CommandContext, *bytes.Buffer, chan struct{}) {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

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

	var dec *wire.Decoder
	if args != "" {
		dec = wire.NewDecoder(strings.NewReader(args))
	}

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "TEST",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, &outBuf, done
}

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "UIDPLUS" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "UIDPLUS")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapUIDPlus {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
}

func TestWrapHandler_ReturnsHandlers(t *testing.T) {
	ext := New()
	for _, name := range []string{"COPY", "EXPUNGE"} {
		if ext.WrapHandler(name, dummyHandler) == nil {
			t.Errorf("WrapHandler(%q) returned nil", name)
		}
	}
}

func TestWrapHandler_UnknownCommand(t *testing.T) {
	ext := New()
	for _, name := range []string{"NOOP", "FETCH", "STORE", "APPEND"} {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) should return nil", name)
		}
	}
}

func TestCopy_WithSessionUIDPlus(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("COPY", dummyHandler).(server.CommandHandlerFunc)

	sess := &uidplusMockSession{
		copyUIDsResult: &imap.CopyData{
			UIDValidity: 42,
			SourceUIDs:  imap.UIDSet{Set: []imap.NumRange{{Start: 1, Stop: 3}}},
			DestUIDs:    imap.UIDSet{Set: []imap.NumRange{{Start: 100, Stop: 102}}},
		},
	}
	ctx := newTestCommandContext(t, "1:3 \"Trash\"", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.copyUIDsCalled {
		t.Fatal("CopyUIDs was not called")
	}
	if sess.copyUIDsDest != "Trash" {
		t.Errorf("dest = %q, want %q", sess.copyUIDsDest, "Trash")
	}
}

func TestCopy_WithoutSessionUIDPlus(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("COPY", dummyHandler).(server.CommandHandlerFunc)

	var copyCalled bool
	var gotDest string
	sess := &mock.Session{
		CopyFunc: func(numSet imap.NumSet, dest string) (*imap.CopyData, error) {
			copyCalled = true
			gotDest = dest
			return &imap.CopyData{
				UIDValidity: 10,
				SourceUIDs:  imap.UIDSet{Set: []imap.NumRange{{Start: 5, Stop: 5}}},
				DestUIDs:    imap.UIDSet{Set: []imap.NumRange{{Start: 50, Stop: 50}}},
			}, nil
		},
	}
	ctx := newTestCommandContext(t, "5 INBOX", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !copyCalled {
		t.Fatal("Copy was not called")
	}
	if gotDest != "INBOX" {
		t.Errorf("dest = %q, want %q", gotDest, "INBOX")
	}
}

func TestCopy_NoUIDValidity_PlainOK(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("COPY", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		CopyFunc: func(numSet imap.NumSet, dest string) (*imap.CopyData, error) {
			return &imap.CopyData{}, nil
		},
	}
	ctx := newTestCommandContext(t, "1 INBOX", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCopy_COPYUID_ResponseCode(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("COPY", dummyHandler).(server.CommandHandlerFunc)

	sess := &uidplusMockSession{
		copyUIDsResult: &imap.CopyData{
			UIDValidity: 42,
			SourceUIDs:  imap.UIDSet{Set: []imap.NumRange{{Start: 1, Stop: 3}}},
			DestUIDs:    imap.UIDSet{Set: []imap.NumRange{{Start: 100, Stop: 102}}},
		},
	}

	ctx, outBuf, done := newTestCommandContextCapture(t, "1:3 \"Trash\"", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "COPYUID 42") {
		t.Errorf("response should contain COPYUID 42, got: %s", output)
	}
	if !strings.Contains(output, "COPY completed") {
		t.Errorf("response should contain 'COPY completed', got: %s", output)
	}
}

func TestUIDCopy_WithSessionUIDPlus(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("COPY", dummyHandler).(server.CommandHandlerFunc)

	sess := &uidplusMockSession{
		copyUIDsResult: &imap.CopyData{
			UIDValidity: 99,
			SourceUIDs:  imap.UIDSet{Set: []imap.NumRange{{Start: 10, Stop: 20}}},
			DestUIDs:    imap.UIDSet{Set: []imap.NumRange{{Start: 200, Stop: 210}}},
		},
	}
	ctx := newTestCommandContext(t, "10:20 INBOX", sess)
	ctx.NumKind = server.NumKindUID

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.copyUIDsCalled {
		t.Fatal("CopyUIDs was not called")
	}
	// Verify it was parsed as a UIDSet
	if _, ok := sess.copyUIDsNumSet.(*imap.UIDSet); !ok {
		t.Errorf("numSet should be *UIDSet for UID COPY, got %T", sess.copyUIDsNumSet)
	}
}

func TestExpunge_Plain(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("EXPUNGE", dummyHandler).(server.CommandHandlerFunc)

	var expungeCalled bool
	var gotUIDs *imap.UIDSet
	sess := &mock.Session{
		ExpungeFunc: func(w *server.ExpungeWriter, uids *imap.UIDSet) error {
			expungeCalled = true
			gotUIDs = uids
			return nil
		},
	}
	// Plain EXPUNGE has no arguments
	ctx := newTestCommandContext(t, "", sess)
	ctx.Decoder = nil

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !expungeCalled {
		t.Fatal("Expunge was not called")
	}
	if gotUIDs != nil {
		t.Error("UIDs should be nil for plain EXPUNGE")
	}
}

func TestExpunge_PlainWithSessionUIDPlus(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("EXPUNGE", dummyHandler).(server.CommandHandlerFunc)

	sess := &uidplusMockSession{}
	var baseCalled bool
	sess.ExpungeFunc = func(w *server.ExpungeWriter, uids *imap.UIDSet) error {
		baseCalled = true
		return nil
	}

	// Plain EXPUNGE should call Session.Expunge, NOT ExpungeUIDs
	ctx := newTestCommandContext(t, "", sess)
	ctx.Decoder = nil

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !baseCalled {
		t.Fatal("Session.Expunge should be called for plain EXPUNGE")
	}
	if sess.expungeUIDsCalled {
		t.Error("ExpungeUIDs should NOT be called for plain EXPUNGE")
	}
}

func TestUIDExpunge_WithSessionUIDPlus(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("EXPUNGE", dummyHandler).(server.CommandHandlerFunc)

	sess := &uidplusMockSession{}
	ctx := newTestCommandContext(t, "1:5", sess)
	ctx.NumKind = server.NumKindUID

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.expungeUIDsCalled {
		t.Fatal("ExpungeUIDs was not called")
	}
	if sess.expungeUIDsUIDs == nil {
		t.Fatal("UIDs should not be nil")
	}
	if sess.expungeUIDsUIDs.String() != "1:5" {
		t.Errorf("UIDs = %q, want %q", sess.expungeUIDsUIDs.String(), "1:5")
	}
}

func TestUIDExpunge_WithoutSessionUIDPlus(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("EXPUNGE", dummyHandler).(server.CommandHandlerFunc)

	var expungeCalled bool
	var gotUIDs *imap.UIDSet
	sess := &mock.Session{
		ExpungeFunc: func(w *server.ExpungeWriter, uids *imap.UIDSet) error {
			expungeCalled = true
			gotUIDs = uids
			return nil
		},
	}
	ctx := newTestCommandContext(t, "10:20", sess)
	ctx.NumKind = server.NumKindUID

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !expungeCalled {
		t.Fatal("Expunge should be called as fallback")
	}
	if gotUIDs == nil {
		t.Fatal("UIDs should be passed through")
	}
	if gotUIDs.String() != "10:20" {
		t.Errorf("UIDs = %q, want %q", gotUIDs.String(), "10:20")
	}
}

func TestCopy_MissingArguments(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("COPY", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCommandContext(t, "", sess)
	ctx.Decoder = nil

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error for missing arguments")
	}
}

func TestCopy_MissingDestination(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("COPY", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCommandContext(t, "1:5", sess)

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
}

func TestCopy_PlainOK_NoCOPYUID(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("COPY", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		CopyFunc: func(numSet imap.NumSet, dest string) (*imap.CopyData, error) {
			return &imap.CopyData{}, nil
		},
	}

	ctx, outBuf, done := newTestCommandContextCapture(t, "1 INBOX", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if strings.Contains(output, "COPYUID") {
		t.Errorf("response should NOT contain COPYUID when UIDValidity is 0, got: %s", output)
	}
	if !strings.Contains(output, "OK") {
		t.Errorf("response should contain OK, got: %s", output)
	}
}
