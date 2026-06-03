package mcptools

import (
	"context"

	"github.com/cipta/crm-for-aiagents/internal/db"
	"github.com/cipta/crm-for-aiagents/internal/email"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Deps struct {
	Repo      *db.Repo
	Pipeline  *email.Pipeline
	BaseURL   string
	ExportDir string // dir to write CSV export files
	Version   string // for health_check
	// health probes (injectable, return nil if healthy):
	PingDB      func(ctx context.Context) error
	PingSMTP    func(ctx context.Context) error // may be nil if not configured
	PingMailgun func(ctx context.Context) error // may be nil
}

func Register(srv *mcp.Server, d *Deps) {
	// Group 1: Contacts (contacts.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "contact_create",
		Description: "Create a new CRM contact",
	}, d.ContactCreate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "contact_update",
		Description: "Update an existing contact by ID or email",
	}, d.ContactUpdate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "contact_list",
		Description: "List CRM contacts with filtering, sorting and projection",
	}, d.ContactList)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "contact_import",
		Description: "Import multiple contacts via a list or a raw CSV string",
	}, d.ContactImport)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "contact_export",
		Description: "Query contacts and export them to a temporary CSV download URL",
	}, d.ContactExport)

	// Group 2: Templates (templates.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "template_create",
		Description: "Create a reusable email template with merge variables",
	}, d.TemplateCreate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "template_list",
		Description: "List all created email templates with their merge variables",
	}, d.TemplateList)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "template_render",
		Description: "Render an email template with merge variables without sending",
	}, d.TemplateRender)

	// Group 3: Email (email.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "email_send",
		Description: "Send a single email to a contact, using a template or raw fields",
	}, d.EmailSend)

	// Group 4: Campaigns (campaigns.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "campaign_create",
		Description: "Create a scheduled or immediate email campaign for a filtered contact segment",
	}, d.CampaignCreate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "campaign_send",
		Description: "Trigger immediate dispatch of an email campaign to all contacts matching its segment",
	}, d.CampaignSend)

	// Group 5: Scheduler (scheduler.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "schedule_task",
		Description: "Schedule a task for delayed/future execution",
	}, d.ScheduleTask)

	// Group 6: Tracking (tracking.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "tracking_link_create",
		Description: "Generate a click-tracked redirect link for a target URL",
	}, d.TrackingLinkCreate)

	// Group 7: Ops (ops.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "health_check",
		Description: "Perform self-test health check of database and email connections",
	}, d.HealthCheck)

	// Group 8: Analytics (analytics.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "analytics_overview",
		Description: "Retrieve high-level CRM and communication performance metrics",
	}, d.AnalyticsOverview)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "campaign_stats",
		Description: "Retrieve detailed delivery, open and click statistics for a specific campaign",
	}, d.CampaignStats)
}
