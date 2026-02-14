package statussize

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/imaptest/mock"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/server/commands"
	"github.com/meszmate/imap-go/wire"
)

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "STATUS=SIZE" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "STATUS=SIZE")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapStatusSize {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
}

func TestWrapHandler_ReturnsNil(t *testing.T) {
	ext := New()
	for _, name := range []string{"FETCH", "SELECT", "EXAMINE", "STORE", "SEARCH", "STATUS"} {
		if ext.WrapHandler(name, nil) != nil {
			t.Errorf("WrapHandler(%q) should return nil", name)
		}
	}
}

func TestCommandHandlers_ReturnsNil(t *testing.T) {
	ext := New()
	if ext.CommandHandlers() != nil {
		t.Error("CommandHandlers() should return nil")
	}
}

func TestSessionExtension_ReturnsNil(t *testing.T) {
	ext := New()
	if ext.SessionExtension() != nil {
		t.Error("SessionExtension() should return nil")
	}
}

func statusOutput(t *testing.T, data *imap.StatusData) string {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)
	if err := conn.SetState(imap.ConnStateAuthenticated); err != nil {
		t.Fatalf("failed to set authenticated state: %v", err)
	}

	sess := &mock.Session{
		StatusFunc: func(mailbox string, options *imap.StatusOptions) (*imap.StatusData, error) {
			return data, nil
		},
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
		Name:    "STATUS",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("INBOX (SIZE)")),
	}

	h := commands.Status()
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	return outBuf.String()
}

func TestStatus_WithSize(t *testing.T) {
	size := int64(123456)
	output := statusOutput(t, &imap.StatusData{
		Mailbox: "INBOX",
		Size:    &size,
	})

	if !strings.Contains(output, "SIZE") {
		t.Errorf("response should contain SIZE, got: %s", output)
	}
	if !strings.Contains(output, "123456") {
		t.Errorf("response should contain 123456, got: %s", output)
	}
}

func TestStatus_WithoutSize(t *testing.T) {
	numMessages := uint32(10)
	output := statusOutput(t, &imap.StatusData{
		Mailbox:     "INBOX",
		NumMessages: &numMessages,
	})

	if strings.Contains(output, "SIZE") {
		t.Errorf("response should NOT contain SIZE, got: %s", output)
	}
}

func TestStatus_RequestsSizeOption(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)
	if err := conn.SetState(imap.ConnStateAuthenticated); err != nil {
		t.Fatalf("failed to set authenticated state: %v", err)
	}

	var gotOpts *imap.StatusOptions
	sess := &mock.Session{
		StatusFunc: func(mailbox string, options *imap.StatusOptions) (*imap.StatusData, error) {
			gotOpts = options
			return &imap.StatusData{Mailbox: mailbox}, nil
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			if _, err := clientConn.Read(buf); err != nil {
				return
			}
		}
	}()

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "STATUS",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("INBOX (SIZE)")),
	}

	h := commands.Status()
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = serverConn.Close()
	<-done

	if gotOpts == nil {
		t.Fatal("expected options to be passed to session")
	}
	if !gotOpts.Size {
		t.Fatal("expected options.Size to be true")
	}
}

func TestStatus_DoesNotRequestSizeWhenMissing(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)
	if err := conn.SetState(imap.ConnStateAuthenticated); err != nil {
		t.Fatalf("failed to set authenticated state: %v", err)
	}

	var gotOpts *imap.StatusOptions
	sess := &mock.Session{
		StatusFunc: func(mailbox string, options *imap.StatusOptions) (*imap.StatusData, error) {
			gotOpts = options
			return &imap.StatusData{Mailbox: mailbox}, nil
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			if _, err := clientConn.Read(buf); err != nil {
				return
			}
		}
	}()

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "STATUS",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("INBOX (MESSAGES)")),
	}

	h := commands.Status()
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = serverConn.Close()
	<-done

	if gotOpts == nil {
		t.Fatal("expected options to be passed to session")
	}
	if gotOpts.Size {
		t.Fatal("expected options.Size to be false")
	}
}
