package mcptools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/db"
	"github.com/cipta/crm-for-aiagents/internal/email"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeSender struct {
	lastMsg email.Message
	err     error
	calls   int
}

func (f *fakeSender) Send(ctx context.Context, msg email.Message) error {
	f.calls++
	f.lastMsg = msg
	return f.err
}

func getTestDeps(t *testing.T) (*Deps, *fakeSender) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set; skipping integration test")
	}
	d, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		d.Close()
		t.Fatalf("failed to migrate DB: %v", err)
	}

	repo := db.NewRepo(d)

	t.Cleanup(func() {
		_, _ = d.Exec("DELETE FROM email_events WHERE contact_id IN (SELECT id FROM contacts WHERE email LIKE 't10_%')")
		_, _ = d.Exec("DELETE FROM tracking_links WHERE target_url LIKE '%t10_%'")
		_, _ = d.Exec("DELETE FROM scheduled_tasks WHERE kind LIKE 't10_%' OR payload LIKE '%t10_%'")
		_, _ = d.Exec("DELETE FROM contacts WHERE email LIKE 't10_%'")
		_, _ = d.Exec("DELETE FROM email_templates WHERE name LIKE 't10_%'")
		_, _ = d.Exec("DELETE FROM campaigns WHERE name LIKE 't10_%'")
		_, _ = d.Exec("DELETE FROM exports WHERE path LIKE '%t10_%'")
		d.Close()
	})

	sender := &fakeSender{}
	pipeline := &email.Pipeline{
		Sender: sender,
		Tmpl:   RepoTemplateStore{Repo: repo},
		Links:  RepoLinkMaker{Repo: repo},
		Events: RepoEventLogger{Repo: repo},
	}

	deps := &Deps{
		Repo:      repo,
		Pipeline:  pipeline,
		BaseURL:   "http://localhost:8080",
		ExportDir: t.TempDir(),
		Version:   "1.0.0",
		PingDB: func(ctx context.Context) error {
			return d.PingContext(ctx)
		},
		PingSMTP: func(ctx context.Context) error {
			return nil
		},
		PingMailgun: func(ctx context.Context) error {
			return nil
		},
	}

	return deps, sender
}

