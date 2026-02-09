package condstore

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

// condstoreMockSession embeds mock.Session and adds StoreConditional.
type condstoreMockSession struct {
	mock.Session
	storeConditionalCalled bool
	storeConditionalOpts   *imap.StoreOptions
	storeConditionalFlags  *imap.StoreFlags
	storeConditionalSet    imap.NumSet
}

func (m *condstoreMockSession) StoreConditional(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
	m.storeConditionalCalled = true
	m.storeConditionalOpts = options
	m.storeConditionalFlags = flags
	m.storeConditionalSet = numSet
	return nil
}

var _ SessionCondStore = (*condstoreMockSession)(nil)

func newTestCommandContext(t *testing.T, args string, sess server.Session) *server.CommandContext {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
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

func newTestCommandContextAuthenticated(t *testing.T, args string, sess server.Session) *server.CommandContext {
	t.Helper()
	ctx := newTestCommandContext(t, args, sess)
	if err := ctx.Conn.SetState(imap.ConnStateAuthenticated); err != nil {
		t.Fatalf("failed to set authenticated state: %v", err)
	}
	return ctx
}

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "CONDSTORE" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "CONDSTORE")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapCondStore {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
}

func TestWrapHandler_ReturnsHandlers(t *testing.T) {
	ext := New()
	for _, name := range []string{"STORE", "FETCH", "SELECT", "EXAMINE"} {
		if ext.WrapHandler(name, dummyHandler) == nil {
			t.Errorf("WrapHandler(%q) returned nil", name)
		}
	}
}

func TestWrapHandler_UnknownCommand(t *testing.T) {
	ext := New()
	if ext.WrapHandler("NOOP", dummyHandler) != nil {
		t.Error("WrapHandler(NOOP) should return nil")
	}
}

func TestStore_WithoutUnchangedSince(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("STORE", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.StoreOptions
	var gotFlags *imap.StoreFlags
	sess := &condstoreMockSession{}
	sess.StoreFunc = func(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
		gotOpts = options
		gotFlags = flags
		return nil
	}
	ctx := newTestCommandContext(t, "1:5 +FLAGS (Seen)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Store was not called")
	}
	if gotOpts.UnchangedSince != 0 {
		t.Errorf("UnchangedSince = %d, want 0", gotOpts.UnchangedSince)
	}
	if gotFlags.Action != imap.StoreFlagsAdd {
		t.Errorf("Action = %v, want StoreFlagsAdd", gotFlags.Action)
	}
	if sess.storeConditionalCalled {
		t.Error("StoreConditional should not be called without UNCHANGEDSINCE")
	}
}

func TestStore_WithUnchangedSince(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("STORE", dummyHandler).(server.CommandHandlerFunc)

	sess := &condstoreMockSession{}
	ctx := newTestCommandContext(t, "1:5 (UNCHANGEDSINCE 12345) +FLAGS (Seen)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.storeConditionalCalled {
		t.Fatal("StoreConditional was not called")
	}
	if sess.storeConditionalOpts.UnchangedSince != 12345 {
		t.Errorf("UnchangedSince = %d, want 12345", sess.storeConditionalOpts.UnchangedSince)
	}
	if sess.storeConditionalFlags.Action != imap.StoreFlagsAdd {
		t.Errorf("Action = %v, want StoreFlagsAdd", sess.storeConditionalFlags.Action)
	}
}

func TestStore_UnchangedSince_FlagsSet(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("STORE", dummyHandler).(server.CommandHandlerFunc)

	sess := &condstoreMockSession{}
	ctx := newTestCommandContext(t, "1 (UNCHANGEDSINCE 99) FLAGS (Deleted)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.storeConditionalCalled {
		t.Fatal("StoreConditional was not called")
	}
	if sess.storeConditionalOpts.UnchangedSince != 99 {
		t.Errorf("UnchangedSince = %d, want 99", sess.storeConditionalOpts.UnchangedSince)
	}
	if sess.storeConditionalFlags.Action != imap.StoreFlagsSet {
		t.Errorf("Action = %v, want StoreFlagsSet", sess.storeConditionalFlags.Action)
	}
}

func TestStore_SilentWithUnchangedSince(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("STORE", dummyHandler).(server.CommandHandlerFunc)

	sess := &condstoreMockSession{}
	ctx := newTestCommandContext(t, "1:10 (UNCHANGEDSINCE 500) -FLAGS.SILENT (Flagged)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.storeConditionalCalled {
		t.Fatal("StoreConditional was not called")
	}
	if sess.storeConditionalOpts.UnchangedSince != 500 {
		t.Errorf("UnchangedSince = %d, want 500", sess.storeConditionalOpts.UnchangedSince)
	}
	if sess.storeConditionalFlags.Action != imap.StoreFlagsDel {
		t.Errorf("Action = %v, want StoreFlagsDel", sess.storeConditionalFlags.Action)
	}
	if !sess.storeConditionalFlags.Silent {
		t.Error("Silent should be true")
	}
}

