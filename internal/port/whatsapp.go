package port

import (
	"context"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
)

// WhatsAppGateway is the outbound interface to a WhatsApp gateway.
// Implementations handle phone normalization, authentication, and API calls.
type WhatsAppGateway interface {
	// Send sends a text message to a phone number. Returns the gateway-assigned
	// message ID and status.
	Send(ctx context.Context, msg WhatsAppMessage) (WhatsAppSendResult, error)

	// Check verifies whether a phone number is registered on WhatsApp.
	Check(ctx context.Context, phone string) (domain.WhatsAppCheck, error)

	// MarkRead marks an outbound message as read by the recipient.
	// phone is the normalized phone number (used for gateway API routing).
	MarkRead(ctx context.Context, messageID, phone string) error

	// DownloadMedia retrieves the media URL for a message with an attachment.
	// Returns a WhatsAppMedia descriptor with the temporary URL.
	DownloadMedia(ctx context.Context, messageID, phone string) (WhatsAppMedia, error)

	// ListGroups returns groups joined by the authenticated device.
	ListGroups(ctx context.Context) ([]WhatsAppGroup, error)

	// ListContacts returns WhatsApp contacts known by the authenticated device.
	ListContacts(ctx context.Context) ([]WhatsAppContact, error)

	// JoinGroup joins a group via invite link. Returns the joined group JID.
	JoinGroup(ctx context.Context, inviteLink string) (string, error)

	// LeaveGroup leaves a group by JID.
	LeaveGroup(ctx context.Context, groupJID string) error

	// GroupInfoFromLink fetches group metadata from an invite link without joining.
	GroupInfoFromLink(ctx context.Context, inviteLink string) (WhatsAppGroup, error)

	// SendMedia sends image/video/file media by URL or local file path.
	SendMedia(ctx context.Context, msg WhatsAppMediaMessage) (WhatsAppSendResult, error)
}

// WhatsAppMessage is the outbound payload for sending a WhatsApp text.
type WhatsAppMessage struct {
	Phone     string
	Body      string // WhatsApp-formatted markdown
	ReplyToID string // optional: gateway message ID to quote-reply to
}

// WhatsAppGroup is a joined WhatsApp group descriptor.
type WhatsAppGroup struct {
	JID         string
	Name        string
	Topic       string
	Participant int
}

// WhatsAppContact is a WhatsApp contact from gateway storage.
type WhatsAppContact struct {
	JID  string
	Name string
}

// WhatsAppMediaMessage is outbound media payload for image/video/file endpoints.
type WhatsAppMediaMessage struct {
	Phone     string
	Kind      string // image | video | file
	URL       string // image_url/video_url
	FilePath  string // multipart file path
	Caption   string
	ReplyToID string
}

// WhatsAppMedia is the result of a media download request.
type WhatsAppMedia struct {
	URL      string // temporary download URL
	FilePath string // local path if gateway auto-downloaded
	MimeType string // MIME type of the media
	Status   string // gateway status message
}

// WhatsAppSendResult is the gateway's response to a send operation.
type WhatsAppSendResult struct {
	MessageID string
	Status    string
}

// WhatsAppParticipant is a group member.
type WhatsAppParticipant struct {
	JID          string
	Phone        string
	LID          string
	DisplayName  string
	IsAdmin      bool
	IsSuperAdmin bool
}

// WhatsAppParticipantResult is the per-participant outcome of a manage action.
type WhatsAppParticipantResult struct {
	Participant string
	Status      string
	Message     string
}

// WhatsAppGroupInfo is detailed group metadata.
type WhatsAppGroupInfo struct {
	JID              string
	Name             string
	Topic            string
	OwnerJID         string
	ParticipantCount int
	IsLocked         bool
	IsAnnounce       bool
	Participants     []WhatsAppParticipant
}

// WhatsAppDevice is a linked device entry.
type WhatsAppDevice struct {
	Name string
	JID  string
}

// WhatsAppConnection is the gateway connection/login status.
type WhatsAppConnection struct {
	Connected bool
	LoggedIn  bool
	DeviceID  string
	JID       string
}

// WhatsAppUserInfo is a looked-up WhatsApp user profile.
type WhatsAppUserInfo struct {
	VerifiedName string
	Status       string
	PictureID    string
	Devices      []string
}

