package service

import (
	"context"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
)

type AnalyticsService struct {
	contacts port.ContactRepo
	events   port.EventRepo
	tasks    port.TaskRepo
}

func NewAnalyticsService(contacts port.ContactRepo, events port.EventRepo, tasks port.TaskRepo) *AnalyticsService {
	return &AnalyticsService{
		contacts: contacts,
		events:   events,
		tasks:    tasks,
	}
}

type Overview struct {
	ContactsByStage map[string]int
	TotalContacts   int
	Sent            int
	Delivered       int
	Opens           int // raw open events
	UniqueOpens     int // DISTINCT contact opens
	Clicks          int
	Bounced         int
	Failed          int
	Unsubscribed    int
	OpenRate        float64 // uniqueOpens/sent guarded
	ClickRate       float64 // clicks/sent guarded
	PendingTasks    int
}

// Overview aggregates overview metrics across contacts, events, and tasks.
func (s *AnalyticsService) Overview(ctx context.Context) (Overview, error) {
	stages, err := s.contacts.CountByStage(ctx)
	if err != nil {
		return Overview{}, err
	}

	total := 0
	for _, count := range stages {
		total += count
	}

	counts, err := s.events.OverviewCounts(ctx)
	if err != nil {
		return Overview{}, err
	}

	uniqueOpens, err := s.events.UniqueOpens(ctx, nil)
	if err != nil {
		return Overview{}, err
	}

	// List pending tasks, capped at 200.
	pending, err := s.tasks.List(ctx, string(domain.TaskPending), 200)
	if err != nil {
		return Overview{}, err
	}

	sent := counts[string(domain.EventSent)]
	var openRate float64
	var clickRate float64
	if sent > 0 {
		openRate = float64(uniqueOpens) / float64(sent)
		clickRate = float64(counts[string(domain.EventClick)]) / float64(sent)
	}

	return Overview{
		ContactsByStage: stages,
		TotalContacts:   total,
		Sent:            sent,
		Delivered:       counts[string(domain.EventDelivered)],
		Opens:           counts[string(domain.EventOpen)],
		UniqueOpens:     uniqueOpens,
		Clicks:          counts[string(domain.EventClick)],
		Bounced:         counts[string(domain.EventBounce)],
		Failed:          counts[string(domain.EventFailed)],
		Unsubscribed:    counts[string(domain.EventUnsubscribe)],
		OpenRate:        openRate,
		ClickRate:       clickRate,
		PendingTasks:    len(pending),
	}, nil
}
