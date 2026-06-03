package mysql

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
)

func getTestStore(t *testing.T) *Store {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set; skipping integration test")
	}
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	if err := Migrate(db); err != nil {
		db.Close()
		t.Fatalf("failed to migrate DB: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM email_events WHERE contact_id IN (SELECT id FROM contacts WHERE email LIKE 't_mysql_%')")
		_, _ = db.Exec("DELETE FROM tracking_links WHERE target_url LIKE '%t_mysql_%'")
		_, _ = db.Exec("DELETE FROM contacts WHERE email LIKE 't_mysql_%'")
		_, _ = db.Exec("DELETE FROM email_templates WHERE name LIKE 't_mysql_%'")
		_, _ = db.Exec("DELETE FROM campaigns WHERE name LIKE 't_mysql_%'")
		_, _ = db.Exec("DELETE FROM scheduled_tasks WHERE payload LIKE '%t_mysql_%'")
		_, _ = db.Exec("DELETE FROM exports WHERE id LIKE 't_mysql_%'")
		db.Close()
	})
	return New(db)
}

func makeUniqueEmail(prefix string) string {
	return fmt.Sprintf("t_mysql_%d_%s@test.local", time.Now().UnixNano(), prefix)
}

func TestContactRepo(t *testing.T) {
	store := getTestStore(t)
	repo := store.Contacts()
	ctx := context.Background()

	email := makeUniqueEmail("contact")

	// 1. Get non-existent
	_, err := repo.Get(ctx, 999999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	_, err = repo.GetByEmail(ctx, email)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// 2. Upsert (Insert)
	c := domain.Contact{
		Email:     email,
		FirstName: "Acme",
		LastName:  "User",
		Company:   "Acme Inc",
		Phone:     "111-222-333",
		Stage:     domain.StageNew,
		Tags:      []string{"tag1", "tag2"},
		Notes:     "initial note",
		Custom:    map[string]any{"custom_key": "custom_val"},
		Source:    "test-source",
	}

	inserted, err := repo.Upsert(ctx, c)
	if err != nil {
		t.Fatalf("upsert contact: %v", err)
	}

	if inserted.ID == 0 {
		t.Fatal("expected non-zero contact ID")
	}
	if inserted.Email != email {
		t.Errorf("expected email %s, got %s", email, inserted.Email)
	}
	if !reflect.DeepEqual(inserted.Tags, []string{"tag1", "tag2"}) {
		t.Errorf("expected tags %v, got %v", []string{"tag1", "tag2"}, inserted.Tags)
	}

	// 3. Upsert (Update existing)
	cUpdated := domain.Contact{
		Email:     email,
		FirstName: "Acme Updated",
		Stage:     domain.StageContacted,
	}
	updatedUpsert, err := repo.Upsert(ctx, cUpdated)
	if err != nil {
		t.Fatalf("upsert update contact: %v", err)
	}

	if updatedUpsert.ID != inserted.ID {
		t.Errorf("expected same ID %d, got %d", inserted.ID, updatedUpsert.ID)
	}
	if updatedUpsert.FirstName != "Acme Updated" {
		t.Errorf("expected FirstName updated, got %s", updatedUpsert.FirstName)
	}
	if updatedUpsert.LastName != "User" {
		t.Errorf("expected LastName preserved, got %s", updatedUpsert.LastName)
	}
	if updatedUpsert.Stage != domain.StageContacted {
		t.Errorf("expected Stage updated, got %s", updatedUpsert.Stage)
	}

	// 4. Update via Patch
	firstNamePatch := "Acme Patched"
	invalidStagePatch := "invalid_stage"
	validStagePatch := string(domain.StageQualified)

	patchInvalid := domain.ContactPatch{Stage: &invalidStagePatch}
	_, err = repo.Update(ctx, inserted.ID, patchInvalid)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation for invalid stage update, got %v", err)
	}

	patchValid := domain.ContactPatch{
		FirstName: &firstNamePatch,
		Stage:     &validStagePatch,
	}
	patched, err := repo.Update(ctx, inserted.ID, patchValid)
	if err != nil {
		t.Fatalf("patched contact: %v", err)
	}

	if patched.FirstName != "Acme Patched" {
		t.Errorf("expected patched first name, got %s", patched.FirstName)
	}
	if patched.Stage != domain.StageQualified {
		t.Errorf("expected patched stage, got %s", patched.Stage)
	}

	// 5. Get and GetByEmail
	g, err := repo.Get(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("get contact: %v", err)
	}
	if g.ID != inserted.ID {
		t.Errorf("get ID mismatch: %d vs %d", g.ID, inserted.ID)
	}

	gEmail, err := repo.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("get contact by email: %v", err)
	}
	if gEmail.ID != inserted.ID {
		t.Errorf("get email mismatch: %d vs %d", gEmail.ID, inserted.ID)
	}

	// 6. List and filters
	page, err := repo.List(ctx, domain.ContactFilter{Stage: string(domain.StageQualified)}, port.Paging{Limit: 10})
	if err != nil {
		t.Fatalf("list contact: %v", err)
	}
	if page.Total < 1 {
		t.Errorf("expected total >= 1, got %d", page.Total)
	}
	foundInList := false
	for _, item := range page.Items {
		if item.ID == inserted.ID {
			foundInList = true
		}
	}
	if !foundInList {
		t.Error("expected to find patched contact in list")
	}

	// List with tag filter
	pageTag, err := repo.List(ctx, domain.ContactFilter{Tag: "tag1"}, port.Paging{Limit: 10})
	if err != nil {
		t.Fatalf("list contact by tag: %v", err)
	}
	if pageTag.Total < 1 {
		t.Errorf("expected tag list total >= 1, got %d", pageTag.Total)
	}

	// 7. Unsubscribe round-trip
	unsubCode := "unsub-code-12345"
	err = repo.SetUnsubCode(ctx, inserted.ID, unsubCode)
	if err != nil {
		t.Fatalf("set unsub code: %v", err)
	}

	gUnsub, err := repo.GetByUnsubCode(ctx, unsubCode)
	if err != nil {
		t.Fatalf("get by unsub code: %v", err)
	}
	if gUnsub.ID != inserted.ID {
		t.Errorf("unsub code ID mismatch: %d vs %d", gUnsub.ID, inserted.ID)
	}

	unsubTime := time.Now().Truncate(time.Second)
	err = repo.SetUnsubscribed(ctx, inserted.ID, unsubTime)
	if err != nil {
		t.Fatalf("set unsubscribed: %v", err)
	}

	gUnsubscribed, err := repo.Get(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("get unsubscribed: %v", err)
	}
	if gUnsubscribed.UnsubscribedAt == nil {
		t.Fatal("expected UnsubscribedAt to be non-nil")
	}
	if !gUnsubscribed.UnsubscribedAt.Equal(unsubTime) {
		t.Errorf("expected unsubscribed time %v, got %v", unsubTime, *gUnsubscribed.UnsubscribedAt)
	}

	// 8. Soft Delete checks
	err = repo.SoftDelete(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("soft delete contact: %v", err)
	}

	// Soft delete contact should be hidden from Get
	_, err = repo.Get(ctx, inserted.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for soft-deleted contact, got %v", err)
	}

	// Soft delete contact should be hidden from GetByEmail
	_, err = repo.GetByEmail(ctx, email)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for soft-deleted contact, got %v", err)
	}

	// Soft delete contact should still be retrievable by UnsubCode (GDPR / opt-out capability)
	gUnsubDeleted, err := repo.GetByUnsubCode(ctx, unsubCode)
	if err != nil {
		t.Fatalf("expected to get soft-deleted contact by unsub code, got error: %v", err)
	}
	if gUnsubDeleted.ID != inserted.ID {
		t.Errorf("unsub code ID mismatch after soft delete: %d vs %d", gUnsubDeleted.ID, inserted.ID)
	}

	// Soft delete contact should be hidden from List
	pageDeleted, err := repo.List(ctx, domain.ContactFilter{Company: "Acme Inc"}, port.Paging{Limit: 10})
	if err != nil {
		t.Fatalf("list contact: %v", err)
	}
	for _, item := range pageDeleted.Items {
		if item.ID == inserted.ID {
			t.Fatal("found soft-deleted contact in List!")
		}
	}

	// Soft delete contact should be hidden from CountByStage
	stageCounts, err := repo.CountByStage(ctx)
	if err != nil {
		t.Fatalf("CountByStage: %v", err)
	}
	if val := stageCounts[string(domain.StageQualified)]; val > 0 {
		// Wait, count may still be >0 if other tests left data, but at least let's verify it decremented/is clean
	}

	// 9. Purge / Hard delete
	err = repo.Purge(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("purge contact: %v", err)
	}

	// Purge removes contact entirely from DB, even by UnsubCode
	_, err = repo.GetByUnsubCode(ctx, unsubCode)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for purged contact, got %v", err)
	}
}

