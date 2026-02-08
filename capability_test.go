package imap

import (
	"sort"
	"strings"
	"testing"
)

func TestNewCapSet_Empty(t *testing.T) {
	cs := NewCapSet()
	if cs.Len() != 0 {
		t.Errorf("NewCapSet() Len = %d, want 0", cs.Len())
	}
	all := cs.All()
	if len(all) != 0 {
		t.Errorf("NewCapSet() All = %v, want empty", all)
	}
}

func TestNewCapSet_WithCaps(t *testing.T) {
	cs := NewCapSet(CapIMAP4rev1, CapIdle, CapMove)
	if cs.Len() != 3 {
		t.Errorf("Len = %d, want 3", cs.Len())
	}
	if !cs.Has(CapIMAP4rev1) {
		t.Error("should have IMAP4rev1")
	}
	if !cs.Has(CapIdle) {
		t.Error("should have IDLE")
	}
	if !cs.Has(CapMove) {
		t.Error("should have MOVE")
	}
	if cs.Has(CapSort) {
		t.Error("should not have SORT")
	}
}

func TestNewCapSet_Duplicates(t *testing.T) {
	cs := NewCapSet(CapIMAP4rev1, CapIMAP4rev1, CapIMAP4rev1)
	if cs.Len() != 1 {
		t.Errorf("Len = %d, want 1 (duplicates should be collapsed)", cs.Len())
	}
}

func TestCapSet_Add(t *testing.T) {
	cs := NewCapSet()
	cs.Add(CapIMAP4rev1)
	if !cs.Has(CapIMAP4rev1) {
		t.Error("should have IMAP4rev1 after Add")
	}
	if cs.Len() != 1 {
		t.Errorf("Len = %d, want 1", cs.Len())
	}

	// Add multiple at once
	cs.Add(CapIdle, CapMove, CapSort)
	if cs.Len() != 4 {
		t.Errorf("Len = %d, want 4", cs.Len())
	}

	// Adding duplicate does not increase length
	cs.Add(CapIdle)
	if cs.Len() != 4 {
		t.Errorf("Len after duplicate Add = %d, want 4", cs.Len())
	}
}

func TestCapSet_Remove(t *testing.T) {
	cs := NewCapSet(CapIMAP4rev1, CapIdle, CapMove)

	cs.Remove(CapIdle)
	if cs.Has(CapIdle) {
		t.Error("should not have IDLE after Remove")
	}
	if cs.Len() != 2 {
		t.Errorf("Len = %d, want 2", cs.Len())
	}

	// Removing non-existent capability is a no-op
	cs.Remove(CapSort)
	if cs.Len() != 2 {
		t.Errorf("Len = %d, want 2 after removing non-existent", cs.Len())
	}

	// Remove multiple
	cs.Remove(CapIMAP4rev1, CapMove)
	if cs.Len() != 0 {
		t.Errorf("Len = %d, want 0", cs.Len())
	}
}

func TestCapSet_Has(t *testing.T) {
	cs := NewCapSet(CapIMAP4rev1, CapAuthPlain)

	tests := []struct {
		cap  Cap
		want bool
	}{
		{CapIMAP4rev1, true},
		{CapAuthPlain, true},
		{CapIdle, false},
		{Cap("NONEXISTENT"), false},
		{Cap(""), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.cap), func(t *testing.T) {
			if got := cs.Has(tt.cap); got != tt.want {
				t.Errorf("Has(%q) = %v, want %v", tt.cap, got, tt.want)
			}
		})
	}
}

func TestCapSet_All(t *testing.T) {
	caps := []Cap{CapIMAP4rev1, CapIdle, CapMove}
	cs := NewCapSet(caps...)

	all := cs.All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d caps, want 3", len(all))
	}

	// Sort for deterministic comparison
	strs := make([]string, len(all))
	for i, c := range all {
		strs[i] = string(c)
	}
	sort.Strings(strs)

	wantStrs := []string{"IDLE", "IMAP4rev1", "MOVE"}
	sort.Strings(wantStrs)
	for i, s := range strs {
		if s != wantStrs[i] {
			t.Errorf("All()[%d] = %q, want %q", i, s, wantStrs[i])
		}
	}
}

func TestCapSet_Len(t *testing.T) {
	cs := NewCapSet()
	if cs.Len() != 0 {
		t.Errorf("Len = %d, want 0", cs.Len())
	}
	cs.Add(CapIMAP4rev1)
	if cs.Len() != 1 {
		t.Errorf("Len = %d, want 1", cs.Len())
	}
	cs.Add(CapIdle)
	if cs.Len() != 2 {
		t.Errorf("Len = %d, want 2", cs.Len())
	}
	cs.Remove(CapIMAP4rev1)
	if cs.Len() != 1 {
		t.Errorf("Len = %d, want 1", cs.Len())
	}
}

