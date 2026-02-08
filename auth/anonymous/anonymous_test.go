package anonymous

import (
	"context"
	"fmt"
	"testing"

	"github.com/meszmate/imap-go/auth"
)

// --- ClientMechanism Tests ---

func TestClientMechanismName(t *testing.T) {
	m := &ClientMechanism{}
	if m.Name() != "ANONYMOUS" {
		t.Errorf("expected name ANONYMOUS, got %s", m.Name())
	}
}

func TestClientMechanismStartWithTrace(t *testing.T) {
	m := &ClientMechanism{Trace: "user@example.com"}
	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(ir) != "user@example.com" {
		t.Errorf("expected 'user@example.com', got %q", string(ir))
	}
}

func TestClientMechanismStartEmptyTrace(t *testing.T) {
	m := &ClientMechanism{Trace: ""}
	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(ir) != "" {
		t.Errorf("expected empty string, got %q", string(ir))
	}
}

func TestClientMechanismStartWithTextTrace(t *testing.T) {
	m := &ClientMechanism{Trace: "sistrstransen"}
	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(ir) != "sistrstransen" {
		t.Errorf("expected 'sistrstransen', got %q", string(ir))
	}
}

func TestClientMechanismNextReturnsError(t *testing.T) {
	m := &ClientMechanism{}
	_, err := m.Next([]byte("challenge"))
	if err == nil {
		t.Fatal("expected error from Next, got nil")
	}
	if err.Error() != "anonymous: unexpected challenge" {
		t.Errorf("expected 'anonymous: unexpected challenge', got %q", err.Error())
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
	if m.Name() != "ANONYMOUS" {
		t.Errorf("expected name ANONYMOUS, got %s", m.Name())
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
	challenge, done, err := m.Next([]byte("user@example.com"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done to be true")
	}
	if challenge != nil {
		t.Errorf("expected nil challenge, got %q", challenge)
	}
	if gotMech != "ANONYMOUS" {
		t.Errorf("expected mechanism ANONYMOUS, got %s", gotMech)
	}
	if gotIdentity != "user@example.com" {
		t.Errorf("expected identity 'user@example.com', got %q", gotIdentity)
	}
	if gotCreds != nil {
		t.Errorf("expected nil credentials, got %q", gotCreds)
	}
}

func TestServerMechanismNextEmptyTrace(t *testing.T) {
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
		t.Errorf("expected empty identity from nil response, got %q", gotIdentity)
	}
}

func TestServerMechanismNextAuthFailure(t *testing.T) {
	expectedErr := fmt.Errorf("anonymous access denied")
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return expectedErr
	})

	m := NewServerMechanism(authenticator)
	_, done, err := m.Next([]byte("trace"))

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
	_, _, err := m.Next([]byte("trace"))
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Second call
	_, done, err := m.Next([]byte("trace"))
	if err == nil {
		t.Fatal("expected error on second call, got nil")
	}
	if err.Error() != "anonymous: mechanism already completed" {
		t.Errorf("expected 'anonymous: mechanism already completed', got %q", err.Error())
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
	_, _, err := m.Next([]byte("trace"))
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

func TestServerMechanismNextTraceAsIdentity(t *testing.T) {
	// Verify that the trace token is passed as the identity
	traces := []string{
		"user@example.com",
		"anonymous",
		"some tracking info",
		"",
	}

	for _, trace := range traces {
		t.Run("trace="+trace, func(t *testing.T) {
			var gotIdentity string
			authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
				gotIdentity = identity
				return nil
			})

			m := NewServerMechanism(authenticator)
			_, _, err := m.Next([]byte(trace))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotIdentity != trace {
				t.Errorf("expected identity %q, got %q", trace, gotIdentity)
			}
		})
	}
}

// --- Constant Tests ---

func TestNameConstant(t *testing.T) {
	if Name != "ANONYMOUS" {
		t.Errorf("expected Name constant to be ANONYMOUS, got %s", Name)
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

func TestAnonymousEndToEnd(t *testing.T) {
	client := &ClientMechanism{
		Trace: "guest@example.com",
	}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		if mechanism != "ANONYMOUS" {
			return fmt.Errorf("wrong mechanism: %s", mechanism)
		}
		if identity != "guest@example.com" {
			return fmt.Errorf("unexpected trace: %s", identity)
		}
		if credentials != nil {
			return fmt.Errorf("expected nil credentials for ANONYMOUS")
		}
		return nil
	})
	server := NewServerMechanism(authenticator)

	// Client produces initial response (trace token)
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

func TestAnonymousEndToEndEmptyTrace(t *testing.T) {
	client := &ClientMechanism{Trace: ""}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
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

func TestAnonymousEndToEndDenied(t *testing.T) {
	client := &ClientMechanism{Trace: "attacker"}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return fmt.Errorf("anonymous access not allowed")
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
		t.Error("expected done to be true even on denial")
	}
}
