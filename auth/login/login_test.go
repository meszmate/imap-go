package login

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
	if m.Name() != "LOGIN" {
		t.Errorf("expected name LOGIN, got %s", m.Name())
	}
}

func TestClientMechanismStartReturnsNil(t *testing.T) {
	m := &ClientMechanism{Username: "user", Password: "pass"}
	ir, err := m.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ir != nil {
		t.Errorf("expected nil initial response, got %q", ir)
	}
}

func TestClientMechanismNextStep0ReturnsUsername(t *testing.T) {
	m := &ClientMechanism{Username: "testuser", Password: "testpass"}

	resp, err := m.Next([]byte("Username:"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "testuser" {
		t.Errorf("expected 'testuser', got %q", string(resp))
	}
}

func TestClientMechanismNextStep1ReturnsPassword(t *testing.T) {
	m := &ClientMechanism{Username: "testuser", Password: "testpass"}

	// Step 0: username
	_, err := m.Next([]byte("Username:"))
	if err != nil {
		t.Fatalf("unexpected error at step 0: %v", err)
	}

	// Step 1: password
	resp, err := m.Next([]byte("Password:"))
	if err != nil {
		t.Fatalf("unexpected error at step 1: %v", err)
	}
	if string(resp) != "testpass" {
		t.Errorf("expected 'testpass', got %q", string(resp))
	}
}

func TestClientMechanismNextStep2ReturnsError(t *testing.T) {
	m := &ClientMechanism{Username: "testuser", Password: "testpass"}

	// Step 0 and 1
	m.Next([]byte("Username:"))
	m.Next([]byte("Password:"))

	// Step 2: unexpected
	_, err := m.Next([]byte("Extra:"))
	if err == nil {
		t.Fatal("expected error at step 2, got nil")
	}
	if err.Error() != "login: unexpected challenge" {
		t.Errorf("expected 'login: unexpected challenge', got %q", err.Error())
	}
}

func TestClientMechanismNextIgnoresChallengeContent(t *testing.T) {
	// The client responds with username/password regardless of challenge content
	m := &ClientMechanism{Username: "user", Password: "pass"}

	resp, err := m.Next([]byte("anything"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "user" {
		t.Errorf("expected 'user', got %q", string(resp))
	}
}

func TestClientMechanismNextEmptyFields(t *testing.T) {
	m := &ClientMechanism{Username: "", Password: ""}

	resp, err := m.Next([]byte("Username:"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "" {
		t.Errorf("expected empty username, got %q", string(resp))
	}

	resp, err = m.Next([]byte("Password:"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "" {
		t.Errorf("expected empty password, got %q", string(resp))
	}
}

// --- ServerMechanism Tests ---

func TestServerMechanismName(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)
	if m.Name() != "LOGIN" {
		t.Errorf("expected name LOGIN, got %s", m.Name())
	}
}

func TestServerMechanismStep0SendsUsernameChallenge(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)

	challenge, done, err := m.Next(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("expected done to be false at step 0")
	}
	if !bytes.Equal(challenge, []byte("Username:")) {
		t.Errorf("expected challenge 'Username:', got %q", string(challenge))
	}
}

func TestServerMechanismStep1SendsPasswordChallenge(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)

	// Step 0: get Username: challenge
	m.Next(nil)

	// Step 1: send username, get Password: challenge
	challenge, done, err := m.Next([]byte("testuser"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("expected done to be false at step 1")
	}
	if !bytes.Equal(challenge, []byte("Password:")) {
		t.Errorf("expected challenge 'Password:', got %q", string(challenge))
	}
}

func TestServerMechanismStep2AuthenticatesSuccess(t *testing.T) {
	var gotMech, gotIdentity string
	var gotCreds []byte

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		gotMech = mechanism
		gotIdentity = identity
		gotCreds = credentials
		return nil
	})
	m := NewServerMechanism(authenticator)

	// Step 0
	m.Next(nil)
	// Step 1
	m.Next([]byte("testuser"))
	// Step 2: send password
	challenge, done, err := m.Next([]byte("testpass"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done to be true at step 2")
	}
	if challenge != nil {
		t.Errorf("expected nil challenge, got %q", challenge)
	}
	if gotMech != "LOGIN" {
		t.Errorf("expected mechanism LOGIN, got %s", gotMech)
	}
	if gotIdentity != "testuser" {
		t.Errorf("expected identity 'testuser', got %s", gotIdentity)
	}
	if string(gotCreds) != "testpass" {
		t.Errorf("expected credentials 'testpass', got %q", string(gotCreds))
	}
}

func TestServerMechanismStep2AuthenticatesFailure(t *testing.T) {
	expectedErr := fmt.Errorf("invalid credentials")
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return expectedErr
	})
	m := NewServerMechanism(authenticator)

	m.Next(nil)
	m.Next([]byte("testuser"))
	_, done, err := m.Next([]byte("wrongpass"))

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

func TestServerMechanismStep3ReturnsError(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)

	m.Next(nil)                  // step 0
	m.Next([]byte("testuser"))   // step 1
	m.Next([]byte("testpass"))   // step 2

	// Step 3: unexpected
	_, done, err := m.Next([]byte("extra"))
	if err == nil {
		t.Fatal("expected error at step 3, got nil")
	}
	if err.Error() != "login: unexpected response" {
		t.Errorf("expected 'login: unexpected response', got %q", err.Error())
	}
	if !done {
		t.Error("expected done to be true")
	}
}

// --- Constant Tests ---

func TestNameConstant(t *testing.T) {
	if Name != "LOGIN" {
		t.Errorf("expected Name constant to be LOGIN, got %s", Name)
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

func TestLoginEndToEnd(t *testing.T) {
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

	// Client starts - no initial response
	ir, err := client.Start()
	if err != nil {
		t.Fatalf("client Start error: %v", err)
	}
	if ir != nil {
		t.Fatalf("expected nil initial response, got %q", ir)
	}

	// Server step 0: sends Username: challenge
	challenge, done, err := server.Next(nil)
	if err != nil {
		t.Fatalf("server step 0 error: %v", err)
	}
	if done {
		t.Fatal("expected not done at step 0")
	}

	// Client responds with username
	resp, err := client.Next(challenge)
	if err != nil {
		t.Fatalf("client step 0 error: %v", err)
	}

	// Server step 1: receives username, sends Password: challenge
	challenge, done, err = server.Next(resp)
	if err != nil {
		t.Fatalf("server step 1 error: %v", err)
	}
	if done {
		t.Fatal("expected not done at step 1")
	}

	// Client responds with password
	resp, err = client.Next(challenge)
	if err != nil {
		t.Fatalf("client step 1 error: %v", err)
	}

	// Server step 2: receives password, authenticates
	_, done, err = server.Next(resp)
	if err != nil {
		t.Fatalf("server step 2 error: %v", err)
	}
	if !done {
		t.Fatal("expected done at step 2")
	}
}

func TestLoginEndToEndFailure(t *testing.T) {
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

	ir, _ := client.Start()
	if ir != nil {
		t.Fatal("expected nil initial response")
	}

	challenge, _, _ := server.Next(nil)
	resp, _ := client.Next(challenge)
	challenge, _, _ = server.Next(resp)
	resp, _ = client.Next(challenge)
	_, done, err := server.Next(resp)

	if err == nil {
		t.Fatal("expected authentication failure")
	}
	if !done {
		t.Error("expected done to be true even on failure")
	}
}
