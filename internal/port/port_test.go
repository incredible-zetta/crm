package port_test

import (
	"context"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// Ensure a trivial stub implements Clock interface.
type stubClock struct {
	now time.Time
}

func (c stubClock) Now() time.Time {
	return c.now
}

var _ port.Clock = stubClock{}

// Ensure a trivial stub implements IDGenerator interface.
type stubIDGen struct{}

func (g stubIDGen) ExportID() (string, error) {
	return "export1234567890", nil
}

func (g stubIDGen) UnsubCode() (string, error) {
	return "unsub12345678901", nil
}

var _ port.IDGenerator = stubIDGen{}

// Stub implementations for repositories and sender to serve as compile-time guards.
type stubContactRepo struct{}

func (r stubContactRepo) Upsert(ctx context.Context, c domain.Contact) (domain.Contact, error) {
	return c, nil
}
func (r stubContactRepo) Get(ctx context.Context, id int64) (domain.Contact, error) {
	return domain.Contact{}, nil
}
func (r stubContactRepo) GetByEmail(ctx context.Context, email string) (domain.Contact, error) {
	return domain.Contact{}, nil
}
func (r stubContactRepo) GetByPhone(ctx context.Context, phone string) (domain.Contact, error) {
	return domain.Contact{}, nil
}
func (r stubContactRepo) GetByUnsubCode(ctx context.Context, code string) (domain.Contact, error) {
	return domain.Contact{}, nil
}
func (r stubContactRepo) Update(ctx context.Context, id int64, patch domain.ContactPatch) (domain.Contact, error) {
	return domain.Contact{}, nil
}
func (r stubContactRepo) List(ctx context.Context, f domain.ContactFilter, p port.Paging) (port.ContactPage, error) {
	return port.ContactPage{}, nil
}
func (r stubContactRepo) CountByStage(ctx context.Context) (map[string]int, error) {
	return nil, nil
}
func (r stubContactRepo) SoftDelete(ctx context.Context, id int64) error {
	return nil
}
func (r stubContactRepo) Purge(ctx context.Context, id int64) error {
	return nil
}
func (r stubContactRepo) SetUnsubscribed(ctx context.Context, id int64, at time.Time) error {
	return nil
}
func (r stubContactRepo) SetUnsubCode(ctx context.Context, id int64, code string) error {
	return nil
}

func (r stubContactRepo) SetEmailStatus(ctx context.Context, id int64, v domain.EmailVerification) error {
	return nil
}

func (r stubContactRepo) SetWhatsAppStatus(ctx context.Context, id int64, v domain.WhatsAppCheck) error {
	return nil
}

var _ port.ContactRepo = stubContactRepo{}

type stubCampaignRepo struct{}

func (r stubCampaignRepo) Create(ctx context.Context, c domain.Campaign) (domain.Campaign, error) {
	return c, nil
}
func (r stubCampaignRepo) Get(ctx context.Context, id int64) (domain.Campaign, error) {
	return domain.Campaign{}, nil
}
func (r stubCampaignRepo) List(ctx context.Context) ([]domain.Campaign, error) {
	return nil, nil
}
func (r stubCampaignRepo) UpdateStatus(ctx context.Context, id int64, status domain.CampaignStatus) error {
	return nil
}
func (r stubCampaignRepo) Update(ctx context.Context, id int64, c domain.Campaign) (domain.Campaign, error) {
	return c, nil
}
func (r stubCampaignRepo) SetStats(ctx context.Context, id int64, stats map[string]any) error {
	return nil
}
func (r stubCampaignRepo) SoftDelete(ctx context.Context, id int64) error {
	return nil
}

var _ port.CampaignRepo = stubCampaignRepo{}

type stubTemplateRepo struct{}

func (r stubTemplateRepo) Create(ctx context.Context, t domain.Template) (domain.Template, error) {
	return t, nil
}
func (r stubTemplateRepo) Get(ctx context.Context, id int64) (domain.Template, error) {
	return domain.Template{}, nil
}
func (r stubTemplateRepo) GetByName(ctx context.Context, name string) (domain.Template, error) {
	return domain.Template{}, nil
}
func (r stubTemplateRepo) List(ctx context.Context) ([]domain.Template, error) {
	return nil, nil
}
func (r stubTemplateRepo) Update(ctx context.Context, id int64, t domain.Template) (domain.Template, error) {
	return t, nil
}
func (r stubTemplateRepo) SoftDelete(ctx context.Context, id int64) error {
	return nil
}

var _ port.TemplateRepo = stubTemplateRepo{}

type stubTaskRepo struct{}

func (r stubTaskRepo) Insert(ctx context.Context, t domain.ScheduledTask) (int64, error) {
	return 0, nil
}
func (r stubTaskRepo) List(ctx context.Context, status string, limit int) ([]domain.ScheduledTask, error) {
	return nil, nil
}
func (r stubTaskRepo) ClaimDue(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledTask, error) {
	return nil, nil
}
func (r stubTaskRepo) MarkDone(ctx context.Context, id int64) error {
	return nil
}
func (r stubTaskRepo) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	return nil
}
func (r stubTaskRepo) Cancel(ctx context.Context, id int64) error {
	return nil
}

var _ port.TaskRepo = stubTaskRepo{}

type stubEventRepo struct{}

func (r stubEventRepo) Insert(ctx context.Context, e domain.EmailEvent) error {
	return nil
}
func (r stubEventRepo) OverviewCounts(ctx context.Context) (map[string]int, error) {
	return nil, nil
}
func (r stubEventRepo) UniqueOpens(ctx context.Context, campaignID *int64) (int, error) {
	return 0, nil
}
func (r stubEventRepo) CampaignCounts(ctx context.Context, campaignID int64) (map[string]int, error) {
	return nil, nil
}
func (r stubEventRepo) CampaignUniqueOpens(ctx context.Context, campaignID int64) (int, error) {
	return 0, nil
}
func (r stubEventRepo) TopLinks(ctx context.Context, campaignID int64, limit int) ([]domain.LinkCount, error) {
	return nil, nil
}

var _ port.EventRepo = stubEventRepo{}

type stubTrackingRepo struct{}

func (r stubTrackingRepo) CreateLink(ctx context.Context, targetURL string, campaignID, contactID *int64) (string, error) {
	return "", nil
}
func (r stubTrackingRepo) GetLink(ctx context.Context, code string) (domain.TrackingLink, error) {
	return domain.TrackingLink{}, nil
}

var _ port.TrackingRepo = stubTrackingRepo{}

type stubExportRepo struct{}

func (r stubExportRepo) Create(ctx context.Context, e domain.Export) error {
	return nil
}
func (r stubExportRepo) Get(ctx context.Context, id string) (domain.Export, error) {
	return domain.Export{}, nil
}

var _ port.ExportRepo = stubExportRepo{}

type stubEmailSender struct{}

func (s stubEmailSender) Send(ctx context.Context, msg port.OutboundMessage) error {
	return nil
}

var _ port.EmailSender = stubEmailSender{}

func TestClockStub(t *testing.T) {
	fixedTime := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	clock := stubClock{now: fixedTime}

	if got := clock.Now(); !got.Equal(fixedTime) {
		t.Errorf("expected %v, got %v", fixedTime, got)
	}
}

func TestIDGeneratorStub(t *testing.T) {
	gen := stubIDGen{}
	exportID, err := gen.ExportID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exportID != "export1234567890" {
		t.Errorf("expected export1234567890, got %s", exportID)
	}

	unsubCode, err := gen.UnsubCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unsubCode != "unsub12345678901" {
		t.Errorf("expected unsub12345678901, got %s", unsubCode)
	}
}
