package service

import (
	"context"
	"fmt"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
)

// CampaignQueue enqueues due scheduled campaigns that do not already have an
// active send task. It repairs overdue scheduled campaigns that were created
// before automatic task scheduling existed.
type CampaignQueue struct {
	campaigns port.CampaignRepo
	tasks     port.TaskRepo
}

// NewCampaignQueue creates a CampaignQueue.
func NewCampaignQueue(campaigns port.CampaignRepo, tasks port.TaskRepo) *CampaignQueue {
	return &CampaignQueue{campaigns: campaigns, tasks: tasks}
}

// EnqueueDue finds scheduled campaigns past their scheduled_at and enqueues
// a campaign send task for each that lacks a pending/running task.
func (q *CampaignQueue) EnqueueDue(ctx context.Context, now time.Time) (int, error) {
	due, err := q.campaigns.ListDueScheduled(ctx, now, 50)
	if err != nil {
		return 0, fmt.Errorf("list due scheduled campaigns: %w", err)
	}

	var enqueued int
	for _, c := range due {
		// ListDueScheduled runs cross-tenant (the scheduler has no tenant in
		// ctx); scope each campaign's task to its owning tenant so the worker
		// later resolves the right data.
		cctx := tenant.With(ctx, c.TenantID)
		active, err := q.tasks.HasActiveCampaignTask(cctx, c.ID)
		if err != nil {
			return enqueued, fmt.Errorf("check active task for campaign %d: %w", c.ID, err)
		}
		if active {
			continue
		}

		runAt := now
		if c.ScheduledAt != nil && c.ScheduledAt.After(runAt) {
			runAt = *c.ScheduledAt
		}

		_, err = q.tasks.Insert(cctx, domain.ScheduledTask{
			Kind:    domain.TaskCampaign,
			Payload: map[string]any{"campaign_id": c.ID},
			RunAt:   runAt,
			Status:  domain.TaskPending,
		})
		if err != nil {
			return enqueued, fmt.Errorf("enqueue campaign %d: %w", c.ID, err)
		}
		enqueued++
	}
	return enqueued, nil
}
