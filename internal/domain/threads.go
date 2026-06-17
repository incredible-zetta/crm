package domain

import "time"

type ThreadsPost struct {
	ID               int64
	ThreadsID        string
	MediaProductType string
	MediaType        string
	Text             string
	Permalink        string
	Timestamp        *time.Time
	Username         string
	TopicTag         string
	IsQuotePost      bool
	RawJSON          []byte
	DeletedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ThreadsReply struct {
	ID         int64
	ReplyID    string
	PostID     string
	ParentID   string
	Text       string
	Username   string
	Timestamp  *time.Time
	HideStatus string
	HasReplies bool
	RawJSON    []byte
	CreatedAt  time.Time
}

type ThreadsMention struct {
	ID        int64
	MentionID string
	Text      string
	Username  string
	Permalink string
	Timestamp *time.Time
	RawJSON   []byte
	CreatedAt time.Time
}

type ThreadsProfile struct {
	ID             string `json:"id,omitempty"`
	Username       string `json:"username,omitempty"`
	Name           string `json:"name,omitempty"`
	PictureURL     string `json:"threads_profile_picture_url,omitempty"`
	Biography      string `json:"threads_biography,omitempty"`
	FollowersCount *int64 `json:"followers_count,omitempty"`
}

// ThreadsPublicProfile is a public profile fetched by username via the Threads
// profile-discovery endpoint (/profile_lookup). Field set differs from the
// authenticated profile: it has engagement counters and follower_count but no
// id, no following count (the API does not expose accounts a user follows).
type ThreadsPublicProfile struct {
	Username      string `json:"username,omitempty"`
	Name          string `json:"name,omitempty"`
	Biography     string `json:"biography,omitempty"`
	PictureURL    string `json:"profile_picture_url,omitempty"`
	IsVerified    *bool  `json:"is_verified,omitempty"`
	FollowerCount *int64 `json:"follower_count,omitempty"`
	LikesCount    *int64 `json:"likes_count,omitempty"`
	QuotesCount   *int64 `json:"quotes_count,omitempty"`
	RepliesCount  *int64 `json:"replies_count,omitempty"`
	RepostsCount  *int64 `json:"reposts_count,omitempty"`
	ViewsCount    *int64 `json:"views_count,omitempty"`
	RawJSON       []byte `json:"-"`
}

// ThreadsDiscoveredPost is a post surfaced by the cookie-only discovery binary
// (x-threads-utils search-posts/viral/latest). It is a separate path from the
// official Graph API: no access token, web-scraped from a logged-in session.
type ThreadsDiscoveredPost struct {
	PK        string `json:"pk"`
	Code      string `json:"code"`
	Caption   string `json:"Caption"`
	LikeCount int    `json:"like_count"`
	TakenAt   int64  `json:"taken_at"`
	User      struct {
		PK       string `json:"pk"`
		Username string `json:"username"`
		FullName string `json:"full_name"`
	} `json:"user"`
}

type ThreadsInsight struct {
	Name     string `json:"name"`
	Value    any    `json:"value,omitempty"`
	RawValue any    `json:"raw_value,omitempty"`
}

type ThreadsAuditEvent struct {
	ID        int64
	Action    string
	ObjectID  string
	OK        bool
	Error     string
	RawJSON   []byte
	CreatedAt time.Time
}

type ThreadsListFilter struct {
	Username string
	Q        string
}