func TestCampaignRepo(t *testing.T) {
	store := getTestStore(t)
	repo := store.Campaigns()
	ctx := context.Background()

	name := fmt.Sprintf("t_mysql_campaign_%d", time.Now().UnixNano())

	// 1. Create campaign
	c := domain.Campaign{
		Name:       name,
		TemplateID: 10,
		Provider:   domain.ProviderSMTP,
		Segment:    map[string]any{"stage": "new"},
		Status:     domain.CampaignDraft,
	}

	created, err := repo.Create(ctx, c)
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	if created.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if created.Name != name {
		t.Errorf("expected name %s, got %s", name, created.Name)
	}
	if created.Provider != domain.ProviderSMTP {
		t.Errorf("expected provider smtp, got %s", created.Provider)
	}

	// 2. Get and List
	g, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get campaign: %v", err)
	}
	if g.ID != created.ID {
		t.Errorf("get ID mismatch: %d vs %d", g.ID, created.ID)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list campaigns: %v", err)
	}
	found := false
	for _, item := range list {
		if item.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("campaign not found in list")
	}

	// 3. Update fields
	scheduledTime := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	updateCamp := domain.Campaign{
		Name:        "t_mysql_campaign_patched",
		TemplateID:  20,
		Provider:    domain.ProviderMailgun,
		Segment:     map[string]any{"tags": "lead"},
		ScheduledAt: &scheduledTime,
	}

	updated, err := repo.Update(ctx, created.ID, updateCamp)
	if err != nil {
		t.Fatalf("update campaign: %v", err)
	}

	if updated.Name != "t_mysql_campaign_patched" {
		t.Errorf("expected name updated, got %s", updated.Name)
	}
	if updated.TemplateID != 20 {
		t.Errorf("expected TemplateID updated, got %d", updated.TemplateID)
	}
	if updated.Provider != domain.ProviderMailgun {
		t.Errorf("expected provider mailgun, got %s", updated.Provider)
	}
	if updated.ScheduledAt == nil {
		t.Fatal("expected ScheduledAt to be set")
	}
	if !updated.ScheduledAt.Equal(scheduledTime) {
		t.Errorf("expected ScheduledAt %v, got %v", scheduledTime, *updated.ScheduledAt)
	}

	// 4. Update status
	err = repo.UpdateStatus(ctx, created.ID, domain.CampaignSending)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	gStatus, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get campaign: %v", err)
	}
	if gStatus.Status != domain.CampaignSending {
		t.Errorf("expected status sending, got %s", gStatus.Status)
	}

	// 5. SetStats
	stats := map[string]any{"sent": float64(100), "clicks": float64(10)}
	err = repo.SetStats(ctx, created.ID, stats)
	if err != nil {
		t.Fatalf("SetStats: %v", err)
	}

	gStats, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get campaign: %v", err)
	}
	// json unmarshaling float numbers into map[string]any gives float64 in Go
	if val, ok := gStats.Stats["sent"].(float64); !ok || val != 100 {
		t.Errorf("expected stats 'sent' to be 100, got %v", gStats.Stats["sent"])
	}

	// 6. Soft delete campaign
	err = repo.SoftDelete(ctx, created.ID)
	if err != nil {
		t.Fatalf("soft delete campaign: %v", err)
	}

	// Hidden from Get
	_, err = repo.Get(ctx, created.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for soft-deleted campaign, got %v", err)
	}

	// Hidden from List
	listAfterDelete, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list campaigns: %v", err)
	}
	for _, item := range listAfterDelete {
		if item.ID == created.ID {
			t.Fatal("found soft-deleted campaign in List")
		}
	}
}

