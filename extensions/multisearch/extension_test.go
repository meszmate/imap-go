package multisearch

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

// multiSearchMockSession embeds mock.Session and adds MultiSearch.
type multiSearchMockSession struct {
	mock.Session
	multiSearchCalled  bool
	multiSearchKind    server.NumKind
	multiSearchSource  *MultiSearchSource
	multiSearchCrit    *imap.SearchCriteria
	multiSearchOpts    *imap.SearchOptions
	multiSearchResult  []imap.MultiSearchResult
	multiSearchErr     error
}

func (m *multiSearchMockSession) MultiSearch(kind server.NumKind, source *MultiSearchSource, criteria *imap.SearchCriteria, options *imap.SearchOptions) ([]imap.MultiSearchResult, error) {
	m.multiSearchCalled = true
	m.multiSearchKind = kind
	m.multiSearchSource = source
	m.multiSearchCrit = criteria
	m.multiSearchOpts = options
	if m.multiSearchResult != nil {
		return m.multiSearchResult, m.multiSearchErr
	}
	return []imap.MultiSearchResult{}, m.multiSearchErr
}

var _ SessionMultiSearch = (*multiSearchMockSession)(nil)

func newTestCommandContext(t *testing.T, args string, sess server.Session) *server.CommandContext {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)
	_ = conn.SetState(imap.ConnStateAuthenticated)

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
		Name:    "ESEARCH",
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
	_ = conn.SetState(imap.ConnStateAuthenticated)

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
		Name:    "ESEARCH",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, &outBuf, done
}

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "MULTISEARCH" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "MULTISEARCH")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapMultiSearch {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
	if len(ext.ExtDependencies) != 1 || ext.ExtDependencies[0] != "ESEARCH" {
		t.Errorf("unexpected dependencies: %v", ext.ExtDependencies)
	}
}

func TestCommandHandlers_ESEARCH(t *testing.T) {
	ext := New()
	handlers := ext.CommandHandlers()
	if handlers == nil {
		t.Fatal("CommandHandlers should not be nil")
	}
	if _, ok := handlers["ESEARCH"]; !ok {
		t.Error("ESEARCH handler not registered")
	}
}

func TestWrapHandler_ReturnsNil(t *testing.T) {
	ext := New()
	dummy := server.CommandHandlerFunc(func(ctx *server.CommandContext) error { return nil })
	for _, name := range []string{"SEARCH", "FETCH", "STORE", "SELECT"} {
		if ext.WrapHandler(name, dummy) != nil {
			t.Errorf("WrapHandler(%q) should return nil", name)
		}
	}
}

func TestMultiSearch_SingleMailbox(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 1, Data: &imap.SearchData{Count: 3}},
		},
	}
	ctx := newTestCommandContext(t, `IN (mailboxes INBOX) RETURN (COUNT) ALL`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.multiSearchCalled {
		t.Fatal("MultiSearch should have been called")
	}
	if sess.multiSearchSource == nil {
		t.Fatal("source should not be nil")
	}
	if sess.multiSearchSource.Filter != "mailboxes" {
		t.Errorf("filter = %q, want %q", sess.multiSearchSource.Filter, "mailboxes")
	}
	if len(sess.multiSearchSource.Mailboxes) != 1 || sess.multiSearchSource.Mailboxes[0] != "INBOX" {
		t.Errorf("mailboxes = %v, want [INBOX]", sess.multiSearchSource.Mailboxes)
	}
}

func TestMultiSearch_MultipleMailboxes(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 1, Data: &imap.SearchData{Count: 2}},
			{Mailbox: "Sent Items", UIDValidity: 5, Data: &imap.SearchData{Count: 1}},
		},
	}
	ctx := newTestCommandContext(t, `IN (mailboxes (INBOX "Sent Items")) RETURN (COUNT) ALL`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.multiSearchCalled {
		t.Fatal("MultiSearch should have been called")
	}
	src := sess.multiSearchSource
	if len(src.Mailboxes) != 2 {
		t.Fatalf("expected 2 mailboxes, got %d", len(src.Mailboxes))
	}
	if src.Mailboxes[0] != "INBOX" {
		t.Errorf("mailbox[0] = %q, want %q", src.Mailboxes[0], "INBOX")
	}
	if src.Mailboxes[1] != "Sent Items" {
		t.Errorf("mailbox[1] = %q, want %q", src.Mailboxes[1], "Sent Items")
	}
}

func TestMultiSearch_SubtreeFilter(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 1, Data: &imap.SearchData{}},
		},
	}
	ctx := newTestCommandContext(t, `IN (subtree INBOX) ALL`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.multiSearchSource.Filter != "subtree" {
		t.Errorf("filter = %q, want %q", sess.multiSearchSource.Filter, "subtree")
	}
	if len(sess.multiSearchSource.Mailboxes) != 1 || sess.multiSearchSource.Mailboxes[0] != "INBOX" {
		t.Errorf("mailboxes = %v, want [INBOX]", sess.multiSearchSource.Mailboxes)
	}
}

