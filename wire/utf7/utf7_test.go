package utf7

import (
	"testing"
)

// ==================== Encode ====================

func TestEncode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "pure ASCII",
			input: "INBOX",
			want:  "INBOX",
		},
		{
			name:  "ASCII with spaces",
			input: "Sent Items",
			want:  "Sent Items",
		},
		{
			name:  "ampersand encoding",
			input: "&",
			want:  "&-",
		},
		{
			name:  "ampersand in middle",
			input: "Tom & Jerry",
			want:  "Tom &- Jerry",
		},
		{
			name:  "multiple ampersands",
			input: "A&B&C",
			want:  "A&-B&-C",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "Japanese characters",
			input: "\u65E5\u672C\u8A9E",
			want:  "&ZeVnLIqe-",
		},
		{
			name:  "mixed ASCII and non-ASCII",
			input: "INBOX.\u65E5\u672C\u8A9E",
			want:  "INBOX.&ZeVnLIqe-",
		},
		{
			name:  "euro sign",
			input: "\u20AC",
			want:  "&IKw-",
		},
		{
			name:  "German umlauts",
			input: "\u00E4\u00F6\u00FC",
			want:  "&AOQA9gD8-",
		},
		{
			name:  "non-ASCII followed by ASCII",
			input: "\u00E4bc",
			want:  "&AOQ-bc",
		},
		{
			name:  "ASCII followed by non-ASCII",
			input: "ab\u00E4",
			want:  "ab&AOQ-",
		},
		{
			name:  "supplementary character (emoji)",
			input: "\U0001F600",
			want:  "&2D3eAA-",
		},
		{
			name:  "supplementary character musical symbol",
			input: "\U0001D11E",
			want:  "&2DTdHg-",
		},
		{
			name:  "printable ASCII range low",
			input: " ",
			want:  " ",
		},
		{
			name:  "printable ASCII range high",
			input: "~",
			want:  "~",
		},
		{
			name:  "control char tab",
			input: "\t",
			want:  "&AAk-",
		},
		{
			name:  "control char newline",
			input: "\n",
			want:  "&AAo-",
		},
		{
			name:  "only non-ASCII",
			input: "\u00C0\u00C1\u00C2",
			want:  "&AMAAwQDC-",
		},
		{
			name:  "at sign (printable ASCII)",
			input: "@",
			want:  "@",
		},
		{
			name:  "mixed with ampersand and non-ASCII",
			input: "&\u00E4",
			want:  "&-&AOQ-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Encode(tt.input)
			if got != tt.want {
				t.Errorf("Encode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ==================== Decode ====================

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "pure ASCII",
			input: "INBOX",
			want:  "INBOX",
		},
		{
			name:  "ASCII with spaces",
			input: "Sent Items",
			want:  "Sent Items",
		},
		{
			name:  "ampersand literal",
			input: "&-",
			want:  "&",
		},
		{
			name:  "ampersand in middle",
			input: "Tom &- Jerry",
			want:  "Tom & Jerry",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "Japanese characters",
			input: "&ZeVnLIqe-",
			want:  "\u65E5\u672C\u8A9E",
		},
		{
			name:  "mixed ASCII and encoded",
			input: "INBOX.&ZeVnLIqe-",
			want:  "INBOX.\u65E5\u672C\u8A9E",
		},
		{
			name:  "euro sign",
			input: "&IKw-",
			want:  "\u20AC",
		},
		{
			name:  "German umlauts",
			input: "&AOQA9gD8-",
			want:  "\u00E4\u00F6\u00FC",
		},
		{
			name:  "supplementary character",
			input: "&2D3eAA-",
			want:  "\U0001F600",
		},
		{
			name:  "supplementary musical symbol",
			input: "&2DTdHg-",
			want:  "\U0001D11E",
		},
		{
			name:  "control char tab",
			input: "&AAk-",
			want:  "\t",
		},
		{
			name:  "mixed ampersand and non-ASCII",
			input: "&-&AOQ-",
			want:  "&\u00E4",
		},
		{
			name:    "unexpected end after ampersand",
			input:   "&",
			wantErr: true,
		},
		{
			name:    "missing closing dash",
			input:   "&ZeVnLIqe",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			input:   "&!!!-",
			wantErr: true,
		},
		{
			name:    "odd bytes in UTF-16",
			input:   "&AA-",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Decode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ==================== Round-trip ====================

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"pure ASCII", "INBOX"},
		{"ampersand", "&"},
		{"Japanese", "\u65E5\u672C\u8A9E"},
		{"mixed", "INBOX.\u65E5\u672C\u8A9E"},
		{"euro", "\u20AC"},
		{"umlauts", "\u00E4\u00F6\u00FC"},
		{"supplementary", "\U0001F600"},
		{"empty", ""},
		{"spaces", "Sent Items"},
		{"complex", "Tom & Jerry \u00E4\u00F6 \u65E5\u672C"},
		{"multiple ampersands", "&&&"},
		{"ASCII special chars", "~!@#$%^*()"},
		{"tab", "\t"},
		{"at sign", "@"},
		{"braces", "{}[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := Encode(tt.input)
			decoded, err := Decode(encoded)
			if err != nil {
				t.Fatalf("Decode(Encode(%q)) error = %v", tt.input, err)
			}
			if decoded != tt.input {
				t.Errorf("round-trip failed: input=%q, encoded=%q, decoded=%q", tt.input, encoded, decoded)
			}
		})
	}
}

