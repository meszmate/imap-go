package contextsearch

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

// contextMockSession embeds mock.Session and adds SearchContext/CancelSearchContext.
type contextMockSession struct {
	mock.Session
	searchContextCalled  bool
	searchContextTag     string
	searchContextKind    server.NumKind
	searchContextCrit    *imap.SearchCriteria
	searchContextOpts    *imap.SearchOptions
	searchContextResult  *imap.SearchData
	searchContextErr     error
	cancelContextCalled  bool
	cancelContextTags    []string
	cancelContextErr     error
}

func (m *contextMockSession) SearchContext(tag string, kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
	m.searchContextCalled = true
	m.searchContextTag = tag
	m.searchContextKind = kind
	m.searchContextCrit = criteria
	m.searchContextOpts = options
	if m.searchContextResult != nil {
		return m.searchContextResult, m.searchContextErr
	}
	return &imap.SearchData{}, m.searchContextErr
}

func (m *contextMockSession) CancelSearchContext(tags []string) error {
	m.cancelContextCalled = true
	m.cancelContextTags = tags
	return m.cancelContextErr
}

var _ SessionContext = (*contextMockSession)(nil)

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
		Name:    "SEARCH",
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

	var dec *wire.Decoder
	if args != "" {
		dec = wire.NewDecoder(strings.NewReader(args))
	}

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SEARCH",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, &outBuf, done
}

func newCancelUpdateContext(t *testing.T, args string, sess server.Session) (*server.CommandContext, *bytes.Buffer, chan struct{}) {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)
	_ = conn.SetState(imap.ConnStateAuthenticated)
	_ = conn.SetState(imap.ConnStateSelected)

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
		Tag:     "B001",
		Name:    "CANCELUPDATE",
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
	if ext.ExtName != "CONTEXT=SEARCH" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "CONTEXT=SEARCH")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapContextSearch {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
	if len(ext.ExtDependencies) != 1 || ext.ExtDependencies[0] != "ESEARCH" {
		t.Errorf("unexpected dependencies: %v", ext.ExtDependencies)
	}
}

func TestWrapHandler_ReturnsHandler(t *testing.T) {
	ext := New()
	if ext.WrapHandler("SEARCH", dummyHandler) == nil {
		t.Error("WrapHandler(SEARCH) returned nil")
	}
}

func TestWrapHandler_UnknownCommand(t *testing.T) {
	ext := New()
	for _, name := range []string{"FETCH", "STORE", "SELECT", "NOOP"} {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) should return nil", name)
		}
	}
}

func TestCommandHandlers_CANCELUPDATE(t *testing.T) {
	ext := New()
	handlers := ext.CommandHandlers()
	if handlers == nil {
		t.Fatal("CommandHandlers should not be nil")
	}
	if _, ok := handlers["CANCELUPDATE"]; !ok {
		t.Error("CANCELUPDATE handler not registered")
	}
}

