package mcptransport_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
	"github.com/cipta/crm-for-aiagents/internal/transport/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestContactGet(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// Seed contact
	c, _ := h.contacts.Upsert(ctx, domain.Contact{
		Email:     "get@test.local",
		FirstName: "Get",
		LastName:  "Test",
		Company:   "TestCo",
	})

	// 1. Get by ID
	res, out, err := h.deps.ContactGet(ctx, nil, mcptransport.ContactGetIn{ID: c.ID})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("unexpected error getting by id: %v, %v", err, res)
	}
	if out.ID != c.ID || out.Email != "get@test.local" || out.FirstName != "Get" {
		t.Errorf("unexpected output: %+v", out)
	}

	// 2. Get by Email
	res, out, err = h.deps.ContactGet(ctx, nil, mcptransport.ContactGetIn{Email: "get@test.local"})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("unexpected error getting by email: %v, %v", err, res)
	}
	if out.ID != c.ID || out.Email != "get@test.local" {
		t.Errorf("unexpected output: %+v", out)
	}

	// 3. Not found
	res, _, err = h.deps.ContactGet(ctx, nil, mcptransport.ContactGetIn{ID: 9999})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected error response")
	}
	var errEnv mcpserver.ErrEnvelope
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errEnv.Error != "not_found" {
		t.Errorf("expected not_found, got: %s", errEnv.Error)
	}

	// 4. Neither ID nor Email provided (invalid_input)
	res, _, err = h.deps.ContactGet(ctx, nil, mcptransport.ContactGetIn{})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected invalid_input error response")
	}
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errEnv.Error != "invalid_input" {
		t.Errorf("expected invalid_input, got: %s", errEnv.Error)
	}
}

func TestContactDelete(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// Seed contact for soft-delete
	c1, _ := h.contacts.Upsert(ctx, domain.Contact{Email: "del1@test.local"})
	// Seed contact for purge
	c2, _ := h.contacts.Upsert(ctx, domain.Contact{Email: "del2@test.local"})

	// 1. Soft-delete
	res, out, err := h.deps.ContactDelete(ctx, nil, mcptransport.ContactDeleteIn{ID: c1.ID, Purge: false})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("failed soft-delete: %v, %v", err, res)
	}
	if out.ID != c1.ID || !out.Deleted || out.Purged {
		t.Errorf("unexpected output: %+v", out)
	}

	// Verify soft-deleted contact is no longer retrievable
	resGet, _, _ := h.deps.ContactGet(ctx, nil, mcptransport.ContactGetIn{ID: c1.ID})
	if resGet == nil || !resGet.IsError {
		t.Error("expected soft-deleted contact to be not found")
	}

	// 2. Purge (hard delete)
	res, out, err = h.deps.ContactDelete(ctx, nil, mcptransport.ContactDeleteIn{ID: c2.ID, Purge: true})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("failed hard-delete: %v, %v", err, res)
	}
	if out.ID != c2.ID || !out.Deleted || !out.Purged {
		t.Errorf("unexpected output: %+v", out)
	}

	// Verify hard-deleted contact is completely gone
	resGet2, _, _ := h.deps.ContactGet(ctx, nil, mcptransport.ContactGetIn{ID: c2.ID})
	if resGet2 == nil || !resGet2.IsError {
		t.Error("expected purged contact to be not found")
	}

	// 3. Delete not found
	res, _, err = h.deps.ContactDelete(ctx, nil, mcptransport.ContactDeleteIn{ID: 9999})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected error envelope")
	}
	var errEnv mcpserver.ErrEnvelope
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errEnv.Error != "not_found" {
		t.Errorf("expected not_found, got: %s", errEnv.Error)
	}
}

