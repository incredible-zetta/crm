package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// fakeTaskRepo records task operations for assertions.
type fakeTaskRepo struct {
	port.TaskRepo
	inserted    []domain.ScheduledTask
	nextID      int64
	listStatus  string
	listLimit   int
	listResult  []domain.ScheduledTask
	cancelID    int64
	cancelErr   error
	cancelCalls int
}

func (f *fakeTaskRepo) Insert(ctx context.Context, t domain.ScheduledTask) (int64, error) {
	f.inserted = append(f.inserted, t)
	f.nextID++
	return f.nextID, nil
}

func (f *fakeTaskRepo) List(ctx context.Context, status string, limit int) ([]domain.ScheduledTask, error) {
	f.listStatus = status
	f.listLimit = limit
	return f.listResult, nil
}

func (f *fakeTaskRepo) Cancel(ctx context.Context, id int64) error {
	f.cancelCalls++
	f.cancelID = id
	return f.cancelErr
}

// newTaskTestEmailService builds a working EmailService backed by simple fakes.
func newTaskTestEmailService(sender *fakeSender, contacts *fakeContactRepo) *EmailService {
	if contacts == nil {
		contacts = &fakeContactRepo{contacts: map[int64]domain.Contact{}}
	}
	return NewEmailService(sender, contacts, &fakeTemplateRepo{templates: map[int64]domain.Template{}},
		&fakeTrackingRepo{}, &fakeEventRepo{}, stubClock{}, stubIDGenerator{}, "http://crm.local")
}

func TestScheduleValidatesKind(t *testing.T) {
	repo := &fakeTaskRepo{}
	svc := NewTaskService(repo, stubClock{}, nil, nil)

	if _, err := svc.Schedule(context.Background(), "sms", nil, time.Now().Add(time.Hour)); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation for bad kind, got %v", err)
	}

	id, err := svc.Schedule(context.Background(), "email", map[string]any{"to": "x@y"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}
	if _, err := svc.Schedule(context.Background(), "campaign", map[string]any{"campaign_id": 1}, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("unexpected error for campaign kind: %v", err)
	}
	if len(repo.inserted) != 2 {
		t.Errorf("expected 2 inserted tasks, got %d", len(repo.inserted))
	}
}

func TestListValidatesStatusAndClamps(t *testing.T) {
	repo := &fakeTaskRepo{}
	svc := NewTaskService(repo, stubClock{}, nil, nil)

	if _, err := svc.List(context.Background(), "bogus", 10); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation for bad status, got %v", err)
	}

	if _, err := svc.List(context.Background(), "", 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.listLimit != 50 {
		t.Errorf("expected clamped limit 50, got %d", repo.listLimit)
	}

	if _, err := svc.List(context.Background(), "pending", 999); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.listLimit != 200 {
		t.Errorf("expected clamped limit 200, got %d", repo.listLimit)
	}
	if repo.listStatus != "pending" {
		t.Errorf("expected status pending, got %q", repo.listStatus)
	}
}

func TestCancelDelegates(t *testing.T) {
	repo := &fakeTaskRepo{}
	svc := NewTaskService(repo, stubClock{}, nil, nil)

	if err := svc.Cancel(context.Background(), 7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.cancelID != 7 || repo.cancelCalls != 1 {
		t.Errorf("expected Cancel(7) once, got id=%d calls=%d", repo.cancelID, repo.cancelCalls)
	}

	repo.cancelErr = domain.ErrNotFound
	if err := svc.Cancel(context.Background(), 8); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound passthrough, got %v", err)
	}

	repo.cancelErr = domain.ErrConflict
	if err := svc.Cancel(context.Background(), 9); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected ErrConflict passthrough, got %v", err)
	}
}

func TestExecuteEmail(t *testing.T) {
	sender := &fakeSender{}
	svc := NewTaskService(&fakeTaskRepo{}, stubClock{}, newTaskTestEmailService(sender, nil), nil)

	task := domain.ScheduledTask{
		Kind:    domain.TaskEmail,
		Payload: map[string]any{"to": "x@y.com", "subject": "Hi", "text": "yo"},
	}
	if err := svc.Execute(context.Background(), task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sender.sent))
	}
	if sender.sent[0].To != "x@y.com" {
		t.Errorf("expected To x@y.com, got %q", sender.sent[0].To)
	}
}

