package domain

import "time"

// InboxCursor tracks the highest IMAP UID processed for a mailbox.
type InboxCursor struct {
	ID              int64
	Mailbox         string
	LastUID         uint32
	LastMessageDate *time.Time
	UpdatedAt       time.Time
}

// InboundMessage is a locally stored inbound email fetched from IMAP.
type InboundMessage struct {
	ID               int64
	Mailbox          string
	UID              uint32
	MessageID        string
	InReplyTo        string
	ReferencesHeader string
	FromEmail        string
	FromName         string
	ToEmail          string
	Subject          string
	ReceivedAt       time.Time
	TextBody         string
	HTMLBody         string
	Snippet          string
	ContactID        *int64
	CampaignID       *int64
	ReadAt           *time.Time
	RepliedAt        *time.Time
	DeletedAt        *time.Time
	NotifiedAt       *time.Time
	RawHeadersJSON   string
	CreatedAt        time.Time
}

// InboxFilter constrains inbound message listing.
type InboxFilter struct {
	UnreadOnly bool
	KnownOnly  bool
	ContactID  *int64
}

// InboxReply describes an outbound reply to an inbound message.
type InboxReply struct {
	InboundID int64
	BodyText  string
	BodyHTML  string
}

// InboxSyncResult summarizes one inbox sync run.
type InboxSyncResult struct {
	Fetched       int
	New           int
	KnownContacts int
	Notified      int
	LastUID       uint32
}
