package service

import (
	"context"
	"fmt"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// TaskService manages background scheduled tasks and their execution.
type TaskService struct {
	repo     port.TaskRepo
	clock    port.Clock
	email    *EmailService
	campaign *CampaignService
}

// NewTaskService creates a new TaskService.
func NewTaskService(repo port.TaskRepo, clock port.Clock, email *EmailService, campaign *CampaignService) *TaskService {
	return &TaskService{
		repo:     repo,
		clock:    clock,
		email:    email,
		campaign: campaign,
	}
}

// Schedule validates and inserts a new scheduled background task.
func (s *TaskService) Schedule(ctx context.Context, kind string, payload map[string]any, runAt time.Time) (id int64, err error) {
	if !domain.TaskKind(kind).Valid() {
		return 0, fmt.Errorf("%w: invalid task kind", domain.ErrValidation)
	}
	if runAt.IsZero() {
		return 0, fmt.Errorf("%w: run_at required", domain.ErrValidation)
	}

	task := domain.ScheduledTask{
		Kind:    domain.TaskKind(kind),
		Payload: payload,
		RunAt:   runAt,
		Status:  domain.TaskPending,
	}
	return s.repo.Insert(ctx, task)
}

// List retrieves scheduled tasks filtered by status and clamped by limit.
func (s *TaskService) List(ctx context.Context, status string, limit int) ([]domain.ScheduledTask, error) {
	if status != "" {
		if !domain.TaskStatus(status).Valid() {
			return nil, fmt.Errorf("%w: invalid task status", domain.ErrValidation)
		}
	}

	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	return s.repo.List(ctx, status, limit)
}

// Cancel transitions a task status to cancelled if it was pending.
func (s *TaskService) Cancel(ctx context.Context, id int64) error {
	return s.repo.Cancel(ctx, id)
}

// Execute decodes a scheduled task payload and runs it.
func (s *TaskService) Execute(ctx context.Context, t domain.ScheduledTask) error {
	switch t.Kind {
	case domain.TaskEmail:
		in := SendInput{}
		if v, ok := getString(t.Payload, "to"); ok {
			in.To = v
		}
		if v, ok := getInt64(t.Payload, "contact_id"); ok {
			in.ContactID = v
		}
		if v, ok := getInt64(t.Payload, "template_id"); ok {
			in.TemplateID = v
		}
		if v, ok := getString(t.Payload, "subject"); ok {
			in.Subject = v
		}
		if v, ok := getString(t.Payload, "html"); ok {
			in.HTML = v
		}
		if v, ok := getString(t.Payload, "text"); ok {
			in.Text = v
		}
		if v, ok := getMap(t.Payload, "vars"); ok {
			in.Vars = v
		}
		if v, ok := getInt64(t.Payload, "campaign_id"); ok {
			cid := v
			in.CampaignID = &cid
		}

		_, _, err := s.email.Send(ctx, in)
		return err

	case domain.TaskCampaign:
		campaignID, ok := getInt64(t.Payload, "campaign_id")
		if !ok {
			return fmt.Errorf("%w: campaign_id required", domain.ErrValidation)
		}

		_, _, _, _, err := s.campaign.Send(ctx, campaignID)
		return err

	default:
		return fmt.Errorf("%w: unknown task kind %q", domain.ErrValidation, t.Kind)
	}
}

// Map value helper functions

func getString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func getInt64(m map[string]any, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return int64(val), true
	case int64:
		return val, true
	case int:
		return int64(val), true
	}
	return 0, false
}

func getMap(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	res, ok := v.(map[string]any)
	return res, ok
}
