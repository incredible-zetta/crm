package xclient

import (
	"encoding/json"
	"strings"
	"time"
)

const userTweetsQueryID = "hr4gzZONlq23okjU8fIe_A"

// TweetSummary is a trimmed tweet for search/timeline/filter output.
type TweetSummary struct {
	RestID      string
	Text        string
	ScreenName  string
	Name        string
	CreatedAt   string
	CreatedTime time.Time
	Likes       int
	Retweets    int
	Replies     int
	HasMedia    bool
	URL         string
}

// UserTweetsOptions configures a UserTweets fetch.
type UserTweetsOptions struct {
	UserID string
	Count  int
	Cursor string
}

// UserTweetsResult holds parsed tweets and an optional next-page cursor.
type UserTweetsResult struct {
	Tweets     []TweetSummary
	NextCursor string
}

// defaultTimelineFeatures mirrors the web client flags for timeline reads
// (UserTweets, etc.). Update from a fresh capture if the API rejects requests.
func defaultTimelineFeatures() map[string]any {
	return map[string]any{
		"rweb_video_screen_enabled":                                               false,
		"profile_label_improvements_pcf_label_in_post_enabled":                    true,
		"responsive_web_profile_redirect_enabled":                                 false,
		"rweb_tipjar_consumption_enabled":                                         false,
		"verified_phone_label_enabled":                                            false,
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"premium_content_api_read_enabled":                                        false,
		"communities_web_enable_tweet_community_results_fetch":                    true,
		"c9s_tweet_anatomy_moderator_badge_enabled":                               true,
		"responsive_web_grok_analyze_button_fetch_trends_enabled":                 false,
		"responsive_web_grok_analyze_post_followups_enabled":                      true,
		"responsive_web_jetfuel_frame":                                            true,
		"responsive_web_grok_share_attachment_enabled":                            true,
		"articles_preview_enabled":                                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                true,
		"responsive_web_grok_show_grok_translated_post":                           true,
		"responsive_web_grok_analysis_button_from_backend":                        true,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_grok_image_annotation_enabled":                            true,
		"responsive_web_grok_imagine_annotation_enabled":                          true,
		"responsive_web_grok_community_note_auto_translation_is_enabled":          true,
		"responsive_web_enhance_cards_enabled":                                    false,
	}
}

type timelineInstruction struct {
	Entries []timelineEntry `json:"entries"`
}

type timelineEntry struct {
	EntryID string          `json:"entryId"`
	Content json.RawMessage `json:"content"`
}

type entryContent struct {
	ItemContent *struct {
		TweetResults *struct {
			Result json.RawMessage `json:"result"`
		} `json:"tweet_results"`
	} `json:"itemContent"`
	Items []struct {
		Item struct {
			ItemContent *struct {
				TweetResults *struct {
					Result json.RawMessage `json:"result"`
				} `json:"tweet_results"`
			} `json:"itemContent"`
		} `json:"item"`
	} `json:"items"`
	Value string `json:"value"`
}

