package mcptransport_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/service"
	"github.com/incredible-zetta/crm/internal/transport/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- In-Memory Fake Repositories ---

type fakeContactRepo struct {
	contacts map[int64]domain.Contact
	nextID   int64
}

func (r *fakeContactRepo) Upsert(ctx context.Context, c domain.Contact) (domain.Contact, error) {
	if c.ID == 0 {
		c.ID = r.nextID
		r.nextID++
	}
	r.contacts[c.ID] = c
	return c, nil
}

func (r *fakeContactRepo) Get(ctx context.Context, id int64) (domain.Contact, error) {
	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.Contact{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *fakeContactRepo) GetByEmail(ctx context.Context, email string) (domain.Contact, error) {
	for _, c := range r.contacts {
		if strings.EqualFold(c.Email, email) && c.DeletedAt == nil {
			return c, nil
		}
	}
	return domain.Contact{}, domain.ErrNotFound
}

func (r *fakeContactRepo) GetByUnsubCode(ctx context.Context, code string) (domain.Contact, error) {
	return domain.Contact{}, domain.ErrNotFound
}

func (r *fakeContactRepo) Update(ctx context.Context, id int64, patch domain.ContactPatch) (domain.Contact, error) {
	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.Contact{}, domain.ErrNotFound
	}
	if patch.Email != nil {
		c.Email = *patch.Email
	}
	if patch.FirstName != nil {
		c.FirstName = *patch.FirstName
	}
	if patch.LastName != nil {
		c.LastName = *patch.LastName
	}
	if patch.Company != nil {
		c.Company = *patch.Company
	}
	if patch.Phone != nil {
		c.Phone = *patch.Phone
	}
	if patch.Stage != nil {
		c.Stage = domain.Stage(*patch.Stage)
	}
	if patch.Tags != nil {
		c.Tags = *patch.Tags
	}
	if patch.Notes != nil {
		c.Notes = *patch.Notes
	}
	if patch.Custom != nil {
		c.Custom = *patch.Custom
	}
	if patch.Source != nil {
		c.Source = *patch.Source
	}
	r.contacts[id] = c
	return c, nil
}

func (r *fakeContactRepo) List(ctx context.Context, f domain.ContactFilter, p port.Paging) (port.ContactPage, error) {
	var matched []domain.Contact
	for _, c := range r.contacts {
		if c.DeletedAt != nil {
			continue
		}
		if f.Stage != "" && string(c.Stage) != f.Stage {
			continue
		}
		if f.Company != "" && !strings.Contains(strings.ToLower(c.Company), strings.ToLower(f.Company)) {
			continue
		}
		matched = append(matched, c)
	}
	return port.ContactPage{
		Items:      matched,
		Total:      len(matched),
		NextCursor: 0,
	}, nil
}

func (r *fakeContactRepo) SetUnsubscribed(ctx context.Context, id int64, t time.Time) error {
	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.ErrNotFound
	}
	c.UnsubscribedAt = &t
	r.contacts[id] = c
	return nil
}

func (r *fakeContactRepo) SetUnsubCode(ctx context.Context, id int64, code string) error {
	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.ErrNotFound
	}
	c.UnsubCode = code
	r.contacts[id] = c
	return nil
}

func (r *fakeContactRepo) SoftDelete(ctx context.Context, id int64) error {
	c, ok := r.contacts[id]
	if !ok {
		return domain.ErrNotFound
	}
	now := time.Now()
	c.DeletedAt = &now
	r.contacts[id] = c
	return nil
}

