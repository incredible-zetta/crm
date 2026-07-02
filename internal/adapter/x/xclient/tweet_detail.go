package xclient

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const tweetDetailQueryID = "jd3V43oDY9cY7obs1YMfbQ"

// TweetDetail is a tweet with full engagement metrics from TweetDetail.
type TweetDetail struct {
	TweetSummary
	Views          int
	ViewState      string
	Quotes         int
	Bookmarks      int
	ConversationID string
}

// TweetDetailByID fetches a tweet and its engagement analytics.
func (c *Client) TweetDetailByID(tweetID string) (*TweetDetail, error) {
	variables := map[string]any{
		"focalTweetId":                           tweetID,
		"with_rux_injections":                    false,
		"includePromotedContent":                 true,
		"withCommunity":                          true,
		"withQuickPromoteEligibilityTweetFields": true,
		"withBirdwatchNotes":                     true,
		"withVoice":                              true,
		"withV2Timeline":                         true,
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

	for _, inst := range resp.Data.ThreadedConversation.Instructions {
		for _, entry := range inst.Entries {
			if entry.EntryID == "tweet-"+tweetID {
				return parseTweetDetail(entry)
			}
		}
	}
	return nil, fmt.Errorf("tweet %s not found in response", tweetID)
}

func parseTweetDetail(entry timelineEntry) (*TweetDetail, error) {
	raw := tweetResultRaw(entry)
	if len(raw) == 0 {
		return nil, fmt.Errorf("no tweet data in entry")
	}

	var node struct {
		Tweet *struct {
			RestID string `json:"rest_id"`
			Core   struct {
				UserResults struct {
					Result struct {
						Core struct {
							Name       string `json:"name"`
							ScreenName string `json:"screen_name"`
						} `json:"core"`
					} `json:"result"`
				} `json:"user_results"`
			} `json:"core"`
			Legacy struct {
				FullText           string `json:"full_text"`
				CreatedAt          string `json:"created_at"`
				FavoriteCount      int    `json:"favorite_count"`
				RetweetCount       int    `json:"retweet_count"`
				ReplyCount         int    `json:"reply_count"`
				QuoteCount         int    `json:"quote_count"`
				BookmarkCount      int    `json:"bookmark_count"`
				ConversationIDStr  string `json:"conversation_id_str"`
			} `json:"legacy"`
			Views struct {
				Count string `json:"count"`
				State string `json:"state"`
			} `json:"views"`
			ExtendedEntities struct {
				Media []any `json:"media"`
			} `json:"extended_entities"`
		} `json:"tweet"`
		RestID string `json:"rest_id"`
		Core   struct {
			UserResults struct {
				Result struct {
					Core struct {
						Name       string `json:"name"`
						ScreenName string `json:"screen_name"`
					} `json:"core"`
				} `json:"result"`
			} `json:"user_results"`
		} `json:"core"`
		Legacy struct {
			FullText          string `json:"full_text"`
			CreatedAt         string `json:"created_at"`
			FavoriteCount     int    `json:"favorite_count"`
			RetweetCount      int    `json:"retweet_count"`
			ReplyCount        int    `json:"reply_count"`
			QuoteCount        int    `json:"quote_count"`
			BookmarkCount     int    `json:"bookmark_count"`
			ConversationIDStr string `json:"conversation_id_str"`
		} `json:"legacy"`
		Views struct {
			Count string `json:"count"`
			State string `json:"state"`
		} `json:"views"`
		ExtendedEntities struct {
			Media []any `json:"media"`
		} `json:"extended_entities"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, err
	}

	var (
		id, text, created, conv string
		likes, rts, reps, quotes, bookmarks int
		views int
		viewState, name, screen string
		hasMedia bool
	)
	parseViews := func(count, state string) (int, string) {
		n, _ := strconv.Atoi(strings.ReplaceAll(count, ",", ""))
		return n, state
	}

	if node.Tweet != nil {
		t := node.Tweet
		id = t.RestID
		text = t.Legacy.FullText
		created = t.Legacy.CreatedAt
		likes = t.Legacy.FavoriteCount
		rts = t.Legacy.RetweetCount
		reps = t.Legacy.ReplyCount
		quotes = t.Legacy.QuoteCount
		bookmarks = t.Legacy.BookmarkCount
		conv = t.Legacy.ConversationIDStr
		views, viewState = parseViews(t.Views.Count, t.Views.State)
		name = t.Core.UserResults.Result.Core.Name
		screen = t.Core.UserResults.Result.Core.ScreenName
		hasMedia = len(t.ExtendedEntities.Media) > 0
	} else {
		id = node.RestID
		text = node.Legacy.FullText
		created = node.Legacy.CreatedAt
		likes = node.Legacy.FavoriteCount
		rts = node.Legacy.RetweetCount
		reps = node.Legacy.ReplyCount
		quotes = node.Legacy.QuoteCount
		bookmarks = node.Legacy.BookmarkCount
		conv = node.Legacy.ConversationIDStr
		views, viewState = parseViews(node.Views.Count, node.Views.State)
		name = node.Core.UserResults.Result.Core.Name
		screen = node.Core.UserResults.Result.Core.ScreenName
		hasMedia = len(node.ExtendedEntities.Media) > 0
	}

	summary := TweetSummary{
		RestID:     id,
		Text:       text,
		ScreenName: screen,
		Name:       name,
		CreatedAt:  created,
		Likes:      likes,
		Retweets:   rts,
		Replies:    reps,
		HasMedia:   hasMedia,
		URL:        "https://x.com/" + screen + "/status/" + id,
	}
	return &TweetDetail{
		TweetSummary:   summary,
		Views:          views,
		ViewState:      viewState,
		Quotes:         quotes,
		Bookmarks:      bookmarks,
		ConversationID: conv,
	}, nil
}

func tweetResultRaw(entry timelineEntry) json.RawMessage {
	var content entryContent
	if err := json.Unmarshal(entry.Content, &content); err != nil {
		return nil
	}
	if content.ItemContent != nil && content.ItemContent.TweetResults != nil {
		return content.ItemContent.TweetResults.Result
	}
	return nil
}
