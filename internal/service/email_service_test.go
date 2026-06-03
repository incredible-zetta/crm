package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
)

type fakeSender struct {
	sent []port.OutboundMessage
	err  error
}

func (f *fakeSender) Send(ctx context.Context, msg port.OutboundMessage) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, msg)
	return nil
}

type fakeContactRepo struct {
	port.ContactRepo
	contacts map[int64]domain.Contact
}

func (f *fakeContactRepo) Get(ctx context.Context, id int64) (domain.Contact, error) {
	c, ok := f.contacts[id]
	if !ok {
		return domain.Contact{}, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeContactRepo) SetUnsubCode(ctx context.Context, id int64, code string) error {
	c, ok := f.contacts[id]
	if !ok {
		return domain.ErrNotFound
	}
	c.UnsubCode = code
	f.contacts[id] = c
	return nil
}

type fakeTemplateRepo struct {
	port.TemplateRepo
	templates map[int64]domain.Template
}

func (f *fakeTemplateRepo) Get(ctx context.Context, id int64) (domain.Template, error) {
	t, ok := f.templates[id]
	if !ok {
		return domain.Template{}, domain.ErrNotFound
	}
	return t, nil
}

type fakeTrackingRepo struct {
	port.TrackingRepo
	createdLinks []createdLink
	nextCode     string
}

type createdLink struct {
	targetURL  string
	campaignID *int64
	contactID  *int64
}

func (f *fakeTrackingRepo) CreateLink(ctx context.Context, targetURL string, campaignID, contactID *int64) (string, error) {
	f.createdLinks = append(f.createdLinks, createdLink{
		targetURL:  targetURL,
		campaignID: campaignID,
		contactID:  contactID,
	})
	return f.nextCode, nil
}

type fakeEventRepo struct {
	port.EventRepo
	events []domain.EmailEvent
	err    error
}

func (f *fakeEventRepo) Insert(ctx context.Context, e domain.EmailEvent) error {
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, e)
	return nil
}

type stubClock struct {
	now time.Time
}

func (s stubClock) Now() time.Time {
	return s.now
}

type stubIDGenerator struct{}

func (s stubIDGenerator) ExportID() (string, error)  { return "export123", nil }
func (s stubIDGenerator) UnsubCode() (string, error) { return "unsub123", nil }

func TestSendResolvesContactEmail(t *testing.T) {
	contacts := &fakeContactRepo{
		contacts: map[int64]domain.Contact{
			1: {
				ID:    1,
				Email: "test@example.com",
			},
		},
	}
	sender := &fakeSender{}
	events := &fakeEventRepo{}
	svc := NewEmailService(sender, contacts, &fakeTemplateRepo{}, &fakeTrackingRepo{}, events, stubClock{}, stubIDGenerator{}, "http://crm.local")

	status, resolvedTo, err := svc.Send(context.Background(), SendInput{
		ContactID: 1,
		Subject:   "Hello",
		Text:      "Test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "sent" {
		t.Errorf("expected status 'sent', got %q", status)
	}
	if resolvedTo != "test@example.com" {
		t.Errorf("expected resolvedTo 'test@example.com', got %q", resolvedTo)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sender.sent))
	}
	if sender.sent[0].To != "test@example.com" {
		t.Errorf("expected recipient 'test@example.com', got %q", sender.sent[0].To)
	}
	if len(events.events) != 1 {
		t.Fatalf("expected 1 event logged, got %d", len(events.events))
	}
	if events.events[0].Type != domain.EventSent {
		t.Errorf("expected event type 'sent', got %q", events.events[0].Type)
	}
}