func TestContactUnsubscribe(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// Seed contact
	c, _ := h.contacts.Upsert(ctx, domain.Contact{Email: "unsub@test.local"})

	// Verify unsubscribed=false initially
	resGet, outGet, err := h.deps.ContactGet(ctx, nil, mcptransport.ContactGetIn{ID: c.ID})
	if err != nil || (resGet != nil && resGet.IsError) {
		t.Fatalf("failed to get: %v, %v", err, resGet)
	}
	if outGet.Unsubscribed {
		t.Error("expected unsubscribed to be false initially")
	}

	// Unsubscribe
	res, out, err := h.deps.ContactUnsubscribe(ctx, nil, mcptransport.ContactUnsubscribeIn{ID: c.ID})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("failed unsubscribe: %v, %v", err, res)
	}
	if out.ID != c.ID || !out.Unsubscribed {
		t.Errorf("unexpected output: %+v", out)
	}

	// Verify subsequent get shows unsubscribed=true
	resGet, outGet, err = h.deps.ContactGet(ctx, nil, mcptransport.ContactGetIn{ID: c.ID})
	if err != nil || (resGet != nil && resGet.IsError) {
		t.Fatalf("failed to get second time: %v, %v", err, resGet)
	}
	if !outGet.Unsubscribed {
		t.Error("expected unsubscribed to be true after unsubscribing")
	}
}