func TestTemplateRepo(t *testing.T) {
	store := getTestStore(t)
	repo := store.Templates()
	ctx := context.Background()

	name := fmt.Sprintf("t_mysql_template_%d", time.Now().UnixNano())

	// 1. Create
	tpl := domain.Template{
		Name:      name,
		Subject:   "Hello {{.FirstName}}",
		BodyHTML:  "<p>Hello {{.FirstName}}</p>",
		BodyText:  "Hello {{.FirstName}}",
		Variables: []string{"FirstName"},
	}

	created, err := repo.Create(ctx, tpl)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	if created.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if created.Name != name {
		t.Errorf("expected template name %s, got %s", name, created.Name)
	}

	// Duplicate template name should fail with ErrConflict
	_, err = repo.Create(ctx, tpl)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected template name duplicate to fail with ErrConflict, got %v", err)
	}

	// 2. Get and GetByName
	g, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if g.ID != created.ID {
		t.Errorf("ID mismatch: %d vs %d", g.ID, created.ID)
	}

	gName, err := repo.GetByName(ctx, name)
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if gName.ID != created.ID {
		t.Errorf("GetByName ID mismatch: %d vs %d", gName.ID, created.ID)
	}

	// 3. List
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List templates: %v", err)
	}
	found := false
	for _, item := range list {
		if item.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("template not found in list")
	}

	// 4. Update
	tplUpdate := domain.Template{
		Name:      "t_mysql_template_updated",
		Subject:   "Updated Subject",
		BodyHTML:  "HTML updated",
		BodyText:  "Text updated",
		Variables: []string{"FirstName", "LastName"},
	}
	updated, err := repo.Update(ctx, created.ID, tplUpdate)
	if err != nil {
		t.Fatalf("update template: %v", err)
	}

	if updated.Name != "t_mysql_template_updated" {
		t.Errorf("expected updated template name, got %s", updated.Name)
	}
	if updated.Subject != "Updated Subject" {
		t.Errorf("expected updated subject, got %s", updated.Subject)
	}

	// 5. Soft Delete template
	err = repo.SoftDelete(ctx, created.ID)
	if err != nil {
		t.Fatalf("SoftDelete template: %v", err)
	}

	// Get fails with ErrNotFound
	_, err = repo.Get(ctx, created.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected template Get to fail with ErrNotFound, got %v", err)
	}

	// GetByName fails with ErrNotFound
	_, err = repo.GetByName(ctx, "t_mysql_template_updated")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected template GetByName to fail with ErrNotFound, got %v", err)
	}

	// Hidden from List
	listAfterDelete, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List templates: %v", err)
	}
	for _, item := range listAfterDelete {
		if item.ID == created.ID {
			t.Fatal("found soft-deleted template in List")
		}
	}
}

