package xoauth2

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/meszmate/imap-go/auth"
)

// --- ClientMechanism Tests ---

func TestClientMechanismName(t *testing.T) {
	m := &ClientMechanism{}
	if m.Name() != "XOAUTH2" {
		t.Errorf("expected name XOAUTH2, got %s", m.Name())
	}
}

func TestClientMechanismStart(t *testing.T) {
	m := &ClientMechanism{
		Username:    "user@example.com",
		AccessToken: "ya29.access-token",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "user=user@example.com\x01auth=Bearer ya29.access-token\x01\x01"
	if string(ir) != expected {
		t.Errorf("expected %q, got %q", expected, string(ir))
	}
}

func TestClientMechanismStartVerifyFormat(t *testing.T) {
	m := &ClientMechanism{
		Username:    "alice",
		AccessToken: "token123",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(ir)

	// Should start with "user="
	if !strings.HasPrefix(s, "user=") {
		t.Error("expected response to start with 'user='")
	}

	// Should contain "auth=Bearer "
	if !strings.Contains(s, "auth=Bearer ") {
		t.Error("expected response to contain 'auth=Bearer '")
	}

	// Should end with \x01\x01
	if !strings.HasSuffix(s, "\x01\x01") {
		t.Error("expected response to end with \\x01\\x01")
	}
}

func TestClientMechanismStartEmptyUsername(t *testing.T) {
	m := &ClientMechanism{
		Username:    "",
		AccessToken: "token",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "user=\x01auth=Bearer token\x01\x01"
	if string(ir) != expected {
		t.Errorf("expected %q, got %q", expected, string(ir))
	}
}

func TestClientMechanismStartEmptyToken(t *testing.T) {
	m := &ClientMechanism{
		Username:    "user",
		AccessToken: "",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "user=user\x01auth=Bearer \x01\x01"
	if string(ir) != expected {
		t.Errorf("expected %q, got %q", expected, string(ir))
	}
}

func TestClientMechanismNextReturnsEmptyResponse(t *testing.T) {
	m := &ClientMechanism{}
	resp, err := m.Next([]byte("error info"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty response, got %q", resp)
	}
}

func TestClientMechanismNextWithNilChallenge(t *testing.T) {
	m := &ClientMechanism{}
	resp, err := m.Next(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty response, got %q", resp)
	}
}

// --- ServerMechanism Tests ---

func TestServerMechanismName(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)
	if m.Name() != "XOAUTH2" {
		t.Errorf("expected name XOAUTH2, got %s", m.Name())
	}
}

func TestServerMechanismNextSuccess(t *testing.T) {
	var gotMech, gotIdentity string
	var gotCreds []byte

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		gotMech = mechanism
		gotIdentity = identity
		gotCreds = credentials
		return nil
	})

	m := NewServerMechanism(authenticator)
	response := []byte("user=testuser\x01auth=Bearer mytoken123\x01\x01")
	challenge, done, err := m.Next(response)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done to be true")
	}
	if challenge != nil {
		t.Errorf("expected nil challenge, got %q", challenge)
	}
	if gotMech != "XOAUTH2" {
		t.Errorf("expected mechanism XOAUTH2, got %s", gotMech)
	}
	if gotIdentity != "testuser" {
		t.Errorf("expected identity 'testuser', got %s", gotIdentity)
	}
	if string(gotCreds) != "mytoken123" {
		t.Errorf("expected token 'mytoken123', got %q", string(gotCreds))
	}
}

func TestServerMechanismNextAuthFailure(t *testing.T) {
	expectedErr := fmt.Errorf("invalid token")
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return expectedErr
	})

	m := NewServerMechanism(authenticator)
	response := []byte("user=testuser\x01auth=Bearer badtoken\x01\x01")
	_, done, err := m.Next(response)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	if !done {
		t.Error("expected done to be true even on failure")
	}
}

func TestServerMechanismNextMissingUsername(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	m := NewServerMechanism(authenticator)
	// No user= field
	response := []byte("auth=Bearer mytoken\x01\x01")
	_, done, err := m.Next(response)

	if err == nil {
		t.Fatal("expected error for missing username, got nil")
	}
	if !strings.Contains(err.Error(), "missing username") {
		t.Errorf("expected error about missing username, got %q", err.Error())
	}
	if !done {
		t.Error("expected done to be true")
	}
}

func TestServerMechanismNextMissingToken(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	m := NewServerMechanism(authenticator)
	// No auth= field
	response := []byte("user=testuser\x01\x01")
	_, done, err := m.Next(response)

	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
	if !strings.Contains(err.Error(), "missing access token") {
		t.Errorf("expected error about missing access token, got %q", err.Error())
	}
	if !done {
		t.Error("expected done to be true")
	}
}

