package imap

import (
	"errors"
	"testing"
)

func TestStatusResponse_Error(t *testing.T) {
	tests := []struct {
		name string
		resp StatusResponse
		want string
	}{
		{
			"OK only",
			StatusResponse{Type: StatusResponseTypeOK},
			"OK",
		},
		{
			"OK with text",
			StatusResponse{Type: StatusResponseTypeOK, Text: "Login completed"},
			"OK Login completed",
		},
		{
			"NO with text",
			StatusResponse{Type: StatusResponseTypeNO, Text: "Mailbox not found"},
			"NO Mailbox not found",
		},
		{
			"BAD with text",
			StatusResponse{Type: StatusResponseTypeBAD, Text: "Command unknown"},
			"BAD Command unknown",
		},
		{
			"BYE with text",
			StatusResponse{Type: StatusResponseTypeBYE, Text: "Server shutting down"},
			"BYE Server shutting down",
		},
		{
			"PREAUTH with text",
			StatusResponse{Type: StatusResponseTypePREAUTH, Text: "Logged in as admin"},
			"PREAUTH Logged in as admin",
		},
		{
			"OK with code",
			StatusResponse{
				Type: StatusResponseTypeOK,
				Code: ResponseCodeCapability,
				Text: "done",
			},
			"OK [CAPABILITY] done",
		},
		{
			"OK with code and arg",
			StatusResponse{
				Type:    StatusResponseTypeOK,
				Code:    ResponseCodeUIDNext,
				CodeArg: 42,
				Text:    "predicted",
			},
			"OK [UIDNEXT 42] predicted",
		},
		{
			"NO with code",
			StatusResponse{
				Type: StatusResponseTypeNO,
				Code: ResponseCodeTryCreate,
				Text: "Mailbox does not exist",
			},
			"NO [TRYCREATE] Mailbox does not exist",
		},
		{
			"code with string arg",
			StatusResponse{
				Type:    StatusResponseTypeOK,
				Code:    ResponseCodeMailboxID,
				CodeArg: "abc123",
			},
			"OK [MAILBOXID abc123]",
		},
		{
			"code no text no arg",
			StatusResponse{
				Type: StatusResponseTypeOK,
				Code: ResponseCodeReadOnly,
			},
			"OK [READ-ONLY]",
		},
		{
			"empty text empty code",
			StatusResponse{Type: StatusResponseTypeOK},
			"OK",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.Error()
			if got != tt.want {
				t.Errorf("StatusResponse.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIMAPError_Error(t *testing.T) {
	err := &IMAPError{&StatusResponse{
		Type: StatusResponseTypeNO,
		Text: "something went wrong",
	}}
	want := "NO something went wrong"
	if got := err.Error(); got != want {
		t.Errorf("IMAPError.Error() = %q, want %q", got, want)
	}
}

func TestIMAPError_Unwrap(t *testing.T) {
	err := &IMAPError{&StatusResponse{
		Type: StatusResponseTypeNO,
		Text: "test",
	}}
	if unwrapped := err.Unwrap(); unwrapped != nil {
		t.Errorf("IMAPError.Unwrap() = %v, want nil", unwrapped)
	}
}

func TestIMAPError_ImplementsError(t *testing.T) {
	var _ error = &IMAPError{}
}

func TestErrNo(t *testing.T) {
	err := ErrNo("mailbox not found")
	if err == nil {
		t.Fatal("ErrNo should return a non-nil error")
	}

	want := "NO mailbox not found"
	if got := err.Error(); got != want {
		t.Errorf("ErrNo.Error() = %q, want %q", got, want)
	}

	// Check the underlying StatusResponse
	if err.Type != StatusResponseTypeNO {
		t.Errorf("Type = %q, want %q", err.Type, StatusResponseTypeNO)
	}
	if err.Text != "mailbox not found" {
		t.Errorf("Text = %q, want %q", err.Text, "mailbox not found")
	}
	if err.Code != "" {
		t.Errorf("Code = %q, want empty", err.Code)
	}
}

func TestErrNoWithCode(t *testing.T) {
	err := ErrNoWithCode(ResponseCodeNonExistent, "mailbox does not exist")

	want := "NO [NONEXISTENT] mailbox does not exist"
	if got := err.Error(); got != want {
		t.Errorf("ErrNoWithCode.Error() = %q, want %q", got, want)
	}

	if err.Type != StatusResponseTypeNO {
		t.Errorf("Type = %q, want %q", err.Type, StatusResponseTypeNO)
	}
	if err.Code != ResponseCodeNonExistent {
		t.Errorf("Code = %q, want %q", err.Code, ResponseCodeNonExistent)
	}
	if err.Text != "mailbox does not exist" {
		t.Errorf("Text = %q, want %q", err.Text, "mailbox does not exist")
	}
}

func TestErrBad(t *testing.T) {
	err := ErrBad("syntax error")

	want := "BAD syntax error"
	if got := err.Error(); got != want {
		t.Errorf("ErrBad.Error() = %q, want %q", got, want)
	}

	if err.Type != StatusResponseTypeBAD {
		t.Errorf("Type = %q, want %q", err.Type, StatusResponseTypeBAD)
	}
	if err.Text != "syntax error" {
		t.Errorf("Text = %q, want %q", err.Text, "syntax error")
	}
	if err.Code != "" {
		t.Errorf("Code = %q, want empty", err.Code)
	}
}

func TestErrBye(t *testing.T) {
	err := ErrBye("server shutting down")

	want := "BYE server shutting down"
	if got := err.Error(); got != want {
		t.Errorf("ErrBye.Error() = %q, want %q", got, want)
	}

	if err.Type != StatusResponseTypeBYE {
		t.Errorf("Type = %q, want %q", err.Type, StatusResponseTypeBYE)
	}
	if err.Text != "server shutting down" {
		t.Errorf("Text = %q, want %q", err.Text, "server shutting down")
	}
}

func TestErrNo_EmptyText(t *testing.T) {
	err := ErrNo("")
	want := "NO"
	if got := err.Error(); got != want {
		t.Errorf("ErrNo(\"\").Error() = %q, want %q", got, want)
	}
}

func TestErrBad_EmptyText(t *testing.T) {
	err := ErrBad("")
	want := "BAD"
	if got := err.Error(); got != want {
		t.Errorf("ErrBad(\"\").Error() = %q, want %q", got, want)
	}
}

func TestErrBye_EmptyText(t *testing.T) {
	err := ErrBye("")
	want := "BYE"
	if got := err.Error(); got != want {
		t.Errorf("ErrBye(\"\").Error() = %q, want %q", got, want)
	}
}

func TestErrNoWithCode_AllCodes(t *testing.T) {
	tests := []struct {
		code     ResponseCode
		text     string
		wantSub  string
	}{
		{ResponseCodeAlert, "pay attention", "[ALERT]"},
		{ResponseCodePermanentFlags, "flags set", "[PERMANENTFLAGS]"},
		{ResponseCodeReadOnly, "read only", "[READ-ONLY]"},
		{ResponseCodeReadWrite, "read write", "[READ-WRITE]"},
		{ResponseCodeTryCreate, "try create", "[TRYCREATE]"},
		{ResponseCodeAlreadyExists, "exists", "[ALREADYEXISTS]"},
		{ResponseCodeNonExistent, "gone", "[NONEXISTENT]"},
		{ResponseCodeNoPerm, "denied", "[NOPERM]"},
		{ResponseCodeOverQuota, "full", "[OVERQUOTA]"},
		{ResponseCodeInUse, "busy", "[INUSE]"},
		{ResponseCodeClosed, "closed", "[CLOSED]"},
	}
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			err := ErrNoWithCode(tt.code, tt.text)
			got := err.Error()
			if got == "" {
				t.Fatal("Error() should not be empty")
			}
			if !contains(got, tt.wantSub) {
				t.Errorf("Error() = %q, should contain %q", got, tt.wantSub)
			}
			if !contains(got, tt.text) {
				t.Errorf("Error() = %q, should contain text %q", got, tt.text)
			}
		})
	}
}

// contains is a helper to check substring presence (avoids importing strings in tests).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestIMAPError_ErrorsIs(t *testing.T) {
	err1 := ErrNo("test")
	// errors.Is should work for identity comparison
	if !errors.Is(err1, err1) {
		t.Error("errors.Is(err, err) should be true for identity")
	}
}