func (r *fakeContactRepo) Purge(ctx context.Context, id int64) error {
	_, ok := r.contacts[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(r.contacts, id)
	return nil
}

func (r *fakeContactRepo) CountByStage(ctx context.Context) (map[string]int, error) {
	stages := make(map[string]int)
	for _, c := range r.contacts {
		if c.DeletedAt != nil {
			continue
		}
		stages[string(c.Stage)]++
	}
	return stages, nil
}

type fakeCampaignRepo struct {
	campaigns map[int64]domain.Campaign
	nextID    int64
}

func (r *fakeCampaignRepo) Create(ctx context.Context, c domain.Campaign) (domain.Campaign, error) {
	if c.ID == 0 {
		c.ID = r.nextID
		r.nextID++
	}
	r.campaigns[c.ID] = c
	return c, nil
}

func (r *fakeCampaignRepo) Get(ctx context.Context, id int64) (domain.Campaign, error) {
	c, ok := r.campaigns[id]
	if !ok || c.DeletedAt != nil {
		return domain.Campaign{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *fakeCampaignRepo) List(ctx context.Context) ([]domain.Campaign, error) {
	var list []domain.Campaign
	for _, c := range r.campaigns {
		if c.DeletedAt != nil {
			continue
		}
		list = append(list, c)
	}
	return list, nil
}

func (r *fakeCampaignRepo) Update(ctx context.Context, id int64, c domain.Campaign) (domain.Campaign, error) {
	existing, ok := r.campaigns[id]
	if !ok || existing.DeletedAt != nil {
		return domain.Campaign{}, domain.ErrNotFound
	}
	r.campaigns[id] = c
	return c, nil
}

func (r *fakeCampaignRepo) UpdateStatus(ctx context.Context, id int64, s domain.CampaignStatus) error {
	c, ok := r.campaigns[id]
	if !ok || c.DeletedAt != nil {
		return domain.ErrNotFound
	}
	c.Status = s
	r.campaigns[id] = c
	return nil
}

func (r *fakeCampaignRepo) SetStats(ctx context.Context, id int64, stats map[string]any) error {
	return nil
}

func (r *fakeCampaignRepo) SoftDelete(ctx context.Context, id int64) error {
	c, ok := r.campaigns[id]
	if !ok {
		return domain.ErrNotFound
	}
	now := time.Now()
	c.DeletedAt = &now
	r.campaigns[id] = c
	return nil
}

type fakeTemplateRepo struct {
	templates map[int64]domain.Template
	nextID    int64
}

func (r *fakeTemplateRepo) Create(ctx context.Context, t domain.Template) (domain.Template, error) {
	if t.ID == 0 {
		t.ID = r.nextID
		r.nextID++
	}
	r.templates[t.ID] = t
	return t, nil
}

func (r *fakeTemplateRepo) Get(ctx context.Context, id int64) (domain.Template, error) {
	t, ok := r.templates[id]
	if !ok || t.DeletedAt != nil {
		return domain.Template{}, domain.ErrNotFound
	}
	return t, nil
}

func (r *fakeTemplateRepo) GetByName(ctx context.Context, name string) (domain.Template, error) {
	for _, t := range r.templates {
		if t.Name == name && t.DeletedAt == nil {
			return t, nil
		}
	}
	return domain.Template{}, domain.ErrNotFound
}

func (r *fakeTemplateRepo) List(ctx context.Context) ([]domain.Template, error) {
	var list []domain.Template
	for _, t := range r.templates {
		if t.DeletedAt != nil {
			continue
		}
		list = append(list, t)
	}
	return list, nil
}

func (r *fakeTemplateRepo) Update(ctx context.Context, id int64, t domain.Template) (domain.Template, error) {
	existing, ok := r.templates[id]
	if !ok || existing.DeletedAt != nil {
		return domain.Template{}, domain.ErrNotFound
	}
	r.templates[id] = t
	return t, nil
}

func (r *fakeTemplateRepo) SoftDelete(ctx context.Context, id int64) error {
	t, ok := r.templates[id]
	if !ok {
		return domain.ErrNotFound
	}
	now := time.Now()
	t.DeletedAt = &now
	r.templates[id] = t
	return nil
}

type fakeTaskRepo struct {
	tasks  map[int64]domain.ScheduledTask
	nextID int64
}

func (r *fakeTaskRepo) Insert(ctx context.Context, t domain.ScheduledTask) (int64, error) {
	if t.ID == 0 {
		t.ID = r.nextID
		r.nextID++
	}
	r.tasks[t.ID] = t
	return t.ID, nil
}

func (r *fakeTaskRepo) List(ctx context.Context, status string, limit int) ([]domain.ScheduledTask, error) {
	var matched []domain.ScheduledTask
	for _, t := range r.tasks {
		if status != "" && string(t.Status) != status {
			continue
		}
		matched = append(matched, t)
		if len(matched) >= limit {
			break
		}
	}
	return matched, nil
}

func (r *fakeTaskRepo) ClaimDue(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledTask, error) {
	return nil, nil
}

func (r *fakeTaskRepo) MarkDone(ctx context.Context, id int64) error {
	return nil
}

func (r *fakeTaskRepo) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	return nil
}

func (r *fakeTaskRepo) Cancel(ctx context.Context, id int64) error {
	t, ok := r.tasks[id]
	if !ok {
		return domain.ErrNotFound
	}
	if t.Status != domain.TaskPending {
		return fmt.Errorf("task not pending: %w", domain.ErrConflict)
	}
	t.Status = domain.TaskCancelled
	r.tasks[id] = t
	return nil
}

type fakeEventRepo struct {
	events []domain.EmailEvent
}

func (r *fakeEventRepo) Insert(ctx context.Context, e domain.EmailEvent) error {
	r.events = append(r.events, e)
	return nil
}

func (r *fakeEventRepo) CampaignCounts(ctx context.Context, id int64) (map[string]int, error) {
	counts := make(map[string]int)
	for _, e := range r.events {
		if e.CampaignID != nil && *e.CampaignID == id {
			counts[string(e.Type)]++
		}
	}
	return counts, nil
}

func (r *fakeEventRepo) CampaignUniqueOpens(ctx context.Context, id int64) (int, error) {
	seen := make(map[int64]bool)
	for _, e := range r.events {
		if e.CampaignID != nil && *e.CampaignID == id && e.Type == domain.EventOpen {
			seen[e.ContactID] = true
		}
	}
	return len(seen), nil
}

func (r *fakeEventRepo) TopLinks(ctx context.Context, campaignID int64, limit int) ([]domain.LinkCount, error) {
	return []domain.LinkCount{
		{LinkCode: "testcode", Clicks: 5},
	}, nil
}

func (r *fakeEventRepo) OverviewCounts(ctx context.Context) (map[string]int, error) {
	counts := make(map[string]int)
	for _, e := range r.events {
		counts[string(e.Type)]++
	}
	return counts, nil
}

func (r *fakeEventRepo) UniqueOpens(ctx context.Context, campaignID *int64) (int, error) {
	seen := make(map[int64]bool)
	for _, e := range r.events {
		if e.Type == domain.EventOpen {
			seen[e.ContactID] = true
		}
	}
	return len(seen), nil
}

type fakeTrackingRepo struct {
	links map[string]domain.TrackingLink
}

func (r *fakeTrackingRepo) CreateLink(ctx context.Context, targetURL string, campaignID, contactID *int64) (string, error) {
	code := "lnk123"
	r.links[code] = domain.TrackingLink{
		Code:       code,
		TargetURL:  targetURL,
		CampaignID: campaignID,
		ContactID:  contactID,
	}
	return code, nil
}

func (r *fakeTrackingRepo) GetLink(ctx context.Context, code string) (domain.TrackingLink, error) {
	l, ok := r.links[code]
	if !ok {
		return domain.TrackingLink{}, domain.ErrNotFound
	}
	return l, nil
}

type fakeExportRepo struct {
	exports map[string]domain.Export
}

func (r *fakeExportRepo) Create(ctx context.Context, e domain.Export) error {
	r.exports[e.ID] = e
	return nil
}

func (r *fakeExportRepo) Get(ctx context.Context, id string) (domain.Export, error) {
	return domain.Export{}, domain.ErrNotFound
}

type fakeSender struct {
	sent   []port.OutboundMessage
	failOn string
}

func (s *fakeSender) Send(ctx context.Context, msg port.OutboundMessage) error {
	if s.failOn != "" && strings.Contains(msg.To, s.failOn) {
		return errors.New("network error sending email")
	}
	s.sent = append(s.sent, msg)
	return nil
}

type fakeClock struct{}

func (fakeClock) Now() time.Time {
	return time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
}

type fakeIDGen struct{}

func (fakeIDGen) ExportID() (string, error)  { return "exp123", nil }
func (fakeIDGen) UnsubCode() (string, error) { return "unsub123", nil }

// --- Test Setup ---

type testHarness struct {
	deps       *mcptransport.Deps
	contacts   *fakeContactRepo
	campaigns  *fakeCampaignRepo
	templates  *fakeTemplateRepo
	tasks      *fakeTaskRepo
	events     *fakeEventRepo
	tracking   *fakeTrackingRepo
	exports    *fakeExportRepo
	fakeSender *fakeSender
}

func setupTestDeps(t *testing.T) *testHarness {
	contacts := &fakeContactRepo{contacts: make(map[int64]domain.Contact), nextID: 1}
	campaigns := &fakeCampaignRepo{campaigns: make(map[int64]domain.Campaign), nextID: 1}
	templates := &fakeTemplateRepo{templates: make(map[int64]domain.Template), nextID: 1}
	tasks := &fakeTaskRepo{tasks: make(map[int64]domain.ScheduledTask), nextID: 1}
	events := &fakeEventRepo{events: []domain.EmailEvent{}}
	tracking := &fakeTrackingRepo{links: make(map[string]domain.TrackingLink)}
	exports := &fakeExportRepo{exports: make(map[string]domain.Export)}
	fsender := &fakeSender{}

	repos := service.Repos{
		Contacts:  contacts,
		Campaigns: campaigns,
		Templates: templates,
		Tasks:     tasks,
		Events:    events,
		Tracking:  tracking,
		Exports:   exports,
	}

	cfg := service.Config{
		BaseURL:   "http://test.local",
		ExportDir: t.TempDir(),
	}

	services := service.New(repos, fsender, fakeClock{}, fakeIDGen{}, cfg)

	deps := &mcptransport.Deps{
		Svc:     services,
		Version: "1.2.3",
		PingDB: func(ctx context.Context) error {
			return nil
		},
		PingSMTP: func(ctx context.Context) error {
			return nil
		},
		PingMailgun: func(ctx context.Context) error {
			return nil
		},
	}

	return &testHarness{
		deps:       deps,
		contacts:   contacts,
		campaigns:  campaigns,
		templates:  templates,
		tasks:      tasks,
		events:     events,
		tracking:   tracking,
		exports:    exports,
		fakeSender: fsender,
	}
}

// --- Test Cases ---

func TestContactCreate(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// Happy path
	res, out, err := h.deps.ContactCreate(ctx, nil, mcptransport.ContactCreateIn{
		Email:     "jane@test.local",
		FirstName: "Jane",
		LastName:  "Doe",
		Stage:     "new",
	})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("unexpected tool error: %v", res)
	}
	if out.ID == 0 || out.Email != "jane@test.local" || out.Stage != "new" {
		t.Errorf("unexpected output: %+v", out)
	}

	// Bad stage validation (invalid_input or bad_stage)
	res, _, err = h.deps.ContactCreate(ctx, nil, mcptransport.ContactCreateIn{
		Email: "john@test.local",
		Stage: "invalid-stage-name",
	})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected validation error envelope")
	}

	var errEnv mcpserver.ErrEnvelope
	errText := res.Content[0].(*mcp.TextContent).Text
	if err := json.Unmarshal([]byte(errText), &errEnv); err != nil {
		t.Fatalf("failed to parse error envelope: %v", err)
	}
	if errEnv.Error != "bad_stage" && errEnv.Error != "invalid_input" {
		t.Errorf("expected validation code, got: %s", errEnv.Error)
	}
	if !strings.Contains(errEnv.Msg, "invalid stage") {
		t.Errorf("expected safe message about stage, got: %s", errEnv.Msg)
	}
}

