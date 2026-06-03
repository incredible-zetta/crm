package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestTrackingRepo(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set; skipping integration test")
	}

	repo := getTestDB(t)
	ctx := context.Background()

	// Cleanup
	t.Cleanup(func() {
		_, _ = repo.db.ExecContext(ctx, "DELETE FROM tracking_links WHERE target_url LIKE 'https://t4trk%'")
	})

	targetURL := fmt.Sprintf("https://t4trk.com/some/path/%d", time.Now().UnixNano())
	campaignID := int64(201)
	contactID := int64(301)

	// 1. CreateLink
	code, err := repo.CreateLink(ctx, targetURL, &campaignID, &contactID)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	if len(code) != 12 {
		t.Errorf("expected 12-character code, got length %d (%q)", len(code), code)
	}

	// 2. GetLink
	link, err := repo.GetLink(ctx, code)
	if err != nil {
		t.Fatalf("GetLink failed: %v", err)
	}

	if link.ID == 0 {
		t.Errorf("expected non-zero ID")
	}
	if link.Code != code {
		t.Errorf("expected Code %q, got %q", code, link.Code)
	}
	if link.TargetURL != targetURL {
		t.Errorf("expected TargetURL %q, got %q", targetURL, link.TargetURL)
	}
	if link.CampaignID == nil || *link.CampaignID != campaignID {
		t.Errorf("expected CampaignID %d, got %v", campaignID, link.CampaignID)
	}
	if link.ContactID == nil || *link.ContactID != contactID {
		t.Errorf("expected ContactID %d, got %v", contactID, link.ContactID)
	}

	// 3. CreateLink with nil pointers
	codeNil, err := repo.CreateLink(ctx, "https://t4trk.com/nil", nil, nil)
	if err != nil {
		t.Fatalf("CreateLink with nil pointers failed: %v", err)
	}
	linkNil, err := repo.GetLink(ctx, codeNil)
	if err != nil {
		t.Fatalf("GetLink for nil pointers failed: %v", err)
	}
	if linkNil.CampaignID != nil {
		t.Errorf("expected nil CampaignID, got %v", linkNil.CampaignID)
	}
	if linkNil.ContactID != nil {
		t.Errorf("expected nil ContactID, got %v", linkNil.ContactID)
	}

	// 4. Get unknown link -> ErrNotFound
	_, err = repo.GetLink(ctx, "nonexistent12")
	if err == nil {
		t.Errorf("expected error for non-existent code, got nil")
	} else if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
