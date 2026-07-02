package xclient

import (
	"fmt"
	"strings"
)

// SearchQueryParts builds an x.com search raw_query from structured inputs.
// At least one of Raw, Hashtag, Cashtag, Mention, or Keyword must be set.
type SearchQueryParts struct {
	Raw     string
	Hashtag string
	Cashtag string
	Mention string
	Keyword string
	// Viral appends min_faves/min_retweets operators for server-side pre-filtering.
	Viral bool
}

// BuildSearchQuery composes the SearchTimeline rawQuery string using x.com
// search operators (#tag, $tag, @mention, min_faves, etc.).
func BuildSearchQuery(p SearchQueryParts) (string, error) {
	if strings.TrimSpace(p.Raw) != "" {
		return strings.TrimSpace(p.Raw), nil
	}

	var parts []string
	if tag := normalizeToken(p.Hashtag, "#"); tag != "" {
		parts = append(parts, "#"+tag)
	}
	if tag := normalizeToken(p.Cashtag, "$"); tag != "" {
		parts = append(parts, "$"+tag)
	}
	if tag := normalizeToken(p.Mention, "@"); tag != "" {
		parts = append(parts, "@"+tag)
	}
	if kw := strings.TrimSpace(p.Keyword); kw != "" {
		parts = append(parts, kw)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("search needs -query or at least one of -hashtag, -cashtag, -mention, -keyword")
	}
	q := strings.Join(parts, " ")
	if p.Viral {
		q += " min_faves:100 min_retweets:25 min_replies:10"
	}
	return q, nil
}

func normalizeToken(s, prefix string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, prefix)
	return strings.TrimSpace(s)
}
