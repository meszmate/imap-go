package esearch

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

// esearchMockSession embeds mock.Session and adds SearchExtended.
type esearchMockSession struct {
	mock.Session
	searchExtendedCalled  bool
	searchExtendedKind    server.NumKind
	searchExtendedCrit    *imap.SearchCriteria
	searchExtendedOpts    *imap.SearchOptions
	searchExtendedResult  *imap.SearchData
	searchExtendedErr     error
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

var _ SessionESearch = (*esearchMockSession)(nil)

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

// newTestCommandContextWithOutput creates a context and captures output.
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

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func TestNew(t *testing.T) {
	ext := New()
	if ext.ExtName != "ESEARCH" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "ESEARCH")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapESearch {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
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

func TestSearch_WithReturnMinMax(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.SearchOptions
	var gotCrit *imap.SearchCriteria
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotOpts = options
			gotCrit = criteria
			return &imap.SearchData{Min: 1, Max: 42, Count: 5}, nil
		},
	}
	ctx := newTestCommandContext(t, "RETURN (MIN MAX) UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Search was not called")
	}
	if !gotOpts.ReturnMin {
		t.Error("ReturnMin should be true")
	}
	if !gotOpts.ReturnMax {
		t.Error("ReturnMax should be true")
	}
	if gotOpts.ReturnAll {
		t.Error("ReturnAll should be false")
	}
	if gotOpts.ReturnCount {
		t.Error("ReturnCount should be false")
	}
	if gotCrit == nil {
		t.Fatal("criteria should not be nil")
	}
	if len(gotCrit.NotFlag) != 1 || gotCrit.NotFlag[0] != imap.FlagSeen {
		t.Errorf("expected UNSEEN criterion, got NotFlag=%v", gotCrit.NotFlag)
	}
}

func TestSearch_WithReturnAllCount(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	seqSet, _ := imap.ParseSeqSet("1:5,10")
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{
				Min:   1,
				Max:   10,
				All:   seqSet,
				Count: 5,
			}, nil
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, "RETURN (ALL COUNT) FLAGGED", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, "ALL") {
		t.Errorf("response should contain ALL, got: %s", output)
	}
	if !strings.Contains(output, "COUNT") {
		t.Errorf("response should contain COUNT, got: %s", output)
	}
}

func TestSearch_WithEmptyReturn(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.SearchOptions
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotOpts = options
			return &imap.SearchData{AllSeqNums: []uint32{1, 2, 3}}, nil
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, "RETURN () ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if gotOpts == nil {
		t.Fatal("Search was not called")
	}
	// Empty RETURN () has no options set
	if gotOpts.ReturnMin || gotOpts.ReturnMax || gotOpts.ReturnAll || gotOpts.ReturnCount || gotOpts.ReturnSave {
		t.Error("no RETURN options should be set for RETURN ()")
	}

	// Empty RETURN () should produce traditional SEARCH response
	output := outBuf.String()
	if !strings.Contains(output, "* SEARCH") {
		t.Errorf("expected traditional SEARCH response, got: %s", output)
	}
	if strings.Contains(output, "ESEARCH") {
		t.Errorf("should not contain ESEARCH for empty RETURN, got: %s", output)
	}
}

func TestSearch_WithoutReturn(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.SearchOptions
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotOpts = options
			return &imap.SearchData{AllSeqNums: []uint32{1, 5, 10}}, nil
		},
	}

	ctx, outBuf, done := newTestCommandContextWithOutput(t, "UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = ctx.Conn.Close()
	<-done

	if gotOpts == nil {
		t.Fatal("Search was not called")
	}
	if gotOpts.ReturnMin || gotOpts.ReturnMax || gotOpts.ReturnAll || gotOpts.ReturnCount || gotOpts.ReturnSave {
		t.Error("no RETURN options should be set")
	}

	output := outBuf.String()
	if !strings.Contains(output, "* SEARCH") {
		t.Errorf("expected traditional SEARCH response, got: %s", output)
	}
	if strings.Contains(output, "ESEARCH") {
		t.Errorf("should not contain ESEARCH, got: %s", output)
	}
}

func TestSearch_SessionESearch(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &esearchMockSession{
		searchExtendedResult: &imap.SearchData{Min: 1, Max: 10, Count: 3},
	}
	ctx := newTestCommandContext(t, "RETURN (MIN MAX COUNT) FLAGGED", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.searchExtendedCalled {
		t.Fatal("SearchExtended should have been called")
	}
	if sess.searchExtendedOpts == nil {
		t.Fatal("options should not be nil")
	}
	if !sess.searchExtendedOpts.ReturnMin {
		t.Error("ReturnMin should be true")
	}
	if !sess.searchExtendedOpts.ReturnMax {
		t.Error("ReturnMax should be true")
	}
	if !sess.searchExtendedOpts.ReturnCount {
		t.Error("ReturnCount should be true")
	}
}

func TestSearch_FallbackToSearch(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var searchCalled bool
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			searchCalled = true
			return &imap.SearchData{Min: 1, Max: 5, Count: 2}, nil
		},
	}
	ctx := newTestCommandContext(t, "RETURN (MIN) UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchCalled {
		t.Fatal("Session.Search should be called as fallback")
	}
}

