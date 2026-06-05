package service

import (
	"context"
	"fmt"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// CampaignMailer sends one already-resolved campaign email to a contact.
// email_service.EmailService will satisfy this in R11 wiring.
type CampaignMailer interface {
	SendToContact(ctx context.Context, c domain.Contact, templateID int64, campaignID int64) error
}

// CampaignStats represents performance stats for a campaign.
type CampaignStats struct {
	CampaignID   int64
	Sent         int
	Delivered    int
	Opened       int
	Clicked      int
	Bounced      int
	Unsubscribed int
	UniqueOpens  int
	OpenRate     float64
	ClickRate    float64
	TopLinks     []domain.LinkCount
}

// CampaignService implements business logic for campaigns.
type CampaignService struct {
	repo     port.CampaignRepo
	contacts port.ContactRepo
	events   port.EventRepo
	mailer   CampaignMailer
}

// NewCampaignService creates a new CampaignService.
func NewCampaignService(repo port.CampaignRepo, contacts port.ContactRepo, events port.EventRepo, mailer CampaignMailer) *CampaignService {
	return &CampaignService{
		repo:     repo,
		contacts: contacts,
		events:   events,
		mailer:   mailer,
	}
}

// Create persists a new campaign with defaults and validations.
func (s *CampaignService) Create(ctx context.Context, c domain.Campaign) (domain.Campaign, error) {
	if c.Name == "" {
		return domain.Campaign{}, fmt.Errorf("%w: name required", domain.ErrValidation)
	}
	if c.TemplateID <= 0 {
		return domain.Campaign{}, fmt.Errorf("%w: template_id required", domain.ErrValidation)
	}
	if c.Provider == "" {
		c.Provider = domain.ProviderSMTP
	} else if !c.Provider.Valid() {
		return domain.Campaign{}, fmt.Errorf("%w: invalid provider %q", domain.ErrValidation, c.Provider)
	}
	if c.Status == "" {
		c.Status = domain.CampaignDraft
	}
	return s.repo.Create(ctx, c)
}

// Get retrieves a campaign by its ID.
func (s *CampaignService) Get(ctx context.Context, id int64) (domain.Campaign, error) {
	return s.repo.Get(ctx, id)
}

// List retrieves all active (non-deleted) campaigns.
func (s *CampaignService) List(ctx context.Context) ([]domain.Campaign, error) {
	return s.repo.List(ctx)
}

// Update updates an existing campaign.
func (s *CampaignService) Update(ctx context.Context, id int64, c domain.Campaign) (domain.Campaign, error) {
	if c.Provider != "" && !c.Provider.Valid() {
		return domain.Campaign{}, fmt.Errorf("%w: invalid provider %q", domain.ErrValidation, c.Provider)
	}
	return s.repo.Update(ctx, id, c)
}

// Delete soft-deletes a campaign.
func (s *CampaignService) Delete(ctx context.Context, id int64) error {
	return s.repo.SoftDelete(ctx, id)
}

// MarkSending transitions a campaign into the sending state. It verifies the
// campaign exists first so the async enqueue path can fail fast with
// ErrNotFound. It is idempotent: a campaign already sending stays sending.
func (s *CampaignService) MarkSending(ctx context.Context, id int64) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, domain.CampaignSending)
}

// Stats returns performance metrics for a campaign.
func (s *CampaignService) Stats(ctx context.Context, id int64) (CampaignStats, error) {
	_, err := s.repo.Get(ctx, id)
	if err != nil {
		return CampaignStats{}, err
	}

	counts, err := s.events.CampaignCounts(ctx, id)
	if err != nil {
		return CampaignStats{}, fmt.Errorf("failed to get campaign counts: %w", err)
	}

	uniqueOpens, err := s.events.CampaignUniqueOpens(ctx, id)
	if err != nil {
		return CampaignStats{}, fmt.Errorf("failed to get campaign unique opens: %w", err)
	}

	topLinks, err := s.events.TopLinks(ctx, id, 5)
	if err != nil {
		return CampaignStats{}, fmt.Errorf("failed to get top links: %w", err)
	}

	sent := counts["sent"]
	var openRate float64
	var clickRate float64

	if sent > 0 {
		openRate = float64(uniqueOpens) / float64(sent)
		clickRate = float64(counts["click"]) / float64(sent)
	}

	return CampaignStats{
		CampaignID:   id,
		Sent:         sent,
		Delivered:    counts["delivered"],
		Opened:       counts["open"],
		Clicked:      counts["click"],
		Bounced:      counts["bounce"],
		Unsubscribed: counts["unsubscribe"],
		UniqueOpens:  uniqueOpens,
		OpenRate:     openRate,
		ClickRate:    clickRate,
		TopLinks:     topLinks,
	}, nil
}

// Send broadcasts a campaign to its matched segment of contacts.
func (s *CampaignService) Send(ctx context.Context, id int64) (recipients, sent, failed, skipped int, err error) {
	campaign, err := s.repo.Get(ctx, id)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	// Mark sending so concurrent callers and pollers observe progress. Best-effort.
	_ = s.repo.UpdateStatus(ctx, campaign.ID, domain.CampaignSending)

	var filter domain.ContactFilter
	if campaign.Segment != nil {
		if val, ok := campaign.Segment["stage"].(string); ok {
			filter.Stage = val
		}
		if val, ok := campaign.Segment["company"].(string); ok {
			filter.Company = val
		}
		if val, ok := campaign.Segment["tag"].(string); ok {
			filter.Tag = val
		}
		if val, ok := campaign.Segment["q"].(string); ok {
			filter.Q = val
		}
	}

	var cursor int64
	for {
		page, err := s.contacts.List(ctx, filter, port.Paging{Limit: 100, Cursor: cursor})
		if err != nil {
			_ = s.repo.UpdateStatus(ctx, campaign.ID, domain.CampaignFailed)
			return recipients, sent, failed, skipped, fmt.Errorf("failed to page contacts: %w", err)
		}
		if len(page.Items) == 0 {
			break
		}

		for _, contact := range page.Items {
			recipients++
			if contact.IsUnsubscribed() {
				skipped++
				continue
			}

			err := s.mailer.SendToContact(ctx, contact, campaign.TemplateID, campaign.ID)
			if err != nil {
				failed++
			} else {
				sent++
			}
		}

		if page.NextCursor == 0 {
			break
		}
		cursor = page.NextCursor
	}

	err = s.repo.UpdateStatus(ctx, campaign.ID, domain.CampaignSent)
	if err != nil {
		return recipients, sent, failed, skipped, fmt.Errorf("failed to update campaign status: %w", err)
	}

	return recipients, sent, failed, skipped, nil
}
