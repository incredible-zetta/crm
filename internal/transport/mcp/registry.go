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
		Description: "List WhatsApp messages (inbound + outbound). Filters: direction, unread, known_only, contact_id, phone, chat_id (group/chat JID e.g. 120363...@g.us), since/until (RFC3339 or YYYY-MM-DD, on created_at), limit, cursor.",
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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_groups",
		Description: "List WhatsApp groups joined by the configured device (gateway limit: up to 500 groups)",
	}, d.WhatsAppGroups)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_contacts",
		Description: "List WhatsApp contacts known by the configured device",
	}, d.WhatsAppContacts)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whatsapp_send_media",
		Description: "Send WhatsApp image, video, or file by URL or local file path to a contact, phone, or group JID",
	}, d.WhatsAppSendMedia)

	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_listener_create", Description: "Enable AI listener for a WhatsApp chat/group JID"}, d.WhatsAppListenerCreate)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_listener_list", Description: "List configured WhatsApp AI listeners"}, d.WhatsAppListenerList)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_listener_update", Description: "Update WhatsApp AI listener settings"}, d.WhatsAppListenerUpdate)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_listener_delete", Description: "Disable/delete a WhatsApp AI listener"}, d.WhatsAppListenerDelete)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_listener_summary", Description: "Build a recent-message summary for one WhatsApp AI listener"}, d.WhatsAppListenerSummary)

	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_join", Description: "Join a WhatsApp group via invite link (https://chat.whatsapp.com/...)"}, d.WhatsAppGroupJoin)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_leave", Description: "Leave a WhatsApp group by group JID (120363...@g.us)"}, d.WhatsAppGroupLeave)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_info_from_link", Description: "Preview WhatsApp group info from an invite link without joining"}, d.WhatsAppGroupInfoFromLink)

	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_create", Description: "Create a WhatsApp group with a title and optional participant phone numbers"}, d.WhatsAppGroupCreate)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_info", Description: "Get detailed WhatsApp group info (name, topic, settings, participants) by group JID"}, d.WhatsAppGroupInfo)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_participants", Description: "List participants of a WhatsApp group by group JID"}, d.WhatsAppGroupParticipants)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_participants_manage", Description: "Add, remove, promote, or demote WhatsApp group participants. action: add|remove|promote|demote"}, d.WhatsAppGroupParticipantsManage)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_requests", Description: "List pending join requests for a WhatsApp group by group JID"}, d.WhatsAppGroupRequests)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_requests_review", Description: "Approve or reject pending WhatsApp group join requests. action: approve|reject"}, d.WhatsAppGroupRequestsReview)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_settings", Description: "Update WhatsApp group settings: name, topic, locked (admin-only info), announce (admin-only messages)"}, d.WhatsAppGroupSettings)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_group_invite_link", Description: "Get or reset the invite link for a WhatsApp group by group JID"}, d.WhatsAppGroupInviteLink)

	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_account_status", Description: "Get WhatsApp gateway connection and login status"}, d.WhatsAppAccountStatus)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_account_devices", Description: "List linked WhatsApp devices"}, d.WhatsAppAccountDevices)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_user_info", Description: "Look up a WhatsApp user's profile (verified name, status, devices) by phone"}, d.WhatsAppUserInfo)
	mcp.AddTool(srv, &mcp.Tool{Name: "whatsapp_set_push_name", Description: "Change the WhatsApp account display (push) name"}, d.WhatsAppSetPushName)

	// Group 11: Threads (threads.go)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_profile", Description: "Fetch Threads profile for configured user, including followers_count (best-effort from user insights; omitted on brand-new accounts or without insights scope)"}, d.ThreadsProfile)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_profile_lookup", Description: "Look up any PUBLIC Threads profile by username (profile discovery). Returns name, biography, picture, is_verified and public counters including follower_count, likes_count, quotes_count, replies_count, reposts_count, views_count. No 100-follower gate. Does NOT return a following count or follower/following lists (the API does not expose them). Requires threads_profile_discovery scope."}, d.ThreadsProfileLookup)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_list", Description: "List live Threads posts and cache them locally"}, d.ThreadsList)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_publish", Description: "Publish a Threads text/image/video post and cache it locally"}, d.ThreadsPublish)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_delete", Description: "Delete a live Threads post by threads_id and soft-delete cached row"}, d.ThreadsDelete)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_insights", Description: "Fetch user-level or media-level Threads insights"}, d.ThreadsInsights)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_daily_summary", Description: "Summarize a single day's Threads posts: lists posts for the day (default today, optional date + IANA timezone), enriches each with media-level insights (views/likes/reposts/quotes) and a reply breakdown (total replies, replies by me, replies by others, needs_reply), computes per-post and account-wide engagement (likes+reposts+quotes+others' replies) and engagement rate, plus follower count. Live API is source of truth; per-post failures are reported inline."}, d.ThreadsDailySummary)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_follower_demographics", Description: "Fetch aggregate follower demographics (country/city/age/gender) for the configured user. Requires the profile to have at least 100 followers. Returns aggregate buckets only — the Threads API does not expose a list of individual followers or any following data."}, d.ThreadsFollowerDemographics)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_replies", Description: "List live replies for a Threads post and cache them locally"}, d.ThreadsReplies)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_reply_tree", Description: "Show a post's full reply conversation as a nested tree. Each node has reply_id, username, is_mine, depth, needs_reply (someone else's comment you have not answered) and children. Use this before replying to find the exact comment reply_id to respond to and to avoid duplicate replies."}, d.ThreadsReplyTree)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_reply", Description: "Reply UNDER a target. To reply to a user's COMMENT, pass that comment's reply_id (from threads_reply_tree/threads_replies). Passing the root post id replies to your own post instead of the comment. One root post can hold replies to the post and replies to other comments; choose the target id deliberately."}, d.ThreadsReply)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_reply_hide", Description: "Hide (moderate) a reply/comment on YOUR Threads post by reply_id. Threads has no delete-comment API; hiding is how you remove an unwanted reply from your post. Cannot hide replies on other people's posts. Requires threads_manage_replies scope."}, d.ThreadsReplyHide)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_reply_unhide", Description: "Unhide a previously hidden reply/comment on your Threads post by reply_id. Requires threads_manage_replies scope."}, d.ThreadsReplyUnhide)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_reply_quota", Description: "Fetch Threads reply publishing quota usage"}, d.ThreadsReplyQuota)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_mentions", Description: "List live Threads mentions and cache them locally"}, d.ThreadsMentions)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_search", Description: "Search Threads via keyword search endpoint; supports TOP/RECENT, KEYWORD/TAG, media type, author username, time range, and fields"}, d.ThreadsSearch)
	mcp.AddTool(srv, &mcp.Tool{Name: "threads_discover", Description: "Cookie-only discovery of PUBLIC Threads posts by topic (separate from the Graph API). mode=posts returns structured JSON (pk, code, username, caption, like_count); mode=viral/latest return engagement-ranked or newest-first text. IDs returned are web IDs, NOT valid on the Graph API; only username bridges the two paths. Needs THREADS_DISCOVERY_BIN + THREADS_COOKIES_FILE."}, d.ThreadsDiscover)

	// Group: X (Twitter) cookie-only channel. Every tool takes a `cookies`
	// field (Netscape blob with auth_token + ct0) identifying the account to
	// act as, so multi-account is per-call. Ported natively from x-utils.
	mcp.AddTool(srv, &mcp.Tool{Name: "x_user", Description: "Fetch a public x.com profile by @handle using the acting account's cookies"}, d.XUser)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_post", Description: "Post a tweet (optionally reply, quote, or attach media by URL) as the acting account"}, d.XPost)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_delete", Description: "Delete a tweet owned by the acting account by tweet_id"}, d.XDelete)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_search", Description: "Search tweets (Top/Latest/People/Media); supports #hashtag $cashtag @mention and operators, paginated"}, d.XSearch)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_user_tweets", Description: "List a user's profile-timeline tweets by user_id or @handle, paginated"}, d.XUserTweets)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_tweet", Description: "Fetch a tweet's detail and engagement analytics (views, likes, retweets, replies, quotes, bookmarks) by tweet_id"}, d.XTweet)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_replies", Description: "List reply tweets in a tweet's conversation thread (author, text, engagement), paginated via next_cursor. Use x_tweet for the reply count only."}, d.XReplies)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_followers", Description: "List followers for a user by user_id or @handle, paginated"}, d.XFollowers)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_following", Description: "List accounts a user follows by user_id or @handle, paginated"}, d.XFollowing)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_dm", Description: "Send a direct message (optional image by URL) to a recipient_id or @handle as the acting account"}, d.XDM)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_account_save", Description: "Persist an x.com account (label + cookie blob) server-side and probe its liveness. Later x_* tools can pass account=<label> instead of raw cookies."}, d.XAccountSave)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_account_list", Description: "List stored x.com accounts for the tenant with their liveness (live/dead/unknown) and last check time. Cookies are never returned."}, d.XAccountList)
	mcp.AddTool(srv, &mcp.Tool{Name: "x_account_delete", Description: "Delete a stored x.com account by label"}, d.XAccountDelete)
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
  threads_reply         Reply UNDER a target id (post or comment)
  threads_reply_hide    Hide (moderate) a reply/comment on your post
  threads_reply_unhide  Unhide a previously hidden reply on your post
  threads_reply_tree    Nested reply conversation with is_mine + needs_reply flags
  threads_reply_quota   Check reply publishing quota before automation
  threads_replies       Read direct (one-level) replies for a post
  threads_search        Keyword/topic search for post IDs
  threads_token_exchange Exchange short-lived token for long-lived token
  threads_token_refresh Refresh long-lived token before expiry

