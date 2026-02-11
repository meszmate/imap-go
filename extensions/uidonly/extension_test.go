package uidonly

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

// uidOnlyMockSession embeds mock.Session and implements SessionUIDOnly.
type uidOnlyMockSession struct {
	mock.Session
	enableCalled bool
	enableErr    error
}

func (m *uidOnlyMockSession) EnableUIDOnly() error {
	m.enableCalled = true
	return m.enableErr
}

var _ SessionUIDOnly = (*uidOnlyMockSession)(nil)

// uidOnlyMoveMockSession adds SessionMove support.
type uidOnlyMoveMockSession struct {
	uidOnlyMockSession
	moveCalled bool
	moveNumSet imap.NumSet
	moveDest   string
	moveErr    error
	moveFunc   func(w *server.MoveWriter, numSet imap.NumSet, dest string) error
}

func (m *uidOnlyMoveMockSession) Move(w *server.MoveWriter, numSet imap.NumSet, dest string) error {
	m.moveCalled = true
	m.moveNumSet = numSet
	m.moveDest = dest
	if m.moveFunc != nil {
		return m.moveFunc(w, numSet, dest)
	}
	return m.moveErr
}

var _ server.SessionMove = (*uidOnlyMoveMockSession)(nil)

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func newTestCtx(t *testing.T, name, args string, sess server.Session) *server.CommandContext {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)

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

func newTestCtxWithOutput(t *testing.T, name, args string, sess server.Session) (*server.CommandContext, *bytes.Buffer, chan struct{}) {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
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
		Name:    name,
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, &outBuf, done
}

// --- Basic extension tests ---

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "UIDONLY" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "UIDONLY")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapUIDOnly {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
	if len(ext.ExtDependencies) != 1 || ext.ExtDependencies[0] != "CONDSTORE" {
		t.Errorf("unexpected dependencies: %v", ext.ExtDependencies)
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
	se := ext.SessionExtension()
	if se == nil {
		t.Fatal("SessionExtension() returned nil")
	}
	if _, ok := se.(*SessionUIDOnly); !ok {
		t.Errorf("SessionExtension() returned %T, want *SessionUIDOnly", se)
	}
}

func TestOnEnabled(t *testing.T) {
	ext := New()
	if err := ext.OnEnabled("test-conn"); err != nil {
		t.Errorf("OnEnabled() returned error: %v", err)
	}
}

// --- WrapHandler coverage ---

func TestWrapHandler_Commands(t *testing.T) {
	ext := New()
	wrapped := []string{
		"ENABLE", "FETCH", "STORE", "COPY", "SEARCH",
		"EXPUNGE", "MOVE", "SORT", "THREAD", "SELECT", "EXAMINE",
	}
	for _, name := range wrapped {
		if ext.WrapHandler(name, dummyHandler) == nil {
			t.Errorf("WrapHandler(%q) returned nil, want non-nil", name)
		}
	}

	unwrapped := []string{"APPEND", "NOOP", "LIST", "STATUS", "CREATE", "DELETE"}
	for _, name := range unwrapped {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) returned non-nil, want nil", name)
		}
	}
}

func TestWrapHandler_InvalidHandler(t *testing.T) {
	ext := New()
	// Pass something that's neither CommandHandlerFunc nor CommandHandler
	if ext.WrapHandler("FETCH", "not a handler") != nil {
		t.Error("WrapHandler with invalid handler should return nil")
	}
}

// --- ENABLE tests ---

func TestEnable_UIDOnly(t *testing.T) {
	ext := New()
	sess := &uidOnlyMockSession{}
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		ctx.Conn.Enabled().Add(imap.CapUIDOnly)
		return nil
	})

	h := ext.WrapHandler("ENABLE", original).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "ENABLE", "UIDONLY", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.enableCalled {
		t.Error("EnableUIDOnly should have been called on session")
	}
}

