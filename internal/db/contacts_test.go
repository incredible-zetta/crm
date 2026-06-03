package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func getTestDB(t *testing.T) *Repo {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set; skipping integration test")
	}
	d, err := Open(dsn)
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	if err := Migrate(d); err != nil {
		d.Close()
		t.Fatalf("failed to migrate DB: %v", err)
	}
	t.Cleanup(func() {
		_, _ = d.Exec("DELETE FROM contacts WHERE email LIKE 't3_%'")
		d.Close()
	})
	return NewRepo(d)
}

func makeUniqueEmail(prefix string) string {
	return fmt.Sprintf("t3_%d_%s@test.local", time.Now().UnixNano(), prefix)
}

func TestUpsertInsertThenUpdate(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	email := makeUniqueEmail("upsert")
	c1 := Contact{
		Email:     email,
		FirstName: "John",
		LastName:  "Doe",
		Company:   "Acme Corp",
		Phone:     "123456789",
		Tags:      []string{"lead", "tech"},
		Notes:     "Some notes",
		Custom:    map[string]any{"industry": "software"},
		Source:    "web",
	}

	// 1. Insert new
	inserted, err := repo.UpsertContact(ctx, c1)
	if err != nil {
		t.Fatalf("UpsertContact insert: %v", err)
	}

	if inserted.ID == 0 {
		t.Errorf("expected non-zero ID")
	}
	if inserted.Email != email {
		t.Errorf("expected email %s, got %s", email, inserted.Email)
	}
	if inserted.FirstName != "John" {
		t.Errorf("expected FirstName John, got %s", inserted.FirstName)
	}
	if inserted.Stage != "new" {
		t.Errorf("expected default stage 'new', got %s", inserted.Stage)
	}
	if !reflect.DeepEqual(inserted.Tags, []string{"lead", "tech"}) {
		t.Errorf("unexpected tags: %v", inserted.Tags)
	}
	if inserted.Custom["industry"] != "software" {
		t.Errorf("unexpected custom field: %v", inserted.Custom)
	}

	// 2. Update with same email, modifying FirstName and Stage
	c2 := Contact{
		Email:     email,
		FirstName: "Johnny",
		Stage:     "contacted",
	}

	updated, err := repo.UpsertContact(ctx, c2)
	if err != nil {
		t.Fatalf("UpsertContact update: %v", err)
	}

	if updated.ID != inserted.ID {
		t.Errorf("expected same ID %d, got %d", inserted.ID, updated.ID)
	}
	if updated.FirstName != "Johnny" {
		t.Errorf("expected updated FirstName 'Johnny', got %s", updated.FirstName)
	}
	if updated.Stage != "contacted" {
		t.Errorf("expected updated Stage 'contacted', got %s", updated.Stage)
	}
	// Check that other existing fields like LastName, Tags, Custom, etc. were NOT overwritten by zero values
	if updated.LastName != "Doe" {
		t.Errorf("expected LastName 'Doe' to be preserved, got %s", updated.LastName)
	}
	if !reflect.DeepEqual(updated.Tags, []string{"lead", "tech"}) {
		t.Errorf("expected Tags to be preserved, got %v", updated.Tags)
	}
	if updated.Custom["industry"] != "software" {
		t.Errorf("expected Custom industry to be preserved, got %v", updated.Custom)
	}
}

func TestUpsertInvalidStage(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	c := Contact{
		Email: makeUniqueEmail("invalid-stage"),
		Stage: "bogus",
	}

	_, err := repo.UpsertContact(ctx, c)
	if err == nil {
		t.Fatal("expected error on invalid stage, got nil")
	}
}

func TestUpsertDefaultsStage(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	c := Contact{
		Email: makeUniqueEmail("default-stage"),
	}

	inserted, err := repo.UpsertContact(ctx, c)
	if err != nil {
		t.Fatalf("UpsertContact: %v", err)
	}

	if inserted.Stage != "new" {
		t.Errorf("expected default stage 'new', got %q", inserted.Stage)
	}
}

func TestListPaginationAndFilter(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	companyName := "T3PaginationInc"

	// Create 3 contacts with stage "qualified" + 1 with stage "new"
	for i := 1; i <= 3; i++ {
		_, err := repo.UpsertContact(ctx, Contact{
			Email:     makeUniqueEmail(fmt.Sprintf("qual%d", i)),
			FirstName: fmt.Sprintf("Qual%d", i),
			Company:   companyName,
			Stage:     "qualified",
		})
		if err != nil {
			t.Fatalf("failed to insert test contact: %v", err)
		}
	}

	_, err := repo.UpsertContact(ctx, Contact{
		Email:     makeUniqueEmail("newbie"),
		FirstName: "Newbie",
		Company:   companyName,
		Stage:     "new",
	})
	if err != nil {
		t.Fatalf("failed to insert test contact: %v", err)
	}

	// ListContacts filter Stage=qualified limit 2 -> 2 items, total 3, nextCursor>0
	f := ContactFilter{
		Stage:   "qualified",
		Company: companyName,
	}

	items, total, nextCursor, err := repo.ListContacts(ctx, f, 2, 0)
	if err != nil {
		t.Fatalf("ListContacts first page: %v", err)
	}

	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	if nextCursor == 0 {
		t.Errorf("expected nextCursor > 0, got 0")
	}

	// Next page with cursor -> 1 item, nextCursor 0
	items2, total2, nextCursor2, err := repo.ListContacts(ctx, f, 2, nextCursor)
	if err != nil {
		t.Fatalf("ListContacts second page: %v", err)
	}

	if total2 != 3 {
		t.Errorf("expected total 3, got %d", total2)
	}
	if len(items2) != 1 {
		t.Errorf("expected 1 item on page 2, got %d", len(items2))
	}
	if nextCursor2 != 0 {
		t.Errorf("expected nextCursor2 to be 0, got %d", nextCursor2)
	}
}