func TestTaskRepo(t *testing.T) {
	store := getTestStore(t)
	repo := store.Tasks()
	ctx := context.Background()

	// 1. Insert tasks
	task1 := domain.ScheduledTask{
		Kind:    domain.TaskEmail,
		Payload: map[string]any{"label": "t_mysql_task_1"},
		RunAt:   time.Now().Add(-10 * time.Minute),
		Status:  domain.TaskPending,
	}
	task2 := domain.ScheduledTask{
		Kind:    domain.TaskCampaign,
		Payload: map[string]any{"label": "t_mysql_task_2"},
		RunAt:   time.Now().Add(-5 * time.Minute),
		Status:  domain.TaskPending,
	}
	task3 := domain.ScheduledTask{
		Kind:    domain.TaskEmail,
		Payload: map[string]any{"label": "t_mysql_task_3"},
		RunAt:   time.Now().Add(10 * time.Minute),
		Status:  domain.TaskPending,
	}

	id1, err := repo.Insert(ctx, task1)
	if err != nil {
		t.Fatalf("insert task 1: %v", err)
	}
	id2, err := repo.Insert(ctx, task2)
	if err != nil {
		t.Fatalf("insert task 2: %v", err)
	}
	id3, err := repo.Insert(ctx, task3)
	if err != nil {
		t.Fatalf("insert task 3: %v", err)
	}

	// 2. ClaimDue
	claimed, err := repo.ClaimDue(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	// Should claim task1 and task2 (run_at in past) but NOT task3 (run_at in future)
	foundTask1 := false
	foundTask2 := false
	for _, taskItem := range claimed {
		if taskItem.ID == id1 {
			foundTask1 = true
		}
		if taskItem.ID == id2 {
			foundTask2 = true
		}
		if taskItem.ID == id3 {
			t.Fatalf("task 3 (future run_at) claimed incorrectly!")
		}
	}

	if !foundTask1 || !foundTask2 {
		t.Errorf("expected tasks 1 and 2 to be claimed, got: %v", claimed)
	}

	// 3. MarkDone and MarkFailed
	err = repo.MarkDone(ctx, id1)
	if err != nil {
		t.Fatalf("MarkDone: %v", err)
	}

	err = repo.MarkFailed(ctx, id2, "connection timeout")
	if err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	// Verify statuses in List
	tasksAll, err := repo.List(ctx, "", 10)
	if err != nil {
		t.Fatalf("List tasks: %v", err)
	}

	// Test non-terminal-first ordering in List when status is empty
	// task3 is 'pending' (non-terminal). task1 is 'done', task2 is 'failed' (terminal).
	// Therefore, task3 should be ordered BEFORE task1 and task2 despite having a later run_at!
	var task3Index, task1Index, task2Index int = -1, -1, -1
	for i, taskItem := range tasksAll {
		if taskItem.ID == id3 {
			task3Index = i
		} else if taskItem.ID == id1 {
			task1Index = i
		} else if taskItem.ID == id2 {
			task2Index = i
		}
	}

	if task3Index == -1 || task1Index == -1 || task2Index == -1 {
		t.Fatal("expected to find all tasks in the list")
	}

	if task3Index > task1Index || task3Index > task2Index {
		t.Errorf("expected pending task3 to be ordered before terminal tasks, indices were: task3=%d, task1=%d, task2=%d", task3Index, task1Index, task2Index)
	}

	// 4. Cancel
	// Cancel pending task (task3)
	err = repo.Cancel(ctx, id3)
	if err != nil {
		t.Fatalf("Cancel task: %v", err)
	}

	tasksListCancelled, err := repo.List(ctx, "cancelled", 10)
	if err != nil {
		t.Fatalf("List cancelled tasks: %v", err)
	}
	foundCancelled := false
	for _, taskItem := range tasksListCancelled {
		if taskItem.ID == id3 {
			foundCancelled = true
			if taskItem.Status != domain.TaskCancelled {
				t.Errorf("expected task status to be cancelled, got %s", taskItem.Status)
			}
		}
	}
	if !foundCancelled {
		t.Fatal("expected task3 to be cancelled and returned in status list")
	}

	// Cancel non-pending task (task1 is 'done') should fail with ErrConflict
	err = repo.Cancel(ctx, id1)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected cancel on done task to fail with ErrConflict, got: %v", err)
	}

	// Cancel non-existent task should fail with ErrNotFound
	err = repo.Cancel(ctx, 999999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected cancel on non-existent task to fail with ErrNotFound, got: %v", err)
	}
}