func TestStore_FallbackWithoutSessionCondStore(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("STORE", dummyHandler).(server.CommandHandlerFunc)

	// Use a plain mock.Session that does NOT implement SessionCondStore
	var storeCalled bool
	var gotOpts *imap.StoreOptions
	sess := &mock.Session{
		StoreFunc: func(w *server.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {
			storeCalled = true
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1 (UNCHANGEDSINCE 100) +FLAGS (Seen)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !storeCalled {
		t.Fatal("Store should be called as fallback")
	}
	if gotOpts.UnchangedSince != 100 {
		t.Errorf("UnchangedSince = %d, want 100", gotOpts.UnchangedSince)
	}
}

func TestFetch_WithoutChangedSince(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1:5 (FLAGS UID)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if gotOpts.ChangedSince != 0 {
		t.Errorf("ChangedSince = %d, want 0", gotOpts.ChangedSince)
	}
	if !gotOpts.Flags {
		t.Error("Flags should be true")
	}
	if !gotOpts.UID {
		t.Error("UID should be true")
	}
}

func TestFetch_WithChangedSince(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1:5 (FLAGS) (CHANGEDSINCE 67890)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if gotOpts.ChangedSince != 67890 {
		t.Errorf("ChangedSince = %d, want 67890", gotOpts.ChangedSince)
	}
	if !gotOpts.ModSeq {
		t.Error("ModSeq should be implicitly set to true")
	}
	if !gotOpts.Flags {
		t.Error("Flags should be true")
	}
}

func TestFetch_SingleItemWithChangedSince(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1 ALL (CHANGEDSINCE 100)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if gotOpts.ChangedSince != 100 {
		t.Errorf("ChangedSince = %d, want 100", gotOpts.ChangedSince)
	}
	if !gotOpts.Flags {
		t.Error("Flags should be true (from ALL macro)")
	}
	if !gotOpts.Envelope {
		t.Error("Envelope should be true (from ALL macro)")
	}
}

func TestSelect_WithoutCondStore(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.SelectOptions
	sess := &mock.Session{
		SelectFunc: func(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
			gotOpts = options
			return &imap.SelectData{Flags: []imap.Flag{"Seen"}, UIDValidity: 1, UIDNext: 2}, nil
		},
	}
	ctx := newTestCommandContextAuthenticated(t, "INBOX", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Select was not called")
	}
	if gotOpts.CondStore {
		t.Error("CondStore should be false")
	}
	if gotOpts.ReadOnly {
		t.Error("ReadOnly should be false for SELECT")
	}
}

func TestSelect_WithCondStore(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.SelectOptions
	sess := &mock.Session{
		SelectFunc: func(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
			gotOpts = options
			return &imap.SelectData{Flags: []imap.Flag{"Seen"}, UIDValidity: 1, UIDNext: 2}, nil
		},
	}
	ctx := newTestCommandContextAuthenticated(t, "INBOX (CONDSTORE)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Select was not called")
	}
	if !gotOpts.CondStore {
		t.Error("CondStore should be true")
	}
}

func TestExamine_WithCondStore(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("EXAMINE", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.SelectOptions
	sess := &mock.Session{
		SelectFunc: func(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
			gotOpts = options
			return &imap.SelectData{Flags: []imap.Flag{"Seen"}, UIDValidity: 1, UIDNext: 2}, nil
		},
	}
	ctx := newTestCommandContextAuthenticated(t, "INBOX (CONDSTORE)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Select was not called")
	}
	if !gotOpts.CondStore {
		t.Error("CondStore should be true")
	}
	if !gotOpts.ReadOnly {
		t.Error("ReadOnly should be true for EXAMINE")
	}
}

func TestSelect_QuotedMailboxWithCondStore(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.SelectOptions
	sess := &mock.Session{
		SelectFunc: func(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
			gotOpts = options
			return &imap.SelectData{Flags: []imap.Flag{"Seen"}, UIDValidity: 1, UIDNext: 2}, nil
		},
	}
	ctx := newTestCommandContextAuthenticated(t, "\"My Folder\" (CONDSTORE)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Select was not called")
	}
	if !gotOpts.CondStore {
		t.Error("CondStore should be true")
	}
}

func TestSelect_WithHighestModSeqResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SelectFunc: func(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
			return &imap.SelectData{
				Flags:         []imap.Flag{"Seen"},
				NumMessages:   5,
				UIDValidity:   42,
				UIDNext:       100,
				HighestModSeq: 99999,
			}, nil
		},
	}

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)
	if err := conn.SetState(imap.ConnStateAuthenticated); err != nil {
		t.Fatalf("failed to set authenticated state: %v", err)
	}

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
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SELECT",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("INBOX (CONDSTORE)")),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "HIGHESTMODSEQ") {
		t.Errorf("response should contain HIGHESTMODSEQ, got: %s", output)
	}
	if !strings.Contains(output, "99999") {
		t.Errorf("response should contain 99999, got: %s", output)
	}
}
