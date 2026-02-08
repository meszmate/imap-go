package external

import (
	"context"
	"fmt"
	"testing"

	"github.com/meszmate/imap-go/auth"
)

// --- ClientMechanism Tests ---

func TestClientMechanismName(t *testing.T) {
	m := &ClientMechanism{}
	if m.Name() != "EXTERNAL" {
		t.Errorf("expected name EXTERNAL, got %s", m.Name())
	}
}

func TestClientMechanismStartWithAuthzID(t *testing.T) {
	m := &ClientMechanism{AuthzID: "admin@example.com"}
	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(ir) != "admin@example.com" {
		t.Errorf("expected 'admin@example.com', got %q", string(ir))
	}
}

func TestClientMechanismStartEmptyAuthzID(t *testing.T) {
	m := &ClientMechanism{AuthzID: ""}
	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(ir) != "" {
		t.Errorf("expected empty string, got %q", string(ir))
	}
}

func TestClientMechanismNextReturnsError(t *testing.T) {
	m := &ClientMechanism{}
	_, err := m.Next([]byte("challenge"))
	if err == nil {
		t.Fatal("expected error from Next, got nil")
	}
	if err.Error() != "external: unexpected challenge" {
		t.Errorf("expected 'external: unexpected challenge', got %q", err.Error())
	}
}

func TestClientMechanismNextNilChallenge(t *testing.T) {
	m := &ClientMechanism{}
	_, err := m.Next(nil)
	if err == nil {
		t.Fatal("expected error from Next with nil challenge, got nil")
	}
}

// --- ServerMechanism Tests ---

func TestServerMechanismName(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)
	if m.Name() != "EXTERNAL" {
		t.Errorf("expected name EXTERNAL, got %s", m.Name())
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
	challenge, done, err := m.Next([]byte("admin@example.com"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done to be true")
	}
	if challenge != nil {
		t.Errorf("expected nil challenge, got %q", challenge)
	}
	if gotMech != "EXTERNAL" {
		t.Errorf("expected mechanism EXTERNAL, got %s", gotMech)
	}
	if gotIdentity != "admin@example.com" {
		t.Errorf("expected identity 'admin@example.com', got %s", gotIdentity)
	}
	if gotCreds != nil {
		t.Errorf("expected nil credentials, got %q", gotCreds)
	}
}

func TestServerMechanismNextEmptyAuthzID(t *testing.T) {
	var gotIdentity string
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		gotIdentity = identity
		return nil
	})

	m := NewServerMechanism(authenticator)
	_, done, err := m.Next([]byte(""))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done to be true")
	}
	if gotIdentity != "" {
		t.Errorf("expected empty identity, got %q", gotIdentity)
	}
}

func TestServerMechanismNextNilResponse(t *testing.T) {
	var gotIdentity string
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		gotIdentity = identity
		return nil
	})

	m := NewServerMechanism(authenticator)
	_, done, err := m.Next(nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done to be true")
	}
	if gotIdentity != "" {
		t.Errorf("expected empty identity, got %q", gotIdentity)
	}
}

func TestServerMechanismNextAuthFailure(t *testing.T) {
	expectedErr := fmt.Errorf("external auth failed")
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return expectedErr
	})

	m := NewServerMechanism(authenticator)
	_, done, err := m.Next([]byte("unknown@example.com"))

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

func TestServerMechanismNextCalledTwice(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	m := NewServerMechanism(authenticator)

	// First call
	_, _, err := m.Next([]byte("user"))
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Second call
	_, done, err := m.Next([]byte("user"))
	if err == nil {
		t.Fatal("expected error on second call, got nil")
	}
	if err.Error() != "external: mechanism already completed" {
		t.Errorf("expected 'external: mechanism already completed', got %q", err.Error())
	}
	if !done {
		t.Error("expected done to be true")
	}
}

func TestServerMechanismNextPassesNilCredentials(t *testing.T) {
	var gotCreds []byte
	credentialsChecked := false

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		gotCreds = credentials
		credentialsChecked = true
		return nil
	})

	m := NewServerMechanism(authenticator)
	_, _, err := m.Next([]byte("user"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !credentialsChecked {
		t.Fatal("authenticator was not called")
	}
	if gotCreds != nil {
		t.Errorf("expected nil credentials, got %q", gotCreds)
	}
}

// --- Constant Tests ---

func TestNameConstant(t *testing.T) {
	if Name != "EXTERNAL" {
		t.Errorf("expected Name constant to be EXTERNAL, got %s", Name)
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

func TestExternalEndToEnd(t *testing.T) {
	client := &ClientMechanism{
		AuthzID: "alice@example.com",
	}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		if identity != "alice@example.com" {
			return fmt.Errorf("unknown identity")
		}
		if credentials != nil {
			return fmt.Errorf("expected nil credentials for EXTERNAL")
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

func TestExternalEndToEndEmptyAuthzID(t *testing.T) {
	client := &ClientMechanism{AuthzID: ""}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		// Server can derive identity from TLS certificate
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

func TestExternalEndToEndFailure(t *testing.T) {
	client := &ClientMechanism{AuthzID: "unauthorized@example.com"}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return fmt.Errorf("not authorized")
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
