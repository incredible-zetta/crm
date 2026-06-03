package mcptools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/db"
	"github.com/cipta/crm-for-aiagents/internal/email"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CampaignCreateIn struct {
	Name        string         `json:"name" jsonschema:"Name of the campaign"`
	TemplateID  int64          `json:"template_id" jsonschema:"Template ID to use for the campaign"`
	Provider    string         `json:"provider" jsonschema:"Email provider to use (smtp or mailgun)"`
	Segment     map[string]any `json:"segment" jsonschema:"Filter segment for contacts (keys: stage, company, tag, q)"`
	ScheduledAt string         `json:"scheduled_at" jsonschema:"Optional scheduled time in RFC3339 format"`
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
}

func (d *Deps) CampaignCreate(ctx context.Context, req *mcp.CallToolRequest, in CampaignCreateIn) (*mcp.CallToolResult, CampaignCreateOut, error) {
	var scheduledTime *time.Time
	status := "draft"
	if in.ScheduledAt != "" {
		t, err := time.Parse(time.RFC3339, in.ScheduledAt)
		if err != nil {
			return mcpserver.Err("invalid_scheduled_at", "invalid RFC3339 format"), CampaignCreateOut{}, nil
		}
		scheduledTime = &t
		status = "scheduled"
	}

	camp := db.Campaign{
		Name:        in.Name,
		TemplateID:  in.TemplateID,
		Provider:    in.Provider,
		Segment:     in.Segment,
		Status:      status,
		ScheduledAt: scheduledTime,
	}

	created, err := d.Repo.CreateCampaign(ctx, camp)
	if err != nil {
		return nil, CampaignCreateOut{}, fmt.Errorf("campaign_create db: %w", err)
	}

	return nil, CampaignCreateOut{
		ID:     created.ID,
		Name:   created.Name,
		Status: created.Status,
	}, nil
}

func (d *Deps) RunCampaign(ctx context.Context, campaignID int64) (recipients, sent, failed int, err error) {
	campaign, err := d.Repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return 0, 0, 0, err
	}

	var filter db.ContactFilter
	if campaign.Segment != nil {
		if val, ok := campaign.Segment["stage"].(string); ok {
			filter.Stage = val
		}
		if val, ok := campaign.Segment["company"].(string); ok {
			filter.Company = val
		}
		if val, ok := campaign.Segment["tag"].(string); ok {
			filter.Tag = val
		}
		if val, ok := campaign.Segment["q"].(string); ok {
			filter.Q = val
		}
	}

	var cursor int64
	for {
		items, _, nextCursor, err := d.Repo.ListContacts(ctx, filter, 100, cursor)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("campaign_send list contacts: %w", err)
		}
		if len(items) == 0 {
			break
		}
		for _, contact := range items {
			recipients++
			vars := map[string]any{
				"email":      contact.Email,
				"first_name": contact.FirstName,
				"last_name":  contact.LastName,
				"company":    contact.Company,
			}

			sendInput := email.SendInput{
				ContactID:  contact.ID,
				CampaignID: &campaign.ID,
				To:         contact.Email,
				TemplateID: campaign.TemplateID,
				Vars:       vars,
			}
			if err := d.Pipeline.Send(ctx, sendInput); err != nil {
				failed++
			} else {
				sent++
			}
		}
		if nextCursor == 0 {
			break
		}
		cursor = nextCursor
	}

	if err := d.Repo.UpdateCampaignStatus(ctx, campaign.ID, "sent"); err != nil {
		return recipients, sent, failed, fmt.Errorf("campaign_send update status: %w", err)
	}

	return recipients, sent, failed, nil
}

func (d *Deps) CampaignSend(ctx context.Context, req *mcp.CallToolRequest, in CampaignSendIn) (*mcp.CallToolResult, CampaignSendOut, error) {
	recipients, sent, failed, err := d.RunCampaign(ctx, in.CampaignID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return mcpserver.Err("not_found", "campaign not found"), CampaignSendOut{}, nil
		}
		return nil, CampaignSendOut{}, fmt.Errorf("campaign_send: %w", err)
	}

	return nil, CampaignSendOut{
		CampaignID: in.CampaignID,
		Recipients: recipients,
		Sent:       sent,
		Failed:     failed,
	}, nil
}
