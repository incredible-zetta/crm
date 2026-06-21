package whatsapp

import (
	"context"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

type fakeGateway struct {
	sends int
}

func (f *fakeGateway) Check(ctx context.Context, phone string) (domain.WhatsAppCheck, error) {
	return domain.WhatsAppCheck{Phone: phone, Status: domain.WhatsAppRegistered}, nil
}
func (f *fakeGateway) Send(ctx context.Context, msg port.WhatsAppMessage) (port.WhatsAppSendResult, error) {
	f.sends++
	return port.WhatsAppSendResult{MessageID: "X", Status: "sent"}, nil
}
func (f *fakeGateway) MarkRead(ctx context.Context, id, phone string) error { return nil }
func (f *fakeGateway) DownloadMedia(ctx context.Context, id, phone string) (port.WhatsAppMedia, error) {
	return port.WhatsAppMedia{}, nil
}
func (f *fakeGateway) ListGroups(ctx context.Context) ([]port.WhatsAppGroup, error) { return nil, nil }
func (f *fakeGateway) ListContacts(ctx context.Context) ([]port.WhatsAppContact, error) {
	return nil, nil
}
func (f *fakeGateway) JoinGroup(ctx context.Context, link string) (string, error) { return "", nil }
func (f *fakeGateway) LeaveGroup(ctx context.Context, jid string) error           { return nil }
func (f *fakeGateway) GroupInfoFromLink(ctx context.Context, link string) (port.WhatsAppGroup, error) {
	return port.WhatsAppGroup{}, nil
}
func (f *fakeGateway) SendMedia(ctx context.Context, msg port.WhatsAppMediaMessage) (port.WhatsAppSendResult, error) {
	return port.WhatsAppSendResult{}, nil
}

type fakeCounter struct {
	perRecipient int
	all          int
}

func (c fakeCounter) CountSentSince(ctx context.Context, phone string, since time.Time) (int, error) {
	return c.perRecipient, nil
}
func (c fakeCounter) CountSentSinceAll(ctx context.Context, since time.Time) (int, error) {
	return c.all, nil
}

func TestSmartSendDailyCap(t *testing.T) {
	gw := &fakeGateway{}
	s := NewSmartSender(gw, SmartSendPolicy{DailyCapPerRecipient: 3}, fakeCounter{perRecipient: 3})
	_, err := s.Send(context.Background(), port.WhatsAppMessage{Phone: "628111", Body: "hi"})
	if err == nil {
		t.Fatal("expected daily cap error")
	}
	if gw.sends != 0 {
		t.Errorf("gateway send should not run when capped")
	}
}

func TestSmartSendWarmup(t *testing.T) {
	gw := &fakeGateway{}
	s := NewSmartSender(gw, SmartSendPolicy{WarmupPerDay: 10}, fakeCounter{all: 10})
	_, err := s.Send(context.Background(), port.WhatsAppMessage{Phone: "628111", Body: "hi"})
	if err == nil {
		t.Fatal("expected warmup error")
	}
}

func TestSmartSendPaces(t *testing.T) {
	gw := &fakeGateway{}
	s := NewSmartSender(gw, SmartSendPolicy{RateMax: 2, RateWindow: time.Second}, nil)

	now := time.Now()
	s.clock = func() time.Time { return now }
	var slept time.Duration
	s.sleep = func(ctx context.Context, d time.Duration) error {
		slept += d
		now = now.Add(d) // advance virtual clock so the bucket refills
		return nil
	}
	s.jitFn = func(min, max time.Duration) time.Duration { return 0 }

	for i := 0; i < 4; i++ {
		if _, err := s.Send(context.Background(), port.WhatsAppMessage{Phone: "628111", Body: "hi"}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if gw.sends != 4 {
		t.Errorf("sends = %d, want 4", gw.sends)
	}
	// First 2 burst free; remaining 2 each wait ~0.5s at 2/sec.
	if slept <= 0 {
		t.Errorf("expected throttle to sleep, slept=%v", slept)
	}
}

func TestSmartSendPassthroughMethods(t *testing.T) {
	gw := &fakeGateway{}
	s := NewSmartSender(gw, SmartSendPolicy{}, nil)
	if _, err := s.Check(context.Background(), "628111"); err != nil {
		t.Fatalf("check passthrough: %v", err)
	}
}
