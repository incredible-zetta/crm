package port

import (
	"context"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
)

// Paging is a reusable cursor-paging request.
type Paging struct {
	Limit  int   // service caps/defaults
	Cursor int64 // 0 = first page
}

// ContactPage is a generic paged result.
//
// Total is the grand total of rows matching the filter (ignoring the cursor),
// suitable for "showing 1-20 of 137" displays. NextCursor is 0 when there is
// no further page.
type ContactPage struct {
	Items      []domain.Contact
	Total      int
	NextCursor int64
}

// InboxPage is a cursor-paged inbound message result.
type InboxPage struct {
	Items      []domain.InboundMessage
	Total      int
	NextCursor int64
}

// ContactRepo defines the database operations for contacts.
// InboxRepo defines operations for storing inbound email messages and IMAP cursors.
type InboxRepo interface {
	GetCursor(ctx context.Context, mailbox string) (domain.InboxCursor, error)
	UpsertCursor(ctx context.Context, cursor domain.InboxCursor) error
	InsertMessage(ctx context.Context, msg domain.InboundMessage) (domain.InboundMessage, bool, error)
	GetMessage(ctx context.Context, id int64) (domain.InboundMessage, error)
	ListMessages(ctx context.Context, f domain.InboxFilter, p Paging) (InboxPage, error)
	MarkRead(ctx context.Context, id int64, at *time.Time) error
	MarkReplied(ctx context.Context, id int64, at time.Time) error
	SoftDeleteMessage(ctx context.Context, id int64) error
	MarkNotified(ctx context.Context, id int64, at time.Time) error
	ListUnnotifiedKnown(ctx context.Context, limit int) ([]domain.InboundMessage, error)
}

// InboxFetcher fetches inbound email messages from a mailbox.
type InboxFetcher interface {
	FetchNew(ctx context.Context, cursor domain.InboxCursor, limit int) ([]domain.InboundMessage, uint32, error)
}

// AdminNotifier notifies an admin about CRM events.
type AdminNotifier interface {
	NotifyInboundMessage(ctx context.Context, msg domain.InboundMessage, contact domain.Contact) error
}

type ContactRepo interface {
	// Upsert inserts or updates a contact.
	Upsert(ctx context.Context, c domain.Contact) (domain.Contact, error)
	// Get retrieves a contact by ID. Excludes soft-deleted.
	Get(ctx context.Context, id int64) (domain.Contact, error)
	// GetByEmail retrieves a contact by email. Excludes soft-deleted.
	GetByEmail(ctx context.Context, email string) (domain.Contact, error)
	// GetByUnsubCode retrieves a contact by its unique opt-out/unsubscribe token.
	GetByUnsubCode(ctx context.Context, code string) (domain.Contact, error)
	// Update updates a contact.
	Update(ctx context.Context, id int64, patch domain.ContactPatch) (domain.Contact, error)
	// List returns a page of contacts based on filter. Excludes soft-deleted.
	List(ctx context.Context, f domain.ContactFilter, p Paging) (ContactPage, error)
	// CountByStage returns contact counts grouped by sales pipeline stage. Excludes soft-deleted.
	CountByStage(ctx context.Context) (map[string]int, error)
	// SoftDelete marks a contact as deleted (sets deleted_at).
	SoftDelete(ctx context.Context, id int64) error
	// Purge hard deletes a contact from the database (GDPR requirement).
	Purge(ctx context.Context, id int64) error
	// SetUnsubscribed sets the unsubscribe timestamp for a contact.
	SetUnsubscribed(ctx context.Context, id int64, at time.Time) error
	// SetUnsubCode assigns a unique unsubscribe opt-out token to a contact.
	SetUnsubCode(ctx context.Context, id int64, code string) error
}