// UserTweets fetches tweets from a user's profile timeline.
func (c *Client) UserTweets(opts UserTweetsOptions) (*UserTweetsResult, error) {
	count := opts.Count
	if count <= 0 {
		count = 20
	}
	variables := map[string]any{
		"userId":                                 opts.UserID,
		"count":                                  count,
		"includePromotedContent":                 true,
		"withQuickPromoteEligibilityTweetFields": true,
		"withVoice":                              true,
		"withV2Timeline":                         true,
	}
	if opts.Cursor != "" {
		variables["cursor"] = opts.Cursor
	}

	var resp struct {
		Data struct {
			User struct {
				Result struct {
					Timeline struct {
						Timeline struct {
							Instructions []timelineInstruction `json:"instructions"`
						} `json:"timeline"`
					} `json:"timeline"`
				} `json:"result"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := c.doGraphQL(userTweetsQueryID, "UserTweets", variables, defaultTimelineFeatures(), nil, &resp); err != nil {
		return nil, err
	}
	tweets, next := parseTimelineInstructions(resp.Data.User.Result.Timeline.Timeline.Instructions)
	return &UserTweetsResult{Tweets: tweets, NextCursor: next}, nil
}

func parseTimelineInstructions(instructions []timelineInstruction) ([]TweetSummary, string) {
	var tweets []TweetSummary
	var nextCursor string
	for _, inst := range instructions {
		for _, entry := range inst.Entries {
			if strings.HasPrefix(entry.EntryID, "cursor-bottom") {
				if c := cursorValue(entry.Content); c != "" {
					nextCursor = c
				}
				continue
			}
			if t := tweetFromEntry(entry); t != nil {
				tweets = append(tweets, *t)
			}
		}
	}
	return tweets, nextCursor
}

func tweetFromEntry(entry timelineEntry) *TweetSummary {
	var content entryContent
	if err := json.Unmarshal(entry.Content, &content); err != nil {
		return nil
	}
	if content.ItemContent != nil && content.ItemContent.TweetResults != nil {
		return parseTweetResult(content.ItemContent.TweetResults.Result)
	}
	for _, item := range content.Items {
		if item.Item.ItemContent != nil && item.Item.ItemContent.TweetResults != nil {
			if t := parseTweetResult(item.Item.ItemContent.TweetResults.Result); t != nil {
				return t
			}
		}
	}
	return nil
}

func parseTweetResult(raw json.RawMessage) *TweetSummary {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var node struct {
		Typename string `json:"__typename"`
		Tweet    *struct {
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
				FullText      string `json:"full_text"`
				CreatedAt     string `json:"created_at"`
				FavoriteCount int    `json:"favorite_count"`
				RetweetCount  int    `json:"retweet_count"`
				ReplyCount    int    `json:"reply_count"`
			} `json:"legacy"`
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
			FullText      string `json:"full_text"`
			CreatedAt     string `json:"created_at"`
			FavoriteCount int    `json:"favorite_count"`
			RetweetCount  int    `json:"retweet_count"`
			ReplyCount    int    `json:"reply_count"`
		} `json:"legacy"`
		ExtendedEntities struct {
			Media []any `json:"media"`
		} `json:"extended_entities"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil
	}
	if node.Typename == "TweetTombstone" {
		return nil
	}

	var (
		id, text, created string
		likes, rts, reps  int
		name, screen      string
		hasMedia          bool
	)
	if node.Tweet != nil {
		id = node.Tweet.RestID
		text = node.Tweet.Legacy.FullText
		created = node.Tweet.Legacy.CreatedAt
		likes = node.Tweet.Legacy.FavoriteCount
		rts = node.Tweet.Legacy.RetweetCount
		reps = node.Tweet.Legacy.ReplyCount
		name = node.Tweet.Core.UserResults.Result.Core.Name
		screen = node.Tweet.Core.UserResults.Result.Core.ScreenName
		hasMedia = len(node.Tweet.ExtendedEntities.Media) > 0
	} else {
		id = node.RestID
		text = node.Legacy.FullText
		created = node.Legacy.CreatedAt
		likes = node.Legacy.FavoriteCount
		rts = node.Legacy.RetweetCount
		reps = node.Legacy.ReplyCount
		name = node.Core.UserResults.Result.Core.Name
		screen = node.Core.UserResults.Result.Core.ScreenName
		hasMedia = len(node.ExtendedEntities.Media) > 0
	}
	if id == "" || text == "" {
		return nil
	}
	createdTime, _ := time.Parse("Mon Jan 2 15:04:05 -0700 2006", created)
	return &TweetSummary{
		RestID:      id,
		Text:        text,
		ScreenName:  screen,
		Name:        name,
		CreatedAt:   created,
		CreatedTime: createdTime,
		Likes:       likes,
		Retweets:    rts,
		Replies:     reps,
		HasMedia:    hasMedia,
		URL:         "https://x.com/" + screen + "/status/" + id,
	}
}

func cursorValue(raw json.RawMessage) string {
	var flat struct {
		Value       string `json:"value"`
		ItemContent *struct {
			Value string `json:"value"`
		} `json:"itemContent"`
	}
	if err := json.Unmarshal(raw, &flat); err != nil {
		return ""
	}
	if flat.ItemContent != nil && flat.ItemContent.Value != "" {
		return flat.ItemContent.Value
	}
	return flat.Value
}