REPLY TARGETING (IMPORTANT):
  threads_reply replies UNDER the id you pass.
  - To reply to the ROOT POST: pass the post id.
  - To reply to someone's COMMENT: pass that comment's reply_id, NOT the post id.
  Passing the post id when you meant a comment will post a top-level reply on
  your own post instead of answering the commenter.
  When replying to a comment, pass ONLY reply_id. If you pass both reply_id and
  threads_id, reply_id wins (the reply nests under the comment); do not rely on
  threads_id to target a comment.
  Recommended flow to answer comments on your post:
    1. threads_reply_tree { threads_id: <post_id> }
    2. Pick nodes where needs_reply=true and is_mine=false.
    3. threads_reply { reply_id: <that node's reply_id>, text: "..." }
  Tree fields: is_mine (authored by you), needs_reply (other user's comment you
  have not answered), depth, children (nested replies), already_replied,
  needs_reply_count.

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

DAILY SUMMARY:
  threads_daily_summary reports one local day's posts in a single call.
  Args: date (YYYY-MM-DD, default today), timezone (IANA, e.g. Asia/Jakarta,
  default server tz), max_posts (default 25, cap 100).
  Per post: media insights (views,likes,reposts,quotes,replies_metric) and a
  reply breakdown from the conversation tree (total_replies, my_replies,
  other_replies, needs_reply). engagement = likes+reposts+quotes+other_replies;
  engagement_rate = engagement/views. Account-wide totals + followers_count are
  included. Per-post failures surface inline (insights_error/replies_error)
  instead of failing the whole summary.
  Note: replies_metric (insights count) may differ from total_replies (distinct
  comments seen in the conversation tree); both are returned for comparison.

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
  1. POST /{threads-user-id}/threads with reply_to_id=<target id>
  2. POST /{threads-user-id}/threads_publish with creation_id
  reply_to_id is the target you pass to threads_reply (post id OR comment reply_id).

QUOTA:
  Call threads_reply_quota before automated replies.
  API returns reply_quota_usage and reply_config with quota_total and quota_duration.

NOTES:
  - Live API is source of truth; MySQL stores cache/audit with raw_json.
  - EDITING IS NOT SUPPORTED: the Threads API has no update/edit endpoint for
    posts OR replies. To change content you must delete and re-create.
  - DELETE: threads_delete removes your own POST (and soft-deletes the cache row).
    There is no API to delete someone else's comment on your post.
  - MODERATE COMMENTS: to remove an unwanted reply on YOUR post, use
    threads_reply_hide (and threads_reply_unhide to reverse). Hiding is the only
    way to take down a reply you do not own; it requires threads_manage_replies.
    You cannot hide replies on other people's posts.
  - FOLLOWERS/FOLLOWING: follower COUNT is available, follower/following LISTS
    are not. Your own account: threads_profile returns followers_count and
    threads_follower_demographics returns aggregate country/city/age/gender
    buckets (needs >=100 followers). ANY public user: threads_profile_lookup
    returns follower_count plus likes/quotes/replies/reposts/views counts with
    no follower gate. There is NO API for a list of followers, a following
    count, a following list, or accounts a user follows.
  - Published post updates are not supported by current API behavior.
  - Permalink alone may not resolve to media ID; prefer list/search/mentions/replies to get IDs.
`
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: "threads://publishing", MIMEType: "text/plain", Text: guide}}}, nil
	})
}
