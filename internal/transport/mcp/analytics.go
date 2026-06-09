package mcptransport

import (
	"context"
	"errors"
	"fmt"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AnalyticsOverviewIn struct{}

type AnalyticsOverviewOut struct {
	ContactsByStage map[string]int `json:"contacts_by_stage"`
	TotalContacts   int            `json:"total_contacts"`
	Sent            int            `json:"sent"`
	Opens           int            `json:"opens"`
	Clicks          int            `json:"clicks"`
	OpenRate        float64        `json:"open_rate"`
	ClickRate       float64        `json:"click_rate"`
	PendingTasks    int            `json:"pending_tasks"`
}

type CampaignStatsIn struct {
	CampaignID int64 `json:"campaign_id" jsonschema:"ID of the campaign"`
}

type CampaignStatsOut struct {
	CampaignID      int64             `json:"campaign_id"`
	Sent            int               `json:"sent"`
	Delivered       int               `json:"delivered"`
	Opened          int               `json:"opened"`
	Clicked         int               `json:"clicked"`
	Bounced         int               `json:"bounced"`
	OpenRate        float64           `json:"open_rate"`
	ClickRate       float64           `json:"click_rate"`
	TopLinks        []map[string]any  `json:"top_links"`
	TrackingSupport map[string]string `json:"tracking_support"`
}

func (d *Deps) AnalyticsOverview(ctx context.Context, req *mcp.CallToolRequest, in AnalyticsOverviewIn) (*mcp.CallToolResult, AnalyticsOverviewOut, error) {
	overview, err := d.Svc.Analytics.Overview(ctx)
	if err != nil {
		return nil, AnalyticsOverviewOut{}, fmt.Errorf("analytics_overview: %w", err)
	}

	return nil, AnalyticsOverviewOut{
		ContactsByStage: overview.ContactsByStage,
		TotalContacts:   overview.TotalContacts,
		Sent:            overview.Sent,
		Opens:           overview.Opens,
		Clicks:          overview.Clicks,
		OpenRate:        overview.OpenRate,
		ClickRate:       overview.ClickRate,
		PendingTasks:    overview.PendingTasks,
	}, nil
}

func (d *Deps) CampaignStats(ctx context.Context, req *mcp.CallToolRequest, in CampaignStatsIn) (*mcp.CallToolResult, CampaignStatsOut, error) {
	stats, err := d.Svc.Campaign.Stats(ctx, in.CampaignID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "campaign not found"), CampaignStatsOut{}, nil
		}
		return nil, CampaignStatsOut{}, fmt.Errorf("campaign_stats: %w", err)
	}

	links := make([]map[string]any, 0, len(stats.TopLinks))
	for _, l := range stats.TopLinks {
		links = append(links, map[string]any{
			"link_code": l.LinkCode,
			"clicks":    l.Clicks,
		})
	}

	return nil, CampaignStatsOut{
		CampaignID:      stats.CampaignID,
		Sent:            stats.Sent,
		Delivered:       stats.Delivered,
		Opened:          stats.Opened,
		Clicked:         stats.Clicked,
		Bounced:         stats.Bounced,
		OpenRate:        stats.OpenRate,
		ClickRate:       stats.ClickRate,
		TopLinks:        links,
		TrackingSupport: stats.TrackingSupport,
	}, nil
}
