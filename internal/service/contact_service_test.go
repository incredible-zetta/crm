package service_test

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/service"
)

// --- Fakes & Stubs ---

type fakeContactRepo struct {
	mu          sync.Mutex
	contacts    map[int64]domain.Contact
	nextID      int64
	softDeleted []int64
	purged      []int64
}

func newFakeContactRepo() *fakeContactRepo {
	return &fakeContactRepo{
		contacts: make(map[int64]domain.Contact),
		nextID:   1,
	}
}

func (r *fakeContactRepo) Upsert(ctx context.Context, c domain.Contact) (domain.Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var existingID int64 = 0
	for id, x := range r.contacts {
		if x.Email == c.Email {
			existingID = id
			break
		}
	}

	if existingID != 0 {
		existing := r.contacts[existingID]
		if c.FirstName != "" {
			existing.FirstName = c.FirstName
		}
		if c.LastName != "" {
			existing.LastName = c.LastName
		}
		if c.Company != "" {
			existing.Company = c.Company
		}
		if c.Phone != "" {
			existing.Phone = c.Phone
		}
		if c.Stage != "" {
			existing.Stage = c.Stage
		}
		if c.Tags != nil {
			existing.Tags = c.Tags
		}
		if c.Notes != "" {
			existing.Notes = c.Notes
		}
		if c.Custom != nil {
			existing.Custom = c.Custom
		}
		if c.Source != "" {
			existing.Source = c.Source
		}
		existing.UpdatedAt = time.Now()
		r.contacts[existingID] = existing
		return existing, nil
	}

	if c.ID == 0 {
		c.ID = r.nextID
		r.nextID++
	}
	if c.Stage == "" {
		c.Stage = domain.StageNew
	}
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	r.contacts[c.ID] = c
	return c, nil
}

func (r *fakeContactRepo) Get(ctx context.Context, id int64) (domain.Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.Contact{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *fakeContactRepo) GetByEmail(ctx context.Context, email string) (domain.Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range r.contacts {
		if c.Email == email && c.DeletedAt == nil {
			return c, nil
		}
	}
	return domain.Contact{}, domain.ErrNotFound
}

func (r *fakeContactRepo) GetByUnsubCode(ctx context.Context, code string) (domain.Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range r.contacts {
		if c.UnsubCode == code && c.DeletedAt == nil {
			return c, nil
		}
	}
	return domain.Contact{}, domain.ErrNotFound
}

func (r *fakeContactRepo) Update(ctx context.Context, id int64, patch domain.ContactPatch) (domain.Contact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

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
	c.UpdatedAt = time.Now()
	r.contacts[id] = c
	return c, nil
}

func (r *fakeContactRepo) List(ctx context.Context, f domain.ContactFilter, p port.Paging) (port.ContactPage, error) {
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
	total := len(matched)
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
			Total:      total,
			NextCursor: nextCursor,
		}, nil
	}

	return port.ContactPage{
		Items:      paged,
		Total:      total,
		NextCursor: 0,
	}, nil
}

func (r *fakeContactRepo) CountByStage(ctx context.Context) (map[string]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	counts := make(map[string]int)
	for _, c := range r.contacts {
		if c.DeletedAt != nil {
			continue
		}
		counts[string(c.Stage)]++
	}
	return counts, nil
}

func (r *fakeContactRepo) SoftDelete(ctx context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.ErrNotFound
	}
	now := time.Now()
	c.DeletedAt = &now
	r.contacts[id] = c
	r.softDeleted = append(r.softDeleted, id)
	return nil
}

func (r *fakeContactRepo) Purge(ctx context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.contacts[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(r.contacts, id)
	r.purged = append(r.purged, id)
	return nil
}

func (r *fakeContactRepo) SetUnsubscribed(ctx context.Context, id int64, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.ErrNotFound
	}
	c.UnsubscribedAt = &at
	r.contacts[id] = c
	return nil
}

func (r *fakeContactRepo) SetUnsubCode(ctx context.Context, id int64, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.ErrNotFound
	}
	c.UnsubCode = code
	r.contacts[id] = c
	return nil
}

func (r *fakeContactRepo) SetEmailStatus(ctx context.Context, id int64, v domain.EmailVerification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.contacts[id]
	if !ok || c.DeletedAt != nil {
		return domain.ErrNotFound
	}
	c.EmailStatus = v.Status
	c.EmailReason = v.Reason
	checked := v.CheckedAt
	c.EmailCheckedAt = &checked
	r.contacts[id] = c
	return nil
}

