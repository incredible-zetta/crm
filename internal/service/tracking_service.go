package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
)

type TrackingService struct {
	tracking port.TrackingRepo
	events   port.EventRepo
	clock    port.Clock
	baseURL  string
}

func NewTrackingService(tracking port.TrackingRepo, events port.EventRepo, clock port.Clock, baseURL string) *TrackingService {
	return &TrackingService{
		tracking: tracking,
		events:   events,
		clock:    clock,
		baseURL:  baseURL,
	}
}

// CreateLink validates targetURL, inserts tracking link via tracking repository,
// and returns unique code and shortened redirect URL.
func (s *TrackingService) CreateLink(ctx context.Context, targetURL string, campaignID, contactID *int64) (code, shortURL string, err error) {
	u, err := url.Parse(targetURL)
	if err != nil || u.Host == "" || (strings.ToLower(u.Scheme) != "http" && strings.ToLower(u.Scheme) != "https") {
		return "", "", fmt.Errorf("%w: target must be absolute http or https URL", domain.ErrValidation)
	}

	code, err = s.tracking.CreateLink(ctx, targetURL, campaignID, contactID)
	if err != nil {
		return "", "", err
	}

	shortURL = strings.TrimSuffix(s.baseURL, "/") + "/t/" + code
	return code, shortURL, nil
}

// ResolveClick resolves the short link code, logs a best-effort click event, and returns target URL.
func (s *TrackingService) ResolveClick(ctx context.Context, code string) (targetURL string, err error) {
	link, err := s.tracking.GetLink(ctx, code)
	if err != nil {
		return "", err
	}

	// Public route: no tenant in ctx. Scope the click event to the link's
	// owning tenant resolved from the row.
	ctx = tenant.With(ctx, link.TenantID)

	var contactID int64
	if link.ContactID != nil {
		contactID = *link.ContactID
	}

	// best-effort event logging
	_ = s.events.Insert(ctx, domain.EmailEvent{
		ContactID:  contactID,
		CampaignID: link.CampaignID,
		Type:       domain.EventClick,
		LinkCode:   code,
		TS:         s.clock.Now(),
	})

	return link.TargetURL, nil
}

// ResolveOpen logs an open event correlate to link_code, best-effort (returning error to caller if insert fails).
func (s *TrackingService) ResolveOpen(ctx context.Context, code string) error {
	// Public route: resolve the owning tenant from the link row so the open
	// event is logged under the correct tenant.
	if link, err := s.tracking.GetLink(ctx, code); err == nil {
		ctx = tenant.With(ctx, link.TenantID)
	}
	err := s.events.Insert(ctx, domain.EmailEvent{
		ContactID: 0,
		Type:      domain.EventOpen,
		LinkCode:  code,
		TS:        s.clock.Now(),
	})
	if err != nil {
		return err
	}
	return nil
}
