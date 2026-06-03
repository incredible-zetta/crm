package domain

import "time"

// Provider represents the email delivery provider to be used.
type Provider string

const (
	// ProviderSMTP represents SMTP email delivery.
	ProviderSMTP Provider = "smtp"
	// ProviderMailgun represents Mailgun email delivery.
	ProviderMailgun Provider = "mailgun"
)

// Providers is the list of all supported email delivery providers.
var Providers = []Provider{ProviderSMTP, ProviderMailgun}

// Valid checks if the provider is supported.
func (p Provider) Valid() bool {
	for _, provider := range Providers {
		if p == provider {
			return true
		}
	}
	return false
}

// CampaignStatus represents the current state of an email campaign.
type CampaignStatus string

const (
	// CampaignDraft indicates a campaign is still being created or edited.
	CampaignDraft CampaignStatus = "draft"
	// CampaignScheduled indicates the campaign is scheduled to run in the future.
	CampaignScheduled CampaignStatus = "scheduled"
	// CampaignSending indicates the campaign is currently sending emails.
	CampaignSending CampaignStatus = "sending"
	// CampaignSent indicates the campaign completed sending emails.
	CampaignSent CampaignStatus = "sent"
	// CampaignFailed indicates the campaign failed during execution.
	CampaignFailed CampaignStatus = "failed"
)

// CampaignStatuses is the list of all valid campaign statuses.
var CampaignStatuses = []CampaignStatus{
	CampaignDraft,
	CampaignScheduled,
	CampaignSending,
	CampaignSent,
	CampaignFailed,
}

// Valid checks if the campaign status is valid.
func (s CampaignStatus) Valid() bool {
	for _, status := range CampaignStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// Campaign represents an email marketing campaign.
type Campaign struct {
	ID          int64
	Name        string
	TemplateID  int64
	Provider    Provider
	Segment     map[string]any
	Status      CampaignStatus
	ScheduledAt *time.Time
	Stats       map[string]any
	DeletedAt   *time.Time
	CreatedAt   time.Time
}

// IsDeleted returns true if the campaign has been soft-deleted.
func (c Campaign) IsDeleted() bool {
	return c.DeletedAt != nil
}
