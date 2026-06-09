package whatsapp

import "testing"

// TestNormalizePhoneEdgeCases extends the base coverage in markdown_test.go
// with separator-stripping, JID group passthrough, and multi-zero inputs.
func TestNormalizePhoneEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"jid group passthrough", "12036304@g.us", "12036304@g.us"},
		{"parens and dashes", "(0812) 345-678", "62812345678"},
		{"only separators", "+-() ", ""},
		{"multiple leading zeros national", "00812345", "812345"},
		{"plus with spaces inside", "+62 812 3456 7890", "6281234567890"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizePhone(c.in); got != c.want {
				t.Errorf("NormalizePhone(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
