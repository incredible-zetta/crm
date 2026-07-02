package xclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// createTweetQueryID is the persisted-query hash for CreateTweet, extracted
// from the web client bundle. Update from a fresh capture if the API returns a
// "query not found" error after an x.com deploy.
const createTweetQueryID = "R5EPiGHgSqbTYFyozd-gFw"

// PostResult holds the outcome of a successful CreateTweet call.
type PostResult struct {
	RestID     string
	ScreenName string
}

// TweetOptions configures a tweet. MediaIDs are the strings returned by
// UploadMedia. Mentions and hashtags are just part of the text — x.com parses
// entities server-side, so include them inline (e.g. "hi @user #tag").
// ReplyTo sets an in-reply-to tweet id; QuoteOf quotes a tweet (id or URL).
type TweetOptions struct {
	Text     string
	MediaIDs []string
	ReplyTo  string
	QuoteOf  string
}

// defaultCreateTweetFeatures mirrors the feature flags the web client sends for
// CreateTweet. The API rejects requests missing required flags.
func defaultCreateTweetFeatures() map[string]any {
	return map[string]any{
		"premium_content_api_read_enabled":                                        false,
		"communities_web_enable_tweet_community_results_fetch":                    true,
		"c9s_tweet_anatomy_moderator_badge_enabled":                               true,
		"responsive_web_grok_analyze_button_fetch_trends_enabled":                 false,
		"responsive_web_grok_analyze_post_followups_enabled":                      true,
		"responsive_web_jetfuel_frame":                                            true,
		"responsive_web_grok_share_attachment_enabled":                            true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                true,
		"tweet_awards_web_tipping_enabled":                                        false,
		"responsive_web_grok_show_grok_translated_post":                           true,
		"responsive_web_grok_analysis_button_from_backend":                        true,
		"creator_subscriptions_quote_tweet_preview_enabled":                       false,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"profile_label_improvements_pcf_label_in_post_enabled":                    true,
		"rweb_cashtags_composer_attachment_enabled":                               true,
		"verified_phone_label_enabled":                                            false,
		"articles_preview_enabled":                                                true,
		"rweb_video_screen_enabled":                                               false,
		"responsive_web_grok_community_note_auto_translation_is_enabled":          true,
		"responsive_web_grok_image_annotation_enabled":                            true,
		"responsive_web_grok_imagine_annotation_enabled":                          true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_enhance_cards_enabled":                                    false,
	}
}

// createTweetResp models the fields we extract from the CreateTweet response.
type createTweetResp struct {
	Data struct {
		CreateTweet struct {
			TweetResults struct {
				Result struct {
					RestID string `json:"rest_id"`
					Core   struct {
						UserResults struct {
							Result struct {
								Core struct {
									ScreenName string `json:"screen_name"`
								} `json:"core"`
							} `json:"result"`
						} `json:"user_results"`
					} `json:"core"`
				} `json:"result"`
			} `json:"tweet_results"`
		} `json:"create_tweet"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

// PostTweet creates a tweet with optional media, reply, or quote.
func (c *Client) PostTweet(opts TweetOptions) (*PostResult, error) {
	variables := map[string]any{
		"tweet_text":               opts.Text,
		"dark_request":             false,
		"semantic_annotation_ids":  []any{},
		"disallowed_reply_options": nil,
	}
	if opts.ReplyTo != "" {
		variables["reply"] = map[string]any{
			"in_reply_to_tweet_id":   opts.ReplyTo,
			"exclude_reply_user_ids": []any{},
		}
	}
	if opts.QuoteOf != "" {
		variables["attachment_url"] = quoteAttachmentURL(opts.QuoteOf)
	}
	if len(opts.MediaIDs) > 0 {
		entities := make([]map[string]any, 0, len(opts.MediaIDs))
		for _, id := range opts.MediaIDs {
			entities = append(entities, map[string]any{
				"media_id":     id,
				"tagged_users": []any{},
			})
		}
		variables["media"] = map[string]any{
			"media_entities":     entities,
			"possibly_sensitive": false,
		}
	}

	payload := map[string]any{
		"variables": variables,
		"features":  defaultCreateTweetFeatures(),
		"queryId":   createTweetQueryID,
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf("%s/%s/CreateTweet", apiBase, createTweetQueryID)
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)
	referer := "https://x.com/home"
	if opts.ReplyTo != "" {
		referer = "https://x.com/i/status/" + opts.ReplyTo
	} else if opts.QuoteOf != "" {
		referer = quoteAttachmentURL(opts.QuoteOf)
	}
	req.Header.Set("referer", referer)
	if err := c.ensureTxn(); err != nil {
		return nil, fmt.Errorf("transaction id: %w", err)
	}
	c.setTransactionID(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var out createTweetResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(out.Errors) > 0 {
		return nil, out.Errors[0]
	}
	r := out.Data.CreateTweet.TweetResults.Result
	if r.RestID == "" {
		return nil, fmt.Errorf("no tweet id in response: %s", truncate(string(body), 300))
	}
	return &PostResult{
		RestID:     r.RestID,
		ScreenName: r.Core.UserResults.Result.Core.ScreenName,
	}, nil
}

func quoteAttachmentURL(idOrURL string) string {
	if strings.HasPrefix(idOrURL, "http://") || strings.HasPrefix(idOrURL, "https://") {
		return idOrURL
	}
	return "https://x.com/i/status/" + idOrURL
}
