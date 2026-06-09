package whatsapp

import (
	"regexp"
	"strings"
)

// WhatsApp text formatting differs from GitHub-flavored Markdown:
//
//	bold:          *text*      (GitHub: **text**)
//	italic:        _text_      (GitHub: *text* or _text_)
//	strikethrough: ~text~      (GitHub: ~~text~~)
//	monospace:     ```text```  (inline code/code blocks)
//	bulleted list: "- " or "* " at line start
//	numbered list: "1. " at line start
//
// WhatsApp has no heading or hyperlink syntax: a "# Heading" renders literally
// and "[label](url)" shows raw. ToWhatsApp converts common GitHub Markdown that
// an LLM is likely to emit into WhatsApp-renderable text so messages look right
// in the chat.
const (
	boldSentinel   = "\x01"
	strikeSentinel = "\x02"
)

var (
	reGhBold    = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reGhBoldU   = regexp.MustCompile(`__(.+?)__`)
	reGhStrike  = regexp.MustCompile(`~~(.+?)~~`)
	reGhItalicA = regexp.MustCompile(`(^|[^*])\*([^*\s][^*]*?)\*`) // *italic* (single asterisk)
	reMdHeading = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s+(.*?)\s*#*\s*$`)
	reMdLink    = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)\s]+)\)`)
	reMdImage   = regexp.MustCompile(`!\[([^\]]*)\]\((https?://[^)\s]+)\)`)
)

// ToWhatsApp converts GitHub-flavored Markdown to WhatsApp formatting.
//
// Conversions:
//   - **bold** / __bold__      -> *bold*
//   - ~~strike~~               -> ~strike~
//   - *italic* (single)        -> _italic_
//   - # Heading                -> *Heading* (bold, on its own line)
//   - [label](url)             -> label (url)
//   - ![alt](url)              -> url
//
// Text that is already WhatsApp-formatted passes through unchanged.
func ToWhatsApp(s string) string {
	if s == "" {
		return s
	}
	s = stripSentinels(s)

	// Images first (so the following link pass does not touch them).
	s = reMdImage.ReplaceAllString(s, "$2")
	// Links -> "label (url)".
	s = reMdLink.ReplaceAllString(s, "$1 ($2)")

	// Bold (double) -> sentinel-wrapped to protect from the italic pass.
	s = reGhBold.ReplaceAllString(s, boldSentinel+"$1"+boldSentinel)
	s = reGhBoldU.ReplaceAllString(s, boldSentinel+"$1"+boldSentinel)
	// Strikethrough (double) -> sentinel.
	s = reGhStrike.ReplaceAllString(s, strikeSentinel+"$1"+strikeSentinel)
	// Remaining single *italic* -> _italic_.
	s = reGhItalicA.ReplaceAllString(s, "${1}_${2}_")

	// Headings -> bold line.
	s = reMdHeading.ReplaceAllString(s, boldSentinel+"$1"+boldSentinel)

	// Restore sentinels to WhatsApp delimiters.
	s = strings.ReplaceAll(s, boldSentinel, "*")
	s = strings.ReplaceAll(s, strikeSentinel, "~")
	return s
}

func stripSentinels(s string) string {
	if !strings.ContainsAny(s, boldSentinel+strikeSentinel) {
		return s
	}
	return strings.NewReplacer(boldSentinel, "", strikeSentinel, "").Replace(s)
}