func TestExecuteEmailContactID(t *testing.T) {
	// subscribed contact -> sends
	sender := &fakeSender{}
	contacts := &fakeContactRepo{contacts: map[int64]domain.Contact{
		5: {ID: 5, Email: "sub@y.com"},
	}}
	svc := NewTaskService(&fakeTaskRepo{}, stubClock{}, newTaskTestEmailService(sender, contacts), nil)
	task := domain.ScheduledTask{Kind: domain.TaskEmail, Payload: map[string]any{"contact_id": float64(5), "subject": "Hi", "text": "yo"}}
	if err := svc.Execute(context.Background(), task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sent) != 1 || sender.sent[0].To != "sub@y.com" {
		t.Fatalf("expected send to sub@y.com, got %+v", sender.sent)
	}

	// unsubscribed contact -> guard error, no send
	now := time.Now()
	sender2 := &fakeSender{}
	contacts2 := &fakeContactRepo{contacts: map[int64]domain.Contact{
		6: {ID: 6, Email: "no@y.com", UnsubscribedAt: &now},
	}}
	svc2 := NewTaskService(&fakeTaskRepo{}, stubClock{}, newTaskTestEmailService(sender2, contacts2), nil)
	task2 := domain.ScheduledTask{Kind: domain.TaskEmail, Payload: map[string]any{"contact_id": float64(6), "subject": "Hi", "text": "yo"}}
	if err := svc2.Execute(context.Background(), task2); err == nil {
		t.Fatal("expected error for unsubscribed contact")
	}
	if len(sender2.sent) != 0 {
		t.Errorf("expected no send for unsubscribed contact, got %d", len(sender2.sent))
	}
}

func TestExecuteCampaign(t *testing.T) {
	// Wire a CampaignService that sends to one subscribed contact.
	sender := &fakeSender{}
	contacts := &fakeContactRepo{contacts: map[int64]domain.Contact{1: {ID: 1, Email: "a@y.com"}}}
	email := NewEmailService(sender, contacts,
		&fakeTemplateRepo{templates: map[int64]domain.Template{1: {ID: 1, Subject: "Hi", BodyText: "Body"}}},
		&fakeTrackingRepo{}, &fakeEventRepo{}, stubClock{}, stubIDGenerator{}, "http://crm.local")

	campRepo := &taskFakeCampaignRepo{campaigns: map[int64]domain.Campaign{
		7: {ID: 7, Name: "c", TemplateID: 1, Status: domain.CampaignDraft},
	}}
	campContacts := &taskFakeListContactRepo{items: []domain.Contact{{ID: 1, Email: "a@y.com"}}}
	camp := NewCampaignService(campRepo, campContacts, &fakeEventRepo{}, email)

	svc := NewTaskService(&fakeTaskRepo{}, stubClock{}, email, camp)
	task := domain.ScheduledTask{Kind: domain.TaskCampaign, Payload: map[string]any{"campaign_id": float64(7)}}
	if err := svc.Execute(context.Background(), task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Errorf("expected campaign to send 1 email, got %d", len(sender.sent))
	}
	if !campRepo.statusUpdated {
		t.Error("expected campaign status updated to sent")
	}
}

func TestExecuteCampaignMissingID(t *testing.T) {
	svc := NewTaskService(&fakeTaskRepo{}, stubClock{}, nil, nil)
	task := domain.ScheduledTask{Kind: domain.TaskCampaign, Payload: map[string]any{}}
	if err := svc.Execute(context.Background(), task); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation for missing campaign_id, got %v", err)
	}
}

func TestExecuteUnknownKind(t *testing.T) {
	svc := NewTaskService(&fakeTaskRepo{}, stubClock{}, nil, nil)
	task := domain.ScheduledTask{Kind: domain.TaskKind("weird"), Payload: map[string]any{}}
	err := svc.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for unknown task kind")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation for unknown kind, got %v", err)
	}
}

func TestExecuteJSONNumberDecode(t *testing.T) {
	sender := &fakeSender{}
	contacts := &fakeContactRepo{contacts: map[int64]domain.Contact{
		5: {ID: 5, Email: "five@y.com"},
	}}
	svc := NewTaskService(&fakeTaskRepo{}, stubClock{}, newTaskTestEmailService(sender, contacts), nil)
	// contact_id as float64(5) (JSON-decoded) must resolve to int64(5)
	task := domain.ScheduledTask{Kind: domain.TaskEmail, Payload: map[string]any{"contact_id": float64(5), "subject": "s", "text": "t"}}
	if err := svc.Execute(context.Background(), task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sent) != 1 || sender.sent[0].To != "five@y.com" {
		t.Fatalf("expected float64 contact_id to decode to id 5, got %+v", sender.sent)
	}
}

// --- minimal campaign fakes (distinct names to avoid collision with campaign_service_test external package) ---

type taskFakeCampaignRepo struct {
	port.CampaignRepo
	campaigns     map[int64]domain.Campaign
	statusUpdated bool
}

func (f *taskFakeCampaignRepo) Get(ctx context.Context, id int64) (domain.Campaign, error) {
	c, ok := f.campaigns[id]
	if !ok {
		return domain.Campaign{}, domain.ErrNotFound
	}
	return c, nil
}

func (f *taskFakeCampaignRepo) UpdateStatus(ctx context.Context, id int64, status domain.CampaignStatus) error {
	f.statusUpdated = true
	return nil
}

type taskFakeListContactRepo struct {
	port.ContactRepo
	items []domain.Contact
}

func (f *taskFakeListContactRepo) List(ctx context.Context, filter domain.ContactFilter, p port.Paging) (port.ContactPage, error) {
	if p.Cursor > 0 {
		return port.ContactPage{}, nil
	}
	return port.ContactPage{Items: f.items, Total: len(f.items)}, nil
}
