package whatsapp

import "strings"

// NormalizePhone converts a raw phone string to the digits-only E.164 form the
// gateway expects (country code prefixed, no '+', no separators).
//
// Rules:
//   - strip everything except digits and a leading '+'
//   - a leading '+' is dropped (gateway wants bare digits)
//   - a leading '0' is treated as an Indonesian national prefix and replaced
//     with '62' (default country). Numbers already starting with a country
//     code are left as-is.
//
// If the input already contains a JID ("@"), it is returned unchanged so
// callers can pass full JIDs through.
func NormalizePhone(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "@") {
		return raw
	}

	hadPlus := strings.HasPrefix(raw, "+")

	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return ""
	}

	// Explicit international form: keep as-is.
	if hadPlus {
		return digits
	}

	// International dialing prefix "00" -> drop it (already country-coded).
	if strings.HasPrefix(digits, "00") {
		return strings.TrimPrefix(digits, "00")
	}

	// National leading zero -> Indonesian country code.
	if strings.HasPrefix(digits, "0") {
		return "62" + strings.TrimLeft(digits, "0")
	}

	return digits
}
