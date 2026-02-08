package oauthbearer

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
	if m.Name() != "OAUTHBEARER" {
		t.Errorf("expected name OAUTHBEARER, got %s", m.Name())
	}
}

func TestClientMechanismStartBasic(t *testing.T) {
	m := &ClientMechanism{
		Username:    "user@example.com",
		AccessToken: "ya29.access-token",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(ir)

	// Should start with GS2 header "n,a=<user>,"
	if !strings.HasPrefix(s, "n,a=user@example.com,") {
		t.Errorf("expected GS2 header prefix, got %q", s)
	}

	// Should contain "auth=Bearer ya29.access-token"
	if !strings.Contains(s, "auth=Bearer ya29.access-token") {
		t.Errorf("expected auth=Bearer token in response, got %q", s)
	}

	// Should end with \x01\x01
	if !strings.HasSuffix(s, "\x01\x01") {
		t.Errorf("expected response to end with \\x01\\x01, got %q", s)
	}
}

func TestClientMechanismStartWithHostAndPort(t *testing.T) {
	m := &ClientMechanism{
		Username:    "user",
		AccessToken: "token",
		Host:        "imap.example.com",
		Port:        "993",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(ir)

	// Should contain host
	if !strings.Contains(s, "host=imap.example.com") {
		t.Errorf("expected host in response, got %q", s)
	}

	// Should contain port
	if !strings.Contains(s, "port=993") {
		t.Errorf("expected port in response, got %q", s)
	}
}

func TestClientMechanismStartWithoutHostAndPort(t *testing.T) {
	m := &ClientMechanism{
		Username:    "user",
		AccessToken: "token",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(ir)

	// Should NOT contain host= or port=
	if strings.Contains(s, "host=") {
		t.Errorf("expected no host in response, got %q", s)
	}
	if strings.Contains(s, "port=") {
		t.Errorf("expected no port in response, got %q", s)
	}
}

func TestClientMechanismStartWithHostOnly(t *testing.T) {
	m := &ClientMechanism{
		Username:    "user",
		AccessToken: "token",
		Host:        "mail.example.com",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(ir)
	if !strings.Contains(s, "host=mail.example.com") {
		t.Errorf("expected host in response, got %q", s)
	}
	if strings.Contains(s, "port=") {
		t.Errorf("expected no port in response, got %q", s)
	}
}

func TestClientMechanismStartWithPortOnly(t *testing.T) {
	m := &ClientMechanism{
		Username:    "user",
		AccessToken: "token",
		Port:        "143",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(ir)
	if strings.Contains(s, "host=") {
		t.Errorf("expected no host in response, got %q", s)
	}
	if !strings.Contains(s, "port=143") {
		t.Errorf("expected port in response, got %q", s)
	}
}

func TestClientMechanismNextAcknowledgesError(t *testing.T) {
	m := &ClientMechanism{}
	resp, err := m.Next([]byte("error details"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return \x01 to acknowledge error
	if len(resp) != 1 || resp[0] != 0x01 {
		t.Errorf("expected [0x01], got %v", resp)
	}
}

func TestClientMechanismNextWithNilChallenge(t *testing.T) {
	m := &ClientMechanism{}
	resp, err := m.Next(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp) != 1 || resp[0] != 0x01 {
		t.Errorf("expected [0x01], got %v", resp)
	}
}

// --- ServerMechanism Tests ---

func TestServerMechanismName(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)
	if m.Name() != "OAUTHBEARER" {
		t.Errorf("expected name OAUTHBEARER, got %s", m.Name())
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
	response := []byte("n,a=testuser,\x01auth=Bearer mytoken\x01\x01")
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
	if gotMech != "OAUTHBEARER" {
		t.Errorf("expected mechanism OAUTHBEARER, got %s", gotMech)
	}
	if gotIdentity != "testuser" {
		t.Errorf("expected identity 'testuser', got %q", gotIdentity)
	}
	if string(gotCreds) != "mytoken" {
		t.Errorf("expected token 'mytoken', got %q", string(gotCreds))
	}
}

func TestServerMechanismNextWithHostAndPort(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	m := NewServerMechanism(authenticator)
	response := []byte("n,a=user,\x01host=imap.example.com\x01port=993\x01auth=Bearer token\x01\x01")
	_, done, err := m.Next(response)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done to be true")
	}
}

func TestServerMechanismNextAuthFailure(t *testing.T) {
	expectedErr := fmt.Errorf("invalid token")
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return expectedErr
	})

	m := NewServerMechanism(authenticator)
	response := []byte("n,a=testuser,\x01auth=Bearer badtoken\x01\x01")
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

func TestServerMechanismNextInvalidGS2Header(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	m := NewServerMechanism(authenticator)
	// Missing "n," prefix
	response := []byte("a=user,\x01auth=Bearer token\x01\x01")
	_, done, err := m.Next(response)

	if err == nil {
		t.Fatal("expected error for invalid GS2 header, got nil")
	}
	if !strings.Contains(err.Error(), "invalid GS2 header") {
		t.Errorf("expected error about invalid GS2 header, got %q", err.Error())
	}
	if !done {
		t.Error("expected done to be true")
	}
}

func TestServerMechanismNextMissingUsername(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	m := NewServerMechanism(authenticator)
	// No a= in GS2 header (empty authz)
	response := []byte("n,,\x01auth=Bearer token\x01\x01")
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
	response := []byte("n,a=user,\x01\x01")
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
	response := []byte("n,a=user,\x01auth=Bearer token\x01\x01")

	// First call
	_, _, err := m.Next(response)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Second call
	_, done, err := m.Next(response)
	if err == nil {
		t.Fatal("expected error on second call, got nil")
	}
	if err.Error() != "oauthbearer: mechanism already completed" {
		t.Errorf("expected 'oauthbearer: mechanism already completed', got %q", err.Error())
	}
	if !done {
		t.Error("expected done to be true")
	}
}

func TestServerMechanismNextInvalidFormat(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	m := NewServerMechanism(authenticator)
	// "n," prefix but no comma after authz part
	response := []byte("n,no-comma-here")
	_, done, err := m.Next(response)

	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
	if !done {
		t.Error("expected done to be true")
	}
}

// --- parseOAuthBearer Tests ---

func TestParseOAuthBearerValid(t *testing.T) {
	data := []byte("n,a=alice@example.com,\x01auth=Bearer ya29.token\x01\x01")
	username, token, err := parseOAuthBearer(data)
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

func TestParseOAuthBearerWithHost(t *testing.T) {
	data := []byte("n,a=user,\x01host=imap.example.com\x01auth=Bearer token\x01\x01")
	username, token, err := parseOAuthBearer(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "user" {
		t.Errorf("expected username 'user', got %q", username)
	}
	if token != "token" {
		t.Errorf("expected token 'token', got %q", token)
	}
}

func TestParseOAuthBearerNoGS2(t *testing.T) {
	data := []byte("invalid data")
	_, _, err := parseOAuthBearer(data)
	if err == nil {
		t.Fatal("expected error for missing GS2 header, got nil")
	}
}

func TestParseOAuthBearerNoBearerPrefix(t *testing.T) {
	data := []byte("n,a=user,\x01auth=rawtoken\x01\x01")
	username, token, err := parseOAuthBearer(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "user" {
		t.Errorf("expected username 'user', got %q", username)
	}
	// Without "Bearer " prefix, it should strip just "auth="
	if token != "rawtoken" {
		t.Errorf("expected token 'rawtoken', got %q", token)
	}
}

// --- Constant Tests ---

func TestNameConstant(t *testing.T) {
	if Name != "OAUTHBEARER" {
		t.Errorf("expected Name constant to be OAUTHBEARER, got %s", Name)
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

func TestOAuthBearerEndToEnd(t *testing.T) {
	client := &ClientMechanism{
		Username:    "alice@example.com",
		AccessToken: "ya29.valid-token",
		Host:        "imap.example.com",
		Port:        "993",
	}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		if identity != "alice@example.com" {
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

func TestOAuthBearerEndToEndFailure(t *testing.T) {
	client := &ClientMechanism{
		Username:    "alice@example.com",
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

func TestOAuthBearerEndToEndNoHostPort(t *testing.T) {
	client := &ClientMechanism{
		Username:    "bob",
		AccessToken: "token123",
	}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		if identity != "bob" || string(credentials) != "token123" {
			return fmt.Errorf("auth failed")
		}
		return nil
	})
	server := NewServerMechanism(authenticator)

	ir, err := client.Start()
	if err != nil {
		t.Fatalf("client Start error: %v", err)
	}

	_, done, err := server.Next(ir)
	if err != nil {
		t.Fatalf("server Next error: %v", err)
	}
	if !done {
		t.Error("expected done to be true")
	}
}
