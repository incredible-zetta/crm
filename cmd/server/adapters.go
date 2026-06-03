package main

import (
	"context"
	"errors"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
	"github.com/cipta/crm-for-aiagents/internal/scheduler"
	"github.com/cipta/crm-for-aiagents/internal/service"
)

// disabledSender is used when no email provider is configured. It satisfies
// port.EmailSender but fails on every send so the failure is explicit at call
// time rather than a nil-pointer panic.
type disabledSender struct{}

func (disabledSender) Send(ctx context.Context, m port.OutboundMessage) error {
	return errors.New("email sending is disabled: no SMTP or Mailgun provider configured")
}

// taskClaimer adapts a port.TaskRepo to the scheduler.TaskClaimer interface,
// translating domain.ScheduledTask into the worker's transport-agnostic Task.
type taskClaimer struct{ repo port.TaskRepo }

func (c taskClaimer) ClaimDue(ctx context.Context, now time.Time, limit int) ([]scheduler.Task, error) {
	dts, err := c.repo.ClaimDue(ctx, now, limit)
	if err != nil {
		return nil, err
	}
	out := make([]scheduler.Task, 0, len(dts))
	for _, d := range dts {
		out = append(out, scheduler.Task{
			ID:       d.ID,
			Kind:     string(d.Kind),
			Payload:  d.Payload,
			Attempts: d.Attempts,
		})
	}
	return out, nil
}

func (c taskClaimer) MarkDone(ctx context.Context, id int64) error {
	return c.repo.MarkDone(ctx, id)
}

func (c taskClaimer) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	return c.repo.MarkFailed(ctx, id, errMsg)
}

// taskExecutor adapts the TaskService to the scheduler.Executor interface,
// converting the worker's Task back into a domain.ScheduledTask.
type taskExecutor struct{ tasks *service.TaskService }

func (e taskExecutor) Execute(ctx context.Context, t scheduler.Task) error {
	return e.tasks.Execute(ctx, domain.ScheduledTask{
		ID:       t.ID,
		Kind:     domain.TaskKind(t.Kind),
		Payload:  t.Payload,
		Attempts: t.Attempts,
	})
}
