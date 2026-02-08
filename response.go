package imap

import (
	"fmt"
	"strings"
)

// StatusResponseType represents the type of a status response.
type StatusResponseType string

const (
	StatusResponseTypeOK      StatusResponseType = "OK"
	StatusResponseTypeNO      StatusResponseType = "NO"
	StatusResponseTypeBAD     StatusResponseType = "BAD"
	StatusResponseTypeBYE     StatusResponseType = "BYE"
	StatusResponseTypePREAUTH StatusResponseType = "PREAUTH"
)

// ResponseCode represents a response code in brackets.
type ResponseCode string

// Standard response codes.
const (
	ResponseCodeAlert          ResponseCode = "ALERT"
	ResponseCodeBadCharset     ResponseCode = "BADCHARSET"
	ResponseCodeCapability     ResponseCode = "CAPABILITY"
	ResponseCodeParse          ResponseCode = "PARSE"
	ResponseCodePermanentFlags ResponseCode = "PERMANENTFLAGS"
	ResponseCodeReadOnly       ResponseCode = "READ-ONLY"
	ResponseCodeReadWrite      ResponseCode = "READ-WRITE"
	ResponseCodeTryCreate      ResponseCode = "TRYCREATE"
	ResponseCodeUIDNext        ResponseCode = "UIDNEXT"
	ResponseCodeUIDValidity    ResponseCode = "UIDVALIDITY"
	ResponseCodeUnseen         ResponseCode = "UNSEEN"
	ResponseCodeAppendUID      ResponseCode = "APPENDUID"
	ResponseCodeCopyUID        ResponseCode = "COPYUID"
	ResponseCodeUIDNotSticky   ResponseCode = "UIDNOTSTICKY"
	ResponseCodeHighestModSeq  ResponseCode = "HIGHESTMODSEQ"
	ResponseCodeModified       ResponseCode = "MODIFIED"
	ResponseCodeNoModSeq       ResponseCode = "NOMODSEQ"
	ResponseCodeClosed         ResponseCode = "CLOSED"
	ResponseCodeOverQuota      ResponseCode = "OVERQUOTA"
	ResponseCodeAlreadyExists  ResponseCode = "ALREADYEXISTS"
	ResponseCodeNonExistent    ResponseCode = "NONEXISTENT"
	ResponseCodeContactAdmin   ResponseCode = "CONTACTADMIN"
	ResponseCodeNoPerm         ResponseCode = "NOPERM"
	ResponseCodeInUse          ResponseCode = "INUSE"
	ResponseCodeExpungeIssued  ResponseCode = "EXPUNGEISSUED"
	ResponseCodeCorruption     ResponseCode = "CORRUPTION"
	ResponseCodeServerBug      ResponseCode = "SERVERBUG"
	ResponseCodeClientBug      ResponseCode = "CLIENTBUG"
	ResponseCodeCannot         ResponseCode = "CANNOT"
	ResponseCodeLimit          ResponseCode = "LIMIT"
	ResponseCodeHasChildren    ResponseCode = "HASCHILDREN"
	ResponseCodeMetadata       ResponseCode = "METADATA"
	ResponseCodeNotSaved       ResponseCode = "NOTSAVED"
	ResponseCodeMailboxID      ResponseCode = "MAILBOXID"
	ResponseCodeObjectID       ResponseCode = "OBJECTID"
	ResponseCodeInProgress     ResponseCode = "INPROGRESS"
)

// StatusResponse represents an IMAP status response.
type StatusResponse struct {
	// Type is the response type (OK, NO, BAD, BYE, PREAUTH).
	Type StatusResponseType
	// Code is the optional response code.
	Code ResponseCode
	// CodeArg is the optional argument to the response code.
	CodeArg interface{}
	// Text is the human-readable text.
	Text string
}

// Error returns the status response as an error string.
func (r *StatusResponse) Error() string {
	var b strings.Builder
	b.WriteString(string(r.Type))
	if r.Code != "" {
		b.WriteString(" [")
		b.WriteString(string(r.Code))
		if r.CodeArg != nil {
			b.WriteString(" ")
			fmt.Fprint(&b, r.CodeArg)
		}
		b.WriteString("]")
	}
	if r.Text != "" {
		b.WriteString(" ")
		b.WriteString(r.Text)
	}
	return b.String()
}

// IMAPError is an error type that wraps an IMAP status response.
type IMAPError struct {
	*StatusResponse
}

// Error implements the error interface.
func (e *IMAPError) Error() string {
	return e.StatusResponse.Error()
}

// Unwrap returns nil (no wrapped error).
func (e *IMAPError) Unwrap() error {
	return nil
}

// ErrNo creates a NO error with the given text.
func ErrNo(text string) *IMAPError {
	return &IMAPError{&StatusResponse{
		Type: StatusResponseTypeNO,
		Text: text,
	}}
}

// ErrNoWithCode creates a NO error with a response code.
func ErrNoWithCode(code ResponseCode, text string) *IMAPError {
	return &IMAPError{&StatusResponse{
		Type: StatusResponseTypeNO,
		Code: code,
		Text: text,
	}}
}

// ErrBad creates a BAD error with the given text.
func ErrBad(text string) *IMAPError {
	return &IMAPError{&StatusResponse{
		Type: StatusResponseTypeBAD,
		Text: text,
	}}
}

// ErrBye creates a BYE response.
func ErrBye(text string) *IMAPError {
	return &IMAPError{&StatusResponse{
		Type: StatusResponseTypeBYE,
		Text: text,
	}}
}