func TestSendUnsubscribedGuard(t *testing.T) {
	now := time.Now()
	contacts := &fakeContactRepo{
		contacts: map[int64]domain.Contact{
			2: {
				ID:             2,
				Email:          "unsubbed@example.com",
				UnsubscribedAt: &now,
			},
		},
	}
	sender := &fakeSender{}
	events := &fakeEventRepo{}
	svc := NewEmailService(sender, contacts, &fakeTemplateRepo{}, &fakeTrackingRepo{}, events, stubClock{}, stubIDGenerator{}, "http://crm.local")

	_, _, err := svc.Send(context.Background(), SendInput{
		ContactID: 2,
		Subject:   "Hello",
		Text:      "Test",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected error to wrap ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "contact is unsubscribed") {
		t.Errorf("expected 'contact is unsubscribed' error message, got %q", err.Error())
	}
	if len(sender.sent) != 0 {
		t.Errorf("sender called when contact unsubscribed")
	}
	if len(events.events) != 0 {
		t.Errorf("events logged when contact unsubscribed")
	}
}

func TestSendRecipientMismatch(t *testing.T) {
	contacts := &fakeContactRepo{
		contacts: map[int64]domain.Contact{
			1: {
				ID:    1,
				Email: "test@example.com",
			},
		},
	}
	sender := &fakeSender{}
	events := &fakeEventRepo{}
	svc := NewEmailService(sender, contacts, &fakeTemplateRepo{}, &fakeTrackingRepo{}, events, stubClock{}, stubIDGenerator{}, "http://crm.local")

	_, _, err := svc.Send(context.Background(), SendInput{
		ContactID: 1,
		To:        "other@example.com",
		Subject:   "Hello",
		Text:      "Test",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected error to wrap ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "to does not match contact email") {
		t.Errorf("expected 'to does not match contact email' error message, got %q", err.Error())
	}
	if len(sender.sent) != 0 {
		t.Errorf("sender called on recipient mismatch")
	}
	if len(events.events) != 0 {
		t.Errorf("events logged on recipient mismatch")
	}
}

func TestSendDirectAddress(t *testing.T) {
	sender := &fakeSender{}
	events := &fakeEventRepo{}
	svc := NewEmailService(sender, &fakeContactRepo{}, &fakeTemplateRepo{}, &fakeTrackingRepo{}, events, stubClock{}, stubIDGenerator{}, "http://crm.local")

	status, resolvedTo, err := svc.Send(context.Background(), SendInput{
		ContactID: 0,
		To:        "direct@example.com",
		Subject:   "Direct",
		Text:      "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "sent" {
		t.Errorf("expected status 'sent', got %q", status)
	}
	if resolvedTo != "direct@example.com" {
		t.Errorf("expected resolvedTo 'direct@example.com', got %q", resolvedTo)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sender.sent))
	}
	if sender.sent[0].To != "direct@example.com" {
		t.Errorf("expected recipient 'direct@example.com', got %q", sender.sent[0].To)
	}

	// empty to address with ContactID = 0
	_, _, err = svc.Send(context.Background(), SendInput{
		ContactID: 0,
		To:        "",
		Subject:   "Direct",
		Text:      "Hello",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected error to wrap ErrValidation, got %v", err)
	}
	if !strings.Contains(err.Error(), "recipient required") {
		t.Errorf("expected 'recipient required' error, got %q", err.Error())
	}
}

func TestSendTemplateRender(t *testing.T) {
	templates := &fakeTemplateRepo{
		templates: map[int64]domain.Template{
			10: {
				ID:       10,
				Subject:  "Hello {{.name}}",
				BodyText: "Welcome {{.name}} to {{.company}}!",
			},
		},
	}
	sender := &fakeSender{}
	svc := NewEmailService(sender, &fakeContactRepo{}, templates, &fakeTrackingRepo{}, &fakeEventRepo{}, stubClock{}, stubIDGenerator{}, "http://crm.local")

	_, _, err := svc.Send(context.Background(), SendInput{
		To:         "test@example.com",
		TemplateID: 10,
		Vars: map[string]any{
			"name":    "Alice",
			"company": "Wonderland",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sender.sent))
	}
	if sender.sent[0].Subject != "Hello Alice" {
		t.Errorf("expected rendered subject 'Hello Alice', got %q", sender.sent[0].Subject)
	}
	if sender.sent[0].Text != "Welcome Alice to Wonderland!" {
		t.Errorf("expected rendered text 'Welcome Alice to Wonderland!', got %q", sender.sent[0].Text)
	}

	// test missing template
	_, _, err = svc.Send(context.Background(), SendInput{
		To:         "test@example.com",
		TemplateID: 999,
	})
	if err == nil {
		t.Fatal("expected error for missing template, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected template ErrNotFound to pass through, got %v", err)
	}
}

func TestSendRewritesLinksAndPixel(t *testing.T) {
	contacts := &fakeContactRepo{
		contacts: map[int64]domain.Contact{
			12: {
				ID:    12,
				Email: "test@example.com",
			},
		},
	}
	sender := &fakeSender{}
	tracking := &fakeTrackingRepo{nextCode: "lnk123"}
	svc := NewEmailService(sender, contacts, &fakeTemplateRepo{}, tracking, &fakeEventRepo{}, stubClock{}, stubIDGenerator{}, "http://crm.local")
	svc.openCodeFn = func() (string, error) { return "pxl123", nil }

	campaignID := int64(45)
	_, _, err := svc.Send(context.Background(), SendInput{
		ContactID:  12,
		CampaignID: &campaignID,
		To:         "test@example.com",
		HTML:       `<html><body><a href="https://google.com">Google</a></body></html>`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sender.sent))
	}
	html := sender.sent[0].HTML

	if !strings.Contains(html, "http://crm.local/t/lnk123") {
		t.Errorf("expected rewritten link with crm.local/t/lnk123, got %q", html)
	}
	if !strings.Contains(html, `src="http://crm.local/o/pxl123.png"`) {
		t.Errorf("expected pixel img tag injected with crm.local/o/pxl123.png, got %q", html)
	}
	if !strings.Contains(html, "http://crm.local/u/unsub123") {
		t.Errorf("expected unsubscribe footer link with crm.local/u/unsub123, got %q", html)
	}

	if len(tracking.createdLinks) != 1 {
		t.Fatalf("expected 1 link registered, got %d", len(tracking.createdLinks))
	}
	cl := tracking.createdLinks[0]
	if cl.targetURL != "https://google.com" {
		t.Errorf("expected target URL 'https://google.com', got %q", cl.targetURL)
	}
	if cl.campaignID == nil || *cl.campaignID != 45 {
		t.Errorf("expected campaign ID 45, got %v", cl.campaignID)
	}
	if cl.contactID == nil || *cl.contactID != 12 {
		t.Errorf("expected contact ID 12, got %v", cl.contactID)
	}
}

func TestSendFailureLogsFailedEvent(t *testing.T) {
	sender := &fakeSender{err: errors.New("smtp timeout")}
	events := &fakeEventRepo{}
	svc := NewEmailService(sender, &fakeContactRepo{}, &fakeTemplateRepo{}, &fakeTrackingRepo{}, events, stubClock{}, stubIDGenerator{}, "http://crm.local")

	_, _, err := svc.Send(context.Background(), SendInput{
		To:      "test@example.com",
		Subject: "Hello",
		Text:    "Test",
	})
	if err == nil {
		t.Fatal("expected sending error, got nil")
	}
	if !strings.Contains(err.Error(), "send failed") {
		t.Errorf("expected 'send failed' wrapping, got %q", err.Error())
	}

	if len(events.events) != 1 {
		t.Fatalf("expected 1 event logged, got %d", len(events.events))
	}
	ev := events.events[0]
	if ev.Type != domain.EventFailed {
		t.Errorf("expected event type 'failed', got %q", ev.Type)
	}
	if ev.Meta == nil || ev.Meta["error"] != "smtp timeout" {
		t.Errorf("expected error in meta, got %v", ev.Meta)
	}
}

func TestSendToContactSatisfiesMailer(t *testing.T) {
	contacts := &fakeContactRepo{
		contacts: map[int64]domain.Contact{
			5: {
				ID:        5,
				Email:     "subscribed@example.com",
				FirstName: "Bob",
				LastName:  "Smith",
				Company:   "Acme",
			},
		},
	}
	sender := &fakeSender{}
	templates := &fakeTemplateRepo{
		templates: map[int64]domain.Template{
			1: {
				ID:       1,
				Subject:  "Hello {{.first_name}}",
				BodyText: "Hi {{.first_name}} {{.last_name}} from {{.company}}!",
			},
		},
	}
	svc := NewEmailService(sender, contacts, templates, &fakeTrackingRepo{}, &fakeEventRepo{}, stubClock{}, stubIDGenerator{}, "http://crm.local")

	err := svc.SendToContact(context.Background(), contacts.contacts[5], 1, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sender.sent))
	}
	msg := sender.sent[0]
	if msg.To != "subscribed@example.com" {
		t.Errorf("expected recipient 'subscribed@example.com', got %q", msg.To)
	}
	if msg.Subject != "Hello Bob" {
		t.Errorf("expected rendered subject 'Hello Bob', got %q", msg.Subject)
	}
	if msg.Text != "Hi Bob Smith from Acme!" {
		t.Errorf("expected rendered text 'Hi Bob Smith from Acme!', got %q", msg.Text)
	}

	// verify that campaign_service.CampaignMailer compile-time assert compiles.
	var _ CampaignMailer = svc
}
