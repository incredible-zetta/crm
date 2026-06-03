package email

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type fakeSender struct {
	lastMsg Message
	err     error
	calls   int
}

func (f *fakeSender) Send(ctx context.Context, msg Message) error {
	f.calls++
	f.lastMsg = msg
	return f.err
}

type fakeTemplateStore struct {
	templates map[int64]TemplateData
	err       error
}

func (f *fakeTemplateStore) GetTemplate(ctx context.Context, id int64) (TemplateData, error) {
	if f.err != nil {
		return TemplateData{}, f.err
	}
	t, ok := f.templates[id]
	if !ok {
		return TemplateData{}, errors.New("template not found")
	}
	return t, nil
}

type fakeLinkMaker struct {
	calls []struct {
		target     string
		campaignID *int64
		contactID  *int64
	}
	err error
}

func (f *fakeLinkMaker) CreateLink(ctx context.Context, targetURL string, campaignID, contactID *int64) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.calls = append(f.calls, struct {
		target     string
		campaignID *int64
		contactID  *int64
	}{target: targetURL, campaignID: campaignID, contactID: contactID})
	return fmt.Sprintf("c%d", len(f.calls)), nil
}

type loggedEvent struct {
	contactID  int64
	campaignID *int64
	eventType  string
	linkCode   string
	meta       map[string]any
}

type fakeEventLogger struct {
	events []loggedEvent
	err    error
}

func (f *fakeEventLogger) LogEvent(ctx context.Context, contactID int64, campaignID *int64, eventType, linkCode string, meta map[string]any) error {
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, loggedEvent{
		contactID:  contactID,
		campaignID: campaignID,
		eventType:  eventType,
		linkCode:   linkCode,
		meta:       meta,
	})
	return nil
}