func TestContactListAndProjection(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	_, _ = h.contacts.Upsert(ctx, domain.Contact{
		Email:     "jane@test.local",
		FirstName: "Jane",
		LastName:  "Doe",
		Company:   "Acme",
		Stage:     domain.StageNew,
		Phone:     "12345",
	})

	// 1. Unprojected list (default fields)
	res, out, err := h.deps.ContactList(ctx, nil, mcptransport.ContactListIn{
		Limit: 10,
	})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("failed to list contacts: %v, %v", err, res)
	}
	if out.Count != 1 || out.Total != 1 {
		t.Errorf("expected total/count 1, got total %d, count %d", out.Total, out.Count)
	}
	item := out.Items[0]
	if item["email"] != "jane@test.local" || item["stage"] != "new" {
		t.Errorf("unexpected projection item: %+v", item)
	}
	if _, ok := item["phone"]; ok {
		t.Error("phone should be omitted in default projection")
	}

	// 2. Projected list (specific fields subset)
	res, out, err = h.deps.ContactList(ctx, nil, mcptransport.ContactListIn{
		Limit:  10,
		Fields: []string{"email", "phone"},
	})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("failed to list projected: %v", err)
	}
	item = out.Items[0]
	if item["email"] != "jane@test.local" || item["phone"] != "12345" {
		t.Errorf("unexpected projected fields: %+v", item)
	}
	if _, ok := item["stage"]; ok {
		t.Error("stage should be omitted in phone/email projection")
	}
}