func TestUIDSearch_ESearchResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			if kind != server.NumKindUID {
				t.Error("expected UID kind")
			}
			return &imap.SearchData{Min: 1, Max: 42, Count: 5}, nil
		},
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
		Decoder: wire.NewDecoder(strings.NewReader("RETURN (MIN MAX COUNT) ALL")),
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
}

func TestSearch_ESearchResponseFormat(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{Min: 1, Max: 42, Count: 5}, nil
		},
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
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("RETURN (MIN MAX COUNT) ALL")),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, `* ESEARCH (TAG "A001") MIN 1 MAX 42 COUNT 5`) {
		t.Errorf("unexpected ESEARCH response format: %s", output)
	}
}

func TestSearch_NoResults(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{}, nil
		},
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
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("RETURN (MIN MAX COUNT) ALL")),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	// Empty ESEARCH: just TAG correlator, no result items
	if !strings.Contains(output, `* ESEARCH (TAG "A001")`) {
		t.Errorf("expected ESEARCH with TAG but no results, got: %s", output)
	}
	if strings.Contains(output, "MIN") || strings.Contains(output, "MAX") || strings.Contains(output, "COUNT") {
		t.Errorf("empty results should not contain result items, got: %s", output)
	}
}

func TestSearch_CriteriaParsed(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotCrit *imap.SearchCriteria
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotCrit = criteria
			return &imap.SearchData{Min: 1, Count: 1}, nil
		},
	}
	ctx := newTestCommandContext(t, "RETURN (COUNT) UNSEEN FLAGGED", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCrit == nil {
		t.Fatal("criteria should not be nil")
	}
	// UNSEEN sets NotFlag=[\Seen]
	hasUnseen := false
	for _, f := range gotCrit.NotFlag {
		if f == imap.FlagSeen {
			hasUnseen = true
		}
	}
	if !hasUnseen {
		t.Error("expected UNSEEN (NotFlag \\Seen)")
	}
	// FLAGGED sets Flag=[\Flagged]
	hasFlagged := false
	for _, f := range gotCrit.Flag {
		if f == imap.FlagFlagged {
			hasFlagged = true
		}
	}
	if !hasFlagged {
		t.Error("expected FLAGGED (Flag \\Flagged)")
	}
}

func TestSearch_WithoutReturnCriteriaParsed(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotCrit *imap.SearchCriteria
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotCrit = criteria
			return &imap.SearchData{AllSeqNums: []uint32{1}}, nil
		},
	}
	ctx := newTestCommandContext(t, "UNSEEN FLAGGED", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCrit == nil {
		t.Fatal("criteria should not be nil")
	}
	hasUnseen := false
	for _, f := range gotCrit.NotFlag {
		if f == imap.FlagSeen {
			hasUnseen = true
		}
	}
	if !hasUnseen {
		t.Error("expected UNSEEN (NotFlag \\Seen)")
	}
	hasFlagged := false
	for _, f := range gotCrit.Flag {
		if f == imap.FlagFlagged {
			hasFlagged = true
		}
	}
	if !hasFlagged {
		t.Error("expected FLAGGED (Flag \\Flagged)")
	}
}

func TestSearch_ReturnSave(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.SearchOptions
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotOpts = options
			return &imap.SearchData{Min: 1, Max: 5, Count: 3}, nil
		},
	}
	ctx := newTestCommandContext(t, "RETURN (SAVE MIN MAX COUNT) ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Search was not called")
	}
	if !gotOpts.ReturnSave {
		t.Error("ReturnSave should be true")
	}
	if !gotOpts.ReturnMin {
		t.Error("ReturnMin should be true")
	}
}

func TestSearch_ModSeqInESearchResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{Min: 1, Max: 5, Count: 2, ModSeq: 99999}, nil
		},
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
		NumKind: server.NumKindSeq,
		Conn:    conn,
		Session: sess,
		Decoder: wire.NewDecoder(strings.NewReader("RETURN (MIN MAX COUNT) ALL")),
	}

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = serverConn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "MODSEQ 99999") {
		t.Errorf("response should contain MODSEQ, got: %s", output)
	}
}