func TestIMAPError_TypeAssertion(t *testing.T) {
	var err error = ErrNo("test")

	imapErr, ok := err.(*IMAPError)
	if !ok {
		t.Fatal("should be able to type-assert to *IMAPError")
	}
	if imapErr.Type != StatusResponseTypeNO {
		t.Errorf("Type = %q, want NO", imapErr.Type)
	}
}

func TestStatusResponse_Error_CodeWithCodeArg(t *testing.T) {
	// Test various CodeArg types
	tests := []struct {
		name    string
		codeArg interface{}
		want    string
	}{
		{"int arg", 42, "OK [UIDNEXT 42]"},
		{"uint32 arg", uint32(100), "OK [UIDNEXT 100]"},
		{"string arg", "hello", "OK [UIDNEXT hello]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := StatusResponse{
				Type:    StatusResponseTypeOK,
				Code:    ResponseCodeUIDNext,
				CodeArg: tt.codeArg,
			}
			got := resp.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusResponseType_Values(t *testing.T) {
	// Verify the string constants
	tests := []struct {
		srt  StatusResponseType
		want string
	}{
		{StatusResponseTypeOK, "OK"},
		{StatusResponseTypeNO, "NO"},
		{StatusResponseTypeBAD, "BAD"},
		{StatusResponseTypeBYE, "BYE"},
		{StatusResponseTypePREAUTH, "PREAUTH"},
	}
	for _, tt := range tests {
		if string(tt.srt) != tt.want {
			t.Errorf("StatusResponseType = %q, want %q", tt.srt, tt.want)
		}
	}
}

func TestResponseCode_Values(t *testing.T) {
	// Spot-check some response code constants
	tests := []struct {
		code ResponseCode
		want string
	}{
		{ResponseCodeAlert, "ALERT"},
		{ResponseCodeCapability, "CAPABILITY"},
		{ResponseCodeReadOnly, "READ-ONLY"},
		{ResponseCodeReadWrite, "READ-WRITE"},
		{ResponseCodeUIDNext, "UIDNEXT"},
		{ResponseCodeUIDValidity, "UIDVALIDITY"},
		{ResponseCodeAppendUID, "APPENDUID"},
		{ResponseCodeCopyUID, "COPYUID"},
		{ResponseCodeHighestModSeq, "HIGHESTMODSEQ"},
		{ResponseCodeClosed, "CLOSED"},
		{ResponseCodeMailboxID, "MAILBOXID"},
	}
	for _, tt := range tests {
		if string(tt.code) != tt.want {
			t.Errorf("ResponseCode = %q, want %q", tt.code, tt.want)
		}
	}
}