func TestEnable_UIDOnly_NoSessionInterface(t *testing.T) {
	ext := New()
	sess := &mock.Session{}
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		ctx.Conn.Enabled().Add(imap.CapUIDOnly)
		return nil
	})

	h := ext.WrapHandler("ENABLE", original).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "ENABLE", "UIDONLY", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnable_OtherCap(t *testing.T) {
	ext := New()
	sess := &uidOnlyMockSession{}
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		ctx.Conn.Enabled().Add(imap.CapCondStore)
		return nil
	})

	h := ext.WrapHandler("ENABLE", original).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "ENABLE", "CONDSTORE", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.enableCalled {
		t.Error("EnableUIDOnly should NOT have been called for non-UIDONLY capability")
	}
}

func TestEnable_OriginalError(t *testing.T) {
	ext := New()
	sess := &uidOnlyMockSession{}
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		return imap.ErrBad("test error")
	})

	h := ext.WrapHandler("ENABLE", original).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "ENABLE", "UIDONLY", sess)

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error from original handler")
	}

	if sess.enableCalled {
		t.Error("EnableUIDOnly should NOT have been called when original handler fails")
	}
}

// --- Seq-mode rejection tests ---

func TestSeqMode_Rejected_WhenUIDOnly(t *testing.T) {
	ext := New()
	commands := []string{"FETCH", "STORE", "COPY", "SEARCH", "MOVE", "SORT", "THREAD"}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			h := ext.WrapHandler(cmd, dummyHandler).(server.CommandHandlerFunc)
			sess := &uidOnlyMockSession{}
			ctx := newTestCtx(t, cmd, "1:5 FLAGS", sess)
			ctx.NumKind = server.NumKindSeq
			ctx.Conn.Enabled().Add(imap.CapUIDOnly)

			err := h.Handle(ctx)
			if err == nil {
				t.Fatal("expected UIDREQUIRED error")
			}

			imapErr, ok := err.(*imap.IMAPError)
			if !ok {
				t.Fatalf("expected *imap.IMAPError, got %T", err)
			}
			if imapErr.Code != imap.ResponseCodeUIDRequired {
				t.Errorf("Code = %q, want %q", imapErr.Code, imap.ResponseCodeUIDRequired)
			}
			if imapErr.Type != imap.StatusResponseTypeBAD {
				t.Errorf("Type = %q, want %q", imapErr.Type, imap.StatusResponseTypeBAD)
			}
		})
	}
}

func TestSeqMode_Allowed_WhenNotUIDOnly(t *testing.T) {
	ext := New()
	commands := []string{"FETCH", "STORE", "COPY", "SEARCH", "MOVE", "SORT", "THREAD"}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			originalCalled := false
			original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
				originalCalled = true
				return nil
			})

			h := ext.WrapHandler(cmd, original).(server.CommandHandlerFunc)
			sess := &uidOnlyMockSession{}
			ctx := newTestCtx(t, cmd, "1:5 FLAGS", sess)
			ctx.NumKind = server.NumKindSeq
			// NOT enabling UIDONLY

			if err := h.Handle(ctx); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !originalCalled {
				t.Error("original handler should have been called when UIDONLY not enabled")
			}
		})
	}
}

func TestUIDMode_Passthrough_WhenNotUIDOnly(t *testing.T) {
	ext := New()
	commands := []string{"FETCH", "STORE", "COPY", "SEARCH", "SORT", "THREAD"}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			originalCalled := false
			original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
				originalCalled = true
				return nil
			})

			h := ext.WrapHandler(cmd, original).(server.CommandHandlerFunc)
			sess := &uidOnlyMockSession{}
			ctx := newTestCtx(t, cmd, "1:5 FLAGS", sess)
			ctx.NumKind = server.NumKindUID

			if err := h.Handle(ctx); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !originalCalled {
				t.Error("original handler should have been called in UID mode without UIDONLY")
			}
		})
	}
}

// --- FETCH UIDONLY tests ---

