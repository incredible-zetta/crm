package mcptools

import (
	"context"
	"fmt"

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
	CampaignID int64            `json:"campaign_id"`
	Sent       int              `json:"sent"`
	Delivered  int              `json:"delivered"`
	Opened     int              `json:"opened"`
	Clicked    int              `json:"clicked"`
	Bounced    int              `json:"bounced"`
	OpenRate   float64          `json:"open_rate"`
	ClickRate  float64          `json:"click_rate"`
	TopLinks   []map[string]any `json:"top_links"`
}

func (d *Deps) AnalyticsOverview(ctx context.Context, req *mcp.CallToolRequest, in AnalyticsOverviewIn) (*mcp.CallToolResult, AnalyticsOverviewOut, error) {
	stages, err := d.Repo.CountByStage(ctx)
	if err != nil {
		return nil, AnalyticsOverviewOut{}, fmt.Errorf("analytics_overview stage count: %w", err)
	}

	totalContacts := 0
	for _, count := range stages {
		totalContacts += count
	}

	counts, err := d.Repo.OverviewCounts(ctx)
	if err != nil {
		return nil, AnalyticsOverviewOut{}, fmt.Errorf("analytics_overview counts: %w", err)
	}

	sent := counts["sent"]
	opens := counts["open"]
	clicks := counts["click"]

	var openRate, clickRate float64
	if sent > 0 {
		openRate = float64(opens) / float64(sent)
		clickRate = float64(clicks) / float64(sent)
	}

	var pendingTasks int
	err = d.Repo.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduled_tasks WHERE status='pending'").Scan(&pendingTasks)
	if err != nil {
		return nil, AnalyticsOverviewOut{}, fmt.Errorf("analytics_overview pending tasks: %w", err)
	}

	return nil, AnalyticsOverviewOut{
		ContactsByStage: stages,
		TotalContacts:   totalContacts,
		Sent:            sent,
		Opens:           opens,
		Clicks:          clicks,
		OpenRate:        openRate,
		ClickRate:       clickRate,
		PendingTasks:    pendingTasks,
	}, nil
}

func (d *Deps) CampaignStats(ctx context.Context, req *mcp.CallToolRequest, in CampaignStatsIn) (*mcp.CallToolResult, CampaignStatsOut, error) {
	counts, err := d.Repo.CampaignCounts(ctx, in.CampaignID)
	if err != nil {
		return nil, CampaignStatsOut{}, fmt.Errorf("campaign_stats counts: %w", err)
	}

	sent := counts["sent"]
	delivered := counts["delivered"]
	opened := counts["open"]
	clicked := counts["click"]
	bounced := counts["bounce"]

	var openRate, clickRate float64
	if sent > 0 {
		openRate = float64(opened) / float64(sent)
		clickRate = float64(clicked) / float64(sent)
	}

	topLinks, err := d.Repo.TopLinks(ctx, in.CampaignID, 10)
	if err != nil {
		return nil, CampaignStatsOut{}, fmt.Errorf("campaign_stats top links: %w", err)
	}

	var links []map[string]any
	for _, l := range topLinks {
		links = append(links, map[string]any{
			"link_code": l.LinkCode,
			"clicks":    l.Clicks,
		})
	}

	return nil, CampaignStatsOut{
		CampaignID: in.CampaignID,
		Sent:       sent,
		Delivered:  delivered,
		Opened:     opened,
		Clicked:    clicked,
		Bounced:    bounced,
		OpenRate:   openRate,
		ClickRate:  clickRate,
		TopLinks:   links,
	}, nil
}
