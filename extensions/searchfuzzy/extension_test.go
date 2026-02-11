package searchfuzzy

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extensions/esearch"
	"github.com/meszmate/imap-go/imaptest/mock"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// fuzzyMockSession embeds mock.Session and implements SessionSearchFuzzy.
type fuzzyMockSession struct {
	mock.Session
	searchFuzzyCalled bool
	searchFuzzyKind   server.NumKind
	searchFuzzyCrit   *imap.SearchCriteria
	searchFuzzyOpts   *imap.SearchOptions
	searchFuzzyResult *imap.SearchData
	searchFuzzyErr    error
}

func (m *fuzzyMockSession) SearchFuzzy(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
	m.searchFuzzyCalled = true
	m.searchFuzzyKind = kind
	m.searchFuzzyCrit = criteria
	m.searchFuzzyOpts = options
	if m.searchFuzzyResult != nil {
		return m.searchFuzzyResult, m.searchFuzzyErr
	}
	return &imap.SearchData{}, m.searchFuzzyErr
}

var _ SessionSearchFuzzy = (*fuzzyMockSession)(nil)

// esearchMockSession embeds mock.Session and implements SessionESearch.
type esearchMockSession struct {
	mock.Session
	searchExtendedCalled bool
	searchExtendedKind   server.NumKind
	searchExtendedCrit   *imap.SearchCriteria
	searchExtendedOpts   *imap.SearchOptions
	searchExtendedResult *imap.SearchData
	searchExtendedErr    error
}

func (m *esearchMockSession) SearchExtended(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
	m.searchExtendedCalled = true
	m.searchExtendedKind = kind
	m.searchExtendedCrit = criteria
	m.searchExtendedOpts = options
	if m.searchExtendedResult != nil {
		return m.searchExtendedResult, m.searchExtendedErr
	}
	return &imap.SearchData{}, m.searchExtendedErr
}

var _ esearch.SessionESearch = (*esearchMockSession)(nil)

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func newTestCtx(t *testing.T, name, args string, sess server.Session) *server.CommandContext {
	t.Helper()

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

	var dec *wire.Decoder
	if args != "" {
		dec = wire.NewDecoder(strings.NewReader(args))
	}

	return &server.CommandContext{
		Context: context.Background(),
		Tag:     "A001",
		Name:    name,
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}
}

func newTestCtxWithOutput(t *testing.T, name, args string, sess server.Session) (*server.CommandContext, *bytes.Buffer, chan struct{}) {
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
		Name:    name,
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: dec,
	}

	return ctx, &outBuf, done
}

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "SEARCH=FUZZY" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "SEARCH=FUZZY")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapSearchFuzzy {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
}

func TestWrapHandler_Commands(t *testing.T) {
	ext := New()
	if ext.WrapHandler("SEARCH", dummyHandler) == nil {
		t.Error("WrapHandler(SEARCH) returned nil, want non-nil")
	}
	for _, name := range []string{"FETCH", "SORT", "NOOP", "SELECT", "LIST"} {
		if ext.WrapHandler(name, dummyHandler) != nil {
			t.Errorf("WrapHandler(%q) returned non-nil, want nil", name)
		}
	}
}

func TestSessionExtension(t *testing.T) {
	ext := New()
	se := ext.SessionExtension()
	if se == nil {
		t.Fatal("SessionExtension() returned nil")
	}
	if _, ok := se.(*SessionSearchFuzzy); !ok {
		t.Errorf("SessionExtension() returned %T, want *SessionSearchFuzzy", se)
	}
}

func TestOnEnabled(t *testing.T) {
	ext := New()
	if err := ext.OnEnabled("test-conn"); err != nil {
		t.Errorf("OnEnabled() returned error: %v", err)
	}
}

func TestSearch_FuzzySubject(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotCrit *imap.SearchCriteria
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotCrit = criteria
			return &imap.SearchData{AllSeqNums: []uint32{1, 2}}, nil
		},
	}

	ctx := newTestCtx(t, "SEARCH", `FUZZY SUBJECT "meeting"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCrit == nil {
		t.Fatal("Search was not called")
	}
	if !gotCrit.Fuzzy {
		t.Error("Fuzzy should be true")
	}
	if len(gotCrit.Header) != 1 || gotCrit.Header[0].Key != "Subject" || gotCrit.Header[0].Value != "meeting" {
		t.Errorf("unexpected criteria Header: %v", gotCrit.Header)
	}
}

func TestSearch_MultipleFuzzy(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotCrit *imap.SearchCriteria
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotCrit = criteria
			return &imap.SearchData{}, nil
		},
	}

	ctx := newTestCtx(t, "SEARCH", `FUZZY SUBJECT "meeting" FUZZY FROM "alice"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCrit == nil {
		t.Fatal("Search was not called")
	}
	if !gotCrit.Fuzzy {
		t.Error("Fuzzy should be true")
	}
	if len(gotCrit.Header) != 2 {
		t.Fatalf("expected 2 header criteria, got %d", len(gotCrit.Header))
	}
	if gotCrit.Header[0].Key != "Subject" || gotCrit.Header[0].Value != "meeting" {
		t.Errorf("first header = %+v, want Subject:meeting", gotCrit.Header[0])
	}
	if gotCrit.Header[1].Key != "From" || gotCrit.Header[1].Value != "alice" {
		t.Errorf("second header = %+v, want From:alice", gotCrit.Header[1])
	}
}

