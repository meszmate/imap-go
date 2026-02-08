package auth

import (
	"context"
	"fmt"
	"sort"
	"testing"
)

// mockClientMechanism is a test helper implementing ClientMechanism.
type mockClientMechanism struct {
	name string
}

func (m *mockClientMechanism) Name() string                        { return m.name }
func (m *mockClientMechanism) Start() ([]byte, error)              { return []byte("initial"), nil }
func (m *mockClientMechanism) Next(challenge []byte) ([]byte, error) { return nil, nil }

// mockServerMechanism is a test helper implementing ServerMechanism.
type mockServerMechanism struct {
	name string
}

func (m *mockServerMechanism) Name() string { return m.name }
func (m *mockServerMechanism) Next(response []byte) ([]byte, bool, error) {
	return nil, true, nil
}

// --- Registry Tests ---

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if len(r.clientFactories) != 0 {
		t.Errorf("expected empty clientFactories, got %d entries", len(r.clientFactories))
	}
	if len(r.serverFactories) != 0 {
		t.Errorf("expected empty serverFactories, got %d entries", len(r.serverFactories))
	}
}

func TestRegistryRegisterClient(t *testing.T) {
	r := NewRegistry()
	r.RegisterClient("TEST", func() ClientMechanism {
		return &mockClientMechanism{name: "TEST"}
	})

	mechs := r.ClientMechanisms()
	if len(mechs) != 1 {
		t.Fatalf("expected 1 client mechanism, got %d", len(mechs))
	}
	if mechs[0] != "TEST" {
		t.Errorf("expected mechanism name TEST, got %s", mechs[0])
	}
}

func TestRegistryRegisterServer(t *testing.T) {
	r := NewRegistry()
	r.RegisterServer("TEST", func(a Authenticator) ServerMechanism {
		return &mockServerMechanism{name: "TEST"}
	})

	mechs := r.ServerMechanisms()
	if len(mechs) != 1 {
		t.Fatalf("expected 1 server mechanism, got %d", len(mechs))
	}
	if mechs[0] != "TEST" {
		t.Errorf("expected mechanism name TEST, got %s", mechs[0])
	}
}

func TestRegistryRegisterClientCaseInsensitive(t *testing.T) {
	r := NewRegistry()
	r.RegisterClient("lowercase", func() ClientMechanism {
		return &mockClientMechanism{name: "LOWERCASE"}
	})

	mech, err := r.NewClientMechanism("LOWERCASE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mech.Name() != "LOWERCASE" {
		t.Errorf("expected mechanism name LOWERCASE, got %s", mech.Name())
	}

	// Should also work with mixed case
	mech2, err := r.NewClientMechanism("Lowercase")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mech2.Name() != "LOWERCASE" {
		t.Errorf("expected mechanism name LOWERCASE, got %s", mech2.Name())
	}
}

func TestRegistryRegisterServerCaseInsensitive(t *testing.T) {
	r := NewRegistry()
	r.RegisterServer("lowercase", func(a Authenticator) ServerMechanism {
		return &mockServerMechanism{name: "LOWERCASE"}
	})

	auth := AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})

	mech, err := r.NewServerMechanism("LOWERCASE", auth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mech.Name() != "LOWERCASE" {
		t.Errorf("expected mechanism name LOWERCASE, got %s", mech.Name())
	}
}

func TestRegistryNewClientMechanismNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.NewClientMechanism("NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for nonexistent mechanism, got nil")
	}
	expected := `auth: unsupported client mechanism "NONEXISTENT"`
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestRegistryNewServerMechanismNotFound(t *testing.T) {
	r := NewRegistry()
	auth := AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	_, err := r.NewServerMechanism("NONEXISTENT", auth)
	if err == nil {
		t.Fatal("expected error for nonexistent mechanism, got nil")
	}
	expected := `auth: unsupported server mechanism "NONEXISTENT"`
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestRegistryNewClientMechanism(t *testing.T) {
	r := NewRegistry()
	r.RegisterClient("MOCK", func() ClientMechanism {
		return &mockClientMechanism{name: "MOCK"}
	})

	mech, err := r.NewClientMechanism("MOCK")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mech == nil {
		t.Fatal("NewClientMechanism returned nil")
	}
	if mech.Name() != "MOCK" {
		t.Errorf("expected name MOCK, got %s", mech.Name())
	}
	ir, err := mech.Start()
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if string(ir) != "initial" {
		t.Errorf("expected initial response 'initial', got %q", string(ir))
	}
}

func TestRegistryNewServerMechanism(t *testing.T) {
	r := NewRegistry()
	r.RegisterServer("MOCK", func(a Authenticator) ServerMechanism {
		return &mockServerMechanism{name: "MOCK"}
	})

	auth := AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	mech, err := r.NewServerMechanism("MOCK", auth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mech == nil {
		t.Fatal("NewServerMechanism returned nil")
	}
	if mech.Name() != "MOCK" {
		t.Errorf("expected name MOCK, got %s", mech.Name())
	}
}

func TestRegistryClientMechanismsEmpty(t *testing.T) {
	r := NewRegistry()
	mechs := r.ClientMechanisms()
	if len(mechs) != 0 {
		t.Errorf("expected 0 client mechanisms, got %d", len(mechs))
	}
}

func TestRegistryServerMechanismsEmpty(t *testing.T) {
	r := NewRegistry()
	mechs := r.ServerMechanisms()
	if len(mechs) != 0 {
		t.Errorf("expected 0 server mechanisms, got %d", len(mechs))
	}
}

func TestRegistryMultipleClientMechanisms(t *testing.T) {
	r := NewRegistry()
	names := []string{"ALPHA", "BETA", "GAMMA"}
	for _, name := range names {
		n := name
		r.RegisterClient(n, func() ClientMechanism {
			return &mockClientMechanism{name: n}
		})
	}

	mechs := r.ClientMechanisms()
	sort.Strings(mechs)
	sort.Strings(names)

	if len(mechs) != len(names) {
		t.Fatalf("expected %d mechanisms, got %d", len(names), len(mechs))
	}
	for i, name := range names {
		if mechs[i] != name {
			t.Errorf("expected mechanism %s at index %d, got %s", name, i, mechs[i])
		}
	}
}

func TestRegistryMultipleServerMechanisms(t *testing.T) {
	r := NewRegistry()
	names := []string{"ALPHA", "BETA", "GAMMA"}
	for _, name := range names {
		n := name
		r.RegisterServer(n, func(a Authenticator) ServerMechanism {
			return &mockServerMechanism{name: n}
		})
	}

	mechs := r.ServerMechanisms()
	sort.Strings(mechs)
	sort.Strings(names)

	if len(mechs) != len(names) {
		t.Fatalf("expected %d mechanisms, got %d", len(names), len(mechs))
	}
	for i, name := range names {
		if mechs[i] != name {
			t.Errorf("expected mechanism %s at index %d, got %s", name, i, mechs[i])
		}
	}
}

func TestRegistryOverwriteClient(t *testing.T) {
	r := NewRegistry()
	r.RegisterClient("TEST", func() ClientMechanism {
		return &mockClientMechanism{name: "OLD"}
	})
	r.RegisterClient("TEST", func() ClientMechanism {
		return &mockClientMechanism{name: "NEW"}
	})

	mech, err := r.NewClientMechanism("TEST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mech.Name() != "NEW" {
		t.Errorf("expected overwritten mechanism name NEW, got %s", mech.Name())
	}

	mechs := r.ClientMechanisms()
	if len(mechs) != 1 {
		t.Errorf("expected 1 mechanism after overwrite, got %d", len(mechs))
	}
}

func TestRegistryOverwriteServer(t *testing.T) {
	r := NewRegistry()
	r.RegisterServer("TEST", func(a Authenticator) ServerMechanism {
		return &mockServerMechanism{name: "OLD"}
	})
	r.RegisterServer("TEST", func(a Authenticator) ServerMechanism {
		return &mockServerMechanism{name: "NEW"}
	})

	auth := AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return nil
	})
	mech, err := r.NewServerMechanism("TEST", auth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mech.Name() != "NEW" {
		t.Errorf("expected overwritten mechanism name NEW, got %s", mech.Name())
	}

	mechs := r.ServerMechanisms()
	if len(mechs) != 1 {
		t.Errorf("expected 1 mechanism after overwrite, got %d", len(mechs))
	}
}

// --- AuthenticatorFunc Tests ---

func TestAuthenticatorFuncSuccess(t *testing.T) {
	var calledMech, calledIdentity string
	var calledCreds []byte

	af := AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		calledMech = mechanism
		calledIdentity = identity
		calledCreds = credentials
		return nil
	})

	err := af.Authenticate(context.Background(), "PLAIN", "user@example.com", []byte("secret"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calledMech != "PLAIN" {
		t.Errorf("expected mechanism PLAIN, got %s", calledMech)
	}
	if calledIdentity != "user@example.com" {
		t.Errorf("expected identity user@example.com, got %s", calledIdentity)
	}
	if string(calledCreds) != "secret" {
		t.Errorf("expected credentials 'secret', got %q", string(calledCreds))
	}
}

func TestAuthenticatorFuncError(t *testing.T) {
	expectedErr := fmt.Errorf("authentication failed")
	af := AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		return expectedErr
	})

	err := af.Authenticate(context.Background(), "PLAIN", "user", []byte("wrong"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestAuthenticatorFuncNilCredentials(t *testing.T) {
	af := AuthenticatorFunc(func(ctx context.Context, mechanism, identity string, credentials []byte) error {
		if credentials != nil {
			return fmt.Errorf("expected nil credentials")
		}
		return nil
	})

	err := af.Authenticate(context.Background(), "EXTERNAL", "user", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DefaultRegistry Tests ---

func TestDefaultRegistryExists(t *testing.T) {
	if DefaultRegistry == nil {
		t.Fatal("DefaultRegistry is nil")
	}
}

// --- Interface compliance tests ---

func TestMockClientMechanismImplementsInterface(t *testing.T) {
	var _ ClientMechanism = &mockClientMechanism{}
}

func TestMockServerMechanismImplementsInterface(t *testing.T) {
	var _ ServerMechanism = &mockServerMechanism{}
}

func TestAuthenticatorFuncImplementsAuthenticator(t *testing.T) {
	var _ Authenticator = AuthenticatorFunc(nil)
}