func TestContactExport(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	_, _ = h.contacts.Upsert(ctx, domain.Contact{
		Email: "test-export@test.local",
	})

	res, out, err := h.deps.ContactExport(ctx, nil, mcptransport.ContactExportIn{})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("failed to export: %v, %v", err, res)
	}

	if out.URL == "" {
		t.Error("expected non-empty URL")
	}
	if !strings.HasPrefix(out.URL, "http://test.local/export/") {
		t.Errorf("expected valid local URL prefix, got %s", out.URL)
	}
	if out.Rows != 1 {
		t.Errorf("expected 1 row exported, got %d", out.Rows)
	}
}

func TestEmailSend(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	c, _ := h.contacts.Upsert(ctx, domain.Contact{
		Email: "recipient@test.local",
		Stage: domain.StageNew,
	})

	// 1. Happy path send (direct)
	res, out, err := h.deps.EmailSend(ctx, nil, mcptransport.EmailSendIn{
		To:      "recipient@test.local",
		Subject: "Hello",
		Text:    "World",
	})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("failed to send: %v, %v", err, res)
	}
	if out.Status != "sent" || out.To != "recipient@test.local" {
		t.Errorf("unexpected output: %+v", out)
	}

	// 2. Unsubscribed recipient validation
	unsub, _ := h.contacts.Upsert(ctx, domain.Contact{
		Email: "unsub@test.local",
		Stage: domain.StageNew,
	})
	_ = h.contacts.SetUnsubscribed(ctx, unsub.ID, time.Now())

	res, _, err = h.deps.EmailSend(ctx, nil, mcptransport.EmailSendIn{
		ContactID: unsub.ID,
		Subject:   "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected validation error for unsubscribed contact")
	}
	var errEnv mcpserver.ErrEnvelope
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errEnv.Error != "invalid_input" {
		t.Errorf("expected invalid_input error code, got: %s", errEnv.Error)
	}
	if !strings.Contains(errEnv.Msg, "unsubscribed") {
		t.Errorf("expected unsubscribed msg, got: %s", errEnv.Msg)
	}

	// 3. Recipient mismatch
	res, _, err = h.deps.EmailSend(ctx, nil, mcptransport.EmailSendIn{
		ContactID: c.ID,
		To:        "mismatch@test.local",
		Subject:   "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected validation error for recipient mismatch")
	}
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errEnv.Error != "invalid_input" {
		t.Errorf("expected invalid_input error code, got: %s", errEnv.Error)
	}
	if !strings.Contains(errEnv.Msg, "match") {
		t.Errorf("expected match msg, got: %s", errEnv.Msg)
	}

	// 4. Infrastructure Send failure (network error/failOn)
	h.fakeSender.failOn = "fail@test.local"
	res, _, err = h.deps.EmailSend(ctx, nil, mcptransport.EmailSendIn{
		To:      "fail@test.local",
		Subject: "test infra failure",
	})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected send_failed tool error")
	}
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errEnv.Error != "send_failed" {
		t.Errorf("expected send_failed error code, got: %s", errEnv.Error)
	}
	if !strings.Contains(errEnv.Msg, "network error sending email") {
		t.Errorf("expected underlying send error detail in msg, got: %s", errEnv.Msg)
	}
}