func TestListFieldsTagQuery(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	email := makeUniqueEmail("tag-query")
	_, err := repo.UpsertContact(ctx, Contact{
		Email:     email,
		FirstName: "Alice",
		LastName:  "Wonder",
		Company:   "Wonderland",
		Tags:      []string{"vip", "curious"},
	})
	if err != nil {
		t.Fatalf("failed to insert test contact: %v", err)
	}

	// Search by Tag
	items, total, _, err := repo.ListContacts(ctx, ContactFilter{Tag: "vip"}, 10, 0)
	if err != nil {
		t.Fatalf("ListContacts by Tag: %v", err)
	}

	found := false
	for _, item := range items {
		if item.Email == email {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find contact by tag 'vip' (total match count: %d)", total)
	}

	// Search by Q (query email/first/last/company)
	itemsQ, _, _, err := repo.ListContacts(ctx, ContactFilter{Q: "Wond"}, 10, 0)
	if err != nil {
		t.Fatalf("ListContacts by Q: %v", err)
	}

	foundQ := false
	for _, item := range itemsQ {
		if item.Email == email {
			foundQ = true
			break
		}
	}
	if !foundQ {
		t.Errorf("expected to find contact by Q 'Wond'")
	}
}

func TestGetContactNotFound(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	_, err := repo.GetContact(ctx, 999999999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected wrapped ErrNotFound, got: %v", err)
	}
}

func TestCountByStage(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	// Ensure we have some contacts
	email1 := makeUniqueEmail("count1")
	_, err := repo.UpsertContact(ctx, Contact{
		Email: email1,
		Stage: "proposal",
	})
	if err != nil {
		t.Fatalf("UpsertContact: %v", err)
	}

	email2 := makeUniqueEmail("count2")
	_, err = repo.UpsertContact(ctx, Contact{
		Email: email2,
		Stage: "proposal",
	})
	if err != nil {
		t.Fatalf("UpsertContact: %v", err)
	}

	counts, err := repo.CountByStage(ctx)
	if err != nil {
		t.Fatalf("CountByStage: %v", err)
	}

	if counts["proposal"] < 2 {
		t.Errorf("expected 'proposal' count to be at least 2, got %d", counts["proposal"])
	}
}

func TestUpdateContactPatch(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	email := makeUniqueEmail("patch")
	c, err := repo.UpsertContact(ctx, Contact{
		Email:     email,
		FirstName: "OriginalFirst",
		LastName:  "OriginalLast",
		Stage:     "new",
	})
	if err != nil {
		t.Fatalf("UpsertContact: %v", err)
	}

	// Update only LastName and Stage
	newLast := "PatchedLast"
	newStage := "qualified"
	tags := []string{"patched-tag"}
	custom := map[string]any{"patched-custom": 123.0}

	patch := ContactPatch{
		LastName: &newLast,
		Stage:    &newStage,
		Tags:     &tags,
		Custom:   &custom,
	}

	updated, err := repo.UpdateContact(ctx, c.ID, patch)
	if err != nil {
		t.Fatalf("UpdateContact: %v", err)
	}

	if updated.FirstName != "OriginalFirst" {
		t.Errorf("expected FirstName 'OriginalFirst' to be unchanged, got %q", updated.FirstName)
	}
	if updated.LastName != "PatchedLast" {
		t.Errorf("expected LastName to be 'PatchedLast', got %q", updated.LastName)
	}
	if updated.Stage != "qualified" {
		t.Errorf("expected Stage to be 'qualified', got %q", updated.Stage)
	}
	if !reflect.DeepEqual(updated.Tags, []string{"patched-tag"}) {
		t.Errorf("expected Tags to be ['patched-tag'], got %v", updated.Tags)
	}
	if updated.Custom["patched-custom"] != 123.0 {
		t.Errorf("expected Custom['patched-custom'] to be 123, got %v", updated.Custom["patched-custom"])
	}

	// Check invalid stage in patch
	badStage := "bogus"
	_, err = repo.UpdateContact(ctx, c.ID, ContactPatch{Stage: &badStage})
	if err == nil {
		t.Error("expected error updating with invalid stage, got nil")
	}
}
