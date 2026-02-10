package partial

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"

	imap "github.com/meszmate/imap-go"
	"github.com/meszmate/imap-go/extensions/esearch"
	"github.com/meszmate/imap-go/extensions/esort"
	"github.com/meszmate/imap-go/imaptest/mock"
	"github.com/meszmate/imap-go/server"
	"github.com/meszmate/imap-go/wire"
)

// partialMockSession embeds mock.Session and implements SessionPartial.
type partialMockSession struct {
	mock.Session
	searchPartialCalled bool
	searchPartialKind   server.NumKind
	searchPartialCrit   *imap.SearchCriteria
	searchPartialOpts   *imap.SearchOptions
	searchPartialResult *imap.SearchData
	searchPartialErr    error
}

func (m *partialMockSession) SearchPartial(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
	m.searchPartialCalled = true
	m.searchPartialKind = kind
	m.searchPartialCrit = criteria
	m.searchPartialOpts = options
	if m.searchPartialResult != nil {
		return m.searchPartialResult, m.searchPartialErr
	}
	return &imap.SearchData{}, m.searchPartialErr
}

var _ SessionPartial = (*partialMockSession)(nil)

// esearchMockSession embeds mock.Session and implements SessionESearch.
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

var _ esearch.SessionESearch = (*esearchMockSession)(nil)

// esortMockSession embeds mock.Session and implements SessionESort.
type esortMockSession struct {
	mock.Session
	sortExtendedCalled bool
	sortExtendedKind   server.NumKind
	sortExtendedCrit   []imap.SortCriterion
	sortExtendedSearch *imap.SearchCriteria
	sortExtendedOpts   *imap.SearchOptions
	sortExtendedResult *imap.SearchData
	sortExtendedErr    error
}

func (m *esortMockSession) SortExtended(kind server.NumKind, criteria []imap.SortCriterion, searchCriteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
	m.sortExtendedCalled = true
	m.sortExtendedKind = kind
	m.sortExtendedCrit = criteria
	m.sortExtendedSearch = searchCriteria
	m.sortExtendedOpts = options
	if m.sortExtendedResult != nil {
		return m.sortExtendedResult, m.sortExtendedErr
	}
	return &imap.SearchData{}, m.sortExtendedErr
}

var _ esort.SessionESort = (*esortMockSession)(nil)

var dummyHandler = server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
	return nil
})

func newTestCtx(t *testing.T, name, args string, sess server.Session) *server.CommandContext {
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
	if ext.ExtName != "PARTIAL" {
		t.Errorf("ExtName = %q, want %q", ext.ExtName, "PARTIAL")
	}
	if len(ext.ExtCapabilities) != 1 || ext.ExtCapabilities[0] != imap.CapPartial {
		t.Errorf("unexpected capabilities: %v", ext.ExtCapabilities)
	}
	if len(ext.ExtDependencies) != 1 || ext.ExtDependencies[0] != "ESEARCH" {
		t.Errorf("unexpected dependencies: %v", ext.ExtDependencies)
	}
}

func TestWrapHandler_Commands(t *testing.T) {
	ext := New()
	for _, name := range []string{"SEARCH", "SORT"} {
		if ext.WrapHandler(name, dummyHandler) == nil {
			t.Errorf("WrapHandler(%q) returned nil, want non-nil", name)
		}
	}
	for _, name := range []string{"FETCH", "NOOP", "SELECT", "LIST"} {
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
	if _, ok := se.(*SessionPartial); !ok {
		t.Errorf("SessionExtension() returned %T, want *SessionPartial", se)
	}
}

func TestOnEnabled(t *testing.T) {
	ext := New()
	if err := ext.OnEnabled("test-conn"); err != nil {
		t.Errorf("OnEnabled() returned error: %v", err)
	}
}

func TestParseReturnOptions_Partial(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("(PARTIAL 1:100)"))
	opts := &imap.SearchOptions{}
	if err := parseReturnOptions(dec, opts); err != nil {
		t.Fatalf("parseReturnOptions() error: %v", err)
	}
	if opts.ReturnPartial == nil {
		t.Fatal("ReturnPartial should not be nil")
	}
	if opts.ReturnPartial.Offset != 1 {
		t.Errorf("Offset = %d, want 1", opts.ReturnPartial.Offset)
	}
	if opts.ReturnPartial.Count != 100 {
		t.Errorf("Count = %d, want 100", opts.ReturnPartial.Count)
	}
}

