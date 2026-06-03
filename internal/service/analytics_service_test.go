package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
	"github.com/cipta/crm-for-aiagents/internal/service"
)

type anFakeContactRepo struct {
	port.ContactRepo
	stages map[string]int
	err    error
}

func (r *anFakeContactRepo) CountByStage(ctx context.Context) (map[string]int, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.stages, nil
}

type anFakeEventRepo struct {
	port.EventRepo
	counts         map[string]int
	uniqueOpens    int
	countsErr      error
	uniqueOpensErr error
}

func (r *anFakeEventRepo) OverviewCounts(ctx context.Context) (map[string]int, error) {
	if r.countsErr != nil {
		return nil, r.countsErr
	}
	return r.counts, nil
}

func (r *anFakeEventRepo) UniqueOpens(ctx context.Context, campaignID *int64) (int, error) {
	if r.uniqueOpensErr != nil {
		return 0, r.uniqueOpensErr
	}
	return r.uniqueOpens, nil
}

type anFakeTaskRepo struct {
	port.TaskRepo
	tasks []domain.ScheduledTask
	err   error
}

func (r *anFakeTaskRepo) List(ctx context.Context, status string, limit int) ([]domain.ScheduledTask, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(r.tasks) > limit {
		return r.tasks[:limit], nil
	}
	return r.tasks, nil
}

func TestOverviewAggregates(t *testing.T) {
	ctx := context.Background()

	stages := map[string]int{
		"new": 2,
		"won": 1,
	}

	counts := map[string]int{
		"sent":        10,
		"delivered":   9,
		"open":        8,
		"click":       4,
		"bounce":      1,
		"failed":      2,
		"unsubscribe": 1,
	}

	tasks := []domain.ScheduledTask{
		{ID: 1, Status: domain.TaskPending},
		{ID: 2, Status: domain.TaskPending},
		{ID: 3, Status: domain.TaskPending},
	}

	contactRepo := &anFakeContactRepo{stages: stages}
	eventRepo := &anFakeEventRepo{counts: counts, uniqueOpens: 5}
	taskRepo := &anFakeTaskRepo{tasks: tasks}

	svc := service.NewAnalyticsService(contactRepo, eventRepo, taskRepo)

	overview, err := svc.Overview(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if overview.TotalContacts != 3 {
		t.Errorf("expected TotalContacts to be 3, got %d", overview.TotalContacts)
	}
	if overview.Sent != 10 {
		t.Errorf("expected Sent to be 10, got %d", overview.Sent)
	}
	if overview.Delivered != 9 {
		t.Errorf("expected Delivered to be 9, got %d", overview.Delivered)
	}
	if overview.Opens != 8 {
		t.Errorf("expected Opens to be 8, got %d", overview.Opens)
	}
	if overview.UniqueOpens != 5 {
		t.Errorf("expected UniqueOpens to be 5, got %d", overview.UniqueOpens)
	}
	if overview.Clicks != 4 {
		t.Errorf("expected Clicks to be 4, got %d", overview.Clicks)
	}
	if overview.Bounced != 1 {
		t.Errorf("expected Bounced to be 1, got %d", overview.Bounced)
	}
	if overview.Failed != 2 {
		t.Errorf("expected Failed to be 2, got %d", overview.Failed)
	}
	if overview.Unsubscribed != 1 {
		t.Errorf("expected Unsubscribed to be 1, got %d", overview.Unsubscribed)
	}
	if overview.OpenRate != 0.5 {
		t.Errorf("expected OpenRate to be 0.5, got %f", overview.OpenRate)
	}
	if overview.ClickRate != 0.4 {
		t.Errorf("expected ClickRate to be 0.4, got %f", overview.ClickRate)
	}
	if overview.PendingTasks != 3 {
		t.Errorf("expected PendingTasks to be 3, got %d", overview.PendingTasks)
	}
}

func TestOverviewZeroSentGuard(t *testing.T) {
	ctx := context.Background()

	stages := map[string]int{
		"new": 2,
	}

	counts := map[string]int{
		"sent": 0,
	}

	contactRepo := &anFakeContactRepo{stages: stages}
	eventRepo := &anFakeEventRepo{counts: counts, uniqueOpens: 0}
	taskRepo := &anFakeTaskRepo{}

	svc := service.NewAnalyticsService(contactRepo, eventRepo, taskRepo)

	overview, err := svc.Overview(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if overview.Sent != 0 {
		t.Errorf("expected Sent to be 0, got %d", overview.Sent)
	}
	if overview.OpenRate != 0.0 {
		t.Errorf("expected OpenRate to be 0.0, got %f", overview.OpenRate)
	}
	if overview.ClickRate != 0.0 {
		t.Errorf("expected ClickRate to be 0.0, got %f", overview.ClickRate)
	}
}

func TestOverviewErrors(t *testing.T) {
	ctx := context.Background()
	expectedErr := fmt.Errorf("some repository error")

	// 1. contact repo error
	{
		contactRepo := &anFakeContactRepo{err: expectedErr}
		eventRepo := &anFakeEventRepo{}
		taskRepo := &anFakeTaskRepo{}
		svc := service.NewAnalyticsService(contactRepo, eventRepo, taskRepo)
		_, err := svc.Overview(ctx)
		if err == nil || err.Error() != expectedErr.Error() {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	}

	// 2. event repo error
	{
		contactRepo := &anFakeContactRepo{}
		eventRepo := &anFakeEventRepo{countsErr: expectedErr}
		taskRepo := &anFakeTaskRepo{}
		svc := service.NewAnalyticsService(contactRepo, eventRepo, taskRepo)
		_, err := svc.Overview(ctx)
		if err == nil || err.Error() != expectedErr.Error() {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	}

	// 3. unique opens error
	{
		contactRepo := &anFakeContactRepo{}
		eventRepo := &anFakeEventRepo{counts: map[string]int{}, uniqueOpensErr: expectedErr}
		taskRepo := &anFakeTaskRepo{}
		svc := service.NewAnalyticsService(contactRepo, eventRepo, taskRepo)
		_, err := svc.Overview(ctx)
		if err == nil || err.Error() != expectedErr.Error() {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	}

	// 4. task repo error
	{
		contactRepo := &anFakeContactRepo{}
		eventRepo := &anFakeEventRepo{counts: map[string]int{}}
		taskRepo := &anFakeTaskRepo{err: expectedErr}
		svc := service.NewAnalyticsService(contactRepo, eventRepo, taskRepo)
		_, err := svc.Overview(ctx)
		if err == nil || err.Error() != expectedErr.Error() {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	}
}