func TestContactCreate(t *testing.T) {
	d, _ := getTestDeps(t)
	ctx := context.Background()

	// 1. Happy path
	res, out, err := d.ContactCreate(ctx, nil, ContactCreateIn{
		Email:     "t10_create_happy@test.local",
		FirstName: "Jane",
		LastName:  "Doe",
		Stage:     "contacted",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("unexpected handler error: %s", res.Content[0].(*mcp.TextContent).Text)
	}
	if out.ID == 0 {
		t.Errorf("expected non-zero ID")
	}
	if out.Email != "t10_create_happy@test.local" {
		t.Errorf("expected email, got %s", out.Email)
	}
	if out.Stage != "contacted" {
		t.Errorf("expected stage 'contacted', got %s", out.Stage)
	}

	// 2. Bad stage
	res, _, err = d.ContactCreate(ctx, nil, ContactCreateIn{
		Email: "t10_create_badstage@test.local",
		Stage: "super_won",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected error result on bad stage")
	}
}

func TestContactUpdate(t *testing.T) {
	d, _ := getTestDeps(t)
	ctx := context.Background()

	// Create initial contact
	_, outC, err := d.ContactCreate(ctx, nil, ContactCreateIn{
		Email: "t10_update_test@test.local",
		Stage: "new",
	})
	if err != nil {
		t.Fatalf("create template error: %v", err)
	}

	// Update by ID
	lastName := "UpdatedLastName"
	res, _, err := d.ContactUpdate(ctx, nil, ContactUpdateIn{
		ID:       outC.ID,
		LastName: &lastName,
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("handler error: %v", res)
	}

	// Verify update in DB
	c, err := d.Repo.GetContact(ctx, outC.ID)
	if err != nil {
		t.Fatalf("get contact: %v", err)
	}
	if c.LastName != "UpdatedLastName" {
		t.Errorf("expected last name UpdatedLastName, got %s", c.LastName)
	}

	// Update by Email
	company := "NewCompany"
	res, _, err = d.ContactUpdate(ctx, nil, ContactUpdateIn{
		Email:   outC.Email,
		Company: &company,
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("handler error: %v", res)
	}

	c, err = d.Repo.GetContact(ctx, outC.ID)
	if err != nil {
		t.Fatalf("get contact: %v", err)
	}
	if c.Company != "NewCompany" {
		t.Errorf("expected company NewCompany, got %s", c.Company)
	}
}

func TestContactList(t *testing.T) {
	d, _ := getTestDeps(t)
	ctx := context.Background()

	// Create some contacts
	for i := 0; i < 5; i++ {
		_, _, err := d.ContactCreate(ctx, nil, ContactCreateIn{
			Email:     fmt.Sprintf("t10_list_%d@test.local", i),
			FirstName: fmt.Sprintf("User%d", i),
			Company:   "t10_list_company",
			Stage:     "new",
		})
		if err != nil {
			t.Fatalf("failed to create contact: %v", err)
		}
	}

	// List contacts with default projection
	_, outList, err := d.ContactList(ctx, nil, ContactListIn{
		Company: "t10_list_company",
		Limit:   0, // should default to 20
	})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if outList.Total < 5 {
		t.Errorf("expected at least 5 total, got %d", outList.Total)
	}
	if len(outList.Items) == 0 {
		t.Fatalf("expected items to be returned")
	}

	// Check default subset keys
	item := outList.Items[0]
	if _, ok := item["email"]; !ok {
		t.Errorf("missing key 'email' in default subset")
	}
	if _, ok := item["first_name"]; !ok {
		t.Errorf("missing key 'first_name' in default subset")
	}
	if _, ok := item["phone"]; ok {
		t.Errorf("unexpected key 'phone' in default subset")
	}

	// List with specific Fields projection
	_, outListProj, err := d.ContactList(ctx, nil, ContactListIn{
		Company: "t10_list_company",
		Fields:  []string{"email", "phone"},
	})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	itemProj := outListProj.Items[0]
	if _, ok := itemProj["email"]; !ok {
		t.Errorf("expected key 'email' with custom projection")
	}
	if _, ok := itemProj["first_name"]; ok {
		t.Errorf("did not expect key 'first_name' with custom projection")
	}
}

func TestContactImport(t *testing.T) {
	d, _ := getTestDeps(t)
	ctx := context.Background()

	// 1. Array import (inserts new)
	_, out, err := d.ContactImport(ctx, nil, ContactImportIn{
		Contacts: []ContactInput{
			{Email: "t10_import_1@test.local", FirstName: "A", Stage: "new"},
			{Email: "t10_import_2@test.local", FirstName: "B", Stage: "contacted"},
		},
	})
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	if out.Inserted != 2 {
		t.Errorf("expected 2 inserted, got %d", out.Inserted)
	}

	// 2. CSV import with update
	csvData := `email,first_name,last_name,company,phone,stage,tags,source
t10_import_1@test.local,A_updated,LastName,Comp,123,qualified,tagA;tagB,sourceA
t10_import_3@test.local,C,LastName,Comp,123,new,,
`
	_, outCSV, err := d.ContactImport(ctx, nil, ContactImportIn{
		CSV: csvData,
	})
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	if outCSV.Inserted != 1 {
		t.Errorf("expected 1 inserted (t10_import_3), got %d", outCSV.Inserted)
	}
	if outCSV.Updated != 1 {
		t.Errorf("expected 1 updated (t10_import_1), got %d", outCSV.Updated)
	}

	// Verify t10_import_1 updated fields
	c, err := d.Repo.GetContactByEmail(ctx, "t10_import_1@test.local")
	if err != nil {
		t.Fatalf("get contact: %v", err)
	}
	if c.FirstName != "A_updated" {
		t.Errorf("expected FirstName updated to 'A_updated', got %s", c.FirstName)
	}
	if c.Stage != "qualified" {
		t.Errorf("expected Stage updated to 'qualified', got %s", c.Stage)
	}
}

func TestContactExport(t *testing.T) {
	d, _ := getTestDeps(t)
	ctx := context.Background()

	// Create some contacts
	for i := 0; i < 3; i++ {
		_, _, err := d.ContactCreate(ctx, nil, ContactCreateIn{
			Email:   fmt.Sprintf("t10_export_%d@test.local", i),
			Company: "t10_export_company",
		})
		if err != nil {
			t.Fatalf("failed to create contact: %v", err)
		}
	}

	res, out, err := d.ContactExport(ctx, nil, ContactExportIn{
		Company: "t10_export_company",
	})
	if err != nil {
		t.Fatalf("export error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("handler error: %v", res)
	}

	if out.Rows < 3 {
		t.Errorf("expected at least 3 rows exported, got %d", out.Rows)
	}
	if out.URL == "" {
		t.Errorf("expected URL to be populated")
	}
	if !strings.HasSuffix(out.URL, ".csv") {
		t.Errorf("expected URL to end with .csv, got %s", out.URL)
	}

	// Verify file is actually written
	filePath := filepath.Join(d.ExportDir, out.ExportID+".csv")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("expected export file at %s to exist", filePath)
	}
}

func TestTemplateCreateAndRender(t *testing.T) {
	d, _ := getTestDeps(t)
	ctx := context.Background()

	// 1. Create Template
	_, outC, err := d.TemplateCreate(ctx, nil, TemplateCreateIn{
		Name:      "t10_tmpl_render",
		Subject:   "Hello {{.first_name}}",
		BodyHTML:  "<p>Welcome to {{.company}}</p>",
		BodyText:  "Welcome to {{.company}}",
		Variables: []string{"first_name", "company"},
	})
	if err != nil {
		t.Fatalf("create template error: %v", err)
	}

	// 2. Render without HTML flag (should default to only Subject and Text)
	_, outR1, err := d.TemplateRender(ctx, nil, TemplateRenderIn{
		TemplateID: outC.ID,
		Vars:       map[string]any{"first_name": "Bob", "company": "Stark Industries"},
	})
	if err != nil {
		t.Fatalf("render template error: %v", err)
	}
	if outR1.Subject != "Hello Bob" {
		t.Errorf("expected Subject 'Hello Bob', got %s", outR1.Subject)
	}
	if outR1.Text != "Welcome to Stark Industries" {
		t.Errorf("expected Text, got %s", outR1.Text)
	}
	if outR1.HTML != "" {
		t.Errorf("expected HTML to be omitted, got %s", outR1.HTML)
	}

	// 3. Render with HTML flag = true
	_, outR2, err := d.TemplateRender(ctx, nil, TemplateRenderIn{
		TemplateID: outC.ID,
		Vars:       map[string]any{"first_name": "Bob", "company": "Stark Industries"},
		HTML:       true,
	})
	if err != nil {
		t.Fatalf("render template error: %v", err)
	}
	if outR2.HTML != "<p>Welcome to Stark Industries</p>" {
		t.Errorf("expected HTML content, got %s", outR2.HTML)
	}
}

func TestEmailSend(t *testing.T) {
	d, sender := getTestDeps(t)
	ctx := context.Background()

	// Create contact first so we can send to ID
	_, outC, err := d.ContactCreate(ctx, nil, ContactCreateIn{
		Email:     "t10_send_test@test.local",
		FirstName: "Alice",
	})
	if err != nil {
		t.Fatalf("failed to create contact: %v", err)
	}

	// Create template
	_, outT, err := d.TemplateCreate(ctx, nil, TemplateCreateIn{
		Name:     "t10_send_template",
		Subject:  "Hi {{.first_name}}",
		BodyHTML: "Welcome!",
		BodyText: "Welcome text!",
	})
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Send Email
	res, outSend, err := d.EmailSend(ctx, nil, EmailSendIn{
		ContactID:  outC.ID,
		TemplateID: outT.ID,
		Vars:       map[string]any{"first_name": "Alice"},
	})
	if err != nil {
		t.Fatalf("send email error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("handler error: %v", res)
	}

	if outSend.Status != "sent" {
		t.Errorf("expected status sent, got %s", outSend.Status)
	}
	if outSend.To != "t10_send_test@test.local" {
		t.Errorf("expected recipient email, got %s", outSend.To)
	}

	// Check that the fake sender was called and captured the message
	if sender.calls != 1 {
		t.Errorf("expected 1 call to sender, got %d", sender.calls)
	}
	if sender.lastMsg.To != "t10_send_test@test.local" {
		t.Errorf("expected to Alice, got %s", sender.lastMsg.To)
	}
	if sender.lastMsg.Subject != "Hi Alice" {
		t.Errorf("expected subject 'Hi Alice', got %s", sender.lastMsg.Subject)
	}
}

func TestCampaignCreateAndSend(t *testing.T) {
	d, sender := getTestDeps(t)
	ctx := context.Background()

	// Create contacts for segment
	for i := 0; i < 3; i++ {
		_, _, err := d.ContactCreate(ctx, nil, ContactCreateIn{
			Email:   fmt.Sprintf("t10_camp_contact_%d@test.local", i),
			Company: "t10_camp_corp",
		})
		if err != nil {
			t.Fatalf("failed to create contact: %v", err)
		}
	}

	// Create template
	_, outT, err := d.TemplateCreate(ctx, nil, TemplateCreateIn{
		Name:     "t10_camp_tmpl",
		Subject:  "News from {{.company}}",
		BodyHTML: "Hello",
		BodyText: "Hello text",
	})
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Create Campaign
	_, outC, err := d.CampaignCreate(ctx, nil, CampaignCreateIn{
		Name:       "t10_camp_test",
		TemplateID: outT.ID,
		Segment:    map[string]any{"company": "t10_camp_corp"},
	})
	if err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}

	// Send Campaign
	_, outSend, err := d.CampaignSend(ctx, nil, CampaignSendIn{
		CampaignID: outC.ID,
	})
	if err != nil {
		t.Fatalf("campaign send error: %v", err)
	}

	if outSend.Recipients != 3 {
		t.Errorf("expected 3 recipients, got %d", outSend.Recipients)
	}
	if outSend.Sent != 3 {
		t.Errorf("expected 3 sent, got %d", outSend.Sent)
	}
	if outSend.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", outSend.Failed)
	}

	// Verify fake sender got called 3 times
	if sender.calls != 3 {
		t.Errorf("expected 3 calls to sender, got %d", sender.calls)
	}

	// Verify campaign status in DB is "sent"
	camp, err := d.Repo.GetCampaign(ctx, outC.ID)
	if err != nil {
		t.Fatalf("failed to get campaign: %v", err)
	}
	if camp.Status != "sent" {
		t.Errorf("expected status 'sent', got %s", camp.Status)
	}
}

func TestTrackingLinkCreate(t *testing.T) {
	d, _ := getTestDeps(t)
	ctx := context.Background()

	// 1. Invalid target URL
	res, _, err := d.TrackingLinkCreate(ctx, nil, TrackingLinkCreateIn{
		TargetURL: "ftp://invalid-scheme.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected validation error for ftp scheme")
	}

	// 2. Valid URL
	_, out, err := d.TrackingLinkCreate(ctx, nil, TrackingLinkCreateIn{
		TargetURL: "https://github.com/t10_valid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Code == "" {
		t.Errorf("expected code to be generated")
	}
	if !strings.HasPrefix(out.URL, "http://localhost:8080/t/") {
		t.Errorf("expected short tracking URL, got %s", out.URL)
	}
}

func TestScheduleTask(t *testing.T) {
	d, _ := getTestDeps(t)
	ctx := context.Background()

	// 1. Invalid kind
	res, _, err := d.ScheduleTask(ctx, nil, ScheduleTaskIn{
		Kind:  "t10_invalid_kind",
		RunAt: time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected error for invalid kind")
	}

	// 2. Valid task
	_, out, err := d.ScheduleTask(ctx, nil, ScheduleTaskIn{
		Kind:    "email",
		Payload: map[string]any{"some_id": "t10_task"},
		RunAt:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TaskID == 0 {
		t.Errorf("expected non-zero TaskID")
	}
	if out.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", out.Status)
	}
}

func TestHealthCheck(t *testing.T) {
	d, _ := getTestDeps(t)
	ctx := context.Background()

	_, out, err := d.HealthCheck(ctx, nil, HealthCheckIn{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Status != "ok" {
		t.Errorf("expected status 'ok', got %s", out.Status)
	}
	if !out.DBOk {
		t.Errorf("expected DBOk to be true")
	}
}

func TestAnalyticsAndCampaignStats(t *testing.T) {
	d, _ := getTestDeps(t)
	ctx := context.Background()

	// 1. Analytics Overview
	_, outA, err := d.AnalyticsOverview(ctx, nil, AnalyticsOverviewIn{})
	if err != nil {
		t.Fatalf("analytics error: %v", err)
	}
	// Check defaults and no divide-by-zero panic
	if outA.OpenRate != 0 || outA.ClickRate != 0 {
		t.Errorf("expected zero rates for empty events")
	}

	// 2. Campaign Stats
	_, outS, err := d.CampaignStats(ctx, nil, CampaignStatsIn{
		CampaignID: 99999, // dummy ID
	})
	if err != nil {
		t.Fatalf("campaign stats error: %v", err)
	}
	if outS.CampaignID != 99999 {
		t.Errorf("expected CampaignID 99999, got %d", outS.CampaignID)
	}
}
