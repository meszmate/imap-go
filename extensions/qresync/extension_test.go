package qresync

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

// qresyncMockSession embeds mock.Session and adds SelectQResync.
type qresyncMockSession struct {
	mock.Session
	selectQResyncCalled bool
	selectQResyncOpts   *imap.SelectOptions
	selectQResyncData   *imap.SelectData
}

func (m *qresyncMockSession) SelectQResync(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
	m.selectQResyncCalled = true
	m.selectQResyncOpts = options
	if m.selectQResyncData != nil {
		return m.selectQResyncData, nil
	}
	return &imap.SelectData{Flags: []imap.Flag{"Seen"}, UIDValidity: 1, UIDNext: 2}, nil
}

var _ SessionQResync = (*qresyncMockSession)(nil)

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

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

func newTestCommandContextWithQResyncEnabled(t *testing.T, args string, sess server.Session) *server.CommandContext {
	t.Helper()
	ctx := newTestCommandContextAuthenticated(t, args, sess)
	ctx.Conn.Enabled().Add(imap.CapQResync)
	return ctx
}

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "QRESYNC" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "QRESYNC")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapQResync {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
	if len(ext.ExtDependencies) != 1 || ext.ExtDependencies[0] != "CONDSTORE" {
		t.Errorf("unexpected dependencies: %v", ext.ExtDependencies)
	}
}

