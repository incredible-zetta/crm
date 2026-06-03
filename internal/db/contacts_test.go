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
		_, _ = d.Exec("DELETE FROM contacts WHERE email LIKE 't3b_%'")
		d.Close()
	})
	return NewRepo(d)
}

func makeUniqueEmail(prefix string) string {
	return fmt.Sprintf("t3_%d_%s@test.local", time.Now().UnixNano(), prefix)
}

func makeUniqueEmailT3B(prefix string) string {
	return fmt.Sprintf("t3b_%d_%s@test.local", time.Now().UnixNano(), prefix)
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

	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	uniqueTag := fmt.Sprintf("vip_%s", uniqueSuffix)
	uniqueCompany := fmt.Sprintf("Wonderland_%s", uniqueSuffix)
	email := makeUniqueEmail("tag-query")

	_, err := repo.UpsertContact(ctx, Contact{
		Email:     email,
		FirstName: "Alice",
		LastName:  "Wonder",
		Company:   uniqueCompany,
		Tags:      []string{uniqueTag, "curious"},
	})
	if err != nil {
		t.Fatalf("failed to insert test contact: %v", err)
	}

	// Search by Tag
	items, total, _, err := repo.ListContacts(ctx, ContactFilter{Tag: uniqueTag}, 10, 0)
	if err != nil {
		t.Fatalf("ListContacts by Tag: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("expected exactly 1 item for unique tag, got %d (total=%d)", len(items), total)
	} else if items[0].Email != email {
		t.Errorf("expected contact email %s, got %s", email, items[0].Email)
	}

	// Search by Q (query email/first/last/company) using the unique company substring
	itemsQ, totalQ, _, err := repo.ListContacts(ctx, ContactFilter{Q: uniqueCompany}, 10, 0)
	if err != nil {
		t.Fatalf("ListContacts by Q: %v", err)
	}

	if len(itemsQ) != 1 {
		t.Errorf("expected exactly 1 item for unique Q, got %d (totalQ=%d)", len(itemsQ), totalQ)
	} else if itemsQ[0].Email != email {
		t.Errorf("expected contact email %s, got %s", email, itemsQ[0].Email)
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

func TestGetContactByEmailNotFound(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	email := fmt.Sprintf("t3b_nope_%d@test.local", time.Now().UnixNano())
	_, err := repo.GetContactByEmail(ctx, email)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestListLimitDefaultAndCap(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	companyName := fmt.Sprintf("t3b_limit_%d", time.Now().UnixNano())

	// Insert 3 contacts for this unique company
	for i := 1; i <= 3; i++ {
		_, err := repo.UpsertContact(ctx, Contact{
			Email:     makeUniqueEmailT3B(fmt.Sprintf("limit_cap_%d", i)),
			FirstName: fmt.Sprintf("Cap%d", i),
			Company:   companyName,
			Stage:     "new",
		})
		if err != nil {
			t.Fatalf("failed to insert contact: %v", err)
		}
	}

	// 1. Call ListContacts with limit=0 (which should default to 20).
	items, total, _, err := repo.ListContacts(ctx, ContactFilter{Company: companyName}, 0, 0)
	if err != nil {
		t.Fatalf("ListContacts with limit=0: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items (since effective limit is 20), got %d", len(items))
	}

	// 2. Call ListContacts with limit=500 (which must be capped at 100).
	itemsLarge, totalLarge, _, err := repo.ListContacts(ctx, ContactFilter{Company: companyName}, 500, 0)
	if err != nil {
		t.Fatalf("ListContacts with limit=500: %v", err)
	}
	if totalLarge != 3 {
		t.Errorf("expected total 3, got %d", totalLarge)
	}
	if len(itemsLarge) > 100 {
		t.Errorf("expected items clamped to <= 100, got %d", len(itemsLarge))
	}
	if len(itemsLarge) != 3 {
		t.Errorf("expected 3 items, got %d", len(itemsLarge))
	}
}

func TestNextCursorExactFullPage(t *testing.T) {
	repo := getTestDB(t)
	ctx := context.Background()

	companyName := fmt.Sprintf("t3b_exact_full_%d", time.Now().UnixNano())

	// Insert exactly 2 matching contacts
	var inserted []Contact
	for i := 1; i <= 2; i++ {
		c, err := repo.UpsertContact(ctx, Contact{
			Email:     makeUniqueEmailT3B(fmt.Sprintf("exact_%d", i)),
			FirstName: fmt.Sprintf("Exact%d", i),
			Company:   companyName,
			Stage:     "new",
		})
		if err != nil {
			t.Fatalf("failed to insert contact: %v", err)
		}
		inserted = append(inserted, c)
	}

	filter := ContactFilter{Company: companyName}

	// 1. ListContacts with limit=2 (the exact number of items matching)
	items, total, nextCursor, err := repo.ListContacts(ctx, filter, 2, 0)
	if err != nil {
		t.Fatalf("ListContacts with limit=2: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	// The core bug fix: since there is no next page, nextCursor must be 0!
	if nextCursor != 0 {
		t.Errorf("expected nextCursor to be 0 for exact full page, got %d", nextCursor)
	}

	// 2. ListContacts with limit=1 (first page)
	itemsPage1, totalPage1, nextCursorPage1, err := repo.ListContacts(ctx, filter, 1, 0)
	if err != nil {
		t.Fatalf("ListContacts page 1 with limit=1: %v", err)
	}
	if totalPage1 != 2 {
		t.Errorf("expected total 2, got %d", totalPage1)
	}
	if len(itemsPage1) != 1 {
		t.Errorf("expected 1 item, got %d", len(itemsPage1))
	}
	if nextCursorPage1 != inserted[0].ID {
		t.Errorf("expected nextCursor to be %d (ID of first contact), got %d", inserted[0].ID, nextCursorPage1)
	}

	// 3. ListContacts with limit=1 and cursor=nextCursorPage1 (second page)
	itemsPage2, totalPage2, nextCursorPage2, err := repo.ListContacts(ctx, filter, 1, nextCursorPage1)
	if err != nil {
		t.Fatalf("ListContacts page 2 with limit=1: %v", err)
	}
	if totalPage2 != 2 {
		t.Errorf("expected total 2, got %d", totalPage2)
	}
	if len(itemsPage2) != 1 {
		t.Errorf("expected 1 item, got %d", len(itemsPage2))
	}
	if itemsPage2[0].ID != inserted[1].ID {
		t.Errorf("expected second contact ID %d, got %d", inserted[1].ID, itemsPage2[0].ID)
	}
	if nextCursorPage2 != 0 {
		t.Errorf("expected nextCursor for last page to be 0, got %d", nextCursorPage2)
	}
}