func TestFetch_UIDOnly_UIFetchResponse(t *testing.T) {
	ext := New()
	sess := &uidOnlyMockSession{}
	sess.FetchFunc = func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
		w.WriteFetchData(&imap.FetchMessageData{
			SeqNum: 1,
			UID:    42,
			Flags:  []imap.Flag{imap.FlagSeen},
		})
		return nil
	}

	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)
	ctx, outBuf, done := newTestCtxWithOutput(t, "FETCH", "1:100 FLAGS", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "UIDFETCH") {
		t.Errorf("response should contain UIDFETCH, got: %s", output)
	}
	if !strings.Contains(output, "* 42 UIDFETCH") {
		t.Errorf("response should use UID as number, got: %s", output)
	}
}

func TestFetch_UIDOnly_IncludesUID(t *testing.T) {
	ext := New()
	var gotOptions *imap.FetchOptions
	sess := &uidOnlyMockSession{}
	sess.FetchFunc = func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
		gotOptions = options
		return nil
	}

	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "FETCH", "1:100 FLAGS", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOptions == nil {
		t.Fatal("Fetch should have been called")
	}
	if !gotOptions.UID {
		t.Error("UID option should be set for UID FETCH")
	}
	if !gotOptions.Flags {
		t.Error("Flags option should be set")
	}
}

func TestFetch_UIDOnly_ChangedSince(t *testing.T) {
	ext := New()
	var gotOptions *imap.FetchOptions
	sess := &uidOnlyMockSession{}
	sess.FetchFunc = func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
		gotOptions = options
		return nil
	}

	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "FETCH", "1:100 FLAGS (CHANGEDSINCE 12345)", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOptions == nil {
		t.Fatal("Fetch should have been called")
	}
	if gotOptions.ChangedSince != 12345 {
		t.Errorf("ChangedSince = %d, want 12345", gotOptions.ChangedSince)
	}
	if !gotOptions.ModSeq {
		t.Error("ModSeq should be true")
	}
}

func TestFetch_UIDOnly_Vanished(t *testing.T) {
	ext := New()
	var gotOptions *imap.FetchOptions
	sess := &uidOnlyMockSession{}
	sess.FetchFunc = func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
		gotOptions = options
		return nil
	}

	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "FETCH", "1:100 FLAGS (CHANGEDSINCE 12345 VANISHED)", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)
	ctx.Conn.Enabled().Add(imap.CapQResync)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOptions == nil {
		t.Fatal("Fetch should have been called")
	}
	if !gotOptions.Vanished {
		t.Error("Vanished should be true when QRESYNC is enabled")
	}
}

// --- STORE UIDONLY tests ---

func TestStore_UIDOnly_UIFetchResponse(t *testing.T) {
	ext := New()
	sess := &uidOnlyMockSession{}
	sess.StoreFunc = func(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
		w.WriteFetchData(&imap.FetchMessageData{
			SeqNum: 1,
			UID:    42,
			Flags:  []imap.Flag{imap.FlagSeen},
		})
		return nil
	}

	h := ext.WrapHandler("STORE", dummyHandler).(server.CommandHandlerFunc)
	ctx, outBuf, done := newTestCtxWithOutput(t, "STORE", "1:100 +FLAGS (Seen)", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "UIDFETCH") {
		t.Errorf("response should contain UIDFETCH, got: %s", output)
	}
	if !strings.Contains(output, "* 42 UIDFETCH") {
		t.Errorf("response should use UID as number, got: %s", output)
	}
}