func TestWrapHandler_ReturnsHandlers(t *testing.T) {
	ext := New()
	for _, name := range []string{"SELECT", "EXAMINE", "FETCH"} {
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

func TestSelect_WithoutParams(t *testing.T) {
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
	if gotOpts.QResync != nil {
		t.Error("QResync should be nil")
	}
}

func TestSelect_WithCondStoreOnly(t *testing.T) {
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
	if gotOpts.QResync != nil {
		t.Error("QResync should be nil")
	}
}

func TestSelect_QResyncBasic(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	sess := &qresyncMockSession{}
	ctx := newTestCommandContextWithQResyncEnabled(t, "INBOX (QRESYNC (123 456))", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.selectQResyncCalled {
		t.Fatal("SelectQResync was not called")
	}
	opts := sess.selectQResyncOpts
	if opts.QResync == nil {
		t.Fatal("QResync options should not be nil")
	}
	if opts.QResync.UIDValidity != 123 {
		t.Errorf("UIDValidity = %d, want 123", opts.QResync.UIDValidity)
	}
	if opts.QResync.ModSeq != 456 {
		t.Errorf("ModSeq = %d, want 456", opts.QResync.ModSeq)
	}
	if opts.QResync.KnownUIDs != nil {
		t.Error("KnownUIDs should be nil")
	}
	if opts.QResync.SeqMatch != nil {
		t.Error("SeqMatch should be nil")
	}
	if !opts.CondStore {
		t.Error("CondStore should be implicitly true")
	}
}

func TestSelect_QResyncWithKnownUIDs(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	sess := &qresyncMockSession{}
	ctx := newTestCommandContextWithQResyncEnabled(t, "INBOX (QRESYNC (123 456 1:100))", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.selectQResyncCalled {
		t.Fatal("SelectQResync was not called")
	}
	opts := sess.selectQResyncOpts
	if opts.QResync.KnownUIDs == nil {
		t.Fatal("KnownUIDs should not be nil")
	}
	if opts.QResync.KnownUIDs.String() != "1:100" {
		t.Errorf("KnownUIDs = %q, want %q", opts.QResync.KnownUIDs.String(), "1:100")
	}
	if opts.QResync.SeqMatch != nil {
		t.Error("SeqMatch should be nil")
	}
}

func TestSelect_QResyncWithSeqMatch(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	sess := &qresyncMockSession{}
	ctx := newTestCommandContextWithQResyncEnabled(t, "INBOX (QRESYNC (123 456 1:100 (1,3,5 1,3,5)))", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.selectQResyncCalled {
		t.Fatal("SelectQResync was not called")
	}
	opts := sess.selectQResyncOpts
	if opts.QResync.KnownUIDs == nil {
		t.Fatal("KnownUIDs should not be nil")
	}
	if opts.QResync.SeqMatch == nil {
		t.Fatal("SeqMatch should not be nil")
	}
	if opts.QResync.SeqMatch.SeqNums.String() != "1,3,5" {
		t.Errorf("SeqMatch.SeqNums = %q, want %q", opts.QResync.SeqMatch.SeqNums.String(), "1,3,5")
	}
	if opts.QResync.SeqMatch.UIDs.String() != "1,3,5" {
		t.Errorf("SeqMatch.UIDs = %q, want %q", opts.QResync.SeqMatch.UIDs.String(), "1,3,5")
	}
}

func TestSelect_QResyncNotEnabled(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	sess := &qresyncMockSession{}
	// Authenticated but QRESYNC NOT enabled
	ctx := newTestCommandContextAuthenticated(t, "INBOX (QRESYNC (123 456))", sess)

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error when QRESYNC not enabled")
	}
	if !strings.Contains(err.Error(), "QRESYNC not enabled") {
		t.Errorf("unexpected error: %v", err)
	}
	if sess.selectQResyncCalled {
		t.Error("SelectQResync should not be called")
	}
}

func TestExamine_QResync(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("EXAMINE", dummyHandler).(server.CommandHandlerFunc)

	sess := &qresyncMockSession{}
	ctx := newTestCommandContextWithQResyncEnabled(t, "INBOX (QRESYNC (42 99))", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.selectQResyncCalled {
		t.Fatal("SelectQResync was not called")
	}
	opts := sess.selectQResyncOpts
	if !opts.ReadOnly {
		t.Error("ReadOnly should be true for EXAMINE")
	}
	if opts.QResync.UIDValidity != 42 {
		t.Errorf("UIDValidity = %d, want 42", opts.QResync.UIDValidity)
	}
}

func TestSelect_QResyncFallbackToSelect(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	// Use a plain mock.Session that does NOT implement SessionQResync
	var selectCalled bool
	var gotOpts *imap.SelectOptions
	sess := &mock.Session{
		SelectFunc: func(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
			selectCalled = true
			gotOpts = options
			return &imap.SelectData{Flags: []imap.Flag{"Seen"}, UIDValidity: 1, UIDNext: 2}, nil
		},
	}
	ctx := newTestCommandContextWithQResyncEnabled(t, "INBOX (QRESYNC (123 456))", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !selectCalled {
		t.Fatal("Select should be called as fallback")
	}
	if gotOpts.QResync == nil {
		t.Error("QResync options should still be populated")
	}
	if !gotOpts.CondStore {
		t.Error("CondStore should be implicitly true")
	}
}

func TestSelect_VanishedEarlierResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	vanishedUIDs := &imap.UIDSet{}
	vanishedUIDs.AddRange(5, 10)
	vanishedUIDs.AddNum(15)

	sess := &qresyncMockSession{
		selectQResyncData: &imap.SelectData{
			Flags:         []imap.Flag{"Seen", "Deleted"},
			NumMessages:   10,
			UIDValidity:   123,
			UIDNext:       200,
			HighestModSeq: 500,
			Vanished:      vanishedUIDs,
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
	conn.Enabled().Add(imap.CapQResync)

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
		Decoder: wire.NewDecoder(strings.NewReader("INBOX (QRESYNC (123 456))")),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	serverConn.Close()
	<-done

	output := outBuf.String()

	if !strings.Contains(output, "VANISHED") {
		t.Errorf("response should contain VANISHED, got: %s", output)
	}
	if !strings.Contains(output, "(EARLIER)") {
		t.Errorf("response should contain (EARLIER), got: %s", output)
	}
	if !strings.Contains(output, "HIGHESTMODSEQ") {
		t.Errorf("response should contain HIGHESTMODSEQ, got: %s", output)
	}
	if !strings.Contains(output, "500") {
		t.Errorf("response should contain 500, got: %s", output)
	}
	if !strings.Contains(output, "EXISTS") {
		t.Errorf("response should contain EXISTS, got: %s", output)
	}
}

func TestFetch_WithoutModifier(t *testing.T) {
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
	if gotOpts.Vanished {
		t.Error("Vanished should be false")
	}
}

func TestFetch_WithChangedSinceNoVanished(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1:5 (FLAGS) (CHANGEDSINCE 12345)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if gotOpts.ChangedSince != 12345 {
		t.Errorf("ChangedSince = %d, want 12345", gotOpts.ChangedSince)
	}
	if !gotOpts.ModSeq {
		t.Error("ModSeq should be implicitly true")
	}
	if gotOpts.Vanished {
		t.Error("Vanished should be false")
	}
}

func TestFetch_WithChangedSinceAndVanished(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1:100 (FLAGS) (CHANGEDSINCE 12345 VANISHED)", sess)
	ctx.NumKind = server.NumKindUID
	ctx.Conn.Enabled().Add(imap.CapQResync)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if gotOpts.ChangedSince != 12345 {
		t.Errorf("ChangedSince = %d, want 12345", gotOpts.ChangedSince)
	}
	if !gotOpts.ModSeq {
		t.Error("ModSeq should be implicitly true")
	}
	if !gotOpts.Vanished {
		t.Error("Vanished should be true")
	}
	if !gotOpts.UID {
		t.Error("UID should be true for UID FETCH")
	}
}

func TestFetch_VanishedWithoutQResyncEnabled(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCommandContext(t, "1:100 (FLAGS) (CHANGEDSINCE 12345 VANISHED)", sess)
	ctx.NumKind = server.NumKindUID
	// QRESYNC NOT enabled

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error when QRESYNC not enabled")
	}
	if !strings.Contains(err.Error(), "QRESYNC not enabled") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetch_VanishedWithoutUID(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCommandContext(t, "1:100 (FLAGS) (CHANGEDSINCE 12345 VANISHED)", sess)
	ctx.NumKind = server.NumKindSeq // NOT UID
	ctx.Conn.Enabled().Add(imap.CapQResync)

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error when not UID FETCH")
	}
	if !strings.Contains(err.Error(), "VANISHED requires UID FETCH") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetch_SingleItem(t *testing.T) {
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

func TestSelect_QuotedMailboxWithQResync(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SELECT", dummyHandler).(server.CommandHandlerFunc)

	var gotMailbox string
	sess := &qresyncMockSession{}
	sess.SelectFunc = func(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
		gotMailbox = mailbox
		return &imap.SelectData{Flags: []imap.Flag{"Seen"}, UIDValidity: 1, UIDNext: 2}, nil
	}

	// Quoted mailbox without QRESYNC params
	ctx := newTestCommandContextAuthenticated(t, "\"My Folder\"", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMailbox != "My Folder" {
		t.Errorf("mailbox = %q, want %q", gotMailbox, "My Folder")
	}
}
