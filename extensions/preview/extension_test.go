package preview

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
		Name:    "FETCH",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}
}

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "PREVIEW" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "PREVIEW")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapPreview {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
	deps := ext.Dependencies()
	if len(deps) != 1 || deps[0] != "CONDSTORE" {
		t.Errorf("unexpected dependencies: %v", deps)
	}
}

func TestWrapHandler_ReturnsHandlerForFETCH(t *testing.T) {
	ext := New()
	if ext.WrapHandler("FETCH", dummyHandler) == nil {
		t.Error("WrapHandler(FETCH) returned nil")
	}
}

func TestWrapHandler_UnknownCommand(t *testing.T) {
	ext := New()
	for _, name := range []string{"STORE", "SELECT", "NOOP", "LIST"} {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) should return nil", name)
		}
	}
}

func TestFetch_PreviewInList(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1 (FLAGS PREVIEW)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if !gotOpts.Preview {
		t.Error("Preview should be true")
	}
	if gotOpts.PreviewLazy {
		t.Error("PreviewLazy should be false")
	}
	if !gotOpts.Flags {
		t.Error("Flags should be true")
	}
}

func TestFetch_PreviewLazyInList(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1 (FLAGS PREVIEW (LAZY))", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if !gotOpts.Preview {
		t.Error("Preview should be true")
	}
	if !gotOpts.PreviewLazy {
		t.Error("PreviewLazy should be true")
	}
	if !gotOpts.Flags {
		t.Error("Flags should be true")
	}
}

func TestFetch_PreviewLazySingleItem(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1 PREVIEW (LAZY)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if !gotOpts.Preview {
		t.Error("Preview should be true")
	}
	if !gotOpts.PreviewLazy {
		t.Error("PreviewLazy should be true")
	}
}

func TestFetch_PreviewSingleItem(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1 PREVIEW", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if !gotOpts.Preview {
		t.Error("Preview should be true")
	}
	if gotOpts.PreviewLazy {
		t.Error("PreviewLazy should be false")
	}
}

func TestFetch_PreviewWithChangedSince(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1 (FLAGS PREVIEW) (CHANGEDSINCE 123)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if !gotOpts.Preview {
		t.Error("Preview should be true")
	}
	if gotOpts.PreviewLazy {
		t.Error("PreviewLazy should be false")
	}
	if gotOpts.ChangedSince != 123 {
		t.Errorf("ChangedSince = %d, want 123", gotOpts.ChangedSince)
	}
	if !gotOpts.ModSeq {
		t.Error("ModSeq should be true")
	}
}

func TestFetch_PreviewLazyWithChangedSince(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1 (FLAGS PREVIEW (LAZY)) (CHANGEDSINCE 456)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if !gotOpts.Preview {
		t.Error("Preview should be true")
	}
	if !gotOpts.PreviewLazy {
		t.Error("PreviewLazy should be true")
	}
	if gotOpts.ChangedSince != 456 {
		t.Errorf("ChangedSince = %d, want 456", gotOpts.ChangedSince)
	}
	if !gotOpts.ModSeq {
		t.Error("ModSeq should be true")
	}
}

func TestFetch_PreviewLazySingleWithChangedSince(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1 PREVIEW (LAZY) (CHANGEDSINCE 789)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if !gotOpts.Preview {
		t.Error("Preview should be true")
	}
	if !gotOpts.PreviewLazy {
		t.Error("PreviewLazy should be true")
	}
	if gotOpts.ChangedSince != 789 {
		t.Errorf("ChangedSince = %d, want 789", gotOpts.ChangedSince)
	}
	if !gotOpts.ModSeq {
		t.Error("ModSeq should be true")
	}
}

func TestFetch_WriterPreviewText(t *testing.T) {
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

	w := server.NewFetchWriter(conn.Encoder())
	w.WriteFetchData(&imap.FetchMessageData{
		SeqNum:  1,
		UID:     42,
		Preview: "Hello world preview",
	})

	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, `PREVIEW "Hello world preview"`) {
		t.Errorf("response should contain PREVIEW text, got: %s", output)
	}
}

func TestFetch_WriterPreviewNIL(t *testing.T) {
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

	w := server.NewFetchWriter(conn.Encoder())
	w.WriteFetchData(&imap.FetchMessageData{
		SeqNum:     1,
		UID:        42,
		PreviewNIL: true,
	})

	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "PREVIEW NIL") {
		t.Errorf("response should contain PREVIEW NIL, got: %s", output)
	}
}

func TestFetch_UIDFetch(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
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

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "FETCH",
		NumKind: server.NumKindUID,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("1:5 (FLAGS PREVIEW)")),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if !gotOpts.UID {
		t.Error("UID should be true for UID FETCH")
	}
	if !gotOpts.Preview {
		t.Error("Preview should be true")
	}
	if !gotOpts.Flags {
		t.Error("Flags should be true")
	}
}

func TestFetch_PreviewLazyInListWithMoreItems(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("FETCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.FetchOptions
	sess := &mock.Session{
		FetchFunc: func(w *server.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, "1 (FLAGS PREVIEW (LAZY) UID)", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Fetch was not called")
	}
	if !gotOpts.Preview {
		t.Error("Preview should be true")
	}
	if !gotOpts.PreviewLazy {
		t.Error("PreviewLazy should be true")
	}
	if !gotOpts.Flags {
		t.Error("Flags should be true")
	}
	if !gotOpts.UID {
		t.Error("UID should be true")
	}
}
