package plain

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/meszmate/imap-go/auth"
)

// --- ClientMechanism Tests ---

func TestClientMechanismName(t *testing.T) {
	m := &ClientMechanism{}
	if m.Name() != "PLAIN" {
		t.Errorf("expected name PLAIN, got %s", m.Name())
	}
}

func TestClientMechanismStart(t *testing.T) {
	m := &ClientMechanism{
		Username: "testuser",
		Password: "testpass",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected format: \0username\0password (empty authzID)
	expected := []byte("\x00testuser\x00testpass")
	if !bytes.Equal(ir, expected) {
		t.Errorf("expected initial response %q, got %q", expected, ir)
	}
}

func TestClientMechanismStartWithAuthzID(t *testing.T) {
	m := &ClientMechanism{
		AuthzID:  "admin",
		Username: "testuser",
		Password: "testpass",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte("admin\x00testuser\x00testpass")
	if !bytes.Equal(ir, expected) {
		t.Errorf("expected initial response %q, got %q", expected, ir)
	}
}

func TestClientMechanismStartEmptyFields(t *testing.T) {
	m := &ClientMechanism{
		Username: "",
		Password: "",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All fields empty: \0\0
	expected := []byte("\x00\x00")
	if !bytes.Equal(ir, expected) {
		t.Errorf("expected initial response %q, got %q", expected, ir)
	}
}

func TestClientMechanismNextReturnsError(t *testing.T) {
	m := &ClientMechanism{}
	_, err := m.Next([]byte("challenge"))
	if err == nil {
		t.Fatal("expected error from Next, got nil")
	}
	if err.Error() != "plain: unexpected challenge" {
		t.Errorf("expected 'plain: unexpected challenge', got %q", err.Error())
	}
}

func TestClientMechanismNextNilChallenge(t *testing.T) {
	m := &ClientMechanism{}
	_, err := m.Next(nil)
	if err == nil {
		t.Fatal("expected error from Next with nil challenge, got nil")
	}
}

func TestClientMechanismStartWithSpecialChars(t *testing.T) {
	m := &ClientMechanism{
		Username: "user@example.com",
		Password: "p@ss=w0rd!#$%",
	}

	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte("\x00user@example.com\x00p@ss=w0rd!#$%")
	if !bytes.Equal(ir, expected) {
		t.Errorf("expected initial response %q, got %q", expected, ir)
	}
}

// --- ServerMechanism Tests ---

func TestServerMechanismName(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)
	if m.Name() != "PLAIN" {
		t.Errorf("expected name PLAIN, got %s", m.Name())
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
	// Format: authzID\0username\0password
	response := []byte("\x00testuser\x00testpass")
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
	if gotMech != "PLAIN" {
		t.Errorf("expected mechanism PLAIN, got %s", gotMech)
	}
	if gotIdentity != "testuser" {
		t.Errorf("expected identity 'testuser', got %s", gotIdentity)
	}
	if string(gotCreds) != "testpass" {
		t.Errorf("expected credentials 'testpass', got %q", string(gotCreds))
	}
}

func TestServerMechanismNextWithAuthzID(t *testing.T) {
	var gotIdentity string

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		gotIdentity = identity
		return nil
	})

	m := NewServerMechanism(authenticator)
	// authzID is "admin", username is "testuser"
	response := []byte("admin\x00testuser\x00testpass")
	_, done, err := m.Next(response)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done to be true")
	}
	// The identity passed to Authenticate should be the username (authcid), not the authzID
	if gotIdentity != "testuser" {
		t.Errorf("expected identity 'testuser', got %s", gotIdentity)
	}
}

func TestServerMechanismNextAuthFailure(t *testing.T) {
	expectedErr := fmt.Errorf("invalid credentials")
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return expectedErr
	})

	m := NewServerMechanism(authenticator)
	response := []byte("\x00testuser\x00wrongpass")
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

func TestServerMechanismNextInvalidFormat(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	tests := []struct {
		name     string
		response []byte
	}{
		{"no separators", []byte("justtext")},
		{"only one separator", []byte("user\x00pass")},
		{"empty response", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewServerMechanism(authenticator)
			_, done, err := m.Next(tt.response)
			if err == nil {
				t.Error("expected error for invalid format, got nil")
			}
			if !done {
				t.Error("expected done to be true even on invalid format")
			}
		})
	}
}

func TestServerMechanismNextCalledTwice(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	m := NewServerMechanism(authenticator)
	response := []byte("\x00testuser\x00testpass")

	// First call should succeed
	_, _, err := m.Next(response)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Second call should fail - mechanism already completed
	_, done, err := m.Next(response)
	if err == nil {
		t.Fatal("expected error on second call, got nil")
	}
	if err.Error() != "plain: mechanism already completed" {
		t.Errorf("expected 'plain: mechanism already completed', got %q", err.Error())
	}
	if !done {
		t.Error("expected done to be true")
	}
}

func TestServerMechanismNextEmptyAuthzIDDefaultsToUsername(t *testing.T) {
	var gotIdentity string
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		gotIdentity = identity
		return nil
	})

	m := NewServerMechanism(authenticator)
	// Empty authzID
	response := []byte("\x00myuser\x00mypass")
	_, _, err := m.Next(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotIdentity != "myuser" {
		t.Errorf("expected identity 'myuser', got %s", gotIdentity)
	}
}

// --- Constant Tests ---

func TestNameConstant(t *testing.T) {
	if Name != "PLAIN" {
		t.Errorf("expected Name constant to be PLAIN, got %s", Name)
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

func TestPlainEndToEnd(t *testing.T) {
	// Simulate a full PLAIN authentication exchange
	client := &ClientMechanism{
		Username: "alice",
		Password: "wonderland",
	}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		if identity != "alice" || string(credentials) != "wonderland" {
			return fmt.Errorf("invalid credentials")
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

func TestPlainEndToEndFailure(t *testing.T) {
	client := &ClientMechanism{
		Username: "alice",
		Password: "wrongpassword",
	}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		if string(credentials) != "wonderland" {
			return fmt.Errorf("invalid credentials")
		}
		return nil
	})
	server := NewServerMechanism(authenticator)

	ir, err := client.Start()
	if err != nil {
		t.Fatalf("client Start error: %v", err)
	}

	_, done, err := server.Next(ir)
	if err == nil {
		t.Fatal("expected authentication failure, got nil error")
	}
	if !done {
		t.Error("expected done to be true even on failure")
	}
}
