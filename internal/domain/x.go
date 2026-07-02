package domain

import "time"

// XUser is a public x.com profile.
type XUser struct {
	RestID       string `json:"rest_id"`
	Name         string `json:"name"`
	ScreenName   string `json:"screen_name"`
	CreatedAt    string `json:"created_at,omitempty"`
	Followers    int    `json:"followers"`
	Following    int    `json:"following"`
	Tweets       int    `json:"tweets"`
	Verified     bool   `json:"verified"`
	BlueVerified bool   `json:"blue_verified"`
}

// XTweet is a trimmed tweet from search/timeline output.
type XTweet struct {
	RestID      string    `json:"rest_id"`
	Text        string    `json:"text"`
	ScreenName  string    `json:"screen_name"`
	Name        string    `json:"name,omitempty"`
	CreatedAt   string    `json:"created_at,omitempty"`
	CreatedTime time.Time `json:"-"`
	Likes       int       `json:"likes"`
	Retweets    int       `json:"retweets"`
	Replies     int       `json:"replies"`
	HasMedia    bool      `json:"has_media"`
	URL         string    `json:"url"`
}

// XTweetDetail adds full engagement analytics to a tweet.
type XTweetDetail struct {
	XTweet
	Views          int    `json:"views"`
	ViewState      string `json:"view_state,omitempty"`
	Quotes         int    `json:"quotes"`
	Bookmarks      int    `json:"bookmarks"`
	ConversationID string `json:"conversation_id,omitempty"`
}

// XTweetPage is a page of tweets with an optional cursor.
type XTweetPage struct {
	Tweets     []XTweet
	NextCursor string
}

// XUserPage is a page of users with an optional cursor.
type XUserPage struct {
	Users      []XUser
	NextCursor string
}

// XPostResult is the outcome of a create-tweet call.
type XPostResult struct {
	RestID     string `json:"rest_id"`
	ScreenName string `json:"screen_name,omitempty"`
	URL        string `json:"url,omitempty"`
}

// XDMResult is the outcome of a send-DM call.
type XDMResult struct {
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
}