func TestTemplateRender(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// 1. Render default text-only
	res, out, err := h.deps.TemplateRender(ctx, nil, mcptransport.TemplateRenderIn{
		Subject:  "Hello {{.name}}",
		BodyText: "Text: {{.name}}",
		Vars:     map[string]any{"name": "Alice"},
	})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("failed to render: %v, %v", err, res)
	}
	if out.Subject != "Hello Alice" || out.Text != "Text: Alice" {
		t.Errorf("unexpected rendering output: %+v", out)
	}
	if out.HTML != "" {
		t.Error("HTML should be omitted when HTML=false")
	}

	// 2. Render HTML when requested
	res, out, err = h.deps.TemplateRender(ctx, nil, mcptransport.TemplateRenderIn{
		Subject:  "Hello {{.name}}",
		BodyHTML: "<b>{{.name}}</b>",
		Vars:     map[string]any{"name": "Alice"},
		HTML:     true,
	})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("failed to render HTML: %v", err)
	}
	if out.HTML != "<b>Alice</b>" {
		t.Errorf("expected html <b>Alice</b>, got %q", out.HTML)
	}

	// 3. Render failures explain the template error so operators can fix input.
	res, _, err = h.deps.TemplateRender(ctx, nil, mcptransport.TemplateRenderIn{
		Subject: "Hello {{.name",
		Vars:    map[string]any{"name": "Alice"},
	})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected render_failed tool error")
	}
	var errEnv mcpserver.ErrEnvelope
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errEnv.Error != "render_failed" {
		t.Fatalf("expected render_failed, got %s", errEnv.Error)
	}
	if !strings.Contains(errEnv.Msg, "failed to parse template") {
		t.Fatalf("expected parse detail, got %q", errEnv.Msg)
	}
}