func TestSearch_WithReturnUpdate(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &contextMockSession{
		searchContextResult: &imap.SearchData{Min: 1, Max: 10, Count: 3},
	}
	ctx := newTestCommandContext(t, `RETURN (UPDATE COUNT) UNSEEN`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.searchContextCalled {
		t.Fatal("SearchContext should have been called")
	}
	if sess.searchContextTag != "A001" {
		t.Errorf("tag = %q, want %q", sess.searchContextTag, "A001")
	}
	if sess.searchContextOpts == nil {
		t.Fatal("options should not be nil")
	}
	if !sess.searchContextOpts.ReturnUpdate {
		t.Error("ReturnUpdate should be true")
	}
	if !sess.searchContextOpts.ReturnCount {
		t.Error("ReturnCount should be true")
	}
}

func TestSearch_WithReturnContext(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &contextMockSession{
		searchContextResult: &imap.SearchData{Min: 1, Max: 5, Count: 2},
	}
	ctx := newTestCommandContext(t, `RETURN (CONTEXT MIN MAX) ALL`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.searchContextCalled {
		t.Fatal("SearchContext should have been called")
	}
	if !sess.searchContextOpts.ReturnContext {
		t.Error("ReturnContext should be true")
	}
	if !sess.searchContextOpts.ReturnMin {
		t.Error("ReturnMin should be true")
	}
	if !sess.searchContextOpts.ReturnMax {
		t.Error("ReturnMax should be true")
	}
}

func TestSearch_WithReturnUpdateAndStandardOptions(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &contextMockSession{
		searchContextResult: &imap.SearchData{Min: 1, Max: 42, Count: 5},
	}
	ctx := newTestCommandContext(t, `RETURN (UPDATE MIN MAX COUNT) FLAGGED`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.searchContextCalled {
		t.Fatal("SearchContext should have been called")
	}
	opts := sess.searchContextOpts
	if !opts.ReturnUpdate {
		t.Error("ReturnUpdate should be true")
	}
	if !opts.ReturnMin {
		t.Error("ReturnMin should be true")
	}
	if !opts.ReturnMax {
		t.Error("ReturnMax should be true")
	}
	if !opts.ReturnCount {
		t.Error("ReturnCount should be true")
	}
}

func TestSearch_WithoutContextOptions_FallsBack(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var searchCalled bool
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			searchCalled = true
			return &imap.SearchData{Min: 1, Max: 5, Count: 2}, nil
		},
	}
	ctx := newTestCommandContext(t, `RETURN (MIN MAX) UNSEEN`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchCalled {
		t.Fatal("Session.Search should be called as fallback")
	}
}

func TestSearch_WithoutReturn_DelegatesToOriginal(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var searchCalled bool
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			searchCalled = true
			return &imap.SearchData{AllSeqNums: []uint32{1, 5, 10}}, nil
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, "UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if !searchCalled {
		t.Fatal("Session.Search should have been called")
	}

	output := outBuf.String()
	if !strings.Contains(output, "* SEARCH") {
		t.Errorf("expected traditional SEARCH response, got: %s", output)
	}
	if strings.Contains(output, "ESEARCH") {
		t.Errorf("should not contain ESEARCH, got: %s", output)
	}
}

func TestSearch_NoContextSupport_SendsNOUPDATE(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	// Use plain mock.Session (no SessionContext)
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{Min: 1, Max: 5, Count: 2}, nil
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, `RETURN (UPDATE MIN) ALL`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "NOUPDATE") {
		t.Errorf("expected NOUPDATE response, got: %s", output)
	}
	if !strings.Contains(output, `"A001"`) {
		t.Errorf("expected tag in NOUPDATE, got: %s", output)
	}
}

func TestSearch_ESearchResponseFormat(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &contextMockSession{
		searchContextResult: &imap.SearchData{Min: 1, Max: 42, Count: 5},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, `RETURN (UPDATE MIN MAX COUNT) ALL`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, `* ESEARCH (TAG "A001") MIN 1 MAX 42 COUNT 5`) {
		t.Errorf("unexpected ESEARCH response format: %s", output)
	}
}

func TestSearch_AddToInResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	addSeqSet, _ := imap.ParseSeqSet("5")
	sess := &contextMockSession{
		searchContextResult: &imap.SearchData{
			Min:   1,
			Max:   10,
			Count: 3,
			AddTo: []imap.SearchContextUpdate{
				{Position: 2, SeqSet: addSeqSet},
			},
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, `RETURN (UPDATE COUNT) ALL`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "ADDTO") {
		t.Errorf("response should contain ADDTO, got: %s", output)
	}
	if !strings.Contains(output, "(2 5)") {
		t.Errorf("ADDTO should contain position and seqset, got: %s", output)
	}
}

func TestSearch_RemoveFromInResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	removeSeqSet, _ := imap.ParseSeqSet("3")
	sess := &contextMockSession{
		searchContextResult: &imap.SearchData{
			Min:   1,
			Max:   10,
			Count: 2,
			RemoveFrom: []imap.SearchContextUpdate{
				{Position: 1, SeqSet: removeSeqSet},
			},
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, `RETURN (UPDATE COUNT) ALL`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "REMOVEFROM") {
		t.Errorf("response should contain REMOVEFROM, got: %s", output)
	}
	if !strings.Contains(output, "(1 3)") {
		t.Errorf("REMOVEFROM should contain position and seqset, got: %s", output)
	}
}