type fakeEventRepo struct {
	mu     sync.Mutex
	events []domain.EmailEvent
}

func (r *fakeEventRepo) Insert(ctx context.Context, e domain.EmailEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

func (r *fakeEventRepo) OverviewCounts(ctx context.Context) (map[string]int, error) {
	return nil, nil
}

func (r *fakeEventRepo) UniqueOpens(ctx context.Context, campaignID *int64) (int, error) {
	return 0, nil
}

func (r *fakeEventRepo) CampaignCounts(ctx context.Context, campaignID int64) (map[string]int, error) {
	return nil, nil
}

func (r *fakeEventRepo) CampaignUniqueOpens(ctx context.Context, campaignID int64) (int, error) {
	return 0, nil
}

func (r *fakeEventRepo) TopLinks(ctx context.Context, campaignID int64, limit int) ([]domain.LinkCount, error) {
	return nil, nil
}

type fakeExportRepo struct {
	mu      sync.Mutex
	exports map[string]domain.Export
}

func newFakeExportRepo() *fakeExportRepo {
	return &fakeExportRepo{
		exports: make(map[string]domain.Export),
	}
}

func (r *fakeExportRepo) Create(ctx context.Context, e domain.Export) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exports[e.ID] = e
	return nil
}

func (r *fakeExportRepo) Get(ctx context.Context, id string) (domain.Export, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.exports[id]
	if !ok {
		return domain.Export{}, domain.ErrNotFound
	}
	return e, nil
}

type stubIDGen struct {
	mu          sync.Mutex
	exportCount int
	unsubCount  int
}

func (g *stubIDGen) ExportID() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.exportCount++
	return fmt.Sprintf("exp-%d", g.exportCount), nil
}

func (g *stubIDGen) UnsubCode() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.unsubCount++
	return fmt.Sprintf("unsub-%d", g.unsubCount), nil
}

type stubClock struct {
	t time.Time
}

func (c *stubClock) Now() time.Time {
	return c.t
}

// --- Test Cases ---

func TestCreateValidatesEmail(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	clock := &stubClock{t: time.Now()}
	svc := service.NewContactService(repo, events, exports, idgen, clock, os.TempDir(), "http://localhost:8080")

	_, err := svc.Create(context.Background(), domain.Contact{Email: ""})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected error to wrap domain.ErrValidation, got: %v", err)
	}
	if !strings.Contains(err.Error(), "email required") {
		t.Fatalf("expected error message to contain 'email required', got: %v", err)
	}

	// Verify repo wasn't called
	if len(repo.contacts) > 0 {
		t.Fatal("repo should not be called when validation fails")
	}
}

func TestCreateDefaultsStage(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	clock := &stubClock{t: time.Now()}
	svc := service.NewContactService(repo, events, exports, idgen, clock, os.TempDir(), "http://localhost:8080")

	c, err := svc.Create(context.Background(), domain.Contact{Email: "test@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Stage != domain.StageNew {
		t.Fatalf("expected stage to default to 'new', got %q", c.Stage)
	}
}

func TestCreateRejectsBadStage(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	clock := &stubClock{t: time.Now()}
	svc := service.NewContactService(repo, events, exports, idgen, clock, os.TempDir(), "http://localhost:8080")

	_, err := svc.Create(context.Background(), domain.Contact{Email: "test@example.com", Stage: "vip"})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected domain.ErrValidation, got: %v", err)
	}
}

func TestUpdateRejectsBadStage(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	clock := &stubClock{t: time.Now()}
	svc := service.NewContactService(repo, events, exports, idgen, clock, os.TempDir(), "http://localhost:8080")

	// Pre-populate contact
	contact, err := repo.Upsert(context.Background(), domain.Contact{Email: "test@example.com", Stage: domain.StageNew})
	if err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	badStage := "vip"
	_, err = svc.Update(context.Background(), contact.ID, domain.ContactPatch{Stage: &badStage})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected domain.ErrValidation, got: %v", err)
	}
}