func TestCampaignSend(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	tmpl, _ := h.templates.Create(ctx, domain.Template{
		Name:    "camp-tmpl",
		Subject: "Promo",
	})

	camp, _ := h.campaigns.Create(ctx, domain.Campaign{
		Name:       "Summer Promo",
		TemplateID: tmpl.ID,
	})

	// Add contacts: one subscribed, one unsubscribed
	_, _ = h.contacts.Upsert(ctx, domain.Contact{
		Email: "active@test.local",
	})
	unsub, _ := h.contacts.Upsert(ctx, domain.Contact{
		Email: "skipped@test.local",
	})
	_ = h.contacts.SetUnsubscribed(ctx, unsub.ID, time.Now())

	res, out, err := h.deps.CampaignSend(ctx, nil, mcptransport.CampaignSendIn{
		CampaignID: camp.ID,
		Sync:       true,
	})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("campaign send failed: %v, %v", err, res)
	}

	if out.CampaignID != camp.ID {
		t.Errorf("expected campaign ID %d, got %d", camp.ID, out.CampaignID)
	}
	if out.Status != "sent" {
		t.Errorf("expected status sent, got %q", out.Status)
	}
	if out.Recipients != 2 {
		t.Errorf("expected 2 recipients in segment, got %d", out.Recipients)
	}
	if out.Sent != 1 {
		t.Errorf("expected 1 sent, got %d", out.Sent)
	}
	if out.Skipped != 1 {
		t.Errorf("expected 1 skipped (unsubscribed), got %d", out.Skipped)
	}
}

func TestCampaignSendAsyncEnqueues(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	tmpl, _ := h.templates.Create(ctx, domain.Template{Name: "async-tmpl", Subject: "Promo"})
	camp, _ := h.campaigns.Create(ctx, domain.Campaign{Name: "Async Promo", TemplateID: tmpl.ID})

	res, out, err := h.deps.CampaignSend(ctx, nil, mcptransport.CampaignSendIn{CampaignID: camp.ID})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("campaign send failed: %v, %v", err, res)
	}
	if out.Status != "queued" {
		t.Errorf("expected status queued, got %q", out.Status)
	}
	if out.TaskID == 0 {
		t.Errorf("expected non-zero task id")
	}
	if out.Sent != 0 || out.Recipients != 0 {
		t.Errorf("async send must not report inline counts, got %+v", out)
	}

	// Campaign should be marked sending, and a campaign task enqueued.
	got, _ := h.campaigns.Get(ctx, camp.ID)
	if got.Status != domain.CampaignSending {
		t.Errorf("expected campaign status sending, got %q", got.Status)
	}
	tasks, _ := h.tasks.List(ctx, "", 10)
	if len(tasks) != 1 || tasks[0].Kind != domain.TaskCampaign {
		t.Fatalf("expected one campaign task enqueued, got %+v", tasks)
	}
}

func TestCampaignSendAsyncNotFound(t *testing.T) {
	h := setupTestDeps(t)
	res, _, err := h.deps.CampaignSend(context.Background(), nil, mcptransport.CampaignSendIn{CampaignID: 999999})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected not_found error result for missing campaign")
	}
}

func TestScheduleTask(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// 1. Happy schedule path
	res, out, err := h.deps.ScheduleTask(ctx, nil, mcptransport.ScheduleTaskIn{
		Kind:    "email",
		Payload: map[string]any{"to": "task@test.local"},
		RunAt:   "2026-06-03T15:00:00Z",
	})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("schedule task failed: %v, %v", err, res)
	}
	if out.TaskID == 0 || out.Status != "pending" {
		t.Errorf("unexpected output: %+v", out)
	}

	// 2. Bad kind validation error
	res, _, err = h.deps.ScheduleTask(ctx, nil, mcptransport.ScheduleTaskIn{
		Kind:  "invalid-kind",
		RunAt: "2026-06-03T15:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected validation error for invalid task kind")
	}
	var errEnv mcpserver.ErrEnvelope
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errEnv.Error != "invalid_input" {
		t.Errorf("expected invalid_input error code, got: %s", errEnv.Error)
	}
	if !strings.Contains(errEnv.Msg, "invalid task kind") {
		t.Errorf("expected message to mention invalid task kind, got: %s", errEnv.Msg)
	}
}

