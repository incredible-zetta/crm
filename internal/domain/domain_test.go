package domain_test

import (
	"testing"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/domain"
)

func TestStageValid(t *testing.T) {
	for _, stage := range domain.Stages {
		if !stage.Valid() {
			t.Errorf("expected stage %q to be valid", stage)
		}
	}
	invalid := []domain.Stage{domain.Stage("vip"), domain.Stage("")}
	for _, stage := range invalid {
		if stage.Valid() {
			t.Errorf("expected stage %q to be invalid", stage)
		}
	}
}

func TestProviderValid(t *testing.T) {
	for _, provider := range domain.Providers {
		if !provider.Valid() {
			t.Errorf("expected provider %q to be valid", provider)
		}
	}
	invalid := []domain.Provider{domain.Provider("junk"), domain.Provider("")}
	for _, provider := range invalid {
		if provider.Valid() {
			t.Errorf("expected provider %q to be invalid", provider)
		}
	}
}

func TestCampaignStatusValid(t *testing.T) {
	for _, status := range domain.CampaignStatuses {
		if !status.Valid() {
			t.Errorf("expected campaign status %q to be valid", status)
		}
	}
	invalid := []domain.CampaignStatus{domain.CampaignStatus("junk"), domain.CampaignStatus("")}
	for _, status := range invalid {
		if status.Valid() {
			t.Errorf("expected campaign status %q to be invalid", status)
		}
	}
}

func TestTaskKindValid(t *testing.T) {
	for _, kind := range domain.TaskKinds {
		if !kind.Valid() {
			t.Errorf("expected task kind %q to be valid", kind)
		}
	}
	invalid := []domain.TaskKind{domain.TaskKind("junk"), domain.TaskKind("")}
	for _, kind := range invalid {
		if kind.Valid() {
			t.Errorf("expected task kind %q to be invalid", kind)
		}
	}
}

func TestTaskStatusValid(t *testing.T) {
	for _, status := range domain.TaskStatuses {
		if !status.Valid() {
			t.Errorf("expected task status %q to be valid", status)
		}
	}
	// Confirm TaskCancelled exists and is valid
	if !domain.TaskCancelled.Valid() {
		t.Errorf("expected domain.TaskCancelled to be valid")
	}
	invalid := []domain.TaskStatus{domain.TaskStatus("junk"), domain.TaskStatus("")}
	for _, status := range invalid {
		if status.Valid() {
			t.Errorf("expected task status %q to be invalid", status)
		}
	}
}

func TestEventTypeValid(t *testing.T) {
	for _, eventType := range domain.EventTypes {
		if !eventType.Valid() {
			t.Errorf("expected event type %q to be valid", eventType)
		}
	}
	// Confirm EventUnsubscribe exists and is valid
	if !domain.EventUnsubscribe.Valid() {
		t.Errorf("expected domain.EventUnsubscribe to be valid")
	}
	invalid := []domain.EventType{domain.EventType("junk"), domain.EventType("")}
	for _, eventType := range invalid {
		if eventType.Valid() {
			t.Errorf("expected event type %q to be invalid", eventType)
		}
	}
}

func TestContactFlags(t *testing.T) {
	c := domain.Contact{}
	if c.IsUnsubscribed() {
		t.Error("expected fresh Contact to not be unsubscribed")
	}
	if c.IsDeleted() {
		t.Error("expected fresh Contact to not be deleted")
	}

	now := time.Now()
	c.UnsubscribedAt = &now
	if !c.IsUnsubscribed() {
		t.Error("expected Contact with UnsubscribedAt set to be unsubscribed")
	}

	c.DeletedAt = &now
	if !c.IsDeleted() {
		t.Error("expected Contact with DeletedAt set to be deleted")
	}
}

func TestStagesOrder(t *testing.T) {
	expected := []domain.Stage{
		domain.StageNew,
		domain.StageContacted,
		domain.StageQualified,
		domain.StageProposal,
		domain.StageWon,
		domain.StageLost,
	}
	if len(domain.Stages) != len(expected) {
		t.Fatalf("expected %d stages, got %d", len(expected), len(domain.Stages))
	}
	for i, stage := range expected {
		if domain.Stages[i] != stage {
			t.Errorf("expected stage at %d to be %q, got %q", i, stage, domain.Stages[i])
		}
	}
}

func TestCampaignFlags(t *testing.T) {
	c := domain.Campaign{}
	if c.IsDeleted() {
		t.Error("expected fresh Campaign to not be deleted")
	}
	now := time.Now()
	c.DeletedAt = &now
	if !c.IsDeleted() {
		t.Error("expected Campaign with DeletedAt set to be deleted")
	}
}

func TestTemplateFlags(t *testing.T) {
	tmpl := domain.Template{}
	if tmpl.IsDeleted() {
		t.Error("expected fresh Template to not be deleted")
	}
	now := time.Now()
	tmpl.DeletedAt = &now
	if !tmpl.IsDeleted() {
		t.Error("expected Template with DeletedAt set to be deleted")
	}
}
