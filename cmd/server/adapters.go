package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/db"
	"github.com/cipta/crm-for-aiagents/internal/email"
	"github.com/cipta/crm-for-aiagents/internal/mcptools"
	"github.com/cipta/crm-for-aiagents/internal/scheduler"
)

type linkResolver struct {
	repo *db.Repo
}

func (l linkResolver) GetLink(ctx context.Context, code string) (string, *int64, *int64, error) {
	link, err := l.repo.GetLink(ctx, code)
	if err != nil {
		return "", nil, nil, err
	}
	return link.TargetURL, link.CampaignID, link.ContactID, nil
}

type eventRecorder struct {
	repo *db.Repo
}

func (e eventRecorder) LogEvent(ctx context.Context, contactID int64, campaignID *int64, eventType, linkCode string, meta map[string]any) error {
	return e.repo.InsertEvent(ctx, db.EmailEvent{
		ContactID:  contactID,
		CampaignID: campaignID,
		Type:       eventType,
		LinkCode:   linkCode,
		Meta:       meta,
	})
}

type exportResolver struct {
	repo *db.Repo
}

func (x exportResolver) GetExport(ctx context.Context, id string) (string, *time.Time, error) {
	exp, err := x.repo.GetExport(ctx, id)
	if err != nil {
		return "", nil, err
	}
	return exp.Path, exp.ExpiresAt, nil
}

type taskClaimer struct {
	repo *db.Repo
}

func (c taskClaimer) ClaimDue(ctx context.Context, now time.Time, limit int) ([]scheduler.Task, error) {
	dts, err := c.repo.ClaimDue(ctx, now, limit)
	if err != nil {
		return nil, err
	}
	out := make([]scheduler.Task, 0, len(dts))
	for _, d := range dts {
		out = append(out, scheduler.Task{
			ID:       d.ID,
			Kind:     d.Kind,
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

type taskExecutor struct {
	repo *db.Repo
	pipe *email.Pipeline
	deps *mcptools.Deps
}

func (t taskExecutor) Execute(ctx context.Context, task scheduler.Task) error {
	switch task.Kind {
	case "email":
		in := email.SendInput{}
		if v, ok := task.Payload["to"].(string); ok {
			in.To = v
		}
		if v, ok := task.Payload["contact_id"].(float64); ok {
			in.ContactID = int64(v)
		}
		if v, ok := task.Payload["template_id"].(float64); ok {
			in.TemplateID = int64(v)
		}
		if v, ok := task.Payload["subject"].(string); ok {
			in.Subject = v
		}
		if v, ok := task.Payload["html"].(string); ok {
			in.HTML = v
		}
		if v, ok := task.Payload["text"].(string); ok {
			in.Text = v
		}
		if v, ok := task.Payload["vars"].(map[string]any); ok {
			in.Vars = v
		}
		if v, ok := task.Payload["campaign_id"].(float64); ok {
			cid := int64(v)
			in.CampaignID = &cid
		}
		return t.pipe.Send(ctx, in)

	case "campaign":
		var campaignID int64
		if v, ok := task.Payload["campaign_id"].(float64); ok {
			campaignID = int64(v)
		} else {
			return fmt.Errorf("campaign task execution failed: missing or invalid campaign_id")
		}
		_, _, _, err := t.deps.RunCampaign(ctx, campaignID)
		return err

	default:
		return fmt.Errorf("unknown task kind %q", task.Kind)
	}
}
