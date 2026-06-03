package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestEventsRepo(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set; skipping integration test")
	}

	repo := getTestDB(t)
	ctx := context.Background()

	// 1. Create a throwaway contact
	var contactID int64
	contactEmail := fmt.Sprintf("t4ev_%d@test.local", time.Now().UnixNano())
	res, err := repo.db.ExecContext(ctx, "INSERT INTO contacts (email, stage) VALUES (?, 'new')", contactEmail)
	if err != nil {
		t.Fatalf("failed to insert throwaway contact: %v", err)
	}
	contactID, err = res.LastInsertId()
	if err != nil {
		t.Fatalf("failed to get insert ID: %v", err)
	}

	// Cleanup events and contact
	t.Cleanup(func() {
		_, _ = repo.db.ExecContext(ctx, "DELETE FROM email_events WHERE link_code LIKE 't4ev_%'")
		_, _ = repo.db.ExecContext(ctx, "DELETE FROM contacts WHERE id = ?", contactID)
	})

	campaignID1 := int64(101)
	campaignID2 := int64(102)

	// 2. Validate Invalid Event Type
	err = repo.InsertEvent(ctx, EmailEvent{
		ContactID:  contactID,
		CampaignID: &campaignID1,
		Type:       "invalid_event_type",
		LinkCode:   "t4ev_invalid",
	})
	if err == nil {
		t.Errorf("expected error with invalid event type, got nil")
	}

	// 3. Insert valid events
	events := []EmailEvent{
		{ContactID: contactID, CampaignID: &campaignID1, Type: "sent", LinkCode: "t4ev_sent1", Meta: map[string]any{"ip": "1.1.1.1"}},
		{ContactID: contactID, CampaignID: &campaignID1, Type: "delivered", LinkCode: "t4ev_deliv1"},
		{ContactID: contactID, CampaignID: &campaignID1, Type: "click", LinkCode: "t4ev_linkA", Meta: map[string]any{"ua": "chrome"}},
		{ContactID: contactID, CampaignID: &campaignID1, Type: "click", LinkCode: "t4ev_linkA"},
		{ContactID: contactID, CampaignID: &campaignID1, Type: "click", LinkCode: "t4ev_linkB"},
		{ContactID: contactID, CampaignID: &campaignID2, Type: "click", LinkCode: "t4ev_linkB"},
		{ContactID: contactID, CampaignID: &campaignID2, Type: "bounce", LinkCode: "t4ev_bounce"},
	}

	for _, ev := range events {
		err := repo.InsertEvent(ctx, ev)
		if err != nil {
			t.Fatalf("InsertEvent failed: %v", err)
		}
	}

	// 4. OverviewCounts
	ov, err := repo.OverviewCounts(ctx)
	if err != nil {
		t.Fatalf("OverviewCounts failed: %v", err)
	}
	if ov["sent"] < 1 {
		t.Errorf("expected overview sent count >= 1, got %d", ov["sent"])
	}
	if ov["click"] < 4 {
		t.Errorf("expected overview click count >= 4, got %d", ov["click"])
	}

	// 5. CampaignCounts
	cc1, err := repo.CampaignCounts(ctx, campaignID1)
	if err != nil {
		t.Fatalf("CampaignCounts 1 failed: %v", err)
	}
	if cc1["sent"] != 1 {
		t.Errorf("expected campaign 1 sent count 1, got %d", cc1["sent"])
	}
	if cc1["click"] != 3 {
		t.Errorf("expected campaign 1 click count 3, got %d", cc1["click"])
	}

	cc2, err := repo.CampaignCounts(ctx, campaignID2)
	if err != nil {
		t.Fatalf("CampaignCounts 2 failed: %v", err)
	}
	if cc2["click"] != 1 {
		t.Errorf("expected campaign 2 click count 1, got %d", cc2["click"])
	}
	if cc2["bounce"] != 1 {
		t.Errorf("expected campaign 2 bounce count 1, got %d", cc2["bounce"])
	}

	// 6. TopLinks
	top1, err := repo.TopLinks(ctx, campaignID1, 10)
	if err != nil {
		t.Fatalf("TopLinks failed: %v", err)
	}

	if len(top1) < 2 {
		t.Fatalf("expected at least 2 top links, got %d", len(top1))
	}

	// top1 should show linkA first (2 clicks) then linkB (1 click)
	var foundA, foundB bool
	var idxA, idxB int = -1, -1
	for idx, tc := range top1 {
		if tc.LinkCode == "t4ev_linkA" {
			foundA = true
			idxA = idx
			if tc.Clicks != 2 {
				t.Errorf("expected linkA to have 2 clicks, got %d", tc.Clicks)
			}
		}
		if tc.LinkCode == "t4ev_linkB" {
			foundB = true
			idxB = idx
			if tc.Clicks != 1 {
				t.Errorf("expected linkB to have 1 click, got %d", tc.Clicks)
			}
		}
	}

	if !foundA || !foundB {
		t.Errorf("expected to find linkA and linkB in top links")
	} else if idxA > idxB {
		t.Errorf("expected linkA to be sorted before linkB because it has more clicks")
	}

	// 7. TopLinks limit parameter
	topLimited, err := repo.TopLinks(ctx, campaignID1, 1)
	if err != nil {
		t.Fatalf("TopLinks with limit failed: %v", err)
	}
	if len(topLimited) != 1 {
		t.Errorf("expected top limited to return exactly 1 link, got %d", len(topLimited))
	}
}