func TestMultiSearch_SubtreeOneFilter(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "", UIDValidity: 1, Data: &imap.SearchData{}},
		},
	}
	ctx := newTestCommandContext(t, `IN (subtree-one "") ALL`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.multiSearchSource.Filter != "subtree-one" {
		t.Errorf("filter = %q, want %q", sess.multiSearchSource.Filter, "subtree-one")
	}
	if len(sess.multiSearchSource.Mailboxes) != 1 || sess.multiSearchSource.Mailboxes[0] != "" {
		t.Errorf("mailboxes = %v, want [\"\"]", sess.multiSearchSource.Mailboxes)
	}
}

func TestMultiSearch_WithReturnOptions(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 1, Data: &imap.SearchData{Min: 1, Max: 10, Count: 5}},
		},
	}
	ctx := newTestCommandContext(t, `IN (mailboxes INBOX) RETURN (MIN MAX COUNT) ALL`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opts := sess.multiSearchOpts
	if opts == nil {
		t.Fatal("options should not be nil")
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
	if opts.ReturnAll {
		t.Error("ReturnAll should be false")
	}
}

func TestMultiSearch_WithCharset(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 1, Data: &imap.SearchData{Count: 1}},
		},
	}
	ctx := newTestCommandContext(t, `IN (mailboxes INBOX) CHARSET UTF-8 UNSEEN`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.multiSearchCalled {
		t.Fatal("MultiSearch should have been called")
	}
	// Verify UNSEEN criterion was parsed
	hasUnseen := false
	for _, f := range sess.multiSearchCrit.NotFlag {
		if f == imap.FlagSeen {
			hasUnseen = true
		}
	}
	if !hasUnseen {
		t.Error("expected UNSEEN criterion")
	}
}

func TestMultiSearch_ResponseFormat(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 67890, Data: &imap.SearchData{Min: 1, Max: 42, Count: 5}},
		},
	}
	ctx, outBuf, done := newTestCommandContextWithOutput(t, `IN (mailboxes INBOX) RETURN (MIN MAX COUNT) ALL`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, `(TAG "A001")`) {
		t.Errorf("response should contain TAG correlator, got: %s", output)
	}
	if !strings.Contains(output, "MAILBOX INBOX") {
		t.Errorf("response should contain MAILBOX, got: %s", output)
	}
	if !strings.Contains(output, "UIDVALIDITY 67890") {
		t.Errorf("response should contain UIDVALIDITY, got: %s", output)
	}
	if !strings.Contains(output, "UID") {
		t.Errorf("response should contain UID indicator, got: %s", output)
	}
	if !strings.Contains(output, "MIN 1") {
		t.Errorf("response should contain MIN, got: %s", output)
	}
	if !strings.Contains(output, "MAX 42") {
		t.Errorf("response should contain MAX, got: %s", output)
	}
	if !strings.Contains(output, "COUNT 5") {
		t.Errorf("response should contain COUNT, got: %s", output)
	}
}

func TestMultiSearch_MultipleMailboxResponses(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 100, Data: &imap.SearchData{Count: 3}},
			{Mailbox: "Sent", UIDValidity: 200, Data: &imap.SearchData{Count: 1}},
		},
	}
	ctx, outBuf, done := newTestCommandContextWithOutput(t, `IN (mailboxes (INBOX Sent)) RETURN (COUNT) ALL`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	// Should have two ESEARCH responses
	count := strings.Count(output, "* ESEARCH")
	if count != 2 {
		t.Errorf("expected 2 ESEARCH responses, got %d in: %s", count, output)
	}
	if !strings.Contains(output, "MAILBOX INBOX") {
		t.Errorf("response should contain MAILBOX INBOX, got: %s", output)
	}
	if !strings.Contains(output, "UIDVALIDITY 100") {
		t.Errorf("response should contain UIDVALIDITY 100, got: %s", output)
	}
	if !strings.Contains(output, "MAILBOX Sent") {
		t.Errorf("response should contain MAILBOX Sent, got: %s", output)
	}
	if !strings.Contains(output, "UIDVALIDITY 200") {
		t.Errorf("response should contain UIDVALIDITY 200, got: %s", output)
	}
}

func TestMultiSearch_NoSession(t *testing.T) {
	// Use plain mock.Session without SessionMultiSearch
	sess := &mock.Session{}
	ctx := newTestCommandContext(t, `IN (mailboxes INBOX) ALL`, sess)

	err := handleMultiSearch(ctx)
	if err == nil {
		t.Fatal("expected error for session without SessionMultiSearch")
	}
	imapErr, ok := err.(*imap.IMAPError)
	if !ok {
		t.Fatalf("expected IMAPError, got %T", err)
	}
	if imapErr.Type != imap.StatusResponseTypeNO {
		t.Errorf("expected NO response, got %s", imapErr.Type)
	}
}