// CampaignRepo defines the database operations for campaigns.
type CampaignRepo interface {
	// Create persists a new campaign.
	Create(ctx context.Context, c domain.Campaign) (domain.Campaign, error)
	// Get retrieves a campaign by ID. Excludes soft-deleted.
	Get(ctx context.Context, id int64) (domain.Campaign, error)
	// List retrieves all campaigns. Excludes soft-deleted.
	List(ctx context.Context) ([]domain.Campaign, error)
	// UpdateStatus updates only the status of a campaign.
	UpdateStatus(ctx context.Context, id int64, status domain.CampaignStatus) error
	// Update updates editable fields of a campaign (name/template/segment/provider/scheduled_at).
	Update(ctx context.Context, id int64, c domain.Campaign) (domain.Campaign, error)
	// SetStats updates campaign performance statistics.
	SetStats(ctx context.Context, id int64, stats map[string]any) error
	// SoftDelete marks a campaign as deleted (sets deleted_at).
	SoftDelete(ctx context.Context, id int64) error
}

// TemplateRepo defines the database operations for email templates.
type TemplateRepo interface {
	// Create persists a new email template.
	Create(ctx context.Context, t domain.Template) (domain.Template, error)
	// Get retrieves a template by ID. Excludes soft-deleted.
	Get(ctx context.Context, id int64) (domain.Template, error)
	// GetByName retrieves a template by name.
	GetByName(ctx context.Context, name string) (domain.Template, error)
	// List retrieves all templates. Excludes soft-deleted.
	List(ctx context.Context) ([]domain.Template, error)
	// Update updates an existing template.
	Update(ctx context.Context, id int64, t domain.Template) (domain.Template, error)
	// SoftDelete marks a template as deleted (sets deleted_at).
	SoftDelete(ctx context.Context, id int64) error
}

// TaskRepo defines operations for scheduled background tasks.
type TaskRepo interface {
	// Insert schedules a new background task.
	Insert(ctx context.Context, t domain.ScheduledTask) (int64, error)
	// List lists tasks filtered by status. If status is empty, it lists all (non-terminal first).
	// Tasks do not have soft delete.
	List(ctx context.Context, status string, limit int) ([]domain.ScheduledTask, error)
	// ClaimDue atomically claims pending tasks that are due for execution.
	ClaimDue(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledTask, error)
	// MarkDone marks a task as successfully completed.
	MarkDone(ctx context.Context, id int64) error
	// MarkFailed marks a task as failed with the given error message.
	MarkFailed(ctx context.Context, id int64, errMsg string) error
	// Cancel cancels a task, transitioning its status to cancelled if it was pending.
	Cancel(ctx context.Context, id int64) error
}

// EventRepo defines operations for tracking email events and analytics.
type EventRepo interface {
	// Insert logs a new email event.
	Insert(ctx context.Context, e domain.EmailEvent) error
	// OverviewCounts returns the total counts of events grouped by type.
	OverviewCounts(ctx context.Context) (map[string]int, error)
	// UniqueOpens returns the distinct count of opens by contact. If campaignID is nil, it calculates globally.
	UniqueOpens(ctx context.Context, campaignID *int64) (int, error)
	// CampaignCounts returns event type counts for a specific campaign.
	CampaignCounts(ctx context.Context, campaignID int64) (map[string]int, error)
	// CampaignUniqueOpens returns the distinct count of opens by contact for a specific campaign.
	CampaignUniqueOpens(ctx context.Context, campaignID int64) (int, error)
	// TopLinks lists the most clicked links in a campaign with click counts up to limit.
	TopLinks(ctx context.Context, campaignID int64, limit int) ([]domain.LinkCount, error)
}

// TrackingRepo defines operations for tracking links.
type TrackingRepo interface {
	// CreateLink creates a tracked redirection link code for a target URL.
	CreateLink(ctx context.Context, targetURL string, campaignID, contactID *int64) (string, error)
	// GetLink retrieves a tracking link by its unique code.
	GetLink(ctx context.Context, code string) (domain.TrackingLink, error)
}

// ExportRepo defines operations for export files and metadata.
type ExportRepo interface {
	// Create persists export metadata.
	Create(ctx context.Context, e domain.Export) error
	// Get retrieves export metadata by export ID.
	Get(ctx context.Context, id string) (domain.Export, error)
}
