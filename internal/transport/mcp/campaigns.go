package mcptransport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CampaignCreateIn struct {
	Name        string         `json:"name" jsonschema:"Name of the campaign"`
	TemplateID  int64          `json:"template_id" jsonschema:"Template ID to use for the campaign"`
	Provider    string         `json:"provider,omitempty" jsonschema:"Email provider to use (smtp or mailgun)"`
	Segment     map[string]any `json:"segment,omitempty" jsonschema:"Filter segment for contacts (keys: stage, company, tag, q)"`
	ScheduledAt string         `json:"scheduled_at,omitempty" jsonschema:"Optional scheduled time in RFC3339 format"`
}

type CampaignCreateOut struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type CampaignSendIn struct {
	CampaignID int64 `json:"campaign_id" jsonschema:"ID of the campaign to send"`
	Sync       bool  `json:"sync,omitempty" jsonschema:"Send synchronously and wait for completion. Default false enqueues a background task and returns immediately."`
}

type CampaignSendOut struct {
	CampaignID int64  `json:"campaign_id"`
	Status     string `json:"status"`
	TaskID     int64  `json:"task_id,omitempty"`
	Recipients int    `json:"recipients,omitempty"`
	Sent       int    `json:"sent,omitempty"`
	Failed     int    `json:"failed,omitempty"`
	Skipped    int    `json:"skipped,omitempty"`
}

type CampaignListIn struct{}

type CampaignListOut struct {
	Count int              `json:"count"`
	Items []map[string]any `json:"items"`
}

type CampaignGetIn struct {
	ID int64 `json:"id" jsonschema:"ID of the campaign to fetch"`
}

type CampaignGetOut struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	TemplateID  int64          `json:"template_id"`
	Provider    string         `json:"provider"`
	Segment     map[string]any `json:"segment,omitempty"`
	ScheduledAt *time.Time     `json:"scheduled_at,omitempty"`
}

type CampaignUpdateIn struct {
	ID          int64          `json:"id" jsonschema:"ID of the campaign to update"`
	Name        *string        `json:"name,omitempty" jsonschema:"New name of the campaign"`
	TemplateID  *int64         `json:"template_id,omitempty" jsonschema:"New template ID"`
	Provider    *string        `json:"provider,omitempty" jsonschema:"New email provider (smtp or mailgun)"`
	Segment     map[string]any `json:"segment,omitempty" jsonschema:"New filter segment"`
	ScheduledAt *string        `json:"scheduled_at,omitempty" jsonschema:"New scheduled time in RFC3339 format"`
}

type CampaignUpdateOut struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type CampaignDeleteIn struct {
	ID int64 `json:"id" jsonschema:"ID of the campaign to delete"`
}

type CampaignDeleteOut struct {
	ID      int64 `json:"id"`
	Deleted bool  `json:"deleted"`
}

func (d *Deps) CampaignCreate(ctx context.Context, req *mcp.CallToolRequest, in CampaignCreateIn) (*mcp.CallToolResult, CampaignCreateOut, error) {
	var scheduledTime *time.Time
	status := domain.CampaignDraft
	if in.ScheduledAt != "" {
		t, err := time.Parse(time.RFC3339, in.ScheduledAt)
		if err != nil {
			return mcpserver.Err("invalid_input", "invalid scheduled_at format"), CampaignCreateOut{}, nil
		}
		scheduledTime = &t
		status = domain.CampaignScheduled
	}

	created, err := d.Svc.Campaign.Create(ctx, domain.Campaign{
		Name:        in.Name,
		TemplateID:  in.TemplateID,
		Provider:    domain.Provider(in.Provider),
		Segment:     in.Segment,
		Status:      status,
		ScheduledAt: scheduledTime,
	})
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			msg := err.Error()
			msg = strings.TrimPrefix(msg, "validation error: ")
			return mcpserver.Err("invalid_input", msg), CampaignCreateOut{}, nil
		}
		return nil, CampaignCreateOut{}, fmt.Errorf("campaign_create: %w", err)
	}

	return nil, CampaignCreateOut{
		ID:     created.ID,
		Name:   created.Name,
		Status: string(created.Status),
	}, nil
}