func TestMultiSearch_NoArgs(t *testing.T) {
	sess := &multiSearchMockSession{}
	ctx := newTestCommandContext(t, "", sess)
	ctx.Decoder = nil

	err := handleMultiSearch(ctx)
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

func TestMultiSearch_InvalidFilter(t *testing.T) {
	sess := &multiSearchMockSession{}
	ctx := newTestCommandContext(t, `IN (badfilter INBOX) ALL`, sess)

	err := handleMultiSearch(ctx)
	if err == nil {
		t.Fatal("expected error for invalid filter type")
	}
	imapErr, ok := err.(*imap.IMAPError)
	if !ok {
		t.Fatalf("expected IMAPError, got %T", err)
	}
	if imapErr.Type != imap.StatusResponseTypeBAD {
		t.Errorf("expected BAD response, got %s", imapErr.Type)
	}
}

func TestMultiSearch_EmptyReturn(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 1, Data: &imap.SearchData{Count: 3}},
		},
	}
	ctx, outBuf, done := newTestCommandContextWithOutput(t, `IN (mailboxes INBOX) RETURN () ALL`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	// Should produce ESEARCH response with no result items (just TAG, MAILBOX, UIDVALIDITY, UID)
	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, "MAILBOX INBOX") {
		t.Errorf("response should contain MAILBOX, got: %s", output)
	}
	// No MIN/MAX/COUNT/ALL since no RETURN options were set
	if strings.Contains(output, "MIN") || strings.Contains(output, "MAX") || strings.Contains(output, "COUNT") {
		t.Errorf("empty RETURN should not include result items, got: %s", output)
	}
}

func TestMultiSearch_StateCheck(t *testing.T) {
	sess := &multiSearchMockSession{}

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)
	// Default state is ConnStateNotAuthenticated

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
		Name:    "ESEARCH",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader(`IN (mailboxes INBOX) ALL`)),
	}

	err := handleMultiSearch(ctx)
	if err == nil {
		t.Fatal("expected error for unauthenticated state")
	}
	imapErr, ok := err.(*imap.IMAPError)
	if !ok {
		t.Fatalf("expected IMAPError, got %T", err)
	}
	if imapErr.Type != imap.StatusResponseTypeBAD {
		t.Errorf("expected BAD response, got %s", imapErr.Type)
	}
}

func TestMultiSearch_SelectedState(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 1, Data: &imap.SearchData{}},
		},
	}

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)
	_ = conn.SetState(imap.ConnStateAuthenticated)
	_ = conn.SetState(imap.ConnStateSelected)

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
		Name:    "ESEARCH",
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader(`IN (mailboxes INBOX) ALL`)),
	}

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error in selected state: %v", err)
	}

	if !sess.multiSearchCalled {
		t.Fatal("MultiSearch should have been called in selected state")
	}
}

func TestMultiSearch_MissingIN(t *testing.T) {
	sess := &multiSearchMockSession{}
	ctx := newTestCommandContext(t, `BADKEYWORD (mailboxes INBOX) ALL`, sess)

	err := handleMultiSearch(ctx)
	if err == nil {
		t.Fatal("expected error for missing IN keyword")
	}
	imapErr, ok := err.(*imap.IMAPError)
	if !ok {
		t.Fatalf("expected IMAPError, got %T", err)
	}
	if imapErr.Type != imap.StatusResponseTypeBAD {
		t.Errorf("expected BAD response, got %s", imapErr.Type)
	}
}

func TestMultiSearch_ModSeqInResponse(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 1, Data: &imap.SearchData{Min: 1, Count: 2, ModSeq: 99999}},
		},
	}
	ctx, outBuf, done := newTestCommandContextWithOutput(t, `IN (mailboxes INBOX) RETURN (MIN COUNT) ALL`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "MODSEQ 99999") {
		t.Errorf("response should contain MODSEQ, got: %s", output)
	}
}

func TestMultiSearch_UIDKind(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 1, Data: &imap.SearchData{Count: 1}},
		},
	}

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	conn := server.NewTestConn(serverConn, nil)
	_ = conn.SetState(imap.ConnStateAuthenticated)

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
		Name:    "ESEARCH",
		NumKind: server.NumKindUID,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader(`IN (mailboxes INBOX) RETURN (COUNT) ALL`)),
	}

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.multiSearchKind != server.NumKindUID {
		t.Error("expected UID kind to be passed to session")
	}
}

func TestMultiSearch_ReturnWithCharset(t *testing.T) {
	sess := &multiSearchMockSession{
		multiSearchResult: []imap.MultiSearchResult{
			{Mailbox: "INBOX", UIDValidity: 1, Data: &imap.SearchData{Min: 1, Max: 5, Count: 2}},
		},
	}
	ctx := newTestCommandContext(t, `IN (mailboxes INBOX) RETURN (MIN MAX) CHARSET UTF-8 FLAGGED`, sess)

	if err := handleMultiSearch(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.multiSearchCalled {
		t.Fatal("MultiSearch should have been called")
	}
	if !sess.multiSearchOpts.ReturnMin || !sess.multiSearchOpts.ReturnMax {
		t.Error("RETURN options should be parsed before CHARSET")
	}
	hasFlagged := false
	for _, f := range sess.multiSearchCrit.Flag {
		if f == imap.FlagFlagged {
			hasFlagged = true
		}
	}
	if !hasFlagged {
		t.Error("expected FLAGGED criterion")
	}
}
