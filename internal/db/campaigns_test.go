package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestCampaignsRepo(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set; skipping integration test")
	}

	repo := getTestDB(t)
	ctx := context.Background()

	// Cleanup campaigns
	t.Cleanup(func() {
		_, _ = repo.db.ExecContext(ctx, "DELETE FROM campaigns WHERE name LIKE 't4cmp_%'")
	})

	uniqueName1 := fmt.Sprintf("t4cmp_%d_1", time.Now().UnixNano())
	uniqueName2 := fmt.Sprintf("t4cmp_%d_2", time.Now().UnixNano())

	schedAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)

	c1 := Campaign{
		Name:        uniqueName1,
		TemplateID:  123,
		Provider:    "mailgun",
		Segment:     map[string]any{"industry": "tech"},
		Status:      "draft",
		ScheduledAt: &schedAt,
		Stats:       map[string]any{"sent": 0.0},
	}

	// 1. CreateCampaign
	created, err := repo.CreateCampaign(ctx, c1)
	if err != nil {
		t.Fatalf("CreateCampaign failed: %v", err)
	}

	if created.ID == 0 {
		t.Errorf("expected non-zero ID")
	}
	if created.Name != uniqueName1 {
		t.Errorf("expected Name %q, got %q", uniqueName1, created.Name)
	}
	if created.Provider != "mailgun" {
		t.Errorf("expected Provider 'mailgun', got %q", created.Provider)
	}
	if created.Segment["industry"] != "tech" {
		t.Errorf("expected Segment['industry'] to be 'tech'")
	}
	if created.ScheduledAt == nil {
		t.Errorf("expected non-nil ScheduledAt")
	} else if !created.ScheduledAt.Equal(schedAt) && !created.ScheduledAt.UTC().Equal(schedAt.UTC()) {
		t.Errorf("expected ScheduledAt %v, got %v", schedAt, *created.ScheduledAt)
	}

	// 2. Validate Defaulting on CreateCampaign
	cDefault := Campaign{
		Name: fmt.Sprintf("t4cmp_%d_def", time.Now().UnixNano()),
	}
	createdDef, err := repo.CreateCampaign(ctx, cDefault)
	if err != nil {
		t.Fatalf("CreateCampaign with defaults failed: %v", err)
	}
	if createdDef.Provider != "smtp" {
		t.Errorf("expected default Provider 'smtp', got %q", createdDef.Provider)
	}
	if createdDef.Status != "draft" {
		t.Errorf("expected default Status 'draft', got %q", createdDef.Status)
	}

	// 3. Validate Invalid Provider/Status
	_, err = repo.CreateCampaign(ctx, Campaign{
		Name:     uniqueName2,
		Provider: "invalid_provider",
	})
	if err == nil {
		t.Errorf("expected error with invalid provider, got nil")
	}

	_, err = repo.CreateCampaign(ctx, Campaign{
		Name:   uniqueName2,
		Status: "invalid_status",
	})
	if err == nil {
		t.Errorf("expected error with invalid status, got nil")
	}

	// 4. GetCampaign
	fetched, err := repo.GetCampaign(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCampaign failed: %v", err)
	}
	if fetched.ID != created.ID || fetched.Name != uniqueName1 {
		t.Errorf("GetCampaign returned incorrect record")
	}

	// 5. UpdateCampaignStatus
	err = repo.UpdateCampaignStatus(ctx, created.ID, "scheduled")
	if err != nil {
		t.Fatalf("UpdateCampaignStatus failed: %v", err)
	}
	fetched2, err := repo.GetCampaign(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCampaign after status update failed: %v", err)
	}
	if fetched2.Status != "scheduled" {
		t.Errorf("expected status 'scheduled', got %q", fetched2.Status)
	}

	// 5b. UpdateCampaignStatus to invalid
	err = repo.UpdateCampaignStatus(ctx, created.ID, "invalid_status")
	if err == nil {
		t.Errorf("expected error on invalid status update")
	}

	// 5c. UpdateCampaignStatus on non-existent
	err = repo.UpdateCampaignStatus(ctx, 999999999, "sent")
	if err == nil {
		t.Errorf("expected error updating status of non-existent campaign")
	}

	// 6. SetCampaignStats
	newStats := map[string]any{"sent": 10.0, "delivered": 9.0}
	err = repo.SetCampaignStats(ctx, created.ID, newStats)
	if err != nil {
		t.Fatalf("SetCampaignStats failed: %v", err)
	}
	fetched3, err := repo.GetCampaign(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCampaign after stats update failed: %v", err)
	}
	if fetched3.Stats["sent"] != 10.0 || fetched3.Stats["delivered"] != 9.0 {
		t.Errorf("unexpected stats: %v", fetched3.Stats)
	}

	// 7. Get non-existent campaign
	_, err = repo.GetCampaign(ctx, 999999999)
	if err == nil {
		t.Errorf("expected error for non-existent campaign ID")
	} else if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing campaign, got %v", err)
	}

	// 8. ListCampaigns (ordered by id DESC)
	c2 := Campaign{
		Name: uniqueName2,
	}
	created2, err := repo.CreateCampaign(ctx, c2)
	if err != nil {
		t.Fatalf("CreateCampaign 2 failed: %v", err)
	}

	list, err := repo.ListCampaigns(ctx)
	if err != nil {
		t.Fatalf("ListCampaigns failed: %v", err)
	}

	var found1, found2 bool
	var idx1, idx2 int = -1, -1
	for idx, item := range list {
		if item.ID == created.ID {
			found1 = true
			idx1 = idx
		}
		if item.ID == created2.ID {
			found2 = true
			idx2 = idx
		}
	}

	if !found1 || !found2 {
		t.Errorf("expected both campaigns in list, got found1=%t, found2=%t", found1, found2)
	}

	// ordered by id DESC
	if created2.ID > created.ID {
		if idx2 > idx1 {
			t.Errorf("expected Campaign 2 (ID: %d) to be before Campaign 1 (ID: %d) in DESC order, got index %d > %d", created2.ID, created.ID, idx2, idx1)
		}
	}
}
