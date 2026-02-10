package listextended

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

// listExtendedMockSession embeds mock.Session and adds ListExtended.
type listExtendedMockSession struct {
	mock.Session
	listExtendedCalled   bool
	listExtendedRef      string
	listExtendedPatterns []string
	listExtendedOpts     *imap.ListOptions
	listExtendedErr      error
}

func (m *listExtendedMockSession) ListExtended(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
	m.listExtendedCalled = true
	m.listExtendedRef = ref
	m.listExtendedPatterns = patterns
	m.listExtendedOpts = options
	return m.listExtendedErr
}

var _ SessionListExtended = (*listExtendedMockSession)(nil)

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
		Name:    "LIST",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}
}

func newTestCommandContextWithOutput(t *testing.T, args string, sess server.Session) (*server.CommandContext, *bytes.Buffer, chan struct{}) {
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
		Name:    "LIST",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, &outBuf, done
}

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "LIST-EXTENDED" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "LIST-EXTENDED")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapListExtended {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
}

func TestWrapHandler_ReturnsHandler(t *testing.T) {
	ext := New()
	if ext.WrapHandler("LIST", dummyHandler) == nil {
		t.Error("WrapHandler(LIST) returned nil")
	}
}

func TestWrapHandler_UnknownCommand(t *testing.T) {
	ext := New()
	for _, name := range []string{"FETCH", "STORE", "SELECT", "SEARCH"} {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) should return nil", name)
		}
	}
}

func TestListExtended_SelectionOptions(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.ListOptions
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, `(SUBSCRIBED REMOTE) "" "*"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("List was not called")
	}
	if !gotOpts.SelectSubscribed {
		t.Error("SelectSubscribed should be true")
	}
	if !gotOpts.SelectRemote {
		t.Error("SelectRemote should be true")
	}
	if gotOpts.SelectRecursiveMatch {
		t.Error("SelectRecursiveMatch should be false")
	}
}

func TestListExtended_ReturnOptions(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.ListOptions
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, `"" "*" RETURN (SUBSCRIBED CHILDREN)`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("List was not called")
	}
	if !gotOpts.ReturnSubscribed {
		t.Error("ReturnSubscribed should be true")
	}
	if !gotOpts.ReturnChildren {
		t.Error("ReturnChildren should be true")
	}
}

func TestListExtended_MultiplePatterns(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	var gotPatterns []string
	sess := &listExtendedMockSession{}
	sess.ListFunc = func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
		gotPatterns = patterns
		return nil
	}

	// Use selection options to trigger extended path, with multiple patterns
	ctx := newTestCommandContext(t, `(SUBSCRIBED) "" ("INBOX" "Sent")`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Since SessionListExtended is implemented, ListExtended should be called
	if !sess.listExtendedCalled {
		t.Fatal("ListExtended should have been called")
	}
	if len(sess.listExtendedPatterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(sess.listExtendedPatterns))
	}
	if sess.listExtendedPatterns[0] != "INBOX" {
		t.Errorf("pattern[0] = %q, want %q", sess.listExtendedPatterns[0], "INBOX")
	}
	if sess.listExtendedPatterns[1] != "Sent" {
		t.Errorf("pattern[1] = %q, want %q", sess.listExtendedPatterns[1], "Sent")
	}
	_ = gotPatterns
}

func TestListExtended_ReturnStatus(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.ListOptions
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, `"" "*" RETURN (STATUS (MESSAGES UIDNEXT))`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("List was not called")
	}
	if gotOpts.ReturnStatus == nil {
		t.Fatal("ReturnStatus should not be nil")
	}
	if !gotOpts.ReturnStatus.NumMessages {
		t.Error("NumMessages should be true")
	}
	if !gotOpts.ReturnStatus.UIDNext {
		t.Error("UIDNext should be true")
	}
	if gotOpts.ReturnStatus.NumUnseen {
		t.Error("NumUnseen should be false")
	}
}

func TestListExtended_BasicFallback(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	var listCalled bool
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			listCalled = true
			if ref != "" {
				t.Errorf("ref = %q, want empty", ref)
			}
			if len(patterns) != 1 || patterns[0] != "*" {
				t.Errorf("patterns = %v, want [*]", patterns)
			}
			return nil
		},
	}
	ctx := newTestCommandContext(t, `"" "*"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !listCalled {
		t.Fatal("Session.List should be called")
	}
}

func TestListExtended_SessionListExtended(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	sess := &listExtendedMockSession{}
	ctx := newTestCommandContext(t, `(SUBSCRIBED) "" "*" RETURN (CHILDREN)`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.listExtendedCalled {
		t.Fatal("ListExtended should have been called")
	}
	if !sess.listExtendedOpts.SelectSubscribed {
		t.Error("SelectSubscribed should be true")
	}
	if !sess.listExtendedOpts.ReturnChildren {
		t.Error("ReturnChildren should be true")
	}
}

func TestListExtended_FallbackToList(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	var listCalled bool
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			listCalled = true
			return nil
		},
	}
	// Extended syntax (has RETURN) but session doesn't implement SessionListExtended
	ctx := newTestCommandContext(t, `"" "*" RETURN (SUBSCRIBED)`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !listCalled {
		t.Fatal("Session.List should be called as fallback")
	}
}