func TestSendRawHTMLRewritesAndLogsSent(t *testing.T) {
	sender := &fakeSender{}
	store := &fakeTemplateStore{}
	links := &fakeLinkMaker{}
	events := &fakeEventLogger{}

	p := &Pipeline{
		Sender:  sender,
		Tmpl:    store,
		Links:   links,
		Events:  events,
		BaseURL: "http://test.com",
	}

	campaignID := int64(42)
	in := SendInput{
		ContactID:  100,
		CampaignID: &campaignID,
		To:         "alice@test.com",
		Subject:    "Hello {{.User}}",
		HTML:       `Click <a href="https://x.com">here</a>!`,
		Vars:       map[string]any{"User": "Alice"},
	}

	ctx := context.Background()
	err := p.Send(ctx, in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Assert links were created
	if len(links.calls) != 1 {
		t.Errorf("expected 1 CreateLink call, got %d", len(links.calls))
	} else {
		call := links.calls[0]
		if call.target != "https://x.com" {
			t.Errorf("expected target 'https://x.com', got %q", call.target)
		}
		if call.campaignID == nil || *call.campaignID != 42 {
			t.Errorf("expected campaign ID 42, got %v", call.campaignID)
		}
		if call.contactID == nil || *call.contactID != 100 {
			t.Errorf("expected contact ID 100, got %v", call.contactID)
		}
	}

	// Assert message properties
	if sender.calls != 1 {
		t.Fatalf("expected 1 sender call, got %d", sender.calls)
	}
	msg := sender.lastMsg
	if msg.To != "alice@test.com" {
		t.Errorf("expected to 'alice@test.com', got %q", msg.To)
	}
	if msg.Subject != "Hello Alice" {
		t.Errorf("expected subject 'Hello Alice', got %q", msg.Subject)
	}
	expectedRewritten := "http://test.com/t/c1"
	if !strings.Contains(msg.HTML, expectedRewritten) {
		t.Errorf("expected HTML to contain %q, got %q", expectedRewritten, msg.HTML)
	}
	if !strings.Contains(msg.HTML, "/o/") {
		t.Errorf("expected HTML to contain pixel marker '/o/', got %q", msg.HTML)
	}

	// Assert exactly one sent event logged
	if len(events.events) != 1 {
		t.Fatalf("expected 1 event logged, got %d", len(events.events))
	}
	evt := events.events[0]
	if evt.contactID != 100 {
		t.Errorf("expected contact ID 100, got %d", evt.contactID)
	}
	if evt.campaignID == nil || *evt.campaignID != 42 {
		t.Errorf("expected campaign ID 42, got %v", evt.campaignID)
	}
	if evt.eventType != "sent" {
		t.Errorf("expected event type 'sent', got %q", evt.eventType)
	}
	openCode, ok := evt.meta["open_code"].(string)
	if !ok || openCode == "" {
		t.Errorf("expected open_code in meta, got %v", evt.meta)
	}
	if evt.linkCode != openCode {
		t.Errorf("expected linkCode == openCode, got linkCode=%q, openCode=%q", evt.linkCode, openCode)
	}
	// Verify pixel URL matches the generated openCode
	expectedPixelURL := "http://test.com/o/" + openCode + ".png"
	if !strings.Contains(msg.HTML, expectedPixelURL) {
		t.Errorf("expected HTML to contain pixel URL %q, got %q", expectedPixelURL, msg.HTML)
	}
}

func TestSendTemplateLoaded(t *testing.T) {
	sender := &fakeSender{}
	store := &fakeTemplateStore{
		templates: map[int64]TemplateData{
			5: {
				ID:       5,
				Subject:  "Hi {{.Name}}",
				BodyHTML: "<p>{{.Name}}</p>",
				BodyText: "Text {{.Name}}",
			},
		},
	}
	links := &fakeLinkMaker{}
	events := &fakeEventLogger{}

	p := &Pipeline{
		Sender:  sender,
		Tmpl:    store,
		Links:   links,
		Events:  events,
		BaseURL: "http://test.com",
	}

	in := SendInput{
		ContactID:  101,
		To:         "sam@test.com",
		TemplateID: 5,
		Vars:       map[string]any{"Name": "Sam"},
	}

	ctx := context.Background()
	err := p.Send(ctx, in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sender.calls != 1 {
		t.Fatalf("expected 1 send, got %d", sender.calls)
	}
	msg := sender.lastMsg
	if msg.Subject != "Hi Sam" {
		t.Errorf("expected subject 'Hi Sam', got %q", msg.Subject)
	}
	if !strings.Contains(msg.HTML, "<p>Sam</p>") {
		t.Errorf("expected HTML to contain 'Sam', got %q", msg.HTML)
	}
	if msg.Text != "Text Sam" {
		t.Errorf("expected Text 'Text Sam', got %q", msg.Text)
	}
}

func TestSendSenderErrorLogsFailed(t *testing.T) {
	sendErr := errors.New("smtp connection failed")
	sender := &fakeSender{err: sendErr}
	store := &fakeTemplateStore{}
	links := &fakeLinkMaker{}
	events := &fakeEventLogger{}

	p := &Pipeline{
		Sender:  sender,
		Tmpl:    store,
		Links:   links,
		Events:  events,
		BaseURL: "http://test.com",
	}

	in := SendInput{
		ContactID: 102,
		To:        "bob@test.com",
		Subject:   "Fail test",
		HTML:      "<p>fail</p>",
	}

	ctx := context.Background()
	err := p.Send(ctx, in)
	if !errors.Is(err, sendErr) && (err == nil || !strings.Contains(err.Error(), sendErr.Error())) {
		t.Fatalf("expected wrapped sender error, got %v", err)
	}

	// Verify failed event was logged
	if len(events.events) != 1 {
		t.Fatalf("expected 1 logged event, got %d", len(events.events))
	}
	evt := events.events[0]
	if evt.eventType != "failed" {
		t.Errorf("expected event type 'failed', got %q", evt.eventType)
	}
	errMsg, ok := evt.meta["error"].(string)
	if !ok || !strings.Contains(errMsg, sendErr.Error()) {
		t.Errorf("expected meta error to contain sender error, got %v", evt.meta)
	}
}

func TestSendMissingTo(t *testing.T) {
	sender := &fakeSender{}
	store := &fakeTemplateStore{}
	links := &fakeLinkMaker{}
	events := &fakeEventLogger{}

	p := &Pipeline{
		Sender:  sender,
		Tmpl:    store,
		Links:   links,
		Events:  events,
		BaseURL: "http://test.com",
	}

	in := SendInput{
		ContactID: 103,
		To:        "", // empty
		Subject:   "No recipient",
	}

	ctx := context.Background()
	err := p.Send(ctx, in)
	if err == nil {
		t.Error("expected error for missing To, got nil")
	}
	if sender.calls > 0 {
		t.Error("sender should not be called")
	}
}

func TestSendTemplateLoadError(t *testing.T) {
	sender := &fakeSender{}
	loadErr := errors.New("db disconnect")
	store := &fakeTemplateStore{err: loadErr}
	links := &fakeLinkMaker{}
	events := &fakeEventLogger{}

	p := &Pipeline{
		Sender:  sender,
		Tmpl:    store,
		Links:   links,
		Events:  events,
		BaseURL: "http://test.com",
	}

	in := SendInput{
		ContactID:  104,
		To:         "err@test.com",
		TemplateID: 999,
	}

	ctx := context.Background()
	err := p.Send(ctx, in)
	if !errors.Is(err, loadErr) && (err == nil || !strings.Contains(err.Error(), loadErr.Error())) {
		t.Fatalf("expected wrapped template load error, got %v", err)
	}
	if sender.calls > 0 {
		t.Error("sender should not be called on template load error")
	}
}

func TestSendInjectsPixelDeterministic(t *testing.T) {
	sender := &fakeSender{}
	store := &fakeTemplateStore{}
	links := &fakeLinkMaker{}
	events := &fakeEventLogger{}

	p := &Pipeline{
		Sender:  sender,
		Tmpl:    store,
		Links:   links,
		Events:  events,
		BaseURL: "http://test.com",
		OpenCode: func() string {
			return "openXYZ"
		},
	}

	in := SendInput{
		ContactID: 105,
		To:        "deterministic@test.com",
		Subject:   "Pixel code test",
		HTML:      "<body>Hello</body>",
	}

	ctx := context.Background()
	err := p.Send(ctx, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := sender.lastMsg
	expectedPixelURL := "http://test.com/o/openXYZ.png"
	if !strings.Contains(msg.HTML, expectedPixelURL) {
		t.Errorf("expected HTML to contain %q, got %q", expectedPixelURL, msg.HTML)
	}

	if len(events.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events.events))
	}
	evt := events.events[0]
	if evt.linkCode != "openXYZ" {
		t.Errorf("expected linkCode 'openXYZ', got %q", evt.linkCode)
	}
	if evt.meta["open_code"] != "openXYZ" {
		t.Errorf("expected open_code 'openXYZ' in meta, got %v", evt.meta)
	}
}

func TestSendTextOnlyNoLinkRewrite(t *testing.T) {
	sender := &fakeSender{}
	store := &fakeTemplateStore{}
	links := &fakeLinkMaker{}
	events := &fakeEventLogger{}

	p := &Pipeline{
		Sender:  sender,
		Tmpl:    store,
		Links:   links,
		Events:  events,
		BaseURL: "http://test.com",
	}

	in := SendInput{
		ContactID: 106,
		To:        "textonly@test.com",
		Subject:   "Text only test",
		Text:      "Just plain text and a link https://x.com",
	}

	ctx := context.Background()
	err := p.Send(ctx, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(links.calls) > 0 {
		t.Errorf("expected 0 CreateLink calls, got %d", len(links.calls))
	}

	msg := sender.lastMsg
	if msg.HTML != "" {
		t.Errorf("expected empty HTML, got %q", msg.HTML)
	}
	if msg.Text != "Just plain text and a link https://x.com" {
		t.Errorf("expected Text unmodified, got %q", msg.Text)
	}

	if len(events.events) != 1 {
		t.Fatalf("expected 1 logged event, got %d", len(events.events))
	}
	evt := events.events[0]
	if evt.eventType != "sent" {
		t.Errorf("expected event type 'sent', got %q", evt.eventType)
	}
	// Since no HTML, no pixel, does it generate/log an openCode?
	// The prompt states: "On success: log a 'sent' event via p.Events.LogEvent(ctx, in.ContactID, in.CampaignID, "sent", openCode, map[string]any{"open_code":openCode})."
	// Wait, does a text-only email have an openCode? Let's check:
	// "Then generate an open code via p.OpenCode(), create a tracking link record for the open pixel too? NO... SIMPLIFY the open-pixel handling: generate openCode := p.OpenCode(); call InjectPixel(html, baseURL, openCode); and log a 'sent' event whose Meta includes {"open_code": openCode}."
	// Wait, if it's text-only, we can still generate an openCode and put it in the log event, even if no pixel is injected into the HTML (since HTML is empty). That keeps the code flow extremely uniform and simple! Let's verify that the test handles it cleanly or expects it.
	openCode, ok := evt.meta["open_code"].(string)
	if !ok || openCode == "" {
		t.Errorf("expected openCode to be populated even for text-only email, got %v", evt.meta)
	}
	if evt.linkCode != openCode {
		t.Errorf("expected linkCode == openCode, got linkCode=%q, openCode=%q", evt.linkCode, openCode)
	}
}
