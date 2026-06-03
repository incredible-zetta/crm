package mcptools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
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
	parsed, err := url.Parse(in.TargetURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return mcpserver.Err("invalid_target_url", "must be an absolute http or https URL"), TrackingLinkCreateOut{}, nil
	}

	code, err := d.Repo.CreateLink(ctx, in.TargetURL, in.CampaignID, in.ContactID)
	if err != nil {
		return nil, TrackingLinkCreateOut{}, fmt.Errorf("tracking_link_create db: %w", err)
	}

	shortURL := strings.TrimSuffix(d.BaseURL, "/") + "/t/" + code

	return nil, TrackingLinkCreateOut{
		Code: code,
		URL:  shortURL,
	}, nil
}