func TestServerMechanismNextCalledTwice(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	m := NewServerMechanism(authenticator)
	response := []byte("user=testuser\x01auth=Bearer token\x01\x01")

	// First call
	_, _, err := m.Next(response)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Second call should fail
	_, done, err := m.Next(response)
	if err == nil {
		t.Fatal("expected error on second call, got nil")
	}
	if err.Error() != "xoauth2: mechanism already completed" {
		t.Errorf("expected 'xoauth2: mechanism already completed', got %q", err.Error())
	}
	if !done {
		t.Error("expected done to be true")
	}
}

func TestServerMechanismNextTokenWithoutBearerPrefix(t *testing.T) {
	var gotCreds []byte
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		gotCreds = credentials
		return nil
	})

	m := NewServerMechanism(authenticator)
	// auth= without "Bearer " prefix
	response := []byte("user=testuser\x01auth=rawtoken\x01\x01")
	_, done, err := m.Next(response)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done to be true")
	}
	if string(gotCreds) != "rawtoken" {
		t.Errorf("expected token 'rawtoken', got %q", string(gotCreds))
	}
}

// --- parseXOAuth2 Tests ---

func TestParseXOAuth2Valid(t *testing.T) {
	data := []byte("user=alice@example.com\x01auth=Bearer ya29.token\x01\x01")
	username, token, err := parseXOAuth2(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "alice@example.com" {
		t.Errorf("expected username 'alice@example.com', got %q", username)
	}
	if token != "ya29.token" {
		t.Errorf("expected token 'ya29.token', got %q", token)
	}
}

func TestParseXOAuth2NoBearer(t *testing.T) {
	data := []byte("user=bob\x01auth=plain-token\x01\x01")
	username, token, err := parseXOAuth2(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "bob" {
		t.Errorf("expected username 'bob', got %q", username)
	}
	if token != "plain-token" {
		t.Errorf("expected token 'plain-token', got %q", token)
	}
}

func TestParseXOAuth2Empty(t *testing.T) {
	_, _, err := parseXOAuth2([]byte{})
	if err == nil {
		t.Fatal("expected error for empty data, got nil")
	}
}

func TestParseXOAuth2OnlyUsername(t *testing.T) {
	data := []byte("user=alice\x01\x01")
	_, _, err := parseXOAuth2(data)
	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
}

func TestParseXOAuth2OnlyToken(t *testing.T) {
	data := []byte("auth=Bearer token\x01\x01")
	_, _, err := parseXOAuth2(data)
	if err == nil {
		t.Fatal("expected error for missing username, got nil")
	}
}

// --- Constant Tests ---

func TestNameConstant(t *testing.T) {
	if Name != "XOAUTH2" {
		t.Errorf("expected Name constant to be XOAUTH2, got %s", Name)
	}
}

// --- Interface Compliance Tests ---

func TestClientMechanismImplementsInterface(t *testing.T) {
	var _ auth.ClientMechanism = &ClientMechanism{}
}

func TestServerMechanismImplementsInterface(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	var _ auth.ServerMechanism = NewServerMechanism(authenticator)
}

// --- End-to-End Test ---

func TestXOAuth2EndToEnd(t *testing.T) {
	client := &ClientMechanism{
		Username:    "alice@gmail.com",
		AccessToken: "ya29.valid-token",
	}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		if identity != "alice@gmail.com" {
			return fmt.Errorf("unknown user")
		}
		if string(credentials) != "ya29.valid-token" {
			return fmt.Errorf("invalid token")
		}
		return nil
	})
	server := NewServerMechanism(authenticator)

	// Client produces initial response
	ir, err := client.Start()
	if err != nil {
		t.Fatalf("client Start error: %v", err)
	}

	// Server processes it
	_, done, err := server.Next(ir)
	if err != nil {
		t.Fatalf("server Next error: %v", err)
	}
	if !done {
		t.Error("expected authentication to be done")
	}
}

func TestXOAuth2EndToEndFailure(t *testing.T) {
	client := &ClientMechanism{
		Username:    "alice@gmail.com",
		AccessToken: "expired-token",
	}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return fmt.Errorf("token expired")
	})
	server := NewServerMechanism(authenticator)

	ir, err := client.Start()
	if err != nil {
		t.Fatalf("client Start error: %v", err)
	}

	_, done, err := server.Next(ir)
	if err == nil {
		t.Fatal("expected authentication failure")
	}
	if !done {
		t.Error("expected done to be true even on failure")
	}
}