func TestStore_UIDOnly_FlagsParsed(t *testing.T) {
	ext := New()
	var gotFlags *imap.StoreFlags
	sess := &uidOnlyMockSession{}
	sess.StoreFunc = func(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
		gotFlags = flags
		return nil
	}

	h := ext.WrapHandler("STORE", dummyHandler).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "STORE", "1:5 +FLAGS (Seen Flagged)", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotFlags == nil {
		t.Fatal("Store should have been called")
	}
	if gotFlags.Action != imap.StoreFlagsAdd {
		t.Errorf("Action = %v, want StoreFlagsAdd", gotFlags.Action)
	}
	if len(gotFlags.Flags) != 2 {
		t.Fatalf("expected 2 flags, got %d", len(gotFlags.Flags))
	}
}

func TestStore_UIDOnly_Silent(t *testing.T) {
	ext := New()
	var gotFlags *imap.StoreFlags
	sess := &uidOnlyMockSession{}
	sess.StoreFunc = func(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
		gotFlags = flags
		return nil
	}

	h := ext.WrapHandler("STORE", dummyHandler).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "STORE", "1:5 FLAGS.SILENT (Seen)", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotFlags == nil {
		t.Fatal("Store should have been called")
	}
	if gotFlags.Action != imap.StoreFlagsSet {
		t.Errorf("Action = %v, want StoreFlagsSet", gotFlags.Action)
	}
	if !gotFlags.Silent {
		t.Error("Silent should be true")
	}
}

func TestStore_UIDOnly_RemoveFlags(t *testing.T) {
	ext := New()
	var gotFlags *imap.StoreFlags
	sess := &uidOnlyMockSession{}
	sess.StoreFunc = func(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
		gotFlags = flags
		return nil
	}

	h := ext.WrapHandler("STORE", dummyHandler).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "STORE", "1:5 -FLAGS (Deleted)", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotFlags == nil {
		t.Fatal("Store should have been called")
	}
	if gotFlags.Action != imap.StoreFlagsDel {
		t.Errorf("Action = %v, want StoreFlagsDel", gotFlags.Action)
	}
}

// --- EXPUNGE UIDONLY tests ---

func TestExpunge_UIDOnly_VanishedResponse(t *testing.T) {
	ext := New()
	sess := &uidOnlyMockSession{}
	sess.ExpungeFunc = func(w *server.ExpungeWriter, uids *imap.UIDSet) error {
		w.WriteExpunge(42) // In UIDONLY mode this should be a UID
		w.WriteExpunge(43)
		return nil
	}

	h := ext.WrapHandler("EXPUNGE", dummyHandler).(server.CommandHandlerFunc)
	ctx, outBuf, done := newTestCtxWithOutput(t, "EXPUNGE", "", sess)
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "VANISHED") {
		t.Errorf("response should contain VANISHED, got: %s", output)
	}
	if strings.Contains(output, " EXPUNGE\r\n") {
		t.Errorf("response should NOT contain EXPUNGE responses, got: %s", output)
	}
	if !strings.Contains(output, "* VANISHED 42") {
		t.Errorf("response should contain * VANISHED 42, got: %s", output)
	}
}

func TestExpunge_UIDOnly_WithUIDs(t *testing.T) {
	ext := New()
	var gotUIDs *imap.UIDSet
	sess := &uidOnlyMockSession{}
	sess.ExpungeFunc = func(w *server.ExpungeWriter, uids *imap.UIDSet) error {
		gotUIDs = uids
		return nil
	}

	h := ext.WrapHandler("EXPUNGE", dummyHandler).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "EXPUNGE", "1:5", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotUIDs == nil {
		t.Fatal("UIDs should have been passed to Expunge")
	}
	if gotUIDs.String() != "1:5" {
		t.Errorf("UIDs = %s, want 1:5", gotUIDs.String())
	}
}

func TestExpunge_NotUIDOnly_Delegates(t *testing.T) {
	ext := New()
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	h := ext.WrapHandler("EXPUNGE", original).(server.CommandHandlerFunc)
	sess := &uidOnlyMockSession{}
	ctx := newTestCtx(t, "EXPUNGE", "", sess)
	// NOT enabling UIDONLY

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called")
	}
}

// --- MOVE UIDONLY tests ---