func TestListExtended_RecursiveMatchRequiresSelection(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCommandContext(t, `(RECURSIVEMATCH) "" "*"`, sess)

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error for RECURSIVEMATCH without other selection option")
	}
	if !strings.Contains(err.Error(), "RECURSIVEMATCH") {
		t.Errorf("error should mention RECURSIVEMATCH, got: %v", err)
	}
}

func TestListExtended_SelectionAndReturn(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	sess := &listExtendedMockSession{}
	ctx := newTestCommandContext(t, `(SUBSCRIBED) "" "*" RETURN (CHILDREN SUBSCRIBED)`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.listExtendedCalled {
		t.Fatal("ListExtended should have been called")
	}
	opts := sess.listExtendedOpts
	if !opts.SelectSubscribed {
		t.Error("SelectSubscribed should be true")
	}
	if !opts.ReturnChildren {
		t.Error("ReturnChildren should be true")
	}
	if !opts.ReturnSubscribed {
		t.Error("ReturnSubscribed should be true")
	}
}

func TestListExtended_SpecialUse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	sess := &listExtendedMockSession{}
	ctx := newTestCommandContext(t, `(SPECIAL-USE) "" "*" RETURN (SPECIAL-USE)`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.listExtendedCalled {
		t.Fatal("ListExtended should have been called")
	}
	if !sess.listExtendedOpts.SelectSpecialUse {
		t.Error("SelectSpecialUse should be true")
	}
	if !sess.listExtendedOpts.ReturnSpecialUse {
		t.Error("ReturnSpecialUse should be true")
	}
}

func TestListExtended_WriteListResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			w.WriteList(&imap.ListData{
				Attrs:   []imap.MailboxAttr{imap.MailboxAttrNoInferiors},
				Delim:   '/',
				Mailbox: "INBOX",
			})
			return nil
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, `"" "*"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "* LIST") {
		t.Errorf("response should contain LIST, got: %s", output)
	}
	if !strings.Contains(output, "INBOX") {
		t.Errorf("response should contain INBOX, got: %s", output)
	}
}

func TestListExtended_WriteExtendedData(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			w.WriteList(&imap.ListData{
				Attrs:     []imap.MailboxAttr{imap.MailboxAttrHasChildren},
				Delim:     '/',
				Mailbox:   "INBOX",
				ChildInfo: []string{"SUBSCRIBED"},
				OldName:   "OldInbox",
			})
			return nil
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, `"" "*" RETURN (CHILDREN)`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "CHILDINFO") {
		t.Errorf("response should contain CHILDINFO, got: %s", output)
	}
	if !strings.Contains(output, "OLDNAME") {
		t.Errorf("response should contain OLDNAME, got: %s", output)
	}
}

func TestListExtended_WriteStatusResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	numMsgs := uint32(42)
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			w.WriteList(&imap.ListData{
				Delim:   '/',
				Mailbox: "INBOX",
				Status: &imap.StatusData{
					Mailbox:     "INBOX",
					NumMessages: &numMsgs,
				},
			})
			return nil
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, `"" "*" RETURN (STATUS (MESSAGES))`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "* LIST") {
		t.Errorf("response should contain LIST, got: %s", output)
	}
	if !strings.Contains(output, "* STATUS") {
		t.Errorf("response should contain STATUS, got: %s", output)
	}
	if !strings.Contains(output, "MESSAGES 42") {
		t.Errorf("response should contain MESSAGES 42, got: %s", output)
	}
}

func TestListExtended_RecursiveMatchWithSubscribed(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	sess := &listExtendedMockSession{}
	ctx := newTestCommandContext(t, `(SUBSCRIBED RECURSIVEMATCH) "" "*"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.listExtendedCalled {
		t.Fatal("ListExtended should have been called")
	}
	if !sess.listExtendedOpts.SelectSubscribed {
		t.Error("SelectSubscribed should be true")
	}
	if !sess.listExtendedOpts.SelectRecursiveMatch {
		t.Error("SelectRecursiveMatch should be true")
	}
}

func TestListExtended_MyRightsReturn(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.ListOptions
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, `"" "*" RETURN (MYRIGHTS)`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("List was not called")
	}
	if !gotOpts.ReturnMyRights {
		t.Error("ReturnMyRights should be true")
	}
}

func TestListExtended_EmptySelectionOptions(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)

	var listCalled bool
	sess := &listExtendedMockSession{}
	sess.ListFunc = func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
		listCalled = true
		return nil
	}

	// Empty selection options ()
	ctx := newTestCommandContext(t, `() "" "*"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty () still triggers extended path, but no selection flags set
	if !sess.listExtendedCalled {
		// If no selection opts set, extended session may or may not be called
		// but List should still work
		if !listCalled {
			t.Fatal("neither ListExtended nor List was called")
		}
	}
}
