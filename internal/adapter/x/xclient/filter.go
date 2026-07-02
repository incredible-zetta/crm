package xclient

import (
	"sort"
	"strings"
	"time"
)

// Viral thresholds used by -viral preset (client-side filter).
const (
	ViralMinLikes    = 100
	ViralMinRetweets = 25
	ViralMinReplies  = 10
)

// FilterOptions applies client-side filters to tweet lists.
type FilterOptions struct {
	Author         string
	MinLikes       int
	MinRetweets    int
	MinReplies     int
	MinEngagement  int // likes + retweets + replies
	HasMedia       *bool
	Keyword        string
	Hashtag        string
	Cashtag        string
	Mention        string
	Since          time.Time
	Until          time.Time
	Viral          bool // apply viral engagement thresholds
	SortEngagement bool // sort by engagement score descending
}

// EngagementScore is a simple viral signal: likes + 2×retweets + replies.
func EngagementScore(t TweetSummary) int {
	return t.Likes + t.Retweets*2 + t.Replies
}

// ApplyViralPreset fills engagement thresholds when Viral is set and values unset.
func (o *FilterOptions) ApplyViralPreset() {
	if !o.Viral {
		return
	}
	if o.MinLikes == 0 {
		o.MinLikes = ViralMinLikes
	}
	if o.MinRetweets == 0 {
		o.MinRetweets = ViralMinRetweets
	}
	if o.MinReplies == 0 {
		o.MinReplies = ViralMinReplies
	}
	o.SortEngagement = true
}

// FilterTweets returns tweets matching all non-zero filter criteria.
func FilterTweets(tweets []TweetSummary, opts FilterOptions) []TweetSummary {
	opts.ApplyViralPreset()

	if opts.isNoop() {
		return tweets
	}

	author := strings.TrimPrefix(strings.ToLower(opts.Author), "@")
	keyword := strings.ToLower(opts.Keyword)
	hashtag := normalizeToken(opts.Hashtag, "#")
	cashtag := normalizeToken(opts.Cashtag, "$")
	mention := normalizeToken(opts.Mention, "@")

	var out []TweetSummary
	for _, t := range tweets {
		if author != "" && strings.ToLower(t.ScreenName) != author {
			continue
		}
		if opts.MinLikes > 0 && t.Likes < opts.MinLikes {
			continue
		}
		if opts.MinRetweets > 0 && t.Retweets < opts.MinRetweets {
			continue
		}
		if opts.MinReplies > 0 && t.Replies < opts.MinReplies {
			continue
		}
		if opts.MinEngagement > 0 && EngagementScore(t) < opts.MinEngagement {
			continue
		}
		if opts.HasMedia != nil && t.HasMedia != *opts.HasMedia {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(t.Text), keyword) {
			continue
		}
		if hashtag != "" && !textHasToken(t.Text, "#"+hashtag) {
			continue
		}
		if cashtag != "" && !textHasToken(t.Text, "$"+cashtag) {
			continue
		}
		if mention != "" && !textHasToken(t.Text, "@"+mention) {
			continue
		}
		if !opts.Since.IsZero() && !t.CreatedTime.IsZero() && t.CreatedTime.Before(opts.Since) {
			continue
		}
		if !opts.Until.IsZero() && !t.CreatedTime.IsZero() && t.CreatedTime.After(opts.Until) {
			continue
		}
		out = append(out, t)
	}

	if opts.SortEngagement {
		sort.Slice(out, func(i, j int) bool {
			return EngagementScore(out[i]) > EngagementScore(out[j])
		})
	}
	return out
}

func (o FilterOptions) isNoop() bool {
	return o.Author == "" && o.MinLikes == 0 && o.MinRetweets == 0 && o.MinReplies == 0 &&
		o.MinEngagement == 0 && o.HasMedia == nil && o.Keyword == "" &&
		o.Hashtag == "" && o.Cashtag == "" && o.Mention == "" &&
		o.Since.IsZero() && o.Until.IsZero() && !o.Viral
}

// textHasToken checks tweet text for a hashtag/cashtag/mention (case-insensitive).
func textHasToken(text, token string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(token))
}
