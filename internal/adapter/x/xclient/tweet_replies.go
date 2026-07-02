package xclient

import (
	"strings"
)

// TweetRepliesOptions configures a reply fetch for a tweet's conversation.
type TweetRepliesOptions struct {
	TweetID string
	Count   int
	Cursor  string
}

// TweetRepliesResult holds parsed reply tweets and an optional next-page
// cursor. Replies are the tweets in the focal tweet's conversation thread,
// excluding the focal tweet itself.
type TweetRepliesResult struct {
	Replies    []TweetSummary
	NextCursor string
}

// TweetReplies fetches replies to a tweet by walking the TweetDetail
// threaded_conversation timeline. The focal tweet (tweet-<id>) is skipped;
// conversation-thread-* entries hold the reply tweets. Pagination uses the
// cursor-bottom / cursor-showmorethreads value.
func (c *Client) TweetReplies(opts TweetRepliesOptions) (*TweetRepliesResult, error) {
	count := opts.Count
	if count <= 0 {
		count = 20
	}
	variables := map[string]any{
		"focalTweetId":                           opts.TweetID,
		"with_rux_injections":                    false,
		"includePromotedContent":                 true,
		"withCommunity":                          true,
		"withQuickPromoteEligibilityTweetFields": true,
		"withBirdwatchNotes":                     true,
		"withVoice":                              true,
		"withV2Timeline":                         true,
	}
	if opts.Cursor != "" {
		variables["cursor"] = opts.Cursor
	}
	fieldToggles := map[string]any{"withAuxiliaryUserLabels": false}

	var resp struct {
		Data struct {
			ThreadedConversation struct {
				Instructions []timelineInstruction `json:"instructions"`
			} `json:"threaded_conversation_with_injections_v2"`
		} `json:"data"`
	}
	if err := c.doGraphQL(tweetDetailQueryID, "TweetDetail", variables, defaultTimelineFeatures(), fieldToggles, &resp); err != nil {
		return nil, err
	}

	focal := "tweet-" + opts.TweetID
	var (
		replies []TweetSummary
		next    string
	)
	for _, inst := range resp.Data.ThreadedConversation.Instructions {
		for _, entry := range inst.Entries {
			switch {
			case entry.EntryID == focal:
				continue
			case strings.HasPrefix(entry.EntryID, "cursor-bottom"),
				strings.HasPrefix(entry.EntryID, "cursor-showmorethreads"):
				if v := cursorValue(entry.Content); v != "" {
					next = v
				}
				continue
			case strings.HasPrefix(entry.EntryID, "conversationthread-"),
				strings.HasPrefix(entry.EntryID, "tweet-"):
				if t := tweetFromEntry(entry); t != nil && t.RestID != opts.TweetID {
					replies = append(replies, *t)
				}
			}
		}
	}
	return &TweetRepliesResult{Replies: replies, NextCursor: next}, nil
}
