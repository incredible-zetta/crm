package mcptransport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
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
}

type CampaignSendOut struct {
	CampaignID int64 `json:"campaign_id"`
	Recipients int   `json:"recipients"`
	Sent       int   `json:"sent"`
	Failed     int   `json:"failed"`
	Skipped    int   `json:"skipped,omitempty"`
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
	recipients, sent, failed, skipped, err := d.Svc.Campaign.Send(ctx, in.CampaignID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "campaign not found"), CampaignSendOut{}, nil
		}
		return nil, CampaignSendOut{}, fmt.Errorf("campaign_send: %w", err)
	}

	return nil, CampaignSendOut{
		CampaignID: in.CampaignID,
		Recipients: recipients,
		Sent:       sent,
		Failed:     failed,
		Skipped:    skipped,
	}, nil
}