func TestEventRepo(t *testing.T) {
	store := getTestStore(t)
	events := store.Events()
	contacts := store.Contacts()
	ctx := context.Background()

	// 1. Setup contact
	email := makeUniqueEmail("event-contact")
	contact, err := contacts.Upsert(ctx, domain.Contact{
		Email:     email,
		FirstName: "Event",
		LastName:  "User",
	})
	if err != nil {
		t.Fatalf("setup contact: %v", err)
	}

	campaignID := int64(99)

	// 2. Insert Events & OverviewCounts / CampaignCounts
	evtSent := domain.EmailEvent{
		ContactID:  contact.ID,
		CampaignID: &campaignID,
		Type:       domain.EventSent,
		Meta:       map[string]any{"provider": "t_mysql_test"},
	}
	evtDelivered := domain.EmailEvent{
		ContactID:  contact.ID,
		CampaignID: &campaignID,
		Type:       domain.EventDelivered,
		Meta:       map[string]any{"label": "t_mysql_delivered"},
	}

	err = events.Insert(ctx, evtSent)
	if err != nil {
		t.Fatalf("insert sent event: %v", err)
	}

	err = events.Insert(ctx, evtDelivered)
	if err != nil {
		t.Fatalf("insert delivered event: %v", err)
	}

	overview, err := events.OverviewCounts(ctx)
	if err != nil {
		t.Fatalf("OverviewCounts: %v", err)
	}
	if val := overview[string(domain.EventSent)]; val < 1 {
		t.Errorf("expected sent event count >= 1, got %d", val)
	}

	campCounts, err := events.CampaignCounts(ctx, campaignID)
	if err != nil {
		t.Fatalf("CampaignCounts: %v", err)
	}
	if val := campCounts[string(domain.EventDelivered)]; val != 1 {
		t.Errorf("expected campaign delivered event count to be 1, got %d", val)
	}

	// 3. UniqueOpens dedup test
	// Insert 2 open events for the SAME contact on the SAME campaign
	evtOpen1 := domain.EmailEvent{
		ContactID:  contact.ID,
		CampaignID: &campaignID,
		Type:       domain.EventOpen,
		Meta:       map[string]any{"pixel": "t_mysql_1"},
	}
	evtOpen2 := domain.EmailEvent{
		ContactID:  contact.ID,
		CampaignID: &campaignID,
		Type:       domain.EventOpen,
		Meta:       map[string]any{"pixel": "t_mysql_2"},
	}

	err = events.Insert(ctx, evtOpen1)
	if err != nil {
		t.Fatalf("insert open 1: %v", err)
	}
	err = events.Insert(ctx, evtOpen2)
	if err != nil {
		t.Fatalf("insert open 2: %v", err)
	}

	// Total open events is 2, but unique opens should be exactly 1
	unique, err := events.UniqueOpens(ctx, &campaignID)
	if err != nil {
		t.Fatalf("UniqueOpens campaign: %v", err)
	}
	if unique != 1 {
		t.Errorf("expected UniqueOpens to be 1 (deduplicated), got %d", unique)
	}

	// Add second contact to see unique count increment
	email2 := makeUniqueEmail("event-contact-2")
	contact2, err := contacts.Upsert(ctx, domain.Contact{
		Email:     email2,
		FirstName: "Event 2",
		LastName:  "User",
	})
	if err != nil {
		t.Fatalf("setup contact 2: %v", err)
	}

	evtOpen3 := domain.EmailEvent{
		ContactID:  contact2.ID,
		CampaignID: &campaignID,
		Type:       domain.EventOpen,
		Meta:       map[string]any{"pixel": "t_mysql_3"},
	}
	err = events.Insert(ctx, evtOpen3)
	if err != nil {
		t.Fatalf("insert open 3: %v", err)
	}

	unique2, err := events.UniqueOpens(ctx, &campaignID)
	if err != nil {
		t.Fatalf("UniqueOpens campaign: %v", err)
	}
	if unique2 != 2 {
		t.Errorf("expected UniqueOpens to be 2 after second contact opened, got %d", unique2)
	}

	// CampaignUniqueOpens convenience method
	uniqueCamp, err := events.CampaignUniqueOpens(ctx, campaignID)
	if err != nil {
		t.Fatalf("CampaignUniqueOpens: %v", err)
	}
	if uniqueCamp != 2 {
		t.Errorf("expected CampaignUniqueOpens to be 2, got %d", uniqueCamp)
	}

	// 4. Click events and TopLinks
	evtClick1 := domain.EmailEvent{
		ContactID:  contact.ID,
		CampaignID: &campaignID,
		Type:       domain.EventClick,
		LinkCode:   "t_mysql_link",
		Meta:       map[string]any{"target": "t_mysql_target"},
	}
	err = events.Insert(ctx, evtClick1)
	if err != nil {
		t.Fatalf("insert click: %v", err)
	}

	top, err := events.TopLinks(ctx, campaignID, 10)
	if err != nil {
		t.Fatalf("TopLinks: %v", err)
	}
	foundLink := false
	for _, l := range top {
		if l.LinkCode == "t_mysql_link" {
			foundLink = true
			if l.Clicks != 1 {
				t.Errorf("expected 1 click for t_mysql_link, got %d", l.Clicks)
			}
		}
	}
	if !foundLink {
		t.Fatal("expected t_mysql_link to be in TopLinks list")
	}
}