func TestParseReturnOptions_PartialWithOtherOptions(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("(PARTIAL 1:50 COUNT)"))
	opts := &imap.SearchOptions{}
	if err := parseReturnOptions(dec, opts); err != nil {
		t.Fatalf("parseReturnOptions() error: %v", err)
	}
	if opts.ReturnPartial == nil {
		t.Fatal("ReturnPartial should not be nil")
	}
	if opts.ReturnPartial.Offset != 1 || opts.ReturnPartial.Count != 50 {
		t.Errorf("Partial = {%d, %d}, want {1, 50}", opts.ReturnPartial.Offset, opts.ReturnPartial.Count)
	}
	if !opts.ReturnCount {
		t.Error("ReturnCount should be true")
	}
}

func TestParseReturnOptions_Empty(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("()"))
	opts := &imap.SearchOptions{}
	if err := parseReturnOptions(dec, opts); err != nil {
		t.Fatalf("parseReturnOptions() error: %v", err)
	}
	if opts.ReturnMin || opts.ReturnMax || opts.ReturnAll || opts.ReturnCount || opts.ReturnSave || opts.ReturnPartial != nil {
		t.Error("empty () should have no options set")
	}
}

func TestParseReturnOptions_StandardOptions(t *testing.T) {
	dec := wire.NewDecoder(strings.NewReader("(MIN MAX ALL COUNT SAVE)"))
	opts := &imap.SearchOptions{}
	if err := parseReturnOptions(dec, opts); err != nil {
		t.Fatalf("parseReturnOptions() error: %v", err)
	}
	if !opts.ReturnMin || !opts.ReturnMax || !opts.ReturnAll || !opts.ReturnCount || !opts.ReturnSave {
		t.Error("all standard options should be set")
	}
	if opts.ReturnPartial != nil {
		t.Error("ReturnPartial should be nil")
	}
}

func TestParsePartialRange(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOffset int32
		wantCount  uint32
		wantErr    bool
	}{
		{"positive", "1:100", 1, 100, false},
		{"negative offset", "-1:100", -1, 100, false},
		{"negative large", "-50:25", -50, 25, false},
		{"invalid no colon", "abc", 0, 0, true},
		{"zero offset", "0:100", 0, 0, true},
		{"zero count", "1:0", 0, 0, true},
		{"invalid offset", "abc:100", 0, 0, true},
		{"invalid count", "1:abc", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, count, err := parsePartialRange(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePartialRange(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr {
				if offset != tt.wantOffset {
					t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
				}
				if count != tt.wantCount {
					t.Errorf("count = %d, want %d", count, tt.wantCount)
				}
			}
		})
	}
}

func TestHasAnyReturnOption(t *testing.T) {
	if hasAnyReturnOption(&imap.SearchOptions{}) {
		t.Error("empty options should return false")
	}
	if !hasAnyReturnOption(&imap.SearchOptions{ReturnMin: true}) {
		t.Error("ReturnMin should return true")
	}
	if !hasAnyReturnOption(&imap.SearchOptions{ReturnSave: true}) {
		t.Error("ReturnSave should return true")
	}
	if !hasAnyReturnOption(&imap.SearchOptions{ReturnPartial: &imap.SearchReturnPartial{Offset: 1, Count: 10}}) {
		t.Error("ReturnPartial should return true")
	}
}

func TestSearch_WithPartial(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.SearchOptions
	var gotCrit *imap.SearchCriteria
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotOpts = options
			gotCrit = criteria
			return &imap.SearchData{
				Partial: &imap.SearchPartialData{
					Offset: 1,
					Total:  200,
					UIDs:   []imap.UID{1, 2, 3, 4, 5},
				},
			}, nil
		},
	}

	ctx := newTestCtx(t, "SEARCH", "RETURN (PARTIAL 1:5) UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotOpts == nil {
		t.Fatal("Search was not called")
	}
	if gotOpts.ReturnPartial == nil {
		t.Fatal("ReturnPartial should not be nil")
	}
	if gotOpts.ReturnPartial.Offset != 1 || gotOpts.ReturnPartial.Count != 5 {
		t.Errorf("Partial = {%d, %d}, want {1, 5}", gotOpts.ReturnPartial.Offset, gotOpts.ReturnPartial.Count)
	}
	if gotCrit == nil {
		t.Fatal("criteria should not be nil")
	}
	if len(gotCrit.NotFlag) != 1 || gotCrit.NotFlag[0] != imap.FlagSeen {
		t.Errorf("expected UNSEEN criterion, got NotFlag=%v", gotCrit.NotFlag)
	}
}

