package scheduler

import (
	"context"
	"log"
	"time"
)

type TaskClaimer interface {
	ClaimDue(ctx context.Context, now time.Time, limit int) ([]Task, error)
	MarkDone(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
}

type Task struct {
	ID       int64
	Kind     string // "email" | "campaign"
	Payload  map[string]any
	Attempts int
}

type Executor interface {
	// Execute runs one task (email or campaign send). Returns error on failure.
	Execute(ctx context.Context, t Task) error
}

// CampaignEnqueuer schedules due campaigns before task claiming runs.
type CampaignEnqueuer interface {
	EnqueueDue(ctx context.Context, now time.Time) (int, error)
}

type Worker struct {
	Claimer  TaskClaimer
	Exec     Executor
	Campaign CampaignEnqueuer // optional repair for overdue scheduled campaigns
	Batch    int              // max tasks per tick (default 10)
	Now      func() time.Time // injectable clock; default time.Now
}

// RunOnce performs one tick of scheduling:
// 1. now := w.Now(); claim up to Batch due tasks via Claimer.ClaimDue(ctx, now, batch).
// 2. For each claimed task: call w.Exec.Execute(ctx, task).
//   - success -> Claimer.MarkDone(task.ID).
//   - failure -> Claimer.MarkFailed(task.ID, err.Error()). (MarkFailed increments attempts in the repo.)
//
// 3. Return count processed (attempted) and the FIRST claim error if ClaimDue itself failed.
//   - If a MarkDone/MarkFailed call errors, continue (best-effort) but we collect/return nothing for those.
//
// 4. Defaults: Batch<=0 -> 10, Now nil -> time.Now.
func (w *Worker) RunOnce(ctx context.Context) (processed int, err error) {
	batch := w.Batch
	if batch <= 0 {
		batch = 10
	}
	nowFn := w.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()

	if w.Campaign != nil {
		if _, err := w.Campaign.EnqueueDue(ctx, now); err != nil {
			log.Printf("scheduler worker: enqueue due campaigns error: %v", err)
		}
	}

	tasks, err := w.Claimer.ClaimDue(ctx, now, batch)
	if err != nil {
		return 0, err
	}

	for _, task := range tasks {
		// Even if ctx is done, we attempt to complete processing tasks already in progress
		// unless the context is deeply cancelled. Let's pass ctx down.
		execErr := w.Exec.Execute(ctx, task)
		if execErr == nil {
			if doneErr := w.Claimer.MarkDone(ctx, task.ID); doneErr != nil {
				log.Printf("scheduler worker: failed to mark task %d done: %v", task.ID, doneErr)
			}
		} else {
			log.Printf("scheduler worker: task %d execution failed: %v", task.ID, execErr)
			if failErr := w.Claimer.MarkFailed(ctx, task.ID, execErr.Error()); failErr != nil {
				log.Printf("scheduler worker: failed to mark task %d failed: %v", task.ID, failErr)
			}
		}
		processed++
	}

	return processed, nil
}

// Start runs an initial RunOnce immediately, then loops calling RunOnce on each interval tick until ctx.Done().
// If interval <= 0, it defaults to a sane value of 15 seconds to prevent ticker panics.
// Log errors from RunOnce but keep looping.
func (w *Worker) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial immediate execution
	if _, err := w.RunOnce(ctx); err != nil {
		log.Printf("scheduler worker: initial RunOnce error: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := w.RunOnce(ctx); err != nil {
				log.Printf("scheduler worker: RunOnce error: %v", err)
			}
		}
	}
}