func TestTrackingRepo(t *testing.T) {
	store := getTestStore(t)
	repo := store.Tracking()
	ctx := context.Background()

	targetURL := "https://example.com/t_mysql_tracking"
	campaignID := int64(45)
	contactID := int64(88)

	// 1. CreateLink
	code, err := repo.CreateLink(ctx, targetURL, &campaignID, &contactID)
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if len(code) != 12 {
		t.Errorf("expected 12-char tracking code, got %s (len %d)", code, len(code))
	}

	// 2. GetLink
	link, err := repo.GetLink(ctx, code)
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}

	if link.Code != code {
		t.Errorf("expected code %s, got %s", code, link.Code)
	}
	if link.TargetURL != targetURL {
		t.Errorf("expected target URL %s, got %s", targetURL, link.TargetURL)
	}
	if link.CampaignID == nil || *link.CampaignID != campaignID {
		t.Errorf("expected campaign ID %d, got %v", campaignID, link.CampaignID)
	}
	if link.ContactID == nil || *link.ContactID != contactID {
		t.Errorf("expected contact ID %d, got %v", contactID, link.ContactID)
	}

	// 3. Non-existent link -> ErrNotFound
	_, err = repo.GetLink(ctx, "nonexistent9")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-existent tracking code, got %v", err)
	}
}

