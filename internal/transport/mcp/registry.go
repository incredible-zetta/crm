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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "email_verify",
		Description: "Verify one contact's email (syntax + DNS/MX + heuristics) and persist the verdict",
	}, d.EmailVerify)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "email_audit",
		Description: "Batch-verify a segment of contacts and persist verdicts; paginate via next_cursor",
	}, d.EmailAudit)

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

	// Group 10: WhatsApp (whatsapp.go)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_check",
		Description: "Check if a phone number is registered on WhatsApp and persist the verdict on the contact",
	}, d.WhatsAppCheck)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_audit",
		Description: "Batch-check WhatsApp registration for a segment of contacts; paginate via next_cursor",
	}, d.WhatsAppAudit)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_send",
		Description: "Send a WhatsApp text message to a contact or phone. Body uses WhatsApp markdown (*bold*, _italic_, ~strike~, ```code```). See whatsapp://formatting resource.",
	}, d.WhatsAppSend)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_list",
		Description: "List WhatsApp messages (inbound + outbound) with filtering",
	}, d.WhatsAppList)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_get",
		Description: "Get a single WhatsApp message by ID",
	}, d.WhatsAppGet)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_reply",
		Description: "Reply to an inbound WhatsApp message, linking via replied_to",
	}, d.WhatsAppReply)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_mark_read",
		Description: "Mark an inbound WhatsApp message as read (locally + on gateway)",
	}, d.WhatsAppMarkRead)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_get_media",
		Description: "Download media URL for a WhatsApp message with attachment",
	}, d.WhatsAppGetMedia)

	// Group 11: Threads (threads.go)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_profile", Description: "Fetch Threads profile for configured user"}, d.ThreadsProfile)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_list", Description: "List live Threads posts and cache them locally"}, d.ThreadsList)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_publish", Description: "Publish a Threads text/image/video post and cache it locally"}, d.ThreadsPublish)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_delete", Description: "Delete a live Threads post by threads_id and soft-delete cached row"}, d.ThreadsDelete)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_insights", Description: "Fetch user-level or media-level Threads insights"}, d.ThreadsInsights)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_replies", Description: "List live replies for a Threads post and cache them locally"}, d.ThreadsReplies)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_reply", Description: "Reply to a Threads post"}, d.ThreadsReply)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_reply_quota", Description: "Fetch Threads reply publishing quota usage"}, d.ThreadsReplyQuota)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_mentions", Description: "List live Threads mentions and cache them locally"}, d.ThreadsMentions)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_search", Description: "Search Threads via keyword search endpoint; supports TOP/RECENT, KEYWORD/TAG, media type, author username, time range, and fields"}, d.ThreadsSearch)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_token_exchange", Description: "Exchange a short-lived Threads token for a long-lived token. Returns sensitive access_token."}, d.ThreadsTokenExchange)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_token_refresh", Description: "Refresh a long-lived Threads token before expiry. Returns sensitive access_token."}, d.ThreadsTokenRefresh)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_list_cached", Description: "List cached Threads posts from MySQL"}, d.ThreadsListCached)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_get_cached", Description: "Get one cached Threads post by local id or threads_id"}, d.ThreadsGetCached)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_history", Description: "List Threads channel audit events"}, d.ThreadsHistory)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_delete_cached", Description: "Soft-delete only the cached Threads post"}, d.ThreadsDeleteCached)

	// WhatsApp formatting resource
	srv.AddResource(&mcp.Resource{
		URI:         "whatsapp://formatting",
		Name:        "WhatsApp Markdown Formatting",
		Description: "Reference guide for WhatsApp text formatting syntax",
		MIMEType:    "text/plain",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		guide := `WhatsApp Text Formatting Guide
================================

WhatsApp uses a simplified markdown syntax. Do NOT use GitHub-flavored markdown.

FORMATTING:
  *bold*          → bold text (single asterisk, NOT double)
  _italic_        → italic text (underscore, NOT single asterisk)
  ~strikethrough~ → strikethrough (single tilde, NOT double)
  ` + "```code```" + `   → monospace code (triple backticks)

LISTS:
  - item          → bulleted list (dash or asterisk at line start)
  1. item         → numbered list

NOT SUPPORTED:
  - Headings (# Title) → renders literally, use *Title* (bold) instead
  - Links [label](url) → renders literally, write "label (url)" instead
  - Images ![alt](url) → not supported, use media send instead
  - Tables → not supported

CONVERSION HELPER:
  The system auto-converts common GitHub markdown to WhatsApp format:
    **bold**    → *bold*
    *italic*    → _italic_
    ~~strike~~  → ~strike~
    # Heading   → *Heading*
    [a](url)    → a (url)

BEST PRACTICES:
  - Keep messages concise (WhatsApp has character limits)
  - Use line breaks (\n\n) for paragraphs
  - Test formatting on a real WhatsApp client before bulk sends
`
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "whatsapp://formatting",
					MIMEType: "text/plain",
					Text:     guide,
				},
			},
		}, nil
	})

	// Threads publishing resource
	srv.AddResource(&mcp.Resource{
		URI:         "threads://publishing",
		Name:        "Threads Publishing Guide",
		Description: "Reference guide for Threads publish, replies, topic tags, and quotas",
		MIMEType:    "text/plain",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		guide := `Threads Publishing Guide
========================

TOOLS:
  threads_publish       Publish text/image/video post
  threads_reply         Reply to a Threads post
  threads_reply_quota   Check reply publishing quota before automation
  threads_replies       Read replies for a post
  threads_search        Keyword/topic search for post IDs
  threads_token_exchange Exchange short-lived token for long-lived token
  threads_token_refresh Refresh long-lived token before expiry

TOKEN LIFECYCLE:
  Graph-generated tokens may be valid for API calls but not refreshable.
  First exchange short-lived token:
    threads_token_exchange
  Then store returned access_token as THREADS_ACCESS_TOKEN.
  Refresh long-lived token before expiry:
    threads_token_refresh
  Refresh requires an unexpired long-lived token.

SEARCH:
  threads_search supports:
  - search_type: TOP (default) or RECENT
  - search_mode: KEYWORD (default) or TAG
  - media_type: TEXT, IMAGE, VIDEO
  - author_username: exact username without @
  - since/until time filters
  - fields override

  Default fields: id,text,media_type,permalink,timestamp,username,has_replies,is_quote_post,is_reply,topic_tag

  If app lacks approved threads_keyword_search permission, public search may only return posts owned by authenticated user.
  In that case use author_username for owned content discovery.

TOPIC TAGS:
  Use topic_tag on threads_publish to set the official Threads topic.
  Example: {"text":"Hello", "topic_tag":"AI Threads"}

RULES:
  - One topic tag per post.
  - Length: 1-50 characters.
  - Disallowed characters: . and &
  - Prefer topic_tag over embedding #topic in text.
  - Inline #topic fallback exists for backward compatibility, but is not preferred.

REPLY FLOW:
  Threads replies use Meta's two-step container flow internally:
  1. POST /{threads-user-id}/threads with reply_to_id
  2. POST /{threads-user-id}/threads_publish with creation_id

QUOTA:
  Call threads_reply_quota before automated replies.
  API returns reply_quota_usage and reply_config with quota_total and quota_duration.

NOTES:
  - Live API is source of truth; MySQL stores cache/audit with raw_json.
  - Published post updates are not supported by current API behavior.
  - Permalink alone may not resolve to media ID; prefer list/search/mentions/replies to get IDs.
`
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: "threads://publishing", MIMEType: "text/plain", Text: guide}}}, nil
	})
}
