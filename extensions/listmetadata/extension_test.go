package listmetadata

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extensions/listextended"
	"github.com/meszmate/imap-go/imaptest/mock"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

func newTestCommandContext(t *testing.T, args string, sess server.Session) *server.CommandContext {
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

func wrappedHandler() server.CommandHandlerFunc {
	ext := listextended.New()
	return ext.WrapHandler("LIST", dummyHandler).(server.CommandHandlerFunc)
}

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "LIST-METADATA" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "LIST-METADATA")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapListMetadata {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
}

func TestReturnMetadata_Parsed(t *testing.T) {
	h := wrappedHandler()

	var gotOpts *imap.ListOptions
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, `"" "*" RETURN (METADATA ("/shared/comment"))`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("List was not called")
	}
	if gotOpts.ReturnMetadata == nil {
		t.Fatal("ReturnMetadata should not be nil")
	}
	if len(gotOpts.ReturnMetadata.Options) != 1 {
		t.Fatalf("expected 1 metadata option, got %d", len(gotOpts.ReturnMetadata.Options))
	}
	if gotOpts.ReturnMetadata.Options[0] != "/shared/comment" {
		t.Errorf("option[0] = %q, want %q", gotOpts.ReturnMetadata.Options[0], "/shared/comment")
	}
}

func TestReturnMetadata_MaxSizeAndDepth(t *testing.T) {
	h := wrappedHandler()

	var gotOpts *imap.ListOptions
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, `"" "*" RETURN (METADATA ("/shared/comment" MAXSIZE 256 DEPTH 1))`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("List was not called")
	}
	if gotOpts.ReturnMetadata == nil {
		t.Fatal("ReturnMetadata should not be nil")
	}
	if len(gotOpts.ReturnMetadata.Options) != 1 {
		t.Fatalf("expected 1 metadata option, got %d", len(gotOpts.ReturnMetadata.Options))
	}
	if gotOpts.ReturnMetadata.Options[0] != "/shared/comment" {
		t.Errorf("option[0] = %q, want %q", gotOpts.ReturnMetadata.Options[0], "/shared/comment")
	}
	if gotOpts.ReturnMetadata.MaxSize != 256 {
		t.Errorf("MaxSize = %d, want 256", gotOpts.ReturnMetadata.MaxSize)
	}
	if gotOpts.ReturnMetadata.Depth != "1" {
		t.Errorf("Depth = %q, want %q", gotOpts.ReturnMetadata.Depth, "1")
	}
}

func TestReturnMetadata_MultipleEntries(t *testing.T) {
	h := wrappedHandler()

	var gotOpts *imap.ListOptions
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, `"" "*" RETURN (METADATA ("/shared/comment" "/private/comment"))`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("List was not called")
	}
	if gotOpts.ReturnMetadata == nil {
		t.Fatal("ReturnMetadata should not be nil")
	}
	if len(gotOpts.ReturnMetadata.Options) != 2 {
		t.Fatalf("expected 2 metadata options, got %d", len(gotOpts.ReturnMetadata.Options))
	}
	if gotOpts.ReturnMetadata.Options[0] != "/shared/comment" {
		t.Errorf("option[0] = %q, want %q", gotOpts.ReturnMetadata.Options[0], "/shared/comment")
	}
	if gotOpts.ReturnMetadata.Options[1] != "/private/comment" {
		t.Errorf("option[1] = %q, want %q", gotOpts.ReturnMetadata.Options[1], "/private/comment")
	}
}

func TestReturnMetadata_WriteMetadataResponse(t *testing.T) {
	h := wrappedHandler()

	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			w.WriteList(&imap.ListData{
				Delim:   '/',
				Mailbox: "INBOX",
				Metadata: map[string]string{
					"/shared/comment": "Hello",
				},
			})
			return nil
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, `"" "*" RETURN (METADATA ("/shared/comment"))`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "* LIST") {
		t.Errorf("response should contain LIST, got: %s", output)
	}
	if !strings.Contains(output, "METADATA") {
		t.Errorf("response should contain METADATA, got: %s", output)
	}
	if !strings.Contains(output, "/shared/comment") {
		t.Errorf("response should contain /shared/comment, got: %s", output)
	}
	if !strings.Contains(output, "Hello") {
		t.Errorf("response should contain Hello, got: %s", output)
	}
}

func TestReturnMetadata_WithOtherReturnOptions(t *testing.T) {
	h := wrappedHandler()

	var gotOpts *imap.ListOptions
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, `"" "*" RETURN (CHILDREN METADATA ("/shared/comment"))`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("List was not called")
	}
	if !gotOpts.ReturnChildren {
		t.Error("ReturnChildren should be true")
	}
	if gotOpts.ReturnMetadata == nil {
		t.Fatal("ReturnMetadata should not be nil")
	}
	if len(gotOpts.ReturnMetadata.Options) != 1 {
		t.Fatalf("expected 1 metadata option, got %d", len(gotOpts.ReturnMetadata.Options))
	}
	if gotOpts.ReturnMetadata.Options[0] != "/shared/comment" {
		t.Errorf("option[0] = %q, want %q", gotOpts.ReturnMetadata.Options[0], "/shared/comment")
	}
}

func TestReturnMetadata_DepthInfinity(t *testing.T) {
	h := wrappedHandler()

	var gotOpts *imap.ListOptions
	sess := &mock.Session{
		ListFunc: func(w *server.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
			gotOpts = options
			return nil
		},
	}
	ctx := newTestCommandContext(t, `"" "*" RETURN (METADATA ("/shared/comment" DEPTH infinity))`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("List was not called")
	}
	if gotOpts.ReturnMetadata == nil {
		t.Fatal("ReturnMetadata should not be nil")
	}
	if gotOpts.ReturnMetadata.Depth != "infinity" {
		t.Errorf("Depth = %q, want %q", gotOpts.ReturnMetadata.Depth, "infinity")
	}
}