func TestMove_UIDOnly_VanishedResponse(t *testing.T) {
	ext := New()
	sess := &uidOnlyMoveMockSession{}
	sess.moveFunc = func(w *server.MoveWriter, numSet imap.NumSet, dest string) error {
		w.WriteExpunge(42)
		return nil
	}

	h := ext.WrapHandler("MOVE", dummyHandler).(server.CommandHandlerFunc)
	ctx, outBuf, done := newTestCtxWithOutput(t, "MOVE", "1:5 Trash", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "VANISHED") {
		t.Errorf("response should contain VANISHED, got: %s", output)
	}
}

func TestMove_UIDOnly_ParsesArgs(t *testing.T) {
	ext := New()
	sess := &uidOnlyMoveMockSession{}

	h := ext.WrapHandler("MOVE", dummyHandler).(server.CommandHandlerFunc)
	ctx := newTestCtx(t, "MOVE", "10:20 Archive", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.moveCalled {
		t.Fatal("Move should have been called")
	}
	if sess.moveNumSet.String() != "10:20" {
		t.Errorf("numSet = %s, want 10:20", sess.moveNumSet.String())
	}
	if sess.moveDest != "Archive" {
		t.Errorf("dest = %q, want %q", sess.moveDest, "Archive")
	}
}

// --- SELECT/EXAMINE UIDONLY tests ---

func TestSelect_UIDOnly_NoParams_Delegates(t *testing.T) {
	ext := New()
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	h := ext.WrapHandler("SELECT", original).(server.CommandHandlerFunc)
	sess := &uidOnlyMockSession{}
	ctx := newTestCtx(t, "SELECT", "INBOX", sess)
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called for SELECT without SeqMatch")
	}
}

func TestSelect_UIDOnly_QResyncNoSeqMatch_Delegates(t *testing.T) {
	ext := New()
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	h := ext.WrapHandler("SELECT", original).(server.CommandHandlerFunc)
	sess := &uidOnlyMockSession{}
	// QRESYNC with uidval+modseq+known-uids but NO SeqMatch
	ctx := newTestCtx(t, "SELECT", "INBOX (QRESYNC (67890007 20050715194045000 41,43:211))", sess)
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called for QRESYNC without SeqMatch")
	}
}

func TestSelect_UIDOnly_QResyncWithSeqMatch_Rejected(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)
	sess := &uidOnlyMockSession{}
	// QRESYNC with SeqMatch (the nested paren pair)
	ctx := newTestCtx(t, "SELECT", "INBOX (QRESYNC (67890007 20050715194045000 41,43:211 (1:5 1,2,3,5,8)))", sess)
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected UIDREQUIRED error for QRESYNC with SeqMatch")
	}

	imapErr, ok := err.(*imap.IMAPError)
	if !ok {
		t.Fatalf("expected *imap.IMAPError, got %T", err)
	}
	if imapErr.Code != imap.ResponseCodeUIDRequired {
		t.Errorf("Code = %q, want %q", imapErr.Code, imap.ResponseCodeUIDRequired)
	}
}

func TestExamine_UIDOnly_QResyncWithSeqMatch_Rejected(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("EXAMINE", dummyHandler).(server.CommandHandlerFunc)
	sess := &uidOnlyMockSession{}
	ctx := newTestCtx(t, "EXAMINE", "INBOX (QRESYNC (67890007 20050715194045000 41,43:211 (1:5 1,2,3,5,8)))", sess)
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected UIDREQUIRED error for EXAMINE with SeqMatch")
	}

	imapErr, ok := err.(*imap.IMAPError)
	if !ok {
		t.Fatalf("expected *imap.IMAPError, got %T", err)
	}
	if imapErr.Code != imap.ResponseCodeUIDRequired {
		t.Errorf("Code = %q, want %q", imapErr.Code, imap.ResponseCodeUIDRequired)
	}
}

