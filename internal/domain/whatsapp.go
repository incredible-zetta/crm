package domain

import "time"

// WhatsAppStatus is the capability verdict for a contact phone number: whether
// the number is reachable on WhatsApp. It parallels EmailStatus.
type WhatsAppStatus string

const (
	// WhatsAppUnknown means the number has not been checked yet.
	WhatsAppUnknown WhatsAppStatus = "unknown"
	// WhatsAppRegistered means the number is reachable on WhatsApp.
	WhatsAppRegistered WhatsAppStatus = "registered"
	// WhatsAppNotRegistered means the number is not on WhatsApp.
	WhatsAppNotRegistered WhatsAppStatus = "not_registered"
)

// WhatsAppStatuses lists all recognized capability verdicts.
var WhatsAppStatuses = []WhatsAppStatus{WhatsAppUnknown, WhatsAppRegistered, WhatsAppNotRegistered}

// Valid reports whether the status is a recognized value.
func (s WhatsAppStatus) Valid() bool {
	for _, st := range WhatsAppStatuses {
		if s == st {
			return true
		}
	}
	return false
}

// WhatsAppCheck is the result of checking one phone number's WhatsApp presence.
type WhatsAppCheck struct {
	Phone     string
	Status    WhatsAppStatus
	Reason    string
	CheckedAt time.Time
}

// WADirection indicates whether a stored message was sent by us or received.
type WADirection string

const (
	// WAOutbound is a message we sent to a contact.
	WAOutbound WADirection = "out"
	// WAInbound is a message a contact sent to us.
	WAInbound WADirection = "in"
)

// WAMessageStatus tracks the delivery lifecycle of an outbound WhatsApp message.
// Inbound messages are always stored with WAStatusReceived.
type WAMessageStatus string

const (
	// WAStatusSent means the gateway accepted the message for delivery.
	WAStatusSent WAMessageStatus = "sent"
	// WAStatusDelivered means the recipient device received the message.
	WAStatusDelivered WAMessageStatus = "delivered"
	// WAStatusRead means the recipient read the message.
	WAStatusRead WAMessageStatus = "read"
	// WAStatusFailed means delivery failed.
	WAStatusFailed WAMessageStatus = "failed"
	// WAStatusReceived marks a stored inbound message.
	WAStatusReceived WAMessageStatus = "received"
)

// WAMediaType classifies attached media on a WhatsApp message.
type WAMediaType string

const (
	// WAMediaNone indicates a text-only message.
	WAMediaNone WAMediaType = ""
	// WAMediaImage is an image attachment.
	WAMediaImage WAMediaType = "image"
	// WAMediaVideo is a video attachment.
	WAMediaVideo WAMediaType = "video"
	// WAMediaAudio is an audio/voice attachment.
	WAMediaAudio WAMediaType = "audio"
	// WAMediaDocument is a document/file attachment.
	WAMediaDocument WAMediaType = "document"
	// WAMediaSticker is a sticker.
	WAMediaSticker WAMediaType = "sticker"
)

// WAMessage is a locally stored WhatsApp message (inbound or outbound).
//
// MessageID is the gateway-assigned WhatsApp message id, used to correlate
// later message.ack receipts (delivered/read) with the outbound row.
type WAMessage struct {
	ID           int64
	MessageID    string // gateway WhatsApp message id (wamid)
	ChatID       string // e.g. 628xxx@s.whatsapp.net
	Direction    WADirection
	Phone        string // normalized E.164 (no +), e.g. 628xxx
	SenderName   string // webhook from_name (push name); inbound only
	ContactID    *int64 // set when matched to a known contact
	Body         string // WhatsApp-formatted text (or caption)
	MediaType    WAMediaType
	MediaURL     string // gateway-served URL when media present
	MediaCaption string
	Status       WAMessageStatus
	Error        string // failure reason when Status=failed
	RepliedTo    string // wamid this message replies to (inbound quote / outbound reply)
	SentAt       *time.Time
	DeliveredAt  *time.Time
	ReadAt       *time.Time
	ReceivedAt   *time.Time // inbound receipt time
	NotifiedAt   *time.Time // admin-notified time (inbound, known contact)
	RepliedAt    *time.Time // when we replied to this inbound message
	DeletedAt    *time.Time
	CreatedAt    time.Time
}

// WAInboundFilter constrains WhatsApp message listing.
type WAInboundFilter struct {
	Direction  string // "in" | "out" | "" (all)
	UnreadOnly bool
	KnownOnly  bool
	ContactID  *int64
	Phone      string
	ChatID     string
	Since      *time.Time // created_at >= Since
	Until      *time.Time // created_at <= Until
}

// WAListener configures AI-visible listening for a WhatsApp chat/group JID.
type WAListener struct {
	ID        int64
	ChatJID   string
	Name      string
	Enabled   bool
	Summary   string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WASendResult summarizes one outbound send.
type WASendResult struct {
	MessageID string
	Status    WAMessageStatus
	StoredID  int64
}

// WAReceiptEvent is a normalized delivered/read receipt parsed from a
// message.ack webhook.
type WAReceiptEvent struct {
	MessageIDs  []string
	ChatID      string
	From        string
	ReceiptType string // "delivered" | "read" | "read-self"
	Timestamp   time.Time
}

// WAInboundEvent is a normalized inbound message parsed from a message webhook.
type WAInboundEvent struct {
	MessageID    string
	ChatID       string
	From         string // sender JID, e.g. 628xxx@s.whatsapp.net
	FromName     string
	IsFromMe     bool
	Body         string
	MediaType    WAMediaType
	MediaURL     string
	MediaCaption string
	RepliedTo    string
	Timestamp    time.Time
}

// WAAuditResult summarizes a batch WhatsApp capability audit.
type WAAuditResult struct {
	Checked       int
	Registered    int
	NotRegistered int
	Unknown       int
	NextCursor    int64
}
