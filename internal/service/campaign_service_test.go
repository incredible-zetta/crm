package service_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/service"
)

// --- Fakes ---

type fakeCampaignRepo struct {
	mu        sync.Mutex
	campaigns map[int64]domain.Campaign
	nextID    int64
}

func newFakeCampaignRepo() *fakeCampaignRepo {
	return &fakeCampaignRepo{
		campaigns: make(map[int64]domain.Campaign),
		nextID:    1,
	}
}

func (r *fakeCampaignRepo) Create(ctx context.Context, c domain.Campaign) (domain.Campaign, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c.ID == 0 {
		c.ID = r.nextID
		r.nextID++
	}
	c.CreatedAt = time.Now()
	r.campaigns[c.ID] = c
	return c, nil
}

func (r *fakeCampaignRepo) Get(ctx context.Context, id int64) (domain.Campaign, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.campaigns[id]
	if !ok || c.DeletedAt != nil {
		return domain.Campaign{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *fakeCampaignRepo) List(ctx context.Context) ([]domain.Campaign, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var list []domain.Campaign
	for _, c := range r.campaigns {
		if c.DeletedAt == nil {
			list = append(list, c)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list, nil
}

func (r *fakeCampaignRepo) UpdateStatus(ctx context.Context, id int64, status domain.CampaignStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.campaigns[id]
	if !ok || c.DeletedAt != nil {
		return domain.ErrNotFound
	}
	c.Status = status
	r.campaigns[id] = c
	return nil
}

func (r *fakeCampaignRepo) Update(ctx context.Context, id int64, c domain.Campaign) (domain.Campaign, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.campaigns[id]
	if !ok || existing.DeletedAt != nil {
		return domain.Campaign{}, domain.ErrNotFound
	}
	if c.Name != "" {
		existing.Name = c.Name
	}
	if c.TemplateID > 0 {
		existing.TemplateID = c.TemplateID
	}
	if c.Provider != "" {
		existing.Provider = c.Provider
	}
	if c.Segment != nil {
		existing.Segment = c.Segment
	}
	if c.Status != "" {
		existing.Status = c.Status
	}
	if c.ScheduledAt != nil {
		existing.ScheduledAt = c.ScheduledAt
	}
	r.campaigns[id] = existing
	return existing, nil
}

func (r *fakeCampaignRepo) SetStats(ctx context.Context, id int64, stats map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.campaigns[id]
	if !ok || c.DeletedAt != nil {
		return domain.ErrNotFound
	}
	c.Stats = stats
	r.campaigns[id] = c
	return nil
}

func (r *fakeCampaignRepo) SoftDelete(ctx context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.campaigns[id]
	if !ok || c.DeletedAt != nil {
		return domain.ErrNotFound
	}
	now := time.Now()
	c.DeletedAt = &now
	r.campaigns[id] = c
	return nil
}

func (r *fakeCampaignRepo) ListDueScheduled(ctx context.Context, now time.Time, limit int) ([]domain.Campaign, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var due []domain.Campaign
	for _, c := range r.campaigns {
		if c.DeletedAt != nil || c.Status != domain.CampaignScheduled || c.ScheduledAt == nil {
			continue
		}
		if !c.ScheduledAt.After(now) {
			due = append(due, c)
		}
	}
	return due, nil
}

type fakeCampaignContactRepo struct {
	mu       sync.Mutex
	contacts map[int64]domain.Contact
}

func newFakeCampaignContactRepo() *fakeCampaignContactRepo {
	return &fakeCampaignContactRepo{
		contacts: make(map[int64]domain.Contact),
	}
}

func (r *fakeCampaignContactRepo) Upsert(ctx context.Context, c domain.Contact) (domain.Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.contacts[c.ID] = c
	return c, nil
}

func (r *fakeCampaignContactRepo) Get(ctx context.Context, id int64) (domain.Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.Contact{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *fakeCampaignContactRepo) GetByEmail(ctx context.Context, email string) (domain.Contact, error) {
	return domain.Contact{}, nil
}

func (r *fakeCampaignContactRepo) GetByUnsubCode(ctx context.Context, code string) (domain.Contact, error) {
	return domain.Contact{}, nil
}

func (r *fakeCampaignContactRepo) Update(ctx context.Context, id int64, patch domain.ContactPatch) (domain.Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.Contact{}, domain.ErrNotFound
	}
	if patch.Stage != nil {
		c.Stage = domain.Stage(*patch.Stage)
	}
	r.contacts[id] = c
	return c, nil
}

func (r *fakeCampaignContactRepo) List(ctx context.Context, f domain.ContactFilter, p port.Paging) (port.ContactPage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

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
		if f.Tag != "" {
			hasTag := false
			for _, t := range c.Tags {
				if t == f.Tag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		if f.Q != "" {
			q := strings.ToLower(f.Q)
			if !strings.Contains(strings.ToLower(c.Email), q) &&
				!strings.Contains(strings.ToLower(c.FirstName), q) &&
				!strings.Contains(strings.ToLower(c.LastName), q) &&
				!strings.Contains(strings.ToLower(c.Company), q) {
				continue
			}
		}
		matched = append(matched, c)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ID < matched[j].ID
	})

	var paged []domain.Contact
	for _, c := range matched {
		if c.ID > p.Cursor {
			paged = append(paged, c)
		}
	}

	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if len(paged) > limit {
		nextCursor := paged[limit-1].ID
		return port.ContactPage{
			Items:      paged[:limit],
			Total:      len(matched),
			NextCursor: nextCursor,
		}, nil
	}

	return port.ContactPage{
		Items:      paged,
		Total:      len(matched),
		NextCursor: 0,
	}, nil
}

func (r *fakeCampaignContactRepo) CountByStage(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

func (r *fakeCampaignContactRepo) SoftDelete(ctx context.Context, id int64) error {
	return nil
}

func (r *fakeCampaignContactRepo) Purge(ctx context.Context, id int64) error {
	return nil
}

func (r *fakeCampaignContactRepo) SetUnsubscribed(ctx context.Context, id int64, at time.Time) error {
	return nil
}

func (r *fakeCampaignContactRepo) SetUnsubCode(ctx context.Context, id int64, code string) error {
	return nil
}

func (r *fakeCampaignContactRepo) SetEmailStatus(ctx context.Context, id int64, v domain.EmailVerification) error {
	return nil
}

type fakeCampaignEventRepo struct {
	mu          sync.Mutex
	events      []domain.EmailEvent
	counts      map[string]int
	uniqueOpens int
	topLinks    []domain.LinkCount
}

func (r *fakeCampaignEventRepo) Insert(ctx context.Context, e domain.EmailEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

func (r *fakeCampaignEventRepo) OverviewCounts(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

func (r *fakeCampaignEventRepo) UniqueOpens(ctx context.Context, campaignID *int64) (int, error) {
	return 0, nil
}

func (r *fakeCampaignEventRepo) CampaignCounts(ctx context.Context, campaignID int64) (map[string]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts, nil
}

func (r *fakeCampaignEventRepo) CampaignUniqueOpens(ctx context.Context, campaignID int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.uniqueOpens, nil
}

func (r *fakeCampaignEventRepo) TopLinks(ctx context.Context, campaignID int64, limit int) ([]domain.LinkCount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.topLinks, nil
}

type fakeCampaignMailer struct {
	mu        sync.Mutex
	sentCalls []sentCall
	errMap    map[int64]error
}

type sentCall struct {
	contactID  int64
	templateID int64
	campaignID int64
}

func (m *fakeCampaignMailer) SendToContact(ctx context.Context, c domain.Contact, templateID int64, campaignID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentCalls = append(m.sentCalls, sentCall{
		contactID:  c.ID,
		templateID: templateID,
		campaignID: campaignID,
	})
	if m.errMap != nil {
		if err, ok := m.errMap[c.ID]; ok {
			return err
		}
	}
	return nil
}

// --- Test Cases ---

func TestCampaignCreateValidates(t *testing.T) {
	repo := newFakeCampaignRepo()
	contacts := newFakeCampaignContactRepo()
	events := &fakeCampaignEventRepo{}
	mailer := &fakeCampaignMailer{}

	svc := service.NewCampaignService(repo, contacts, events, mailer)

	// Case 1: Empty name
	_, err := svc.Create(context.Background(), domain.Campaign{
		TemplateID: 1,
	})
	if err == nil || !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for empty name, got: %v", err)
	}

	// Case 2: TemplateID <= 0
	_, err = svc.Create(context.Background(), domain.Campaign{
		Name:       "Test Campaign",
		TemplateID: 0,
	})
	if err == nil || !errors.Is(err, domain.ErrValidation) || !strings.Contains(err.Error(), "template_id required") {
		t.Fatalf("expected validation error for zero template ID, got: %v", err)
	}

	// Case 3: Bad provider
	_, err = svc.Create(context.Background(), domain.Campaign{
		Name:       "Test Campaign",
		TemplateID: 1,
		Provider:   "invalid-provider",
	})
	if err == nil || !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for bad provider, got: %v", err)
	}

	// Case 4: Valid campaign (defaults applied)
	created, err := svc.Create(context.Background(), domain.Campaign{
		Name:       "Test Campaign",
		TemplateID: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error creating campaign: %v", err)
	}
	if created.Provider != domain.ProviderSMTP {
		t.Fatalf("expected default provider %q, got: %q", domain.ProviderSMTP, created.Provider)
	}
	if created.Status != domain.CampaignDraft {
		t.Fatalf("expected default status %q, got: %q", domain.CampaignDraft, created.Status)
	}
}

func TestCampaignCRUD(t *testing.T) {
	repo := newFakeCampaignRepo()
	contacts := newFakeCampaignContactRepo()
	events := &fakeCampaignEventRepo{}
	mailer := &fakeCampaignMailer{}

	svc := service.NewCampaignService(repo, contacts, events, mailer)
	ctx := context.Background()

	// 1. Create
	c1, err := svc.Create(ctx, domain.Campaign{
		Name:       "Camp 1",
		TemplateID: 10,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	c2, err := svc.Create(ctx, domain.Campaign{
		Name:       "Camp 2",
		TemplateID: 20,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// 2. Get
	fetched, err := svc.Get(ctx, c1.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if fetched.Name != "Camp 1" {
		t.Fatalf("expected Camp 1, got: %s", fetched.Name)
	}

	// 3. List
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 campaigns, got: %d", len(list))
	}

	// 4. Update
	c1.Name = "Camp 1 Updated"
	c1.Provider = domain.ProviderMailgun
	updated, err := svc.Update(ctx, c1.ID, c1)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Name != "Camp 1 Updated" {
		t.Fatalf("update name mismatch: %s", updated.Name)
	}
	if updated.Provider != domain.ProviderMailgun {
		t.Fatalf("update provider mismatch: %s", updated.Provider)
	}

	// 4b. Update with bad provider
	c1.Provider = "bad"
	_, err = svc.Update(ctx, c1.ID, c1)
	if err == nil || !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error for bad provider update, got: %v", err)
	}

	// 5. Delete (soft-delete hides from list and get)
	err = svc.Delete(ctx, c2.ID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = svc.Get(ctx, c2.ID)
	if err == nil || !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for soft-deleted campaign, got: %v", err)
	}

	list2, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list failed after delete: %v", err)
	}
	if len(list2) != 1 {
		t.Fatalf("expected 1 campaign after delete, got: %d", len(list2))
	}
}

func TestCampaignSendSkipsUnsubscribed(t *testing.T) {
	repo := newFakeCampaignRepo()
	contacts := newFakeCampaignContactRepo()
	events := &fakeCampaignEventRepo{}
	mailer := &fakeCampaignMailer{}

	svc := service.NewCampaignService(repo, contacts, events, mailer)
	ctx := context.Background()

	// Create campaign
	camp, err := svc.Create(ctx, domain.Campaign{
		Name:       "Promo",
		TemplateID: 42,
		Segment:    map[string]any{"stage": "won"},
	})
	if err != nil {
		t.Fatalf("create campaign failed: %v", err)
	}

	// Create contacts
	now := time.Now()
	c1 := domain.Contact{ID: 1, Email: "a@example.com", Stage: "won"}
	c2 := domain.Contact{ID: 2, Email: "b@example.com", Stage: "won", UnsubscribedAt: &now}
	c3 := domain.Contact{ID: 3, Email: "c@example.com", Stage: "won"}

	_, _ = contacts.Upsert(ctx, c1)
	_, _ = contacts.Upsert(ctx, c2)
	_, _ = contacts.Upsert(ctx, c3)

	recipients, sent, failed, skipped, err := svc.Send(ctx, camp.ID)
	if err != nil {
		t.Fatalf("send campaign failed: %v", err)
	}

	if recipients != 3 {
		t.Errorf("expected 3 recipients, got %d", recipients)
	}
	if skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", skipped)
	}
	if sent != 2 {
		t.Errorf("expected 2 sent, got %d", sent)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}

	// Verify mailer calls
	if len(mailer.sentCalls) != 2 {
		t.Errorf("expected 2 mailer calls, got %d", len(mailer.sentCalls))
	}
	for _, call := range mailer.sentCalls {
		if call.contactID == 2 {
			t.Error("unsubscribed contact should not receive email")
		}
	}

	// Verify campaign status updated to "sent"
	updatedCamp, err := svc.Get(ctx, camp.ID)
	if err != nil {
		t.Fatalf("failed to fetch campaign: %v", err)
	}
	if updatedCamp.Status != domain.CampaignSent {
		t.Errorf("expected status to be sent, got %s", updatedCamp.Status)
	}
}

func TestCampaignSendCountsFailures(t *testing.T) {
	repo := newFakeCampaignRepo()
	contacts := newFakeCampaignContactRepo()
	events := &fakeCampaignEventRepo{}
	mailer := &fakeCampaignMailer{
		errMap: map[int64]error{
			2: errors.New("smtp timeout"),
		},
	}

	svc := service.NewCampaignService(repo, contacts, events, mailer)
	ctx := context.Background()

	camp, _ := svc.Create(ctx, domain.Campaign{Name: "Failures", TemplateID: 1})
	_, _ = contacts.Upsert(ctx, domain.Contact{ID: 1, Email: "a@example.com"})
	_, _ = contacts.Upsert(ctx, domain.Contact{ID: 2, Email: "b@example.com"})
	_, _ = contacts.Upsert(ctx, domain.Contact{ID: 3, Email: "c@example.com"})

	recipients, sent, failed, skipped, err := svc.Send(ctx, camp.ID)
	if err != nil {
		t.Fatalf("send campaign failed: %v", err)
	}

	if recipients != 3 {
		t.Errorf("expected 3 recipients, got %d", recipients)
	}
	if sent != 2 {
		t.Errorf("expected 2 sent, got %d", sent)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", skipped)
	}
}

func TestCampaignSendPaging(t *testing.T) {
	repo := newFakeCampaignRepo()
	contacts := newFakeCampaignContactRepo()
	events := &fakeCampaignEventRepo{}
	mailer := &fakeCampaignMailer{}

	svc := service.NewCampaignService(repo, contacts, events, mailer)
	ctx := context.Background()

	camp, _ := svc.Create(ctx, domain.Campaign{Name: "Paging", TemplateID: 1})

	// Create 105 contacts (more than the paging limit 100)
	for i := int64(1); i <= 105; i++ {
		_, _ = contacts.Upsert(ctx, domain.Contact{ID: i, Email: "contact@example.com"})
	}

	recipients, sent, failed, skipped, err := svc.Send(ctx, camp.ID)
	if err != nil {
		t.Fatalf("send campaign failed: %v", err)
	}

	if recipients != 105 {
		t.Errorf("expected 105 recipients, got %d", recipients)
	}
	if sent != 105 {
		t.Errorf("expected 105 sent, got %d", sent)
	}
	if len(mailer.sentCalls) != 105 {
		t.Errorf("expected 105 mailer calls, got %d", len(mailer.sentCalls))
	}
	_ = failed
	_ = skipped
}

func TestCampaignStatsRates(t *testing.T) {
	repo := newFakeCampaignRepo()
	contacts := newFakeCampaignContactRepo()
	events := &fakeCampaignEventRepo{}
	mailer := &fakeCampaignMailer{}

	svc := service.NewCampaignService(repo, contacts, events, mailer)
	ctx := context.Background()

	camp, _ := svc.Create(ctx, domain.Campaign{Name: "Stats", TemplateID: 1})

	// Setup canned stats
	events.counts = map[string]int{
		"sent":        10,
		"delivered":   9,
		"open":        8,
		"click":       2,
		"bounce":      1,
		"unsubscribe": 1,
	}
	events.uniqueOpens = 5
	events.topLinks = []domain.LinkCount{
		{LinkCode: "test-code", Clicks: 2},
	}

	stats, err := svc.Stats(ctx, camp.ID)
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}

	if stats.CampaignID != camp.ID {
		t.Errorf("expected CampaignID %d, got %d", camp.ID, stats.CampaignID)
	}
	if stats.Sent != 10 {
		t.Errorf("expected Sent 10, got %d", stats.Sent)
	}
	if stats.Delivered != 9 {
		t.Errorf("expected Delivered 9, got %d", stats.Delivered)
	}
	if stats.Opened != 8 {
		t.Errorf("expected Opened 8, got %d", stats.Opened)
	}
	if stats.Clicked != 2 {
		t.Errorf("expected Clicked 2, got %d", stats.Clicked)
	}
	if stats.Bounced != 1 {
		t.Errorf("expected Bounced 1, got %d", stats.Bounced)
	}
	if stats.Unsubscribed != 1 {
		t.Errorf("expected Unsubscribed 1, got %d", stats.Unsubscribed)
	}
	if stats.UniqueOpens != 5 {
		t.Errorf("expected UniqueOpens 5, got %d", stats.UniqueOpens)
	}
	if stats.OpenRate != 0.5 { // 5 / 10
		t.Errorf("expected OpenRate 0.5, got %f", stats.OpenRate)
	}
	if stats.ClickRate != 0.2 { // 2 / 10
		t.Errorf("expected ClickRate 0.2, got %f", stats.ClickRate)
	}
	if len(stats.TopLinks) != 1 || stats.TopLinks[0].LinkCode != "test-code" {
		t.Errorf("unexpected TopLinks: %v", stats.TopLinks)
	}

	// Zero division check
	events.counts = map[string]int{
		"sent": 0,
	}
	events.uniqueOpens = 0

	stats2, err := svc.Stats(ctx, camp.ID)
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats2.OpenRate != 0.0 {
		t.Errorf("expected OpenRate 0.0, got %f", stats2.OpenRate)
	}
	if stats2.ClickRate != 0.0 {
		t.Errorf("expected ClickRate 0.0, got %f", stats2.ClickRate)
	}
}

func TestCampaignSendPromotesNewLeads(t *testing.T) {
	repo := newFakeCampaignRepo()
	contacts := newFakeCampaignContactRepo()
	events := &fakeCampaignEventRepo{}
	mailer := &fakeCampaignMailer{}

	svc := service.NewCampaignService(repo, contacts, events, mailer)
	ctx := context.Background()

	camp, err := svc.Create(ctx, domain.Campaign{
		Name:       "B2B Promo",
		TemplateID: 1,
		Segment:    map[string]any{"tag": "segment_b2b"},
	})
	if err != nil {
		t.Fatalf("create campaign failed: %v", err)
	}

	for i := int64(1); i <= 3; i++ {
		stage := domain.StageNew
		if i == 3 {
			stage = domain.StageQualified
		}
		_, _ = contacts.Upsert(ctx, domain.Contact{
			ID:    i,
			Email: "user@example.com",
			Stage: stage,
			Tags:  []string{"segment_b2b"},
		})
	}

	_, sent, _, _, err := svc.Send(ctx, camp.ID)
	if err != nil {
		t.Fatalf("send campaign failed: %v", err)
	}
	if sent != 3 {
		t.Fatalf("expected 3 sent, got %d", sent)
	}

	c1, _ := contacts.Get(ctx, 1)
	c2, _ := contacts.Get(ctx, 2)
	c3, _ := contacts.Get(ctx, 3)
	if c1.Stage != domain.StageContacted || c2.Stage != domain.StageContacted {
		t.Fatalf("expected new leads promoted to contacted, got %s and %s", c1.Stage, c2.Stage)
	}
	if c3.Stage != domain.StageQualified {
		t.Fatalf("expected qualified lead unchanged, got %s", c3.Stage)
	}
}

func TestCampaignStatsRepairsDraftWithSentEvents(t *testing.T) {
	repo := newFakeCampaignRepo()
	contacts := newFakeCampaignContactRepo()
	events := &fakeCampaignEventRepo{counts: map[string]int{"sent": 5}}
	mailer := &fakeCampaignMailer{}

	svc := service.NewCampaignService(repo, contacts, events, mailer)
	ctx := context.Background()

	camp, _ := svc.Create(ctx, domain.Campaign{Name: "Stale", TemplateID: 1})
	_ = repo.UpdateStatus(ctx, camp.ID, domain.CampaignDraft)

	stats, err := svc.Stats(ctx, camp.ID)
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats.Sent != 5 {
		t.Fatalf("expected sent 5, got %d", stats.Sent)
	}
	if stats.TrackingSupport["delivery"] != "unsupported" {
		t.Fatalf("expected delivery unsupported for smtp, got %q", stats.TrackingSupport["delivery"])
	}

	updated, _ := svc.Get(ctx, camp.ID)
	if updated.Status != domain.CampaignSent {
		t.Fatalf("expected status repaired to sent, got %s", updated.Status)
	}
}

func TestCampaignSendMissing(t *testing.T) {
	repo := newFakeCampaignRepo()
	contacts := newFakeCampaignContactRepo()
	events := &fakeCampaignEventRepo{}
	mailer := &fakeCampaignMailer{}

	svc := service.NewCampaignService(repo, contacts, events, mailer)
	ctx := context.Background()

	_, _, _, _, err := svc.Send(ctx, 9999)
	if err == nil || !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}
