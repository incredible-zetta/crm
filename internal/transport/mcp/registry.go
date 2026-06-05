package mcptransport

import (
	"context"

	"github.com/incredible-zetta/crm/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Deps struct {
	Svc         *service.Services
	Version     string
	PingDB      func(ctx context.Context) error // for health_check; may be nil
	PingSMTP    func(ctx context.Context) error // optional
	PingMailgun func(ctx context.Context) error // optional
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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "contact_get",
		Description: "Fetch a single contact by id or email",
	}, d.ContactGet)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "contact_delete",
		Description: "Delete a contact (soft-delete or purge) by ID",
	}, d.ContactDelete)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "contact_unsubscribe",
		Description: "Unsubscribe a contact by ID",
	}, d.ContactUnsubscribe)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "contact_bulk_update",
		Description: "Apply a partial update to many contacts by ID list (max 500). Tags are additive via add_tags/remove_tags or replaced via set_tags.",
	}, d.ContactBulkUpdate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "contact_bulk_update_by_filter",
		Description: "Apply a partial update to every contact matching a segment filter (stage, company, tag, q).",
	}, d.ContactBulkUpdateByFilter)

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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "template_get",
		Description: "Fetch a single email template by ID or name",
	}, d.TemplateGet)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "template_update",
		Description: "Update fields on an existing email template by ID",
	}, d.TemplateUpdate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "template_delete",
		Description: "Soft-delete an email template by ID",
	}, d.TemplateDelete)

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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "campaign_list",
		Description: "List all created email campaigns",
	}, d.CampaignList)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "campaign_get",
		Description: "Fetch a single email campaign by ID",
	}, d.CampaignGet)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "campaign_update",
		Description: "Update fields on an existing email campaign by ID",
	}, d.CampaignUpdate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "campaign_delete",
		Description: "Soft-delete an email campaign by ID",
	}, d.CampaignDelete)

	// Group 5: Scheduler (scheduler.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "schedule_task",
		Description: "Schedule a task for delayed/future execution",
	}, d.ScheduleTask)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "task_list",
		Description: "List background scheduled tasks filtered by status",
	}, d.TaskList)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "task_cancel",
		Description: "Cancel a pending background scheduled task by ID",
	}, d.TaskCancel)

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

	// Group 9: Inbox (inbox.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "inbox_sync",
		Description: "Fetch new inbound replies from the configured IMAP inbox",
	}, d.InboxSync)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "inbox_list",
		Description: "List stored inbound messages with snippets",
	}, d.InboxList)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "inbox_get",
		Description: "Read a full stored inbound message",
	}, d.InboxGet)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "inbox_mark_read",
		Description: "Mark a stored inbound message read or unread",
	}, d.InboxMarkRead)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "inbox_reply",
		Description: "Reply to an inbound message using the configured sender identity",
	}, d.InboxReply)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "inbox_delete",
		Description: "Soft-delete a stored inbound message locally",
	}, d.InboxDelete)
}