func TestSearch_EmptyReturn(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{AllSeqNums: []uint32{1, 2, 3}}, nil
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, `RETURN () ALL`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "* SEARCH") {
		t.Errorf("expected traditional SEARCH response, got: %s", output)
	}
	if strings.Contains(output, "ESEARCH") {
		t.Errorf("should not contain ESEARCH for empty RETURN, got: %s", output)
	}
}

func TestCancelUpdate_SingleTag(t *testing.T) {
	sess := &contextMockSession{}
	ctx, outBuf, done := newCancelUpdateContext(t, `"search1"`, sess)

	if err := handleCancelUpdate(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if !sess.cancelContextCalled {
		t.Fatal("CancelSearchContext should have been called")
	}
	if len(sess.cancelContextTags) != 1 || sess.cancelContextTags[0] != "search1" {
		t.Errorf("expected tags [search1], got %v", sess.cancelContextTags)
	}

	output := outBuf.String()
	if !strings.Contains(output, "OK") {
		t.Errorf("expected OK response, got: %s", output)
	}
}

func TestCancelUpdate_MultipleTags(t *testing.T) {
	sess := &contextMockSession{}
	ctx, outBuf, done := newCancelUpdateContext(t, `"search1" "search2"`, sess)

	if err := handleCancelUpdate(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if !sess.cancelContextCalled {
		t.Fatal("CancelSearchContext should have been called")
	}
	if len(sess.cancelContextTags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(sess.cancelContextTags))
	}
	if sess.cancelContextTags[0] != "search1" || sess.cancelContextTags[1] != "search2" {
		t.Errorf("expected tags [search1 search2], got %v", sess.cancelContextTags)
	}

	output := outBuf.String()
	if !strings.Contains(output, "OK") {
		t.Errorf("expected OK response, got: %s", output)
	}
}

func TestCancelUpdate_NoSession(t *testing.T) {
	sess := &mock.Session{}
	ctx, _, done := newCancelUpdateContext(t, `"search1"`, sess)

	err := handleCancelUpdate(ctx)

	_ = ctx.Conn.Close()
	<-done

	if err == nil {
		t.Fatal("expected error for session without SessionContext")
	}
	imapErr, ok := err.(*imap.IMAPError)
	if !ok {
		t.Fatalf("expected IMAPError, got %T", err)
	}
	if imapErr.Type != imap.StatusResponseTypeNO {
		t.Errorf("expected NO response, got %s", imapErr.Type)
	}
}

func TestCancelUpdate_NoArgs(t *testing.T) {
	sess := &contextMockSession{}
	ctx, _, done := newCancelUpdateContext(t, "", sess)
	ctx.Decoder = nil

	err := handleCancelUpdate(ctx)

	_ = ctx.Conn.Close()
	<-done

	if err == nil {
		t.Fatal("expected error for missing arguments")
	}
	imapErr, ok := err.(*imap.IMAPError)
	if !ok {
		t.Fatalf("expected IMAPError, got %T", err)
	}
	if imapErr.Type != imap.StatusResponseTypeBAD {
		t.Errorf("expected BAD response, got %s", imapErr.Type)
	}
}

func TestSearch_UIDKind(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &contextMockSession{
		searchContextResult: &imap.SearchData{Min: 1, Max: 42, Count: 5},
	}

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

	ctx := &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    "SEARCH",
		NumKind: server.NumKindUID,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader(`RETURN (UPDATE MIN MAX COUNT) ALL`)),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, "UID") {
		t.Errorf("response should contain UID flag, got: %s", output)
	}
	if !strings.Contains(output, `(TAG "A001")`) {
		t.Errorf("response should contain TAG correlator, got: %s", output)
	}

	if sess.searchContextKind != server.NumKindUID {
		t.Error("expected UID kind")
	}
}
