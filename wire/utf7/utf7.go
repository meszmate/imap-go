// Package utf7 implements the modified UTF-7 encoding defined in RFC 2152
// as used by IMAP mailbox names (RFC 3501 Section 5.1.3).
//
// Modified UTF-7 uses & as the shift character instead of +, and uses , instead
// of / in the base64 alphabet. The & character is encoded as &-.
package utf7

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf16"
)

// modifiedBase64 is the base64 encoding used in modified UTF-7.
// It uses , instead of / from standard base64.
var modifiedBase64 = base64.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+,").WithPadding(base64.NoPadding)

// Encode encodes a UTF-8 string to modified UTF-7.
func Encode(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))

	var b64buf []byte

	flush := func() {
		if len(b64buf) > 0 {
			buf.WriteByte('&')
			encoded := modifiedBase64.EncodeToString(b64buf)
			buf.WriteString(encoded)
			buf.WriteByte('-')
			b64buf = b64buf[:0]
		}
	}

	for _, r := range s {
		if r >= 0x20 && r <= 0x7e {
			flush()
			if r == '&' {
				buf.WriteString("&-")
			} else {
				buf.WriteRune(r)
			}
		} else {
			// Encode as UTF-16BE in base64
			if r >= 0x10000 {
				// Supplementary character - use surrogate pair
				r1, r2 := utf16.EncodeRune(r)
				b64buf = append(b64buf, byte(r1>>8), byte(r1&0xff))
				b64buf = append(b64buf, byte(r2>>8), byte(r2&0xff))
			} else {
				b64buf = append(b64buf, byte(r>>8), byte(r&0xff))
			}
		}
	}
	flush()

	return buf.String()
}

// Decode decodes a modified UTF-7 string to UTF-8.
func Decode(s string) (string, error) {
	var buf strings.Builder
	buf.Grow(len(s))

	i := 0
	for i < len(s) {
		if s[i] != '&' {
			buf.WriteByte(s[i])
			i++
			continue
		}

		// Found '&'
		i++
		if i >= len(s) {
			return "", fmt.Errorf("utf7: unexpected end after '&'")
		}

		if s[i] == '-' {
			// &- encodes literal '&'
			buf.WriteByte('&')
			i++
			continue
		}

		// Find the closing '-'
		end := strings.IndexByte(s[i:], '-')
		if end < 0 {
			return "", fmt.Errorf("utf7: missing closing '-' for base64 section")
		}

		encoded := s[i : i+end]
		i += end + 1 // skip past '-'

		// Decode base64 to UTF-16BE
		decoded, err := modifiedBase64.DecodeString(encoded)
		if err != nil {
			return "", fmt.Errorf("utf7: invalid base64: %w", err)
		}

		if len(decoded)%2 != 0 {
			return "", fmt.Errorf("utf7: odd number of bytes in UTF-16 data")
		}

		// Convert UTF-16BE to runes
		for j := 0; j < len(decoded); j += 2 {
			code := uint16(decoded[j])<<8 | uint16(decoded[j+1])
			if utf16.IsSurrogate(rune(code)) {
				if j+3 >= len(decoded) {
					return "", fmt.Errorf("utf7: incomplete surrogate pair")
				}
				j += 2
				code2 := uint16(decoded[j])<<8 | uint16(decoded[j+1])
				r := utf16.DecodeRune(rune(code), rune(code2))
				if r == '\uFFFD' {
					return "", fmt.Errorf("utf7: invalid surrogate pair")
				}
				buf.WriteRune(r)
			} else {
				buf.WriteRune(rune(code))
			}
		}
	}

	return buf.String(), nil
}