func TestSearch_FuzzyWithReturn(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotCrit *imap.SearchCriteria
	var gotOpts *imap.SearchOptions
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotCrit = criteria
			gotOpts = options
			return &imap.SearchData{Count: 5}, nil
		},
	}

	ctx := newTestCtx(t, "SEARCH", `RETURN (COUNT) FUZZY SUBJECT "meeting"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCrit == nil {
		t.Fatal("Search was not called")
	}
	if !gotCrit.Fuzzy {
		t.Error("Fuzzy should be true")
	}
	if gotOpts == nil || !gotOpts.ReturnCount {
		t.Error("ReturnCount should be true")
	}
}

func TestSearch_NoFuzzy_Passthrough(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotCrit *imap.SearchCriteria
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotCrit = criteria
			return &imap.SearchData{AllSeqNums: []uint32{1}}, nil
		},
	}

	ctx := newTestCtx(t, "SEARCH", "UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCrit == nil {
		t.Fatal("Search was not called")
	}
	if gotCrit.Fuzzy {
		t.Error("Fuzzy should be false for non-FUZZY criteria")
	}
	if len(gotCrit.NotFlag) != 1 || gotCrit.NotFlag[0] != imap.FlagSeen {
		t.Errorf("expected UNSEEN criterion, got NotFlag=%v", gotCrit.NotFlag)
	}
}

func TestSearch_MixedFuzzyAndNonFuzzy(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotCrit *imap.SearchCriteria
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotCrit = criteria
			return &imap.SearchData{}, nil
		},
	}

	ctx := newTestCtx(t, "SEARCH", `FUZZY SUBJECT "meeting" UNSEEN`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCrit == nil {
		t.Fatal("Search was not called")
	}
	if !gotCrit.Fuzzy {
		t.Error("Fuzzy should be true (at least one FUZZY prefix)")
	}
	if len(gotCrit.Header) != 1 || gotCrit.Header[0].Key != "Subject" {
		t.Errorf("expected Subject header, got %v", gotCrit.Header)
	}
	if len(gotCrit.NotFlag) != 1 || gotCrit.NotFlag[0] != imap.FlagSeen {
		t.Errorf("expected UNSEEN, got NotFlag=%v", gotCrit.NotFlag)
	}
}

func TestSearch_SessionFuzzyRouting(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &fuzzyMockSession{
		searchFuzzyResult: &imap.SearchData{AllSeqNums: []uint32{1, 2, 3}},
	}

	ctx := newTestCtx(t, "SEARCH", `FUZZY SUBJECT "meeting"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.searchFuzzyCalled {
		t.Fatal("SearchFuzzy should have been called")
	}
	if sess.searchFuzzyCrit == nil || !sess.searchFuzzyCrit.Fuzzy {
		t.Error("criteria.Fuzzy should be true")
	}
}

func TestSearch_FallbackToESearch(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &esearchMockSession{
		searchExtendedResult: &imap.SearchData{Count: 3},
	}

	ctx := newTestCtx(t, "SEARCH", `RETURN (COUNT) FUZZY SUBJECT "meeting"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.searchExtendedCalled {
		t.Fatal("SearchExtended should have been called as fallback")
	}
	if sess.searchExtendedCrit == nil || !sess.searchExtendedCrit.Fuzzy {
		t.Error("criteria.Fuzzy should be true")
	}
}

func TestSearch_FallbackToSearch(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var searchCalled bool
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			searchCalled = true
			return &imap.SearchData{}, nil
		},
	}

	ctx := newTestCtx(t, "SEARCH", `FUZZY SUBJECT "meeting"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchCalled {
		t.Fatal("Session.Search should be called as last resort fallback")
	}
}

func TestSearch_FuzzyWithUID(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			if kind != server.NumKindUID {
				t.Error("expected UID kind")
			}
			return &imap.SearchData{Count: 2}, nil
		},
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "SEARCH", `RETURN (COUNT) FUZZY SUBJECT "test"`, sess)
	ctx.NumKind = server.NumKindUID

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
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
}

func TestSearch_FuzzyTraditionalResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{AllSeqNums: []uint32{1, 5, 10}}, nil
		},
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "SEARCH", `FUZZY SUBJECT "meeting"`, sess)

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
		t.Errorf("should not contain ESEARCH without RETURN, got: %s", output)
	}
}

func TestSearch_FuzzyESearchResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{Min: 1, Max: 10, Count: 5}, nil
		},
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "SEARCH", `RETURN (MIN MAX COUNT) FUZZY SUBJECT "test"`, sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, "MIN 1") {
		t.Errorf("response should contain MIN 1, got: %s", output)
	}
	if !strings.Contains(output, "MAX 10") {
		t.Errorf("response should contain MAX 10, got: %s", output)
	}
	if !strings.Contains(output, "COUNT 5") {
		t.Errorf("response should contain COUNT 5, got: %s", output)
	}
}
