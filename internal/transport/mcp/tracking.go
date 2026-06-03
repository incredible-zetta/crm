package mcptransport

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type TrackingLinkCreateIn struct {
	TargetURL  string `json:"target_url" jsonschema:"The absolute target URL"`
	CampaignID *int64 `json:"campaign_id,omitempty" jsonschema:"Optional campaign ID"`
	ContactID  *int64 `json:"contact_id,omitempty" jsonschema:"Optional contact ID"`
}

type TrackingLinkCreateOut struct {
	Code string `json:"code"`
	URL  string `json:"url"`
}

func (d *Deps) TrackingLinkCreate(ctx context.Context, req *mcp.CallToolRequest, in TrackingLinkCreateIn) (*mcp.CallToolResult, TrackingLinkCreateOut, error) {
	code, urlVal, err := d.Svc.Tracking.CreateLink(ctx, in.TargetURL, in.CampaignID, in.ContactID)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			msg := err.Error()
			msg = strings.TrimPrefix(msg, "validation error: ")
			return mcpserver.Err("invalid_input", msg), TrackingLinkCreateOut{}, nil
		}
		return nil, TrackingLinkCreateOut{}, fmt.Errorf("tracking_link_create: %w", err)
	}

	return nil, TrackingLinkCreateOut{
		Code: code,
		URL:  urlVal,
	}, nil
}
