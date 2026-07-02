package xclient

// User is a trimmed representation of the UserByScreenName GraphQL response.
type User struct {
	RestID       string
	Name         string
	ScreenName   string
	CreatedAt    string
	Followers    int
	Following    int
	Tweets       int
	Verified     bool
	BlueVerified bool
}

// defaultUserFeatures mirrors the feature flags the web client sends for
// UserByScreenName. The API rejects requests missing required flags.
func defaultUserFeatures() map[string]any {
	return map[string]any{
		"hidden_profile_subscriptions_enabled":                              true,
		"profile_label_improvements_pcf_label_in_post_enabled":              true,
		"responsive_web_profile_redirect_enabled":                           false,
		"rweb_tipjar_consumption_enabled":                                   false,
		"verified_phone_label_enabled":                                      false,
		"subscriptions_verification_info_is_identity_verified_enabled":      true,
		"subscriptions_verification_info_verified_since_enabled":            true,
		"highlights_tweets_tab_ui_enabled":                                  true,
		"responsive_web_twitter_article_notes_tab_enabled":                  true,
		"subscriptions_feature_can_gift_premium":                            true,
		"creator_subscriptions_tweet_preview_api_enabled":                   true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled": false,
		"responsive_web_graphql_timeline_navigation_enabled":                true,
	}
}

// userByScreenNameQueryID is the persisted-query hash observed on the web app.
// It changes when x.com ships a new build; update from a fresh capture if the
// API returns a "query not found" error.
const userByScreenNameQueryID = "2qvSHpkWTMS9i0zJAwDNiA"

// userByRestIdQueryID is the persisted-query hash for UserByRestId (lookup by
// numeric user id). Used to resolve the acting account's own profile from the
// twid cookie without knowing the handle. Update from a fresh capture if the
// API returns a "query not found" error.
const userByRestIdQueryID = "DaeC_2LfMgwCujE03HSZtw"

// userByScreenNameResp models the nested JSON we care about.
type userByScreenNameResp struct {
	Data struct {
		User struct {
			Result struct {
				RestID string `json:"rest_id"`
				Core   struct {
					Name       string `json:"name"`
					ScreenName string `json:"screen_name"`
					CreatedAt  string `json:"created_at"`
				} `json:"core"`
				IsBlueVerified bool `json:"is_blue_verified"`
				Verification   struct {
					Verified bool `json:"verified"`
				} `json:"verification"`
				Legacy struct {
					FollowersCount int `json:"followers_count"`
					FriendsCount   int `json:"friends_count"`
					StatusesCount  int `json:"statuses_count"`
				} `json:"legacy"`
			} `json:"result"`
		} `json:"user"`
	} `json:"data"`
}

// UserByScreenName fetches a user's public profile by @handle.
func (c *Client) UserByScreenName(screenName string) (*User, error) {
	variables := map[string]any{
		"screen_name":           screenName,
		"withGrokTranslatedBio": true,
	}
	fieldToggles := map[string]any{
		"withPayments":            false,
		"withAuxiliaryUserLabels": true,
	}
	var resp userByScreenNameResp
	if err := c.doGraphQL(userByScreenNameQueryID, "UserByScreenName", variables, defaultUserFeatures(), fieldToggles, &resp); err != nil {
		return nil, err
	}
	r := resp.Data.User.Result
	return &User{
		RestID:       r.RestID,
		Name:         r.Core.Name,
		ScreenName:   r.Core.ScreenName,
		CreatedAt:    r.Core.CreatedAt,
		Followers:    r.Legacy.FollowersCount,
		Following:    r.Legacy.FriendsCount,
		Tweets:       r.Legacy.StatusesCount,
		Verified:     r.Verification.Verified,
		BlueVerified: r.IsBlueVerified,
	}, nil
}

// userByRestIdResp models the UserByRestId JSON (same result shape as
// UserByScreenName, nested under data.user instead of data.user).
type userByRestIdResp struct {
	Data struct {
		User struct {
			Result struct {
				RestID string `json:"rest_id"`
				Core   struct {
					Name       string `json:"name"`
					ScreenName string `json:"screen_name"`
					CreatedAt  string `json:"created_at"`
				} `json:"core"`
				IsBlueVerified bool `json:"is_blue_verified"`
				Verification   struct {
					Verified bool `json:"verified"`
				} `json:"verification"`
				Legacy struct {
					ScreenName     string `json:"screen_name"`
					Name           string `json:"name"`
					FollowersCount int    `json:"followers_count"`
					FriendsCount   int    `json:"friends_count"`
					StatusesCount  int    `json:"statuses_count"`
				} `json:"legacy"`
			} `json:"result"`
		} `json:"user"`
	} `json:"data"`
}

// UserByRestId fetches a user's profile by numeric user id. Unlike
// UserByScreenName it needs no handle, so it resolves the acting account's own
// profile from the twid cookie (MeUserID). Newer builds put name/screen_name
// under core; older ones under legacy — fall back so either works.
func (c *Client) UserByRestId(userID string) (*User, error) {
	variables := map[string]any{
		"userId":                   userID,
		"withSafetyModeUserFields": true,
	}
	var resp userByRestIdResp
	if err := c.doGraphQL(userByRestIdQueryID, "UserByRestId", variables, defaultUserFeatures(), nil, &resp); err != nil {
		return nil, err
	}
	r := resp.Data.User.Result
	name := r.Core.Name
	if name == "" {
		name = r.Legacy.Name
	}
	screen := r.Core.ScreenName
	if screen == "" {
		screen = r.Legacy.ScreenName
	}
	return &User{
		RestID:       r.RestID,
		Name:         name,
		ScreenName:   screen,
		CreatedAt:    r.Core.CreatedAt,
		Followers:    r.Legacy.FollowersCount,
		Following:    r.Legacy.FriendsCount,
		Tweets:       r.Legacy.StatusesCount,
		Verified:     r.Verification.Verified,
		BlueVerified: r.IsBlueVerified,
	}, nil
}
