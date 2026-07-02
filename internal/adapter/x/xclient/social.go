package xclient

import (
	"encoding/json"
	"strings"
)

const (
	followersQueryID = "4yeuNabfz3qFlfncCAy8Yw"
	followingQueryID = "eNoXdfXv5rU75RBzlmfuPA"
)

// UserSummary is a trimmed user for followers/following lists.
type UserSummary struct {
	RestID     string
	Name       string
	ScreenName string
	Followers  int
	Following  int
	Tweets     int
	Verified   bool
}

// SocialListOptions configures a followers or following fetch.
type SocialListOptions struct {
	UserID string
	Count  int
	Cursor string
}

// SocialListResult holds users and an optional next-page cursor.
type SocialListResult struct {
	Users      []UserSummary
	NextCursor string
}

// Followers returns a page of users following the given user id.
func (c *Client) Followers(opts SocialListOptions) (*SocialListResult, error) {
	return c.socialList(followersQueryID, "Followers", "followers", opts)
}

// Following returns a page of users the given user id follows.
func (c *Client) Following(opts SocialListOptions) (*SocialListResult, error) {
	return c.socialList(followingQueryID, "Following", "following", opts)
}

func (c *Client) socialList(queryID, opName, timelineKey string, opts SocialListOptions) (*SocialListResult, error) {
	count := opts.Count
	if count <= 0 {
		count = 20
	}
	variables := map[string]any{
		"userId":                 opts.UserID,
		"count":                  count,
		"includePromotedContent": false,
	}
	if opts.Cursor != "" {
		variables["cursor"] = opts.Cursor
	}

	body, err := c.doGraphQLRaw(queryID, opName, variables, defaultTimelineFeatures(), nil)
	if err != nil {
		return nil, err
	}

	users, next := parseSocialList(body, timelineKey)
	return &SocialListResult{Users: users, NextCursor: next}, nil
}

func (c *Client) doGraphQLRaw(queryID, opName string, variables, features, fieldToggles map[string]any) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.doGraphQL(queryID, opName, variables, features, fieldToggles, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func parseSocialList(body json.RawMessage, timelineKey string) ([]UserSummary, string) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, ""
	}
	instructions := findSocialInstructions(root, timelineKey)
	if len(instructions) == 0 {
		instructions = findAnyUserInstructions(root)
	}
	var users []UserSummary
	var nextCursor string
	for _, inst := range instructions {
		entries, _ := inst["entries"].([]any)
		for _, e := range entries {
			entry, _ := e.(map[string]any)
			if entry == nil {
				continue
			}
			entryID, _ := entry["entryId"].(string)
			if strings.HasPrefix(entryID, "cursor-bottom") {
				if c := cursorFromMap(entry); c != "" {
					nextCursor = c
				}
				continue
			}
			if !strings.HasPrefix(entryID, "user") {
				continue
			}
			if u := userFromEntryMap(entry); u != nil {
				users = append(users, *u)
			}
		}
	}
	return users, nextCursor
}

func findSocialInstructions(root map[string]any, timelineKey string) []map[string]any {
	data, _ := root["data"].(map[string]any)
	user, _ := data["user"].(map[string]any)
	result, _ := user["result"].(map[string]any)
	timeline, _ := result[timelineKey].(map[string]any)
	if timeline == nil {
		timeline, _ = result[timelineKey+"_timeline"].(map[string]any)
	}
	inner, _ := timeline["timeline"].(map[string]any)
	inner2, _ := inner["timeline"].(map[string]any)
	raw, _ := inner2["instructions"].([]any)
	var out []map[string]any
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// findAnyUserInstructions walks the response tree and returns the first
// instructions list containing user-* entries (handles API shape changes).
func findAnyUserInstructions(v any) []map[string]any {
	switch node := v.(type) {
	case map[string]any:
		if entries, ok := node["entries"].([]any); ok && hasUserEntry(entries) {
			return []map[string]any{node}
		}
		for _, child := range node {
			if found := findAnyUserInstructions(child); len(found) > 0 {
				return found
			}
		}
	case []any:
		for _, child := range node {
			if found := findAnyUserInstructions(child); len(found) > 0 {
				return found
			}
		}
	}
	return nil
}

func hasUserEntry(entries []any) bool {
	for _, e := range entries {
		entry, _ := e.(map[string]any)
		if entry == nil {
			continue
		}
		if id, _ := entry["entryId"].(string); strings.HasPrefix(id, "user") {
			return true
		}
	}
	return false
}

func cursorFromMap(entry map[string]any) string {
	content, _ := entry["content"].(map[string]any)
	if content == nil {
		return ""
	}
	if v, ok := content["value"].(string); ok {
		return v
	}
	item, _ := content["itemContent"].(map[string]any)
	if item != nil {
		if v, ok := item["value"].(string); ok {
			return v
		}
	}
	return ""
}

func userFromEntryMap(entry map[string]any) *UserSummary {
	content, _ := entry["content"].(map[string]any)
	item, _ := content["itemContent"].(map[string]any)
	userResults, _ := item["user_results"].(map[string]any)
	result, _ := userResults["result"].(map[string]any)
	if result == nil || result["__typename"] == "UserUnavailable" {
		return nil
	}
	core, _ := result["core"].(map[string]any)
	legacy, _ := result["legacy"].(map[string]any)
	verification, _ := result["verification"].(map[string]any)
	if core == nil {
		return nil
	}
	screen, _ := core["screen_name"].(string)
	name, _ := core["name"].(string)
	id, _ := result["rest_id"].(string)
	followers, _ := legacy["followers_count"].(float64)
	following, _ := legacy["friends_count"].(float64)
	tweets, _ := legacy["statuses_count"].(float64)
	verified, _ := verification["verified"].(bool)
	if screen == "" {
		return nil
	}
	return &UserSummary{
		RestID:     id,
		Name:       name,
		ScreenName: screen,
		Followers:  int(followers),
		Following:  int(following),
		Tweets:     int(tweets),
		Verified:   verified,
	}
}
