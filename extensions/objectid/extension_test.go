package objectid

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
	if ext.ExtName != "OBJECTID" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "OBJECTID")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapObjectID {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
}

func TestWrapHandler_ReturnsNil(t *testing.T) {
	ext := New()
	for _, name := range []string{"FETCH", "SELECT", "EXAMINE", "STORE", "SEARCH"} {
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

func TestSessionExtension_Type(t *testing.T) {
	ext := New()
	iface := ext.SessionExtension()
	if iface == nil {
		t.Fatal("SessionExtension() should not return nil")
	}
	// Verify it's a *SessionObjectID nil pointer
	if _, ok := iface.(*SessionObjectID); !ok {
		t.Error("SessionExtension() should return *SessionObjectID type")
	}
}

func selectOutput(t *testing.T, data *imap.SelectData) string {
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
		SelectFunc: func(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
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
		Name:    "SELECT",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("INBOX")),
	}

	h := commands.Select()
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	return outBuf.String()
}

func TestSelect_WithMailboxID(t *testing.T) {
	output := selectOutput(t, &imap.SelectData{
		Flags:       []imap.Flag{"Seen"},
		UIDValidity: 1,
		UIDNext:     2,
		MailboxID:   "testid123",
	})

	if !strings.Contains(output, "MAILBOXID") {
		t.Errorf("response should contain MAILBOXID, got: %s", output)
	}
	if !strings.Contains(output, "(testid123)") {
		t.Errorf("response should contain (testid123), got: %s", output)
	}
}

func TestSelect_WithoutMailboxID(t *testing.T) {
	output := selectOutput(t, &imap.SelectData{
		Flags:       []imap.Flag{"Seen"},
		UIDValidity: 1,
		UIDNext:     2,
	})

	if strings.Contains(output, "MAILBOXID") {
		t.Errorf("response should NOT contain MAILBOXID, got: %s", output)
	}
}

func TestSelect_MailboxIDWithHighestModSeq(t *testing.T) {
	output := selectOutput(t, &imap.SelectData{
		Flags:         []imap.Flag{"Seen"},
		UIDValidity:   42,
		UIDNext:       100,
		HighestModSeq: 99999,
		MailboxID:     "mbox-abc",
	})

	if !strings.Contains(output, "HIGHESTMODSEQ") {
		t.Errorf("response should contain HIGHESTMODSEQ, got: %s", output)
	}
	if !strings.Contains(output, "MAILBOXID") {
		t.Errorf("response should contain MAILBOXID, got: %s", output)
	}
	if !strings.Contains(output, "(mbox-abc)") {
		t.Errorf("response should contain (mbox-abc), got: %s", output)
	}

	// MAILBOXID should appear after HIGHESTMODSEQ
	hmsIdx := strings.Index(output, "HIGHESTMODSEQ")
	mbxIdx := strings.Index(output, "MAILBOXID")
	if mbxIdx <= hmsIdx {
		t.Errorf("MAILBOXID should appear after HIGHESTMODSEQ in output")
	}
}