func TestSearch_PartialResponse(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{
				Partial: &imap.SearchPartialData{
					Offset: 1,
					Total:  200,
					UIDs:   []imap.UID{10, 11, 12},
				},
			}, nil
		},
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "SEARCH", "RETURN (PARTIAL 1:100) ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, "PARTIAL") {
		t.Errorf("response should contain PARTIAL, got: %s", output)
	}
	if !strings.Contains(output, "1:100") {
		t.Errorf("response should contain range 1:100, got: %s", output)
	}
	if !strings.Contains(output, "200") {
		t.Errorf("response should contain total 200, got: %s", output)
	}
	if !strings.Contains(output, "10,11,12") && !strings.Contains(output, "10:12") {
		t.Errorf("response should contain UIDs, got: %s", output)
	}
}

func TestSearch_PartialEmptyResult(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{}, nil
		},
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "SEARCH", "RETURN (PARTIAL 1:100) ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, "PARTIAL") {
		t.Errorf("response should contain PARTIAL, got: %s", output)
	}
	// Should have PARTIAL (1:100 0) for empty results
	if !strings.Contains(output, "1:100 0") {
		t.Errorf("response should contain 1:100 0 for empty result, got: %s", output)
	}
}

func TestSearch_PartialNegativeOffset(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	var gotOpts *imap.SearchOptions
	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			gotOpts = options
			return &imap.SearchData{
				Partial: &imap.SearchPartialData{
					Offset: -1,
					Total:  50,
					UIDs:   []imap.UID{50},
				},
			}, nil
		},
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "SEARCH", "RETURN (PARTIAL -1:100) ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	if gotOpts == nil || gotOpts.ReturnPartial == nil {
		t.Fatal("ReturnPartial should be set")
	}
	if gotOpts.ReturnPartial.Offset != -1 {
		t.Errorf("Offset = %d, want -1", gotOpts.ReturnPartial.Offset)
	}

	output := outBuf.String()
	if !strings.Contains(output, "-1:100") {
		t.Errorf("response should contain -1:100, got: %s", output)
	}
}

func TestSearch_SessionPartialRouting(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &partialMockSession{
		searchPartialResult: &imap.SearchData{
			Partial: &imap.SearchPartialData{
				Offset: 1,
				Total:  100,
				UIDs:   []imap.UID{1, 2, 3},
			},
		},
	}

	ctx := newTestCtx(t, "SEARCH", "RETURN (PARTIAL 1:10) ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.searchPartialCalled {
		t.Fatal("SearchPartial should have been called")
	}
	if sess.searchPartialOpts == nil || sess.searchPartialOpts.ReturnPartial == nil {
		t.Fatal("ReturnPartial should be set in options")
	}
}

func TestSearch_FallbackToSearchExtended(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &esearchMockSession{
		searchExtendedResult: &imap.SearchData{
			Partial: &imap.SearchPartialData{
				Offset: 1,
				Total:  50,
				UIDs:   []imap.UID{10, 20},
			},
		},
	}

	ctx := newTestCtx(t, "SEARCH", "RETURN (PARTIAL 1:10) ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.searchExtendedCalled {
		t.Fatal("SearchExtended should have been called as fallback")
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

	ctx := newTestCtx(t, "SEARCH", "RETURN (PARTIAL 1:10) ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchCalled {
		t.Fatal("Session.Search should be called as last resort fallback")
	}
}

func TestSearch_WithoutReturn(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{AllSeqNums: []uint32{1, 5, 10}}, nil
		},
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "SEARCH", "UNSEEN", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "* SEARCH") {
		t.Errorf("expected traditional SEARCH response, got: %s", output)
	}
	if strings.Contains(output, "ESEARCH") {
		t.Errorf("should not contain ESEARCH, got: %s", output)
	}
}

