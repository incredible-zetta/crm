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
	ID         string `json:"id,omitempty"`
	Username   string `json:"username,omitempty"`
	Name       string `json:"name,omitempty"`
	PictureURL string `json:"threads_profile_picture_url,omitempty"`
	Biography  string `json:"threads_biography,omitempty"`
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