func TestListClampsLimit(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	clock := &stubClock{t: time.Now()}
	svc := service.NewContactService(repo, events, exports, idgen, clock, os.TempDir(), "http://localhost:8080")

	// Seed 20 contacts
	for i := 1; i <= 150; i++ {
		_, _ = repo.Upsert(context.Background(), domain.Contact{Email: fmt.Sprintf("test%d@example.com", i)})
	}

	// Test 0 limit (should default to 20)
	page0, err := svc.List(context.Background(), domain.ContactFilter{}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page0.Items) != 20 {
		t.Fatalf("expected 20 items for 0 limit, got %d", len(page0.Items))
	}

	// Test 500 limit (should clamp to 100)
	page500, err := svc.List(context.Background(), domain.ContactFilter{}, 500, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page500.Items) != 100 {
		t.Fatalf("expected 100 items for 500 limit, got %d", len(page500.Items))
	}
}

func TestImportMixed(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	clock := &stubClock{t: time.Now()}
	svc := service.NewContactService(repo, events, exports, idgen, clock, os.TempDir(), "http://localhost:8080")

	// Seed one contact to test updates vs inserts
	_, _ = repo.Upsert(context.Background(), domain.Contact{Email: "existing@example.com", FirstName: "Old"})

	arr := []domain.Contact{
		{Email: "good-arr@example.com", Stage: domain.StageNew},
		{Email: "", FirstName: "NoEmail"},                        // missing email -> skip
		{Email: "bad-stage@example.com", Stage: "invalid-stage"}, // bad stage -> skip
	}

	csvData := `email,first_name,last_name,company,phone,stage,tags,source
existing@example.com,NewName,Last,Corp,1234,contacted,"tag1;tag2",inbound
csv-good@example.com,CSV,Good,Corp,,new,tag3,outbound
,Missing,CSV,Company,,,tags,source
`

	inserted, updated, skipped, errs, err := svc.Import(context.Background(), arr, csvData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inserted != 2 { // "good-arr@example.com", "csv-good@example.com"
		t.Errorf("expected 2 inserted, got %d", inserted)
	}
	if updated != 1 { // "existing@example.com"
		t.Errorf("expected 1 updated, got %d", updated)
	}
	if skipped != 3 { // missing-email (arr), invalid-stage (arr), missing-email (csv)
		t.Errorf("expected 3 skipped, got %d", skipped)
	}

	if len(errs) != 3 {
		t.Fatalf("expected 3 error entries, got %d: %v", len(errs), errs)
	}

	// Assert substrings
	hasMissingEmailArr := false
	hasInvalidStageArr := false
	hasMissingEmailCSV := false
	for _, e := range errs {
		if strings.Contains(e, "contact 1: missing email") {
			hasMissingEmailArr = true
		}
		if strings.Contains(e, "contact 2: invalid stage") {
			hasInvalidStageArr = true
		}
		if strings.Contains(e, "row 4: missing email") {
			hasMissingEmailCSV = true
		}
	}

	if !hasMissingEmailArr {
		t.Errorf("missing error for missing email in array: %v", errs)
	}
	if !hasInvalidStageArr {
		t.Errorf("missing error for invalid stage in array: %v", errs)
	}
	if !hasMissingEmailCSV {
		t.Errorf("missing error for missing email in CSV: %v", errs)
	}
}

func TestImportEmpty(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	clock := &stubClock{t: time.Now()}
	svc := service.NewContactService(repo, events, exports, idgen, clock, os.TempDir(), "http://localhost:8080")

	_, _, _, _, err := svc.Import(context.Background(), nil, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "empty import") {
		t.Fatalf("expected error message to contain 'empty import', got: %v", err)
	}
}

func TestExportWritesCSVAndRow(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	fixedTime := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	clock := &stubClock{t: fixedTime}

	tempDir, err := os.MkdirTemp("", "crm-export-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := service.NewContactService(repo, events, exports, idgen, clock, tempDir, "http://localhost:8080")

	// Seed 3 contacts
	_, _ = repo.Upsert(context.Background(), domain.Contact{Email: "c1@example.com", FirstName: "A", Stage: domain.StageNew, Tags: []string{"tag1"}})
	_, _ = repo.Upsert(context.Background(), domain.Contact{Email: "c2@example.com", FirstName: "B", Stage: domain.StageContacted, Tags: []string{"tag2"}})
	_, _ = repo.Upsert(context.Background(), domain.Contact{Email: "c3@example.com", FirstName: "C", Stage: domain.StageQualified})

	exportID, url, rows, err := svc.Export(context.Background(), domain.ContactFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exportID != "exp-1" {
		t.Errorf("expected exportID 'exp-1', got %q", exportID)
	}
	expectedURL := "http://localhost:8080/export/exp-1.csv"
	if url != expectedURL {
		t.Errorf("expected url %q, got %q", expectedURL, url)
	}
	if rows != 3 {
		t.Errorf("expected rows 3, got %d", rows)
	}

	// Verify file exists and has correct content
	filePath := filepath.Join(tempDir, "exp-1.csv")
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open export file: %v", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		t.Fatalf("failed to read header: %v", err)
	}
	expectedHeader := []string{"id", "email", "first_name", "last_name", "company", "phone", "stage", "tags", "notes", "source"}
	if len(header) != len(expectedHeader) {
		t.Fatalf("expected header size %d, got %d", len(expectedHeader), len(header))
	}
	for i, h := range header {
		if h != expectedHeader[i] {
			t.Errorf("expected header column %q, got %q", expectedHeader[i], h)
		}
	}

	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to read all records: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 data rows, got %d", len(records))
	}

	// Check metadata stored in ExportRepo
	expRow, err := exports.Get(context.Background(), "exp-1")
	if err != nil {
		t.Fatalf("metadata not found: %v", err)
	}
	if expRow.Rows != 3 {
		t.Errorf("expected metadata rows 3, got %d", expRow.Rows)
	}
	expectedExpiresAt := fixedTime.Add(24 * time.Hour)
	if expRow.ExpiresAt == nil || !expRow.ExpiresAt.Equal(expectedExpiresAt) {
		t.Errorf("expected ExpiresAt to be %v, got %v", expectedExpiresAt, expRow.ExpiresAt)
	}
}

func TestSoftDeleteVsPurge(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	clock := &stubClock{t: time.Now()}
	svc := service.NewContactService(repo, events, exports, idgen, clock, os.TempDir(), "http://localhost:8080")

	c1, _ := repo.Upsert(context.Background(), domain.Contact{Email: "c1@example.com"})
	c2, _ := repo.Upsert(context.Background(), domain.Contact{Email: "c2@example.com"})

	// Soft delete c1
	err := svc.Delete(context.Background(), c1.ID, false)
	if err != nil {
		t.Fatalf("unexpected soft delete error: %v", err)
	}
	if len(repo.softDeleted) != 1 || repo.softDeleted[0] != c1.ID {
		t.Errorf("expected repo.SoftDelete to be called with ID %d, got: %v", c1.ID, repo.softDeleted)
	}

	// Purge c2
	err = svc.Delete(context.Background(), c2.ID, true)
	if err != nil {
		t.Fatalf("unexpected purge error: %v", err)
	}
	if len(repo.purged) != 1 || repo.purged[0] != c2.ID {
		t.Errorf("expected repo.Purge to be called with ID %d, got: %v", c2.ID, repo.purged)
	}

	// Delete unknown contact -> ErrNotFound
	err = svc.Delete(context.Background(), 999, false)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUnsubscribeByCode(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	fixedTime := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	clock := &stubClock{t: fixedTime}
	svc := service.NewContactService(repo, events, exports, idgen, clock, os.TempDir(), "http://localhost:8080")

	c, _ := repo.Upsert(context.Background(), domain.Contact{Email: "unsub@example.com", UnsubCode: "u1"})

	updated, err := svc.UnsubscribeByCode(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.UnsubscribedAt == nil || !updated.UnsubscribedAt.Equal(fixedTime) {
		t.Errorf("expected UnsubscribedAt to be set to %v, got %v", fixedTime, updated.UnsubscribedAt)
	}

	// Check event was logged
	if len(events.events) != 1 {
		t.Fatalf("expected 1 event to be logged, got %d", len(events.events))
	}
	ev := events.events[0]
	if ev.ContactID != c.ID || ev.Type != domain.EventUnsubscribe || !ev.TS.Equal(fixedTime) {
		t.Errorf("incorrect event logged: %+v", ev)
	}

	// Test unsubscribing unknown code -> ErrNotFound
	_, err = svc.UnsubscribeByCode(context.Background(), "unknown")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestEnsureUnsubCode(t *testing.T) {
	repo := newFakeContactRepo()
	events := &fakeEventRepo{}
	exports := newFakeExportRepo()
	idgen := &stubIDGen{}
	clock := &stubClock{t: time.Now()}
	svc := service.NewContactService(repo, events, exports, idgen, clock, os.TempDir(), "http://localhost:8080")

	// Pre-seeded with unsub code
	c1, _ := repo.Upsert(context.Background(), domain.Contact{Email: "c1@example.com", UnsubCode: "existing-code"})
	code, err := svc.EnsureUnsubCode(context.Background(), c1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "existing-code" {
		t.Errorf("expected 'existing-code', got %q", code)
	}

	// Pre-seeded without unsub code
	c2, _ := repo.Upsert(context.Background(), domain.Contact{Email: "c2@example.com"})
	code, err = svc.EnsureUnsubCode(context.Background(), c2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "unsub-1" {
		t.Errorf("expected generated code 'unsub-1', got %q", code)
	}

	// Verify repo has it updated
	updatedC2, _ := repo.Get(context.Background(), c2.ID)
	if updatedC2.UnsubCode != "unsub-1" {
		t.Errorf("expected UnsubCode in repo to be updated, got %q", updatedC2.UnsubCode)
	}
}

func TestBulkUpdateByIDsTagsAndStage(t *testing.T) {
	repo := newFakeContactRepo()
	ctx := context.Background()
	a, _ := repo.Upsert(ctx, domain.Contact{Email: "a@x.com", Tags: []string{"keep"}})
	b, _ := repo.Upsert(ctx, domain.Contact{Email: "b@x.com", Tags: []string{"old", "keep"}})
	svc := service.NewContactService(repo, &fakeEventRepo{}, newFakeExportRepo(), &stubIDGen{}, &stubClock{}, "", "http://crm.local")

	stage := "qualified"
	res, err := svc.BulkUpdateByIDs(ctx, []int64{a.ID, b.ID}, service.BulkPatch{
		Stage:      &stage,
		AddTags:    []string{"vip"},
		RemoveTags: []string{"old"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Matched != 2 || res.Updated != 2 || res.Skipped != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	ga, _ := repo.Get(ctx, a.ID)
	gb, _ := repo.Get(ctx, b.ID)
	if ga.Stage != domain.StageQualified || gb.Stage != domain.StageQualified {
		t.Errorf("expected stage qualified, got %q %q", ga.Stage, gb.Stage)
	}
	if !hasTag(ga.Tags, "vip") || !hasTag(ga.Tags, "keep") {
		t.Errorf("a tags wrong: %v", ga.Tags)
	}
	if hasTag(gb.Tags, "old") || !hasTag(gb.Tags, "vip") || !hasTag(gb.Tags, "keep") {
		t.Errorf("b tags wrong: %v", gb.Tags)
	}
}

func TestBulkUpdateByIDsValidation(t *testing.T) {
	repo := newFakeContactRepo()
	svc := service.NewContactService(repo, &fakeEventRepo{}, newFakeExportRepo(), &stubIDGen{}, &stubClock{}, "", "http://crm.local")
	ctx := context.Background()

	if _, err := svc.BulkUpdateByIDs(ctx, nil, service.BulkPatch{AddTags: []string{"x"}}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected validation error for empty ids")
	}
	if _, err := svc.BulkUpdateByIDs(ctx, []int64{1}, service.BulkPatch{}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected validation error for empty patch")
	}
	bad := "nope"
	if _, err := svc.BulkUpdateByIDs(ctx, []int64{1}, service.BulkPatch{Stage: &bad}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected validation error for invalid stage")
	}
}

func TestBulkUpdateByIDsMissingContactSkips(t *testing.T) {
	repo := newFakeContactRepo()
	ctx := context.Background()
	a, _ := repo.Upsert(ctx, domain.Contact{Email: "a@x.com"})
	svc := service.NewContactService(repo, &fakeEventRepo{}, newFakeExportRepo(), &stubIDGen{}, &stubClock{}, "", "http://crm.local")

	co := "Acme"
	res, err := svc.BulkUpdateByIDs(ctx, []int64{a.ID, 9999}, service.BulkPatch{Company: &co})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Matched != 2 || res.Updated != 1 || res.Skipped != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(res.Errors) != 1 {
		t.Errorf("expected one error entry, got %v", res.Errors)
	}
}

func TestBulkUpdateByFilter(t *testing.T) {
	repo := newFakeContactRepo()
	ctx := context.Background()
	_, _ = repo.Upsert(ctx, domain.Contact{Email: "a@x.com", Stage: domain.StageProposal})
	_, _ = repo.Upsert(ctx, domain.Contact{Email: "b@x.com", Stage: domain.StageProposal})
	_, _ = repo.Upsert(ctx, domain.Contact{Email: "c@x.com", Stage: domain.StageNew})
	svc := service.NewContactService(repo, &fakeEventRepo{}, newFakeExportRepo(), &stubIDGen{}, &stubClock{}, "", "http://crm.local")

	res, err := svc.BulkUpdateByFilter(ctx, domain.ContactFilter{Stage: "proposal"}, service.BulkPatch{AddTags: []string{"q3"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Matched != 2 || res.Updated != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}
	all, _ := repo.List(ctx, domain.ContactFilter{Tag: "q3"}, port.Paging{Limit: 100})
	if all.Total != 2 {
		t.Errorf("expected 2 tagged q3, got %d", all.Total)
	}
}

func TestBulkSetTagsOverwrites(t *testing.T) {
	repo := newFakeContactRepo()
	ctx := context.Background()
	a, _ := repo.Upsert(ctx, domain.Contact{Email: "a@x.com", Tags: []string{"old1", "old2"}})
	svc := service.NewContactService(repo, &fakeEventRepo{}, newFakeExportRepo(), &stubIDGen{}, &stubClock{}, "", "http://crm.local")

	newTags := []string{"only"}
	if _, err := svc.BulkUpdateByIDs(ctx, []int64{a.ID}, service.BulkPatch{SetTags: &newTags, AddTags: []string{"ignored"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ga, _ := repo.Get(ctx, a.ID)
	if len(ga.Tags) != 1 || ga.Tags[0] != "only" {
		t.Errorf("expected set_tags to overwrite, got %v", ga.Tags)
	}
}

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

type svcFakeVerifier struct{}

func (svcFakeVerifier) Verify(ctx context.Context, email string) domain.EmailVerification {
	st := domain.EmailValid
	reason := "deliverable"
	if strings.Contains(email, "bad") {
		st = domain.EmailInvalid
		reason = "domain has no mail server"
	}
	return domain.EmailVerification{Email: email, Status: st, Reason: reason, CheckedAt: time.Unix(0, 0)}
}

func TestCreateVerifiesEmailWhenVerifierSet(t *testing.T) {
	repo := newFakeContactRepo()
	svc := service.NewContactService(repo, &fakeEventRepo{}, newFakeExportRepo(), &stubIDGen{}, &stubClock{}, "", "http://crm.local")
	svc.SetVerifier(svcFakeVerifier{})
	ctx := context.Background()

	c, err := svc.Create(ctx, domain.Contact{Email: "good@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.EmailStatus != domain.EmailValid || c.EmailCheckedAt == nil {
		t.Fatalf("expected valid verified contact, got %+v", c)
	}
}

func TestAuditEmailsCountsByStatus(t *testing.T) {
	repo := newFakeContactRepo()
	svc := service.NewContactService(repo, &fakeEventRepo{}, newFakeExportRepo(), &stubIDGen{}, &stubClock{}, "", "http://crm.local")
	svc.SetVerifier(svcFakeVerifier{})
	ctx := context.Background()
	_, _ = repo.Upsert(ctx, domain.Contact{Email: "good1@example.com"})
	_, _ = repo.Upsert(ctx, domain.Contact{Email: "good2@example.com"})
	_, _ = repo.Upsert(ctx, domain.Contact{Email: "bad@nodomain.test"})

	res, err := svc.AuditEmails(ctx, domain.ContactFilter{}, false, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Checked != 3 || res.Valid != 2 || res.Invalid != 1 {
		t.Fatalf("unexpected audit result: %+v", res)
	}
}

func TestAuditEmailsDisabled(t *testing.T) {
	repo := newFakeContactRepo()
	svc := service.NewContactService(repo, &fakeEventRepo{}, newFakeExportRepo(), &stubIDGen{}, &stubClock{}, "", "http://crm.local")
	if _, err := svc.AuditEmails(context.Background(), domain.ContactFilter{}, false, 100, 0); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error when verifier disabled, got %v", err)
	}
}