func TestCapSet_String(t *testing.T) {
	cs := NewCapSet(CapIMAP4rev1)
	got := cs.String()
	if got != "IMAP4rev1" {
		t.Errorf("String() = %q, want %q", got, "IMAP4rev1")
	}

	// For multiple caps, the order is non-deterministic, so check all parts are present
	cs.Add(CapIdle)
	str := cs.String()
	parts := strings.Split(str, " ")
	if len(parts) != 2 {
		t.Fatalf("String() = %q, expected 2 space-separated parts", str)
	}
	sort.Strings(parts)
	want := []string{"IDLE", "IMAP4rev1"}
	for i, p := range parts {
		if p != want[i] {
			t.Errorf("String() part[%d] = %q, want %q", i, p, want[i])
		}
	}
}

func TestCapSet_StringEmpty(t *testing.T) {
	cs := NewCapSet()
	if got := cs.String(); got != "" {
		t.Errorf("empty CapSet.String() = %q, want %q", got, "")
	}
}

func TestCapSet_Clone(t *testing.T) {
	original := NewCapSet(CapIMAP4rev1, CapIdle, CapMove)
	cloned := original.Clone()

	// Clone should have the same capabilities
	if cloned.Len() != original.Len() {
		t.Errorf("cloned Len = %d, original Len = %d", cloned.Len(), original.Len())
	}
	if !cloned.Has(CapIMAP4rev1) {
		t.Error("clone should have IMAP4rev1")
	}
	if !cloned.Has(CapIdle) {
		t.Error("clone should have IDLE")
	}
	if !cloned.Has(CapMove) {
		t.Error("clone should have MOVE")
	}

	// Modifying clone should not affect original
	cloned.Remove(CapIdle)
	if !original.Has(CapIdle) {
		t.Error("removing from clone should not affect original")
	}

	// Modifying original should not affect clone
	original.Add(CapSort)
	if cloned.Has(CapSort) {
		t.Error("adding to original should not affect clone")
	}
}

func TestCapSet_CloneEmpty(t *testing.T) {
	original := NewCapSet()
	cloned := original.Clone()
	if cloned.Len() != 0 {
		t.Errorf("cloned empty set Len = %d, want 0", cloned.Len())
	}
}

func TestCapSet_HasAuth(t *testing.T) {
	cs := NewCapSet(CapAuthPlain, CapAuthLogin, CapAuthXOAuth2)

	tests := []struct {
		mechanism string
		want      bool
	}{
		{"PLAIN", true},
		{"plain", true},
		{"Plain", true},
		{"LOGIN", true},
		{"login", true},
		{"XOAUTH2", true},
		{"CRAM-MD5", false},
		{"SCRAM-SHA-256", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.mechanism, func(t *testing.T) {
			if got := cs.HasAuth(tt.mechanism); got != tt.want {
				t.Errorf("HasAuth(%q) = %v, want %v", tt.mechanism, got, tt.want)
			}
		})
	}
}

func TestCapSet_HasAuthCaseSensitivity(t *testing.T) {
	// HasAuth converts to upper-case, so the set must contain the upper-case variant
	cs := NewCapSet(Cap("AUTH=SCRAM-SHA-1"))
	if !cs.HasAuth("scram-sha-1") {
		t.Error("HasAuth should match case-insensitively (converts to upper)")
	}
	if !cs.HasAuth("SCRAM-SHA-1") {
		t.Error("HasAuth should match exact upper case")
	}
}

func TestCapSet_AuthCapabilities(t *testing.T) {
	// Verify that auth constants match the expected pattern
	authCaps := []struct {
		cap       Cap
		mechanism string
	}{
		{CapAuthPlain, "PLAIN"},
		{CapAuthLogin, "LOGIN"},
		{CapAuthCRAMMD5, "CRAM-MD5"},
		{CapAuthSCRAMSHA1, "SCRAM-SHA-1"},
		{CapAuthSCRAMSHA256, "SCRAM-SHA-256"},
		{CapAuthXOAuth2, "XOAUTH2"},
		{CapAuthOAuthBearer, "OAUTHBEARER"},
		{CapAuthExternal, "EXTERNAL"},
		{CapAuthAnonymous, "ANONYMOUS"},
	}
	for _, tt := range authCaps {
		t.Run(string(tt.cap), func(t *testing.T) {
			cs := NewCapSet(tt.cap)
			if !cs.HasAuth(tt.mechanism) {
				t.Errorf("CapSet with %q should HasAuth(%q)", tt.cap, tt.mechanism)
			}
		})
	}
}

func TestCapSet_AddAndRemoveSequence(t *testing.T) {
	cs := NewCapSet()

	cs.Add(CapIMAP4rev1)
	cs.Add(CapIdle)
	cs.Add(CapMove)
	if cs.Len() != 3 {
		t.Fatalf("Len = %d, want 3", cs.Len())
	}

	cs.Remove(CapIdle)
	if cs.Len() != 2 {
		t.Fatalf("Len = %d, want 2", cs.Len())
	}
	if cs.Has(CapIdle) {
		t.Error("should not have IDLE")
	}

	cs.Add(CapIdle)
	if cs.Len() != 3 {
		t.Fatalf("Len = %d, want 3", cs.Len())
	}
	if !cs.Has(CapIdle) {
		t.Error("should have IDLE again")
	}
}