func TestCampaignListAndGetAndUpdateAndDelete(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// Seed campaign
	camp1, _ := h.campaigns.Create(ctx, domain.Campaign{
		Name:       "Campaign One",
		TemplateID: 101,
		Provider:   domain.ProviderSMTP,
	})

	// 1. Campaign List
	resList, outList, err := h.deps.CampaignList(ctx, nil, mcptransport.CampaignListIn{})
	if err != nil || (resList != nil && resList.IsError) {
		t.Fatalf("failed to list campaigns: %v, %v", err, resList)
	}
	if outList.Count != 1 {
		t.Errorf("expected 1 campaign, got %d", outList.Count)
	}
	item := outList.Items[0]
	if item["id"] != camp1.ID || item["name"] != "Campaign One" || item["provider"] != "smtp" {
		t.Errorf("unexpected list item: %+v", item)
	}

	// 2. Campaign Get
	resGet, outGet, err := h.deps.CampaignGet(ctx, nil, mcptransport.CampaignGetIn{ID: camp1.ID})
	if err != nil || (resGet != nil && resGet.IsError) {
		t.Fatalf("failed to get campaign: %v, %v", err, resGet)
	}
	if outGet.ID != camp1.ID || outGet.Name != "Campaign One" || outGet.Provider != "smtp" {
		t.Errorf("unexpected get output: %+v", outGet)
	}

	// 3. Campaign Get Not Found
	resGetBad, _, err := h.deps.CampaignGet(ctx, nil, mcptransport.CampaignGetIn{ID: 9999})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if resGetBad == nil || !resGetBad.IsError {
		t.Fatal("expected error envelope")
	}
	var errEnv mcpserver.ErrEnvelope
	if err := json.Unmarshal([]byte(resGetBad.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errEnv.Error != "not_found" {
		t.Errorf("expected not_found, got: %s", errEnv.Error)
	}

	// 4. Campaign Update (Happy Path)
	newName := "Campaign Updated"
	newProvider := "mailgun"
	resUpdate, outUpdate, err := h.deps.CampaignUpdate(ctx, nil, mcptransport.CampaignUpdateIn{
		ID:       camp1.ID,
		Name:     &newName,
		Provider: &newProvider,
	})
	if err != nil || (resUpdate != nil && resUpdate.IsError) {
		t.Fatalf("failed to update campaign: %v, %v", err, resUpdate)
	}
	if outUpdate.ID != camp1.ID || outUpdate.Name != "Campaign Updated" {
		t.Errorf("unexpected update output: %+v", outUpdate)
	}

	// Verify updated fields in Get
	_, outGet2, _ := h.deps.CampaignGet(ctx, nil, mcptransport.CampaignGetIn{ID: camp1.ID})
	if outGet2.Name != "Campaign Updated" || outGet2.Provider != "mailgun" {
		t.Errorf("updated fields not persisted: %+v", outGet2)
	}

	// 5. Campaign Update (Bad Provider validation -> invalid_input)
	badProvider := "invalid-provider"
	resUpdateBad, _, err := h.deps.CampaignUpdate(ctx, nil, mcptransport.CampaignUpdateIn{
		ID:       camp1.ID,
		Provider: &badProvider,
	})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if resUpdateBad == nil || !resUpdateBad.IsError {
		t.Fatal("expected validation error response")
	}
	if err := json.Unmarshal([]byte(resUpdateBad.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errEnv.Error != "invalid_input" {
		t.Errorf("expected invalid_input, got: %s", errEnv.Error)
	}

	// 6. Campaign Delete
	resDel, outDel, err := h.deps.CampaignDelete(ctx, nil, mcptransport.CampaignDeleteIn{ID: camp1.ID})
	if err != nil || (resDel != nil && resDel.IsError) {
		t.Fatalf("failed campaign delete: %v, %v", err, resDel)
	}
	if outDel.ID != camp1.ID || !outDel.Deleted {
		t.Errorf("unexpected output: %+v", outDel)
	}

	// Verify no longer retrievable
	resGetDeleted, _, _ := h.deps.CampaignGet(ctx, nil, mcptransport.CampaignGetIn{ID: camp1.ID})
	if resGetDeleted == nil || !resGetDeleted.IsError {
		t.Error("expected campaign to be soft-deleted")
	}
}

func TestTemplateGetAndUpdateAndDelete(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// Seed Template
	tmpl, _ := h.templates.Create(ctx, domain.Template{
		Name:    "welcome",
		Subject: "Welcome Subject",
	})

	// 1. Template Get by ID
	resGet, outGet, err := h.deps.TemplateGet(ctx, nil, mcptransport.TemplateGetIn{ID: tmpl.ID})
	if err != nil || (resGet != nil && resGet.IsError) {
		t.Fatalf("failed template get by id: %v, %v", err, resGet)
	}
	if outGet.ID != tmpl.ID || outGet.Name != "welcome" || outGet.Subject != "Welcome Subject" {
		t.Errorf("unexpected get output: %+v", outGet)
	}

	// 2. Template Get by Name
	resGet2, outGet2, err := h.deps.TemplateGet(ctx, nil, mcptransport.TemplateGetIn{Name: "welcome"})
	if err != nil || (resGet2 != nil && resGet2.IsError) {
		t.Fatalf("failed template get by name: %v, %v", err, resGet2)
	}
	if outGet2.ID != tmpl.ID || outGet2.Name != "welcome" {
		t.Errorf("unexpected get output: %+v", outGet2)
	}

	// 3. Template Get Not Found
	resGetBad, _, err := h.deps.TemplateGet(ctx, nil, mcptransport.TemplateGetIn{ID: 9999})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if resGetBad == nil || !resGetBad.IsError {
		t.Fatal("expected error response")
	}
	var errEnv mcpserver.ErrEnvelope
	if err := json.Unmarshal([]byte(resGetBad.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errEnv.Error != "not_found" {
		t.Errorf("expected not_found, got: %s", errEnv.Error)
	}

	// 4. Template Update
	newName := "welcome-new"
	newSubject := "Welcome to CRM"
	resUpdate, outUpdate, err := h.deps.TemplateUpdate(ctx, nil, mcptransport.TemplateUpdateIn{
		ID:      tmpl.ID,
		Name:    &newName,
		Subject: &newSubject,
	})
	if err != nil || (resUpdate != nil && resUpdate.IsError) {
		t.Fatalf("failed template update: %v, %v", err, resUpdate)
	}
	if outUpdate.ID != tmpl.ID || outUpdate.Name != "welcome-new" {
		t.Errorf("unexpected output: %+v", outUpdate)
	}

	// Verify changes
	_, outGet3, _ := h.deps.TemplateGet(ctx, nil, mcptransport.TemplateGetIn{ID: tmpl.ID})
	if outGet3.Name != "welcome-new" || outGet3.Subject != "Welcome to CRM" {
		t.Errorf("updates not persisted: %+v", outGet3)
	}

	// 5. Template Delete
	resDel, outDel, err := h.deps.TemplateDelete(ctx, nil, mcptransport.TemplateDeleteIn{ID: tmpl.ID})
	if err != nil || (resDel != nil && resDel.IsError) {
		t.Fatalf("failed template delete: %v, %v", err, resDel)
	}
	if outDel.ID != tmpl.ID || !outDel.Deleted {
		t.Errorf("unexpected output: %+v", outDel)
	}

	// Verify no longer retrievable
	resGetDeleted, _, _ := h.deps.TemplateGet(ctx, nil, mcptransport.TemplateGetIn{ID: tmpl.ID})
	if resGetDeleted == nil || !resGetDeleted.IsError {
		t.Error("expected template to be soft-deleted")
	}
}

func TestTaskListAndCancel(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// 1. Seed tasks via Schedule
	resSched1, outSched1, err := h.deps.ScheduleTask(ctx, nil, mcptransport.ScheduleTaskIn{
		Kind:    "email",
		Payload: map[string]any{"to": "t1@test.local"},
		RunAt:   "2026-06-03T15:00:00Z",
	})
	if err != nil || (resSched1 != nil && resSched1.IsError) {
		t.Fatalf("failed schedule: %v, %v", err, resSched1)
	}

	resSched2, outSched2, err := h.deps.ScheduleTask(ctx, nil, mcptransport.ScheduleTaskIn{
		Kind:    "campaign",
		Payload: map[string]any{"campaign_id": float64(10)},
		RunAt:   "2026-06-03T16:00:00Z",
	})
	if err != nil || (resSched2 != nil && resSched2.IsError) {
		t.Fatalf("failed schedule: %v, %v", err, resSched2)
	}

	// 2. Task List
	resList, outList, err := h.deps.TaskList(ctx, nil, mcptransport.TaskListIn{})
	if err != nil || (resList != nil && resList.IsError) {
		t.Fatalf("failed task list: %v, %v", err, resList)
	}
	if outList.Count != 2 {
		t.Errorf("expected 2 tasks, got %d", outList.Count)
	}

	// 3. Task Cancel (Happy Path -> Cancel pending task)
	resCancel, outCancel, err := h.deps.TaskCancel(ctx, nil, mcptransport.TaskCancelIn{ID: outSched1.TaskID})
	if err != nil || (resCancel != nil && resCancel.IsError) {
		t.Fatalf("failed to cancel pending task: %v, %v", err, resCancel)
	}
	if outCancel.ID != outSched1.TaskID || !outCancel.Cancelled {
		t.Errorf("unexpected cancel output: %+v", outCancel)
	}

	// 4. Cancel Non-Pending Task -> Conflict
	// Modify task status to "running" directly
	runningTask, ok := h.tasks.tasks[outSched2.TaskID]
	if !ok {
		t.Fatalf("task not found in fake: %d", outSched2.TaskID)
	}
	runningTask.Status = domain.TaskRunning
	h.tasks.tasks[outSched2.TaskID] = runningTask

	resCancelBad, _, err := h.deps.TaskCancel(ctx, nil, mcptransport.TaskCancelIn{ID: outSched2.TaskID})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if resCancelBad == nil || !resCancelBad.IsError {
		t.Fatal("expected conflict error envelope")
	}
	var errEnv mcpserver.ErrEnvelope
	if err := json.Unmarshal([]byte(resCancelBad.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errEnv.Error != "conflict" {
		t.Errorf("expected conflict, got: %s", errEnv.Error)
	}
	if !strings.Contains(errEnv.Msg, "not pending") {
		t.Errorf("expected message to say 'not pending', got %q", errEnv.Msg)
	}

	// 5. Cancel missing task -> Not Found
	resCancelMissing, _, err := h.deps.TaskCancel(ctx, nil, mcptransport.TaskCancelIn{ID: 9999})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if resCancelMissing == nil || !resCancelMissing.IsError {
		t.Fatal("expected not_found error envelope")
	}
	if err := json.Unmarshal([]byte(resCancelMissing.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errEnv.Error != "not_found" {
		t.Errorf("expected not_found, got: %s", errEnv.Error)
	}
}