func TestHealthCheck(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	res, out, err := h.deps.HealthCheck(ctx, nil, mcptransport.HealthCheckIn{})
	if err != nil || (res != nil && res.IsError) {
		t.Fatalf("health check failed: %v, %v", err, res)
	}

	if out.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", out.Status)
	}
	if !out.DBOk {
		t.Error("expected DBOk to be true")
	}
	if out.Version != "1.2.3" {
		t.Errorf("expected version '1.2.3', got %q", out.Version)
	}
}

func TestAnalyticsOverviewAndCampaignStats(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// Seed contacts
	_, _ = h.contacts.Upsert(ctx, domain.Contact{Email: "alice@test.local", Stage: domain.StageNew})
	_, _ = h.contacts.Upsert(ctx, domain.Contact{Email: "bob@test.local", Stage: domain.StageContacted})

	// Seed events
	_ = h.events.Insert(ctx, domain.EmailEvent{Type: domain.EventSent})
	_ = h.events.Insert(ctx, domain.EmailEvent{Type: domain.EventOpen, ContactID: 1})

	// 1. Test AnalyticsOverview
	res1, out1, err := h.deps.AnalyticsOverview(ctx, nil, mcptransport.AnalyticsOverviewIn{})
	if err != nil || (res1 != nil && res1.IsError) {
		t.Fatalf("analytics overview failed: %v, %v", err, res1)
	}

	if out1.TotalContacts != 2 {
		t.Errorf("expected 2 total contacts, got %d", out1.TotalContacts)
	}
	if out1.Sent != 1 || out1.Opens != 1 {
		t.Errorf("expected sent=1, opens=1; got sent=%d, opens=%d", out1.Sent, out1.Opens)
	}
	if out1.OpenRate != 1.0 {
		t.Errorf("expected open rate 1.0, got %f", out1.OpenRate)
	}

	// Seed campaign
	camp, _ := h.campaigns.Create(ctx, domain.Campaign{Name: "Stats Camp"})
	_ = h.events.Insert(ctx, domain.EmailEvent{CampaignID: &camp.ID, Type: domain.EventSent})
	_ = h.events.Insert(ctx, domain.EmailEvent{CampaignID: &camp.ID, Type: domain.EventDelivered})
	_ = h.events.Insert(ctx, domain.EmailEvent{CampaignID: &camp.ID, Type: domain.EventOpen})

	// 2. Test CampaignStats
	res2, out2, err := h.deps.CampaignStats(ctx, nil, mcptransport.CampaignStatsIn{CampaignID: camp.ID})
	if err != nil || (res2 != nil && res2.IsError) {
		t.Fatalf("campaign stats failed: %v, %v", err, res2)
	}

	if out2.CampaignID != camp.ID {
		t.Errorf("expected campaign ID %d, got %d", camp.ID, out2.CampaignID)
	}
	if out2.Sent != 1 || out2.Delivered != 1 || out2.Opened != 1 {
		t.Errorf("unexpected counts: %+v", out2)
	}
	if len(out2.TopLinks) != 1 || out2.TopLinks[0]["link_code"] != "testcode" {
		t.Errorf("unexpected top links: %+v", out2.TopLinks)
	}
}

func TestErrorContractNotFound(t *testing.T) {
	h := setupTestDeps(t)
	ctx := context.Background()

	// Call campaign stats with non-existent campaign id (999)
	res, _, err := h.deps.CampaignStats(ctx, nil, mcptransport.CampaignStatsIn{CampaignID: 999})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected error response envelope")
	}

	var errEnv mcpserver.ErrEnvelope
	errText := res.Content[0].(*mcp.TextContent).Text
	if err := json.Unmarshal([]byte(errText), &errEnv); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if errEnv.Error != "not_found" {
		t.Errorf("expected error code 'not_found', got %q", errEnv.Error)
	}
	if errEnv.Msg != "campaign not found" {
		t.Errorf("expected message 'campaign not found', got %q", errEnv.Msg)
	}
}