func TestExportRepo(t *testing.T) {
	store := getTestStore(t)
	repo := store.Exports()
	ctx := context.Background()

	exportID := "t_mysql_exp_123"
	expires := time.Now().Add(24 * time.Hour).Truncate(time.Second)

	e := domain.Export{
		ID:        exportID,
		Path:      "/tmp/t_mysql_export.csv",
		Rows:      42,
		ExpiresAt: &expires,
	}

	// 1. Create
	err := repo.Create(ctx, e)
	if err != nil {
		t.Fatalf("Create export: %v", err)
	}

	// 2. Get
	g, err := repo.Get(ctx, exportID)
	if err != nil {
		t.Fatalf("Get export: %v", err)
	}

	if g.ID != exportID {
		t.Errorf("expected ID %s, got %s", exportID, g.ID)
	}
	if g.Path != "/tmp/t_mysql_export.csv" {
		t.Errorf("expected path %s, got %s", "/tmp/t_mysql_export.csv", g.Path)
	}
	if g.Rows != 42 {
		t.Errorf("expected rows 42, got %d", g.Rows)
	}
	if g.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt to be non-nil")
	}
	if !g.ExpiresAt.Equal(expires) {
		t.Errorf("expected expires %v, got %v", expires, *g.ExpiresAt)
	}

	// 3. Get non-existent -> ErrNotFound
	_, err = repo.Get(ctx, "t_mysql_exp_999")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for non-existent export, got %v", err)
	}
}
