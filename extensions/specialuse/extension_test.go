package specialuse

import (
	"context"
	"net"
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/imaptest/mock"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// specialUseMockSession embeds mock.Session and adds CreateSpecialUse.
type specialUseMockSession struct {
	mock.Session
	createSpecialUseCalled  bool
	createSpecialUseMailbox string
	createSpecialUseOptions *imap.CreateOptions
	createSpecialUseErr     error
}

func (m *specialUseMockSession) ListSpecialUse(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
	return nil
}

func (m *specialUseMockSession) CreateSpecialUse(mailbox string, options *imap.CreateOptions) error {
	m.createSpecialUseCalled = true
	m.createSpecialUseMailbox = mailbox
	m.createSpecialUseOptions = options
	return m.createSpecialUseErr
}

var _ SessionSpecialUse = (*specialUseMockSession)(nil)

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
		Name:    "CREATE",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}
}

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "SPECIAL-USE" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "SPECIAL-USE")
	}
	if len(ext.ExtCapabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(ext.ExtCapabilities))
	}
	if ext.ExtCapabilities[0] != imap.CapSpecialUse {
		t.Errorf("capability[0] = %q, want %q", ext.ExtCapabilities[0], imap.CapSpecialUse)
	}
	if ext.ExtCapabilities[1] != imap.CapCreateSpecialUse {
		t.Errorf("capability[1] = %q, want %q", ext.ExtCapabilities[1], imap.CapCreateSpecialUse)
	}
}

func TestWrapHandler_Create(t *testing.T) {
	ext := New()
	if ext.WrapHandler("CREATE", dummyHandler) == nil {
		t.Error("WrapHandler(CREATE) returned nil")
	}
}

func TestWrapHandler_UnknownCommand(t *testing.T) {
	ext := New()
	for _, name := range []string{"LIST", "NOOP", "FETCH", "EXPUNGE"} {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) should return nil", name)
		}
	}
}

func TestCreateSpecialUse_ParseUse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("CREATE", dummyHandler).(server.CommandHandlerFunc)

	sess := &specialUseMockSession{}
	ctx := newTestCommandContext(t, "Sent (USE (\\Sent))", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.createSpecialUseCalled {
		t.Fatal("CreateSpecialUse was not called")
	}
	if sess.createSpecialUseMailbox != "Sent" {
		t.Errorf("mailbox = %q, want %q", sess.createSpecialUseMailbox, "Sent")
	}
	if sess.createSpecialUseOptions.SpecialUse != imap.MailboxAttrSent {
		t.Errorf("SpecialUse = %q, want %q", sess.createSpecialUseOptions.SpecialUse, imap.MailboxAttrSent)
	}
}

func TestCreateSpecialUse_NoOptions(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("CREATE", dummyHandler).(server.CommandHandlerFunc)

	var createCalled bool
	var gotMailbox string
	var gotOptions *imap.CreateOptions
	sess := &mock.Session{
		CreateFunc: func(mailbox string, options *imap.CreateOptions) error {
			createCalled = true
			gotMailbox = mailbox
			gotOptions = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "MyFolder", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !createCalled {
		t.Fatal("Create was not called")
	}
	if gotMailbox != "MyFolder" {
		t.Errorf("mailbox = %q, want %q", gotMailbox, "MyFolder")
	}
	if gotOptions.SpecialUse != "" {
		t.Errorf("SpecialUse should be empty, got %q", gotOptions.SpecialUse)
	}
}

func TestCreateSpecialUse_FallbackToCreate(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("CREATE", dummyHandler).(server.CommandHandlerFunc)

	var createCalled bool
	var gotOptions *imap.CreateOptions
	sess := &mock.Session{
		CreateFunc: func(mailbox string, options *imap.CreateOptions) error {
			createCalled = true
			gotOptions = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "Sent (USE (\\Sent))", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !createCalled {
		t.Fatal("Create should be called as fallback")
	}
	if gotOptions.SpecialUse != imap.MailboxAttrSent {
		t.Errorf("SpecialUse = %q, want %q", gotOptions.SpecialUse, imap.MailboxAttrSent)
	}
}

func TestCreateSpecialUse_Drafts(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("CREATE", dummyHandler).(server.CommandHandlerFunc)

	sess := &specialUseMockSession{}
	ctx := newTestCommandContext(t, "Drafts (USE (\\Drafts))", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.createSpecialUseCalled {
		t.Fatal("CreateSpecialUse was not called")
	}
	if sess.createSpecialUseOptions.SpecialUse != imap.MailboxAttrDrafts {
		t.Errorf("SpecialUse = %q, want %q", sess.createSpecialUseOptions.SpecialUse, imap.MailboxAttrDrafts)
	}
}

func TestCreateSpecialUse_QuotedMailbox(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("CREATE", dummyHandler).(server.CommandHandlerFunc)

	sess := &specialUseMockSession{}
	ctx := newTestCommandContext(t, "\"My Folder\" (USE (\\Sent))", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.createSpecialUseCalled {
		t.Fatal("CreateSpecialUse was not called")
	}
	if sess.createSpecialUseMailbox != "My Folder" {
		t.Errorf("mailbox = %q, want %q", sess.createSpecialUseMailbox, "My Folder")
	}
	if sess.createSpecialUseOptions.SpecialUse != imap.MailboxAttrSent {
		t.Errorf("SpecialUse = %q, want %q", sess.createSpecialUseOptions.SpecialUse, imap.MailboxAttrSent)
	}
}

func TestCreateSpecialUse_MissingDecoder(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("CREATE", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCommandContext(t, "", sess)
	ctx.Decoder = nil

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error for missing decoder")
	}
}