func TestSelect_NotUIDOnly_Delegates(t *testing.T) {
	ext := New()
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	h := ext.WrapHandler("SELECT", original).(server.CommandHandlerFunc)
	sess := &uidOnlyMockSession{}
	ctx := newTestCtx(t, "SELECT", "INBOX", sess)
	// NOT enabling UIDONLY

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called when UIDONLY not enabled")
	}
}

func TestSelect_UIDOnly_NilDecoder_Delegates(t *testing.T) {
	ext := New()
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	h := ext.WrapHandler("SELECT", original).(server.CommandHandlerFunc)
	sess := &uidOnlyMockSession{}
	ctx := newTestCtx(t, "SELECT", "", sess)
	ctx.Decoder = nil
	ctx.Conn.Enabled().Add(imap.CapUIDOnly)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called with nil decoder")
	}
}

// --- hasQResyncSeqMatch tests ---

func TestHasQResyncSeqMatch(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"no qresync", "INBOX", false},
		{"condstore only", "INBOX (CONDSTORE)", false},
		{"qresync no seqmatch", "INBOX (QRESYNC (67890007 20050715194045000 41,43:211))", false},
		{"qresync minimal", "INBOX (QRESYNC (12345 67890))", false},
		{"qresync with seqmatch", "INBOX (QRESYNC (67890007 20050715194045000 41,43:211 (1:5 1,2,3,5,8)))", true},
		{"qresync seqmatch no known-uids", "INBOX (QRESYNC (12345 67890 (1:5 1,2,3,5,8)))", true},
		{"lowercase qresync", "INBOX (qresync (12345 67890 1:5 (1:3 1,2,3)))", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasQResyncSeqMatch(tt.raw)
			if got != tt.want {
				t.Errorf("hasQResyncSeqMatch(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

// --- FetchWriter UIDONLY tests ---

func TestFetchWriter_WriteFlags_UIDOnly(t *testing.T) {
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

	w := server.NewFetchWriter(conn.Encoder())
	w.SetUIDOnly(true)
	w.WriteFlags(42, []imap.Flag{imap.FlagSeen})

	_ = conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "UIDFETCH") {
		t.Errorf("expected UIDFETCH in output, got: %s", output)
	}
	if !strings.Contains(output, "* 42 UIDFETCH") {
		t.Errorf("expected * 42 UIDFETCH in output, got: %s", output)
	}
}

func TestFetchWriter_WriteFlags_Normal(t *testing.T) {
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

	w := server.NewFetchWriter(conn.Encoder())
	// NOT setting UIDOnly
	w.WriteFlags(1, []imap.Flag{imap.FlagSeen})

	_ = conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "* 1 FETCH") {
		t.Errorf("expected * 1 FETCH in output, got: %s", output)
	}
	if strings.Contains(output, "UIDFETCH") {
		t.Errorf("should NOT contain UIDFETCH in normal mode, got: %s", output)
	}
}

// --- ExpungeWriter UIDONLY tests ---

func TestExpungeWriter_UIDOnly(t *testing.T) {
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

	w := server.NewExpungeWriter(conn.Encoder())
	w.SetUIDOnly(true)
	w.WriteExpunge(42)

	_ = conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "* VANISHED 42") {
		t.Errorf("expected * VANISHED 42 in output, got: %s", output)
	}
	if strings.Contains(output, "EXPUNGE") {
		t.Errorf("should NOT contain EXPUNGE in UIDONLY mode, got: %s", output)
	}
}

func TestExpungeWriter_Normal(t *testing.T) {
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

	w := server.NewExpungeWriter(conn.Encoder())
	// NOT setting UIDOnly
	w.WriteExpunge(3)

	_ = conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "3 EXPUNGE") {
		t.Errorf("expected 3 EXPUNGE in output, got: %s", output)
	}
	if strings.Contains(output, "VANISHED") {
		t.Errorf("should NOT contain VANISHED in normal mode, got: %s", output)
	}
}