func TestSearch_ReturnWithoutPartial_UsesESearch(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &esearchMockSession{
		searchExtendedResult: &imap.SearchData{Min: 1, Max: 10, Count: 5},
	}

	ctx := newTestCtx(t, "SEARCH", "RETURN (MIN MAX COUNT) FLAGGED", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.searchExtendedCalled {
		t.Fatal("SearchExtended should have been called for RETURN without PARTIAL")
	}
}

func TestSort_WithReturnPartial(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &esortMockSession{
		sortExtendedResult: &imap.SearchData{
			Partial: &imap.SearchPartialData{
				Offset: 1,
				Total:  300,
				UIDs:   []imap.UID{100, 101, 102},
			},
		},
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "SORT", "RETURN (PARTIAL 1:50) (DATE) UTF-8 ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	if !sess.sortExtendedCalled {
		t.Fatal("SortExtended should have been called")
	}
	if sess.sortExtendedOpts == nil || sess.sortExtendedOpts.ReturnPartial == nil {
		t.Fatal("ReturnPartial should be set in sort options")
	}
	if sess.sortExtendedOpts.ReturnPartial.Offset != 1 || sess.sortExtendedOpts.ReturnPartial.Count != 50 {
		t.Errorf("Partial = {%d, %d}, want {1, 50}", sess.sortExtendedOpts.ReturnPartial.Offset, sess.sortExtendedOpts.ReturnPartial.Count)
	}
	if len(sess.sortExtendedCrit) != 1 || sess.sortExtendedCrit[0].Key != "DATE" {
		t.Errorf("sort criteria = %v, want [{DATE false}]", sess.sortExtendedCrit)
	}

	output := outBuf.String()
	if !strings.Contains(output, "ESEARCH") {
		t.Errorf("response should contain ESEARCH, got: %s", output)
	}
	if !strings.Contains(output, "PARTIAL") {
		t.Errorf("response should contain PARTIAL, got: %s", output)
	}
}

func TestSort_WithoutReturn_Delegates(t *testing.T) {
	originalCalled := false
	original := server.CommandHandlerFunc(func(ctx *server.CommandContext) error {
		originalCalled = true
		return nil
	})

	ext := New()
	h := ext.WrapHandler("SORT", original).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCtx(t, "SORT", "(DATE) UTF-8 ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalCalled {
		t.Error("original handler should have been called for SORT without RETURN")
	}
}

func TestSort_WithReverseCriteria(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &esortMockSession{
		sortExtendedResult: &imap.SearchData{
			Partial: &imap.SearchPartialData{
				Offset: 1,
				Total:  10,
				UIDs:   []imap.UID{5},
			},
		},
	}

	ctx := newTestCtx(t, "SORT", "RETURN (PARTIAL 1:10) (REVERSE DATE SUBJECT) UTF-8 ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sess.sortExtendedCalled {
		t.Fatal("SortExtended should have been called")
	}
	if len(sess.sortExtendedCrit) != 2 {
		t.Fatalf("expected 2 sort criteria, got %d", len(sess.sortExtendedCrit))
	}
	if !sess.sortExtendedCrit[0].Reverse || sess.sortExtendedCrit[0].Key != "DATE" {
		t.Errorf("first criterion = %+v, want {REVERSE DATE}", sess.sortExtendedCrit[0])
	}
	if sess.sortExtendedCrit[1].Reverse || sess.sortExtendedCrit[1].Key != "SUBJECT" {
		t.Errorf("second criterion = %+v, want {SUBJECT}", sess.sortExtendedCrit[1])
	}
}

func TestSort_NoESort_ReturnsError(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SORT", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{}
	ctx := newTestCtx(t, "SORT", "RETURN (PARTIAL 1:10) (DATE) UTF-8 ALL", sess)

	err := h.Handle(ctx)
	if err == nil {
		t.Fatal("expected error when session doesn't implement SessionESort")
	}
}

func TestSearch_PartialWithCount(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			return &imap.SearchData{
				Count: 200,
				Partial: &imap.SearchPartialData{
					Offset: 1,
					Total:  200,
					UIDs:   []imap.UID{1, 2, 3},
				},
			}, nil
		},
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "SEARCH", "RETURN (PARTIAL 1:10 COUNT) ALL", sess)

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "COUNT 200") {
		t.Errorf("response should contain COUNT 200, got: %s", output)
	}
	if !strings.Contains(output, "PARTIAL") {
		t.Errorf("response should contain PARTIAL, got: %s", output)
	}
}

func TestSearch_UIDKind(t *testing.T) {
	ext := New()
	h := ext.WrapHandler("SEARCH", dummyHandler).(server.CommandHandlerFunc)

	sess := &mock.Session{
		SearchFunc: func(kind server.NumKind, criteria *imap.SearchCriteria, options *imap.SearchOptions) (*imap.SearchData, error) {
			if kind != server.NumKindUID {
				t.Error("expected UID kind")
			}
			return &imap.SearchData{
				Partial: &imap.SearchPartialData{
					Offset: 1,
					Total:  10,
					UIDs:   []imap.UID{1},
				},
			}, nil
		},
	}

	ctx, outBuf, done := newTestCtxWithOutput(t, "SEARCH", "RETURN (PARTIAL 1:10) ALL", sess)
	ctx.NumKind = server.NumKindUID

	if err := h.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx.Conn.Close()
	<-done

	output := outBuf.String()
	if !strings.Contains(output, "UID") {
		t.Errorf("response should contain UID flag, got: %s", output)
	}
	if !strings.Contains(output, `(TAG "A001")`) {
		t.Errorf("response should contain TAG correlator, got: %s", output)
	}
}