// ==================== Edge cases ====================

func TestDecodeIncompleteSurrogatePair(t *testing.T) {
	// Manually craft a base64 section that contains a high surrogate
	// without a low surrogate. High surrogate D800 = 0xD800.
	// In modified base64: 0xD8, 0x00 = &2AA- (encoded as base64)
	// This should produce an error for incomplete surrogate pair.
	_, err := Decode("&2AA-")
	if err == nil {
		t.Error("expected error for incomplete surrogate pair")
	}
}

func TestEncodeAllPrintableASCII(t *testing.T) {
	// All printable ASCII (0x20-0x7E) should pass through unchanged,
	// except '&' which becomes '&-'
	for b := byte(0x20); b <= 0x7e; b++ {
		input := string([]byte{b})
		got := Encode(input)
		if b == '&' {
			if got != "&-" {
				t.Errorf("Encode(%q) = %q, want %q", input, got, "&-")
			}
		} else {
			if got != input {
				t.Errorf("Encode(%q) = %q, want %q (passthrough)", input, got, input)
			}
		}
	}
}

func TestDecodePassthroughASCII(t *testing.T) {
	// Non-& printable ASCII should pass through unchanged
	for b := byte(0x20); b <= 0x7e; b++ {
		if b == '&' {
			continue
		}
		input := string([]byte{b})
		got, err := Decode(input)
		if err != nil {
			t.Fatalf("Decode(%q) error = %v", input, err)
		}
		if got != input {
			t.Errorf("Decode(%q) = %q, want %q", input, got, input)
		}
	}
}

func TestEncodeConsecutiveNonASCII(t *testing.T) {
	// Multiple consecutive non-ASCII characters should be grouped in one base64 block
	input := "\u00C0\u00C1\u00C2"
	encoded := Encode(input)

	// Should have exactly one &...- block
	ampCount := 0
	for _, c := range encoded {
		if c == '&' {
			ampCount++
		}
	}
	if ampCount != 1 {
		t.Errorf("expected 1 base64 block, got %d ampersands in %q", ampCount, encoded)
	}

	// Verify round-trip
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != input {
		t.Errorf("round-trip failed: %q -> %q -> %q", input, encoded, decoded)
	}
}

func TestEncodeLongString(t *testing.T) {
	// Test with a longer string mixing ASCII and non-ASCII
	input := "Hello, \u4E16\u754C! This is a \u30C6\u30B9\u30C8."
	encoded := Encode(input)
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode error = %v", err)
	}
	if decoded != input {
		t.Errorf("round-trip failed:\n  input:   %q\n  encoded: %q\n  decoded: %q", input, encoded, decoded)
	}
}