func (d *Deps) CampaignSend(ctx context.Context, req *mcp.CallToolRequest, in CampaignSendIn) (*mcp.CallToolResult, CampaignSendOut, error) {
	// Synchronous escape hatch: send inline and wait. Useful for small test
	// sends. Default path enqueues a background task to avoid blocking the
	// MCP call and risking request timeouts on large segments.
	if in.Sync {
		recipients, sent, failed, skipped, err := d.Svc.Campaign.Send(ctx, in.CampaignID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return mcpserver.Err("not_found", "campaign not found"), CampaignSendOut{}, nil
			}
			return nil, CampaignSendOut{}, fmt.Errorf("campaign_send: %w", err)
		}
		return nil, CampaignSendOut{
			CampaignID: in.CampaignID,
			Status:     "sent",
			Recipients: recipients,
			Sent:       sent,
			Failed:     failed,
			Skipped:    skipped,
		}, nil
	}

	// Mark the campaign as sending up front so it fails fast if the campaign
	// does not exist and so pollers see progress before the worker picks it up.
	if err := d.Svc.Campaign.MarkSending(ctx, in.CampaignID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "campaign not found"), CampaignSendOut{}, nil
		}
		return nil, CampaignSendOut{}, fmt.Errorf("campaign_send: %w", err)
	}

	taskID, err := d.Svc.Task.Schedule(ctx, string(domain.TaskCampaign), map[string]any{"campaign_id": in.CampaignID}, time.Now())
	if err != nil {
		return nil, CampaignSendOut{}, fmt.Errorf("campaign_send enqueue: %w", err)
	}

	return nil, CampaignSendOut{
		CampaignID: in.CampaignID,
		Status:     "queued",
		TaskID:     taskID,
	}, nil
}

func (d *Deps) CampaignList(ctx context.Context, req *mcp.CallToolRequest, in CampaignListIn) (*mcp.CallToolResult, CampaignListOut, error) {
	list, err := d.Svc.Campaign.List(ctx)
	if err != nil {
		return nil, CampaignListOut{}, fmt.Errorf("campaign_list: %w", err)
	}

	var items []map[string]any
	for _, c := range list {
		item := map[string]any{
			"id":          c.ID,
			"name":        c.Name,
			"status":      string(c.Status),
			"template_id": c.TemplateID,
			"provider":    string(c.Provider),
		}
		if c.ScheduledAt != nil {
			item["scheduled_at"] = c.ScheduledAt.Format(time.RFC3339)
		}
		items = append(items, item)
	}

	return nil, CampaignListOut{
		Count: len(items),
		Items: items,
	}, nil
}

func (d *Deps) CampaignGet(ctx context.Context, req *mcp.CallToolRequest, in CampaignGetIn) (*mcp.CallToolResult, CampaignGetOut, error) {
	c, err := d.Svc.Campaign.Get(ctx, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "campaign not found"), CampaignGetOut{}, nil
		}
		return nil, CampaignGetOut{}, fmt.Errorf("campaign_get: %w", err)
	}

	return nil, CampaignGetOut{
		ID:          c.ID,
		Name:        c.Name,
		Status:      string(c.Status),
		TemplateID:  c.TemplateID,
		Provider:    string(c.Provider),
		Segment:     c.Segment,
		ScheduledAt: c.ScheduledAt,
	}, nil
}

func (d *Deps) CampaignUpdate(ctx context.Context, req *mcp.CallToolRequest, in CampaignUpdateIn) (*mcp.CallToolResult, CampaignUpdateOut, error) {
	existing, err := d.Svc.Campaign.Get(ctx, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "campaign not found"), CampaignUpdateOut{}, nil
		}
		return nil, CampaignUpdateOut{}, fmt.Errorf("campaign_update get: %w", err)
	}

	campaign := existing
	if in.Name != nil {
		campaign.Name = *in.Name
	}
	if in.TemplateID != nil {
		campaign.TemplateID = *in.TemplateID
	}
	if in.Provider != nil {
		campaign.Provider = domain.Provider(*in.Provider)
	}
	if in.Segment != nil {
		campaign.Segment = in.Segment
	}
	if in.ScheduledAt != nil {
		if *in.ScheduledAt == "" {
			campaign.ScheduledAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *in.ScheduledAt)
			if err != nil {
				return mcpserver.Err("invalid_input", "invalid scheduled_at format"), CampaignUpdateOut{}, nil
			}
			campaign.ScheduledAt = &t
		}
	}

	updated, err := d.Svc.Campaign.Update(ctx, in.ID, campaign)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return mcpserver.Err("invalid_input", err.Error()), CampaignUpdateOut{}, nil
		}
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "campaign not found"), CampaignUpdateOut{}, nil
		}
		return nil, CampaignUpdateOut{}, fmt.Errorf("campaign_update: %w", err)
	}

	return nil, CampaignUpdateOut{
		ID:     updated.ID,
		Name:   updated.Name,
		Status: string(updated.Status),
	}, nil
}

func (d *Deps) CampaignDelete(ctx context.Context, req *mcp.CallToolRequest, in CampaignDeleteIn) (*mcp.CallToolResult, CampaignDeleteOut, error) {
	err := d.Svc.Campaign.Delete(ctx, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "campaign not found"), CampaignDeleteOut{}, nil
		}
		return nil, CampaignDeleteOut{}, fmt.Errorf("campaign_delete: %w", err)
	}

	return nil, CampaignDeleteOut{
		ID:      in.ID,
		Deleted: true,
	}, nil
}
