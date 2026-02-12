package savedate

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/imaptest/mock"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/server/commands"
	"github.com/meszmate/imap-go/wire"
)

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "SAVEDATE" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "SAVEDATE")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapSaveDate {
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
	if _, ok := iface.(*SessionSaveDate); !ok {
		t.Error("SessionExtension() should return *SessionSaveDate type")
	}
}

func fetchOutput(t *testing.T, data *imap.FetchMessageData) string {
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
	if err := conn.SetState(imap.ConnStateSelected); err != nil {
		t.Fatalf("failed to set selected state: %v", err)
	}

	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			w.WriteFetchData(data)
			return nil
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
		Name:    "FETCH",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("1 (SAVEDATE)")),
	}

	h := commands.Fetch()
	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	return outBuf.String()
}

func TestFetch_WithSaveDate(t *testing.T) {
	saveDate := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	output := fetchOutput(t, &imap.FetchMessageData{
		SeqNum:   1,
		SaveDate: &saveDate,
	})

	if !strings.Contains(output, "SAVEDATE") {
		t.Errorf("response should contain SAVEDATE, got: %s", output)
	}
	if strings.Contains(output, "SAVEDATE NIL") {
		t.Errorf("response should NOT contain SAVEDATE NIL, got: %s", output)
	}
}

func TestFetch_WithSaveDateNIL(t *testing.T) {
	output := fetchOutput(t, &imap.FetchMessageData{
		SeqNum:      1,
		SaveDateNIL: true,
	})

	if !strings.Contains(output, "SAVEDATE NIL") {
		t.Errorf("response should contain SAVEDATE NIL, got: %s", output)
	}
}

func TestFetch_WithoutSaveDate(t *testing.T) {
	output := fetchOutput(t, &imap.FetchMessageData{
		SeqNum: 1,
		Flags:  []imap.Flag{imap.FlagSeen},
	})

	if strings.Contains(output, "SAVEDATE") {
		t.Errorf("response should NOT contain SAVEDATE, got: %s", output)
	}
}
