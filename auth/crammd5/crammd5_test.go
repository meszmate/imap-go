package crammd5

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/meszmate/imap-go/auth"
)

// --- ClientMechanism Tests ---

func TestClientMechanismName(t *testing.T) {
	m := &ClientMechanism{}
	if m.Name() != "CRAM-MD5" {
		t.Errorf("expected name CRAM-MD5, got %s", m.Name())
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

func TestClientMechanismNextComputesHMAC(t *testing.T) {
	m := &ClientMechanism{
		Username: "testuser",
		Password: "testpass",
	}

	challenge := []byte("<1234.5678@localhost>")
	resp, err := m.Next(challenge)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the HMAC-MD5 computation manually
	h := hmac.New(md5.New, []byte("testpass"))
	h.Write(challenge)
	expectedDigest := hex.EncodeToString(h.Sum(nil))
	expectedResp := "testuser " + expectedDigest

	if string(resp) != expectedResp {
		t.Errorf("expected response %q, got %q", expectedResp, string(resp))
	}
}

func TestClientMechanismNextFormat(t *testing.T) {
	m := &ClientMechanism{
		Username: "joe",
		Password: "secret",
	}

	challenge := []byte("<challenge@host>")
	resp, err := m.Next(challenge)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Response should be "username space hex-digest"
	parts := strings.SplitN(string(resp), " ", 2)
	if len(parts) != 2 {
		t.Fatalf("expected response in 'username digest' format, got %q", string(resp))
	}
	if parts[0] != "joe" {
		t.Errorf("expected username 'joe', got %q", parts[0])
	}
	// Digest should be 32 hex characters (128-bit MD5)
	if len(parts[1]) != 32 {
		t.Errorf("expected 32-char hex digest, got %d chars: %q", len(parts[1]), parts[1])
	}
}

func TestClientMechanismNextEmptyChallenge(t *testing.T) {
	m := &ClientMechanism{
		Username: "user",
		Password: "pass",
	}

	resp, err := m.Next([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still compute HMAC with empty challenge
	h := hmac.New(md5.New, []byte("pass"))
	h.Write([]byte{})
	expectedDigest := hex.EncodeToString(h.Sum(nil))
	expected := "user " + expectedDigest
	if string(resp) != expected {
		t.Errorf("expected %q, got %q", expected, string(resp))
	}
}

func TestClientMechanismNextDifferentPasswords(t *testing.T) {
	challenge := []byte("<test@localhost>")

	m1 := &ClientMechanism{Username: "user", Password: "pass1"}
	m2 := &ClientMechanism{Username: "user", Password: "pass2"}

	resp1, _ := m1.Next(challenge)
	resp2, _ := m2.Next(challenge)

	if string(resp1) == string(resp2) {
		t.Error("different passwords should produce different responses")
	}
}

func TestClientMechanismNextDifferentChallenges(t *testing.T) {
	m := &ClientMechanism{Username: "user", Password: "pass"}

	resp1, _ := m.Next([]byte("<challenge1@host>"))
	resp2, _ := m.Next([]byte("<challenge2@host>"))

	if string(resp1) == string(resp2) {
		t.Error("different challenges should produce different responses")
	}
}

// --- ServerMechanism Tests ---

func TestServerMechanismName(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)
	if m.Name() != "CRAM-MD5" {
		t.Errorf("expected name CRAM-MD5, got %s", m.Name())
	}
}

func TestServerMechanismStep0SendsChallenge(t *testing.T) {
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
	if len(challenge) == 0 {
		t.Error("expected non-empty challenge")
	}
	// Challenge format should be <...@localhost>
	challengeStr := string(challenge)
	if !strings.HasPrefix(challengeStr, "<") || !strings.HasSuffix(challengeStr, "@localhost>") {
		t.Errorf("unexpected challenge format: %q", challengeStr)
	}
}

func TestServerMechanismStep1Success(t *testing.T) {
	var gotMech, gotIdentity string
	var gotCreds []byte

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		gotMech = mechanism
		gotIdentity = identity
		gotCreds = credentials
		return nil
	})
	m := NewServerMechanism(authenticator)

	// Step 0: get challenge
	if _, _, err := m.Next(nil); err != nil {
		t.Fatalf("step 0: unexpected error: %v", err)
	}

	// Step 1: send "username digest"
	response := []byte("testuser abc123def456")
	_, done, err := m.Next(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done to be true at step 1")
	}
	if gotMech != "CRAM-MD5" {
		t.Errorf("expected mechanism CRAM-MD5, got %s", gotMech)
	}
	if gotIdentity != "testuser" {
		t.Errorf("expected identity 'testuser', got %s", gotIdentity)
	}
	if string(gotCreds) != "testuser abc123def456" {
		t.Errorf("expected full response as credentials, got %q", string(gotCreds))
	}
}

func TestServerMechanismStep1AuthFailure(t *testing.T) {
	expectedErr := fmt.Errorf("authentication failed")
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return expectedErr
	})
	m := NewServerMechanism(authenticator)

	if _, _, err := m.Next(nil); err != nil {
		t.Fatalf("step 0: unexpected error: %v", err)
	}

	response := []byte("testuser baddigest")
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

