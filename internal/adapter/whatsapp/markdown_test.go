package whatsapp

import "testing"

func TestNormalizePhone(t *testing.T) {
	cases := []struct{ in, want string }{
		{"08123456789", "628123456789"},
		{"+628123456789", "628123456789"},
		{"628123456789", "628123456789"},
		{"0812-345-6789", "628123456789"},
		{"+62 812 3456 789", "628123456789"},
		{"628123456789@s.whatsapp.net", "628123456789@s.whatsapp.net"},
		{"  ", ""},
		{"", ""},
		{"00628123456789", "628123456789"},
	}
	for _, c := range cases {
		if got := NormalizePhone(c.in); got != c.want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToWhatsApp(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"bold double", "**hi**", "*hi*"},
		{"bold underscore", "__hi__", "*hi*"},
		{"italic single", "*hi*", "_hi_"},
		{"strike", "~~hi~~", "~hi~"},
		{"heading", "# Title", "*Title*"},
		{"heading h3", "### Sub", "*Sub*"},
		{"link", "see [docs](https://x.io)", "see docs (https://x.io)"},
		{"image", "![alt](https://x.io/a.png)", "https://x.io/a.png"},
		{"bold not reitalicized", "**bold** and *it*", "*bold* and _it_"},
		{"plain", "no markdown here", "no markdown here"},
		{"math untouched", "2 * 3 = 6", "2 * 3 = 6"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := ToWhatsApp(c.in); got != c.want {
			t.Errorf("%s: ToWhatsApp(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}
