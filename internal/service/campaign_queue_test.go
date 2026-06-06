package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/service"
)

type queueTaskRepo struct {
	inserted []domain.ScheduledTask
	active   map[int64]bool
}

func (r *queueTaskRepo) Insert(ctx context.Context, t domain.ScheduledTask) (int64, error) {
	r.inserted = append(r.inserted, t)
	return int64(len(r.inserted)), nil
}

func (r *queueTaskRepo) List(ctx context.Context, status string, limit int) ([]domain.ScheduledTask, error) {
	return nil, nil
}
func (r *queueTaskRepo) ClaimDue(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledTask, error) {
	return nil, nil
}
func (r *queueTaskRepo) MarkDone(ctx context.Context, id int64) error { return nil }
func (r *queueTaskRepo) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	return nil
}
func (r *queueTaskRepo) Cancel(ctx context.Context, id int64) error { return nil }

func (r *queueTaskRepo) HasActiveCampaignTask(ctx context.Context, campaignID int64) (bool, error) {
	return r.active[campaignID], nil
}

type queueCampaignRepo struct {
	due []domain.Campaign
}

func (r *queueCampaignRepo) Create(ctx context.Context, c domain.Campaign) (domain.Campaign, error) {
	return c, nil
}
func (r *queueCampaignRepo) Get(ctx context.Context, id int64) (domain.Campaign, error) {
	return domain.Campaign{}, domain.ErrNotFound
}
func (r *queueCampaignRepo) List(ctx context.Context) ([]domain.Campaign, error) { return nil, nil }
func (r *queueCampaignRepo) UpdateStatus(ctx context.Context, id int64, status domain.CampaignStatus) error {
	return nil
}
func (r *queueCampaignRepo) Update(ctx context.Context, id int64, c domain.Campaign) (domain.Campaign, error) {
	return c, nil
}
func (r *queueCampaignRepo) SetStats(ctx context.Context, id int64, stats map[string]any) error {
	return nil
}
func (r *queueCampaignRepo) SoftDelete(ctx context.Context, id int64) error { return nil }

func (r *queueCampaignRepo) ListDueScheduled(ctx context.Context, now time.Time, limit int) ([]domain.Campaign, error) {
	return r.due, nil
}

func TestCampaignQueueEnqueueDueSkipsActive(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	campaigns := &queueCampaignRepo{due: []domain.Campaign{{ID: 3, Status: domain.CampaignScheduled, ScheduledAt: &past}}}
	tasks := &queueTaskRepo{active: map[int64]bool{3: true}}
	q := service.NewCampaignQueue(campaigns, tasks)

	n, err := q.EnqueueDue(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 enqueued, got %d", n)
	}
	if len(tasks.inserted) != 0 {
		t.Fatalf("expected no tasks inserted, got %d", len(tasks.inserted))
	}
}

func TestCampaignQueueEnqueueDueCreatesTask(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	campaigns := &queueCampaignRepo{due: []domain.Campaign{{ID: 3, Status: domain.CampaignScheduled, ScheduledAt: &past}}}
	tasks := &queueTaskRepo{active: map[int64]bool{}}
	q := service.NewCampaignQueue(campaigns, tasks)

	n, err := q.EnqueueDue(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 enqueued, got %d", n)
	}
	if len(tasks.inserted) != 1 {
		t.Fatalf("expected 1 task inserted, got %d", len(tasks.inserted))
	}
	if tasks.inserted[0].Kind != domain.TaskCampaign {
		t.Fatalf("expected campaign task, got %s", tasks.inserted[0].Kind)
	}
	if tasks.inserted[0].Payload["campaign_id"] != int64(3) {
		t.Fatalf("unexpected payload: %v", tasks.inserted[0].Payload)
	}
}