func TestServerMechanismStep1InvalidFormat(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)

	if _, _, err := m.Next(nil); err != nil {
		t.Fatalf("step 0: unexpected error: %v", err)
	}

	// No space separator
	_, done, err := m.Next([]byte("nospacehere"))
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
	if !done {
		t.Error("expected done to be true")
	}
	if err.Error() != "cram-md5: invalid response format" {
		t.Errorf("expected 'cram-md5: invalid response format', got %q", err.Error())
	}
}

func TestServerMechanismStep2ReturnsError(t *testing.T) {
	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	m := NewServerMechanism(authenticator)

	if _, _, err := m.Next(nil); err != nil {
		t.Fatalf("step 0: unexpected error: %v", err)
	}
	if _, _, err := m.Next([]byte("user digest")); err != nil {
		t.Fatalf("step 1: unexpected error: %v", err)
	}

	_, done, err := m.Next([]byte("extra")) // step 2: no space => invalid format
	if err == nil {
		t.Fatal("expected error at step 2, got nil")
	}
	if err.Error() != "cram-md5: invalid response format" {
		t.Errorf("expected 'cram-md5: invalid response format', got %q", err.Error())
	}
	if !done {
		t.Error("expected done to be true")
	}
}

// --- Constant Tests ---

func TestNameConstant(t *testing.T) {
	if Name != "CRAM-MD5" {
		t.Errorf("expected Name constant to be CRAM-MD5, got %s", Name)
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

func TestCRAMMD5EndToEnd(t *testing.T) {
	password := "secret"

	client := &ClientMechanism{
		Username: "alice",
		Password: password,
	}

	authenticator := auth.AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		if identity != "alice" {
			return fmt.Errorf("unknown user")
		}
		// Parse the credentials to verify the HMAC
		parts := strings.SplitN(string(credentials), " ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid format")
		}
		// In a real server, we would recompute the HMAC using the stored password
		// and the challenge we sent. For this test, we just verify the format.
		if parts[0] != "alice" {
			return fmt.Errorf("username mismatch")
		}
		if len(parts[1]) != 32 {
			return fmt.Errorf("invalid digest length")
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
		t.Fatalf("expected nil initial response")
	}

	// Server step 0: sends challenge
	challenge, done, err := server.Next(nil)
	if err != nil {
		t.Fatalf("server step 0 error: %v", err)
	}
	if done {
		t.Fatal("expected not done at step 0")
	}

	// Client computes HMAC-MD5 response
	resp, err := client.Next(challenge)
	if err != nil {
		t.Fatalf("client Next error: %v", err)
	}

	// Server step 1: verifies response
	_, done, err = server.Next(resp)
	if err != nil {
		t.Fatalf("server step 1 error: %v", err)
	}
	if !done {
		t.Fatal("expected done at step 1")
	}
}

func TestCRAMMD5EndToEndWithVerification(t *testing.T) {
	// This test performs full cryptographic verification
	password := "tanstraafl"
	username := "tim"

	client := &ClientMechanism{
		Username: username,
		Password: password,
	}

	server := NewServerMechanism(nil) // we'll manually handle steps

	// Skip authenticator and verify HMAC manually
	// Step 0: get challenge
	challenge, done, _ := server.Next(nil)
	if done {
		t.Fatal("expected not done at step 0")
	}

	// Client computes response
	resp, err := client.Next(challenge)
	if err != nil {
		t.Fatalf("client Next error: %v", err)
	}

	// Parse client response
	parts := strings.SplitN(string(resp), " ", 2)
	if len(parts) != 2 {
		t.Fatalf("invalid response format")
	}

	if parts[0] != username {
		t.Errorf("expected username %q, got %q", username, parts[0])
	}

	// Recompute HMAC to verify
	h := hmac.New(md5.New, []byte(password))
	h.Write(challenge)
	expectedDigest := hex.EncodeToString(h.Sum(nil))

	if parts[1] != expectedDigest {
		t.Errorf("HMAC mismatch: expected %q, got %q", expectedDigest, parts[1])
	}
}
