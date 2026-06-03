package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
	"github.com/cipta/crm-for-aiagents/internal/service"
)

type trkStubClock struct {
	now time.Time
}

func (c *trkStubClock) Now() time.Time {
	return c.now
}

type trkFakeTrackingRepo struct {
	port.TrackingRepo
	links  map[string]domain.TrackingLink
	nextID int64
	err    error
}

func (r *trkFakeTrackingRepo) CreateLink(ctx context.Context, targetURL string, campaignID, contactID *int64) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	code := fmt.Sprintf("code%d", r.nextID)
	r.nextID++
	link := domain.TrackingLink{
		ID:         r.nextID,
		Code:       code,
		TargetURL:  targetURL,
		CampaignID: campaignID,
		ContactID:  contactID,
		CreatedAt:  time.Now(),
	}
	if r.links == nil {
		r.links = make(map[string]domain.TrackingLink)
	}
	r.links[code] = link
	return code, nil
}

func (r *trkFakeTrackingRepo) GetLink(ctx context.Context, code string) (domain.TrackingLink, error) {
	if r.err != nil {
		return domain.TrackingLink{}, r.err
	}
	link, ok := r.links[code]
	if !ok {
		return domain.TrackingLink{}, domain.ErrNotFound
	}
	return link, nil
}

type trkFakeEventRepo struct {
	port.EventRepo
	inserted []domain.EmailEvent
	err      error
}

func (r *trkFakeEventRepo) Insert(ctx context.Context, e domain.EmailEvent) error {
	if r.err != nil {
		return r.err
	}
	r.inserted = append(r.inserted, e)
	return nil
}

func TestCreateLinkValidatesURL(t *testing.T) {
	ctx := context.Background()
	baseURL := "https://crm.local/"

	trackingRepo := &trkFakeTrackingRepo{nextID: 1}
	eventRepo := &trkFakeEventRepo{}
	clock := &trkStubClock{now: time.Now()}

	svc := service.NewTrackingService(trackingRepo, eventRepo, clock, baseURL)

	// Test invalid URLs
	invalidURLs := []string{
		"ftp://x",
		"http://",
		"https://",
		"invalid-url",
		"://foo",
	}

	for _, target := range invalidURLs {
		_, _, err := svc.CreateLink(ctx, target, nil, nil)
		if err == nil {
			t.Errorf("expected error for invalid URL: %s", target)
		} else if !errors.Is(err, domain.ErrValidation) {
			t.Errorf("expected validation error, got %v for %s", err, target)
		}
	}

	// Test valid URLs
	validURLs := []string{
		"https://x.com",
		"http://localhost:8080/path?query=1",
	}

	for _, target := range validURLs {
		code, shortURL, err := svc.CreateLink(ctx, target, nil, nil)
		if err != nil {
			t.Errorf("unexpected error for valid URL %s: %v", target, err)
			continue
		}
		expectedURL := "https://crm.local/t/" + code
		if shortURL != expectedURL {
			t.Errorf("expected shortURL to be %q, got %q", expectedURL, shortURL)
		}
	}
}

func TestResolveClickLogsAndReturnsTarget(t *testing.T) {
	ctx := context.Background()
	baseURL := "https://crm.local/"

	trackingRepo := &trkFakeTrackingRepo{nextID: 1}
	eventRepo := &trkFakeEventRepo{}
	clockTime := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	clock := &trkStubClock{now: clockTime}

	svc := service.NewTrackingService(trackingRepo, eventRepo, clock, baseURL)

	campaignID := int64(101)
	contactID := int64(202)

	// Seed a link
	code, _, err := svc.CreateLink(ctx, "https://t.co", &campaignID, &contactID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve the click
	target, err := svc.ResolveClick(ctx, code)
	if err != nil {
		t.Fatalf("unexpected error resolving click: %v", err)
	}

	if target != "https://t.co" {
		t.Errorf("expected target URL %q, got %q", "https://t.co", target)
	}

	// Check that a click event was logged
	if len(eventRepo.inserted) != 1 {
		t.Fatalf("expected exactly 1 email event logged, got %d", len(eventRepo.inserted))
	}

	evt := eventRepo.inserted[0]
	if evt.Type != domain.EventClick {
		t.Errorf("expected event type %q, got %q", domain.EventClick, evt.Type)
	}
	if evt.ContactID != contactID {
		t.Errorf("expected contact ID %d, got %d", contactID, evt.ContactID)
	}
	if evt.CampaignID == nil || *evt.CampaignID != campaignID {
		t.Errorf("expected campaign ID %d, got %v", campaignID, evt.CampaignID)
	}
	if evt.LinkCode != code {
		t.Errorf("expected link code %q, got %q", code, evt.LinkCode)
	}
	if !evt.TS.Equal(clockTime) {
		t.Errorf("expected timestamp %v, got %v", clockTime, evt.TS)
	}

	// Unknown code should return ErrNotFound
	_, err = svc.ResolveClick(ctx, "unknown-code")
	if err == nil {
		t.Error("expected error for unknown code, got nil")
	} else if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestResolveOpenLogsEvent(t *testing.T) {
	ctx := context.Background()
	baseURL := "https://crm.local/"

	trackingRepo := &trkFakeTrackingRepo{nextID: 1}
	eventRepo := &trkFakeEventRepo{}
	clockTime := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	clock := &trkStubClock{now: clockTime}

	svc := service.NewTrackingService(trackingRepo, eventRepo, clock, baseURL)

	err := svc.ResolveOpen(ctx, "oc1")
	if err != nil {
		t.Fatalf("unexpected error resolving open: %v", err)
	}

	if len(eventRepo.inserted) != 1 {
		t.Fatalf("expected exactly 1 email event logged, got %d", len(eventRepo.inserted))
	}

	evt := eventRepo.inserted[0]
	if evt.Type != domain.EventOpen {
		t.Errorf("expected event type %q, got %q", domain.EventOpen, evt.Type)
	}
	if evt.ContactID != 0 {
		t.Errorf("expected contact ID to be 0, got %d", evt.ContactID)
	}
	if evt.CampaignID != nil {
		t.Errorf("expected campaign ID to be nil, got %v", evt.CampaignID)
	}
	if evt.LinkCode != "oc1" {
		t.Errorf("expected link code %q, got %q", "oc1", evt.LinkCode)
	}
	if !evt.TS.Equal(clockTime) {
		t.Errorf("expected timestamp %v, got %v", clockTime, evt.TS)
	}
}

func TestResolveOpenInsertError(t *testing.T) {
	ctx := context.Background()
	baseURL := "https://crm.local/"

	trackingRepo := &trkFakeTrackingRepo{}
	expectedErr := fmt.Errorf("insert fail")
	eventRepo := &trkFakeEventRepo{err: expectedErr}
	clock := &trkStubClock{now: time.Now()}

	svc := service.NewTrackingService(trackingRepo, eventRepo, clock, baseURL)

	err := svc.ResolveOpen(ctx, "oc1")
	if err == nil || err.Error() != expectedErr.Error() {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