// WhatsAppManageGateway is the optional extended gateway surface for group and
// account management. The real HTTP Client implements it; test fakes need not.
type WhatsAppManageGateway interface {
	CreateGroup(ctx context.Context, title string, participants []string) (string, error)
	GroupInfo(ctx context.Context, groupJID string) (WhatsAppGroupInfo, error)
	GroupParticipants(ctx context.Context, groupJID string) ([]WhatsAppParticipant, error)
	ManageParticipants(ctx context.Context, groupJID, action string, participants []string) ([]WhatsAppParticipantResult, error)
	GroupParticipantRequests(ctx context.Context, groupJID string) ([]WhatsAppParticipant, error)
	ReviewParticipantRequests(ctx context.Context, groupJID, action string, participants []string) error
	SetGroupName(ctx context.Context, groupJID, name string) error
	SetGroupTopic(ctx context.Context, groupJID, topic string) error
	SetGroupLocked(ctx context.Context, groupJID string, locked bool) error
	SetGroupAnnounce(ctx context.Context, groupJID string, announce bool) error
	GroupInviteLink(ctx context.Context, groupJID string, reset bool) (string, error)
	ConnectionStatus(ctx context.Context) (WhatsAppConnection, error)
	ListDevices(ctx context.Context) ([]WhatsAppDevice, error)
	Logout(ctx context.Context) error
	Reconnect(ctx context.Context) error
	UserInfo(ctx context.Context, phone string) (WhatsAppUserInfo, error)
	SetPushName(ctx context.Context, name string) error
}

// WAMessageRepo persists WhatsApp messages (both inbound and outbound).
type WAMessageRepo interface {
	// Insert stores a message. If messageID already exists, returns the existing
	// row and isNew=false (idempotent).
	Insert(ctx context.Context, msg domain.WAMessage) (domain.WAMessage, bool, error)

	// Get retrieves a message by local ID.
	Get(ctx context.Context, id int64) (domain.WAMessage, error)

	// List returns a page of messages matching the filter.
	List(ctx context.Context, f domain.WAInboundFilter, p Paging) (WAMessagePage, error)

	// UpdateStatus updates the delivery/read status of a message by gateway ID.
	UpdateStatus(ctx context.Context, messageID string, status domain.WAMessageStatus, at time.Time) error

	// MarkRead sets the read_at timestamp for an inbound message.
	MarkRead(ctx context.Context, id int64, at *time.Time) error

	// MarkReplied sets the replied_at timestamp for an inbound message.
	MarkReplied(ctx context.Context, id int64, at time.Time) error

	// MarkNotified sets the notified_at timestamp for an inbound message.
	MarkNotified(ctx context.Context, id int64, at time.Time) error

	// SetRepliedTo links an outbound message to the inbound message it replies to.
	SetRepliedTo(ctx context.Context, outboundID int64, inboundMessageID string) error

	// SoftDelete marks a message as deleted.
	SoftDelete(ctx context.Context, id int64) error

	// CountSentSince counts outbound messages to a specific phone since a given time.
	CountSentSince(ctx context.Context, phone string, since time.Time) (int, error)

	// CountSentSinceAll counts all outbound messages since a given time.
	CountSentSinceAll(ctx context.Context, since time.Time) (int, error)
}

// WAMessagePage is a cursor-paged result set for WhatsApp messages.
type WAMessagePage struct {
	Items      []domain.WAMessage
	Total      int
	NextCursor int64
}

// WAListenerRepo persists WhatsApp listener configs.
type WAListenerRepo interface {
	Create(ctx context.Context, l domain.WAListener) (domain.WAListener, error)
	Get(ctx context.Context, id int64) (domain.WAListener, error)
	GetByChatJID(ctx context.Context, chatJID string) (domain.WAListener, error)
	List(ctx context.Context, enabledOnly bool) ([]domain.WAListener, error)
	Update(ctx context.Context, id int64, l domain.WAListener) (domain.WAListener, error)
	SoftDelete(ctx context.Context, id int64) error
	SetSummary(ctx context.Context, id int64, summary string) error
}

// SmartSendPolicy configures rate-limiting and safety checks for outbound sends.
type SmartSendPolicy struct {
	// BlockNotRegistered rejects sends to contacts verified as not on WhatsApp.
	BlockNotRegistered bool

	// MaxPerSecond is the maximum outbound messages per second (token bucket).
	// Zero or negative disables rate-limiting.
	MaxPerSecond float64
}
