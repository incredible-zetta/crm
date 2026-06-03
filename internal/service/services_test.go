package service

import (
	"context"
	"testing"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
)

// aggFakeRepo is a no-op repo satisfying every repo port via embedding, used to
// confirm the Services wiring compiles and connects all services.
type aggFakeContacts struct{ port.ContactRepo }
type aggFakeCampaigns struct{ port.CampaignRepo }
type aggFakeTemplates struct{ port.TemplateRepo }
type aggFakeTasks struct{ port.TaskRepo }
type aggFakeEvents struct{ port.EventRepo }
type aggFakeTracking struct{ port.TrackingRepo }
type aggFakeExports struct{ port.ExportRepo }

type aggFakeSender struct{}

func (aggFakeSender) Send(ctx context.Context, m port.OutboundMessage) error { return nil }

type aggClock struct{}

func (aggClock) Now() time.Time { return time.Unix(0, 0) }

type aggIDGen struct{}

func (aggIDGen) ExportID() (string, error)  { return "exp0000000000000", nil }
func (aggIDGen) UnsubCode() (string, error) { return "uns0000000000000", nil }

func TestNewServicesWiresAll(t *testing.T) {
	repos := Repos{
		Contacts:  aggFakeContacts{},
		Campaigns: aggFakeCampaigns{},
		Templates: aggFakeTemplates{},
		Tasks:     aggFakeTasks{},
		Events:    aggFakeEvents{},
		Tracking:  aggFakeTracking{},
		Exports:   aggFakeExports{},
	}
	svc := New(repos, aggFakeSender{}, aggClock{}, aggIDGen{}, Config{BaseURL: "http://crm.local", ExportDir: "/tmp"})

	if svc.Contact == nil || svc.Template == nil || svc.Campaign == nil ||
		svc.Email == nil || svc.Task == nil || svc.Analytics == nil || svc.Tracking == nil {
		t.Fatal("New must wire all seven services non-nil")
	}

	// EmailService must satisfy CampaignMailer (compile-time + behavioral link).
	var _ CampaignMailer = svc.Email

	// Sanity: a validation path works through the wired service.
	if _, err := svc.Contact.Create(context.Background(), domain.Contact{}); err == nil {
		t.Error("expected validation error creating contact without email")
	}
}
