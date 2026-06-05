package mcptransport

import (
	"context"
	"errors"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type InboxSyncIn struct {
	Limit int `json:"limit,omitempty"`
}
type InboxSyncOut struct {
	Fetched       int `json:"fetched"`
	New           int `json:"new"`
	KnownContacts int `json:"known_contacts"`
	Notified      int `json:"notified"`
}

type InboxListIn struct {
	Unread    bool  `json:"unread,omitempty"`
	KnownOnly bool  `json:"known_only,omitempty"`
	ContactID int64 `json:"contact_id,omitempty"`
	Limit     int   `json:"limit,omitempty"`
	Cursor    int64 `json:"cursor,omitempty"`
}

type InboxItemOut struct {
	ID         int64  `json:"id"`
	From       string `json:"from"`
	FromName   string `json:"from_name,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Snippet    string `json:"snippet,omitempty"`
	ReceivedAt string `json:"received_at"`
	ContactID  *int64 `json:"contact_id,omitempty"`
	Read       bool   `json:"read"`
	Replied    bool   `json:"replied"`
}

type InboxListOut struct {
	Items      []InboxItemOut `json:"items"`
	NextCursor int64          `json:"next_cursor"`
}

type InboxGetIn struct {
	ID int64 `json:"id"`
}
type InboxGetOut struct {
	ID               int64  `json:"id"`
	Mailbox          string `json:"mailbox"`
	UID              uint32 `json:"uid"`
	MessageID        string `json:"message_id,omitempty"`
	InReplyTo        string `json:"in_reply_to,omitempty"`
	ReferencesHeader string `json:"references_header,omitempty"`
	From             string `json:"from"`
	FromName         string `json:"from_name,omitempty"`
	To               string `json:"to,omitempty"`
	Subject          string `json:"subject,omitempty"`
	ReceivedAt       string `json:"received_at"`
	TextBody         string `json:"text_body,omitempty"`
	HTMLBody         string `json:"html_body,omitempty"`
	ContactID        *int64 `json:"contact_id,omitempty"`
	Read             bool   `json:"read"`
	Replied          bool   `json:"replied"`
	RawHeadersJSON   string `json:"raw_headers_json,omitempty"`
}

type InboxMarkReadIn struct {
	ID   int64 `json:"id"`
	Read bool  `json:"read"`
}
type InboxMarkReadOut struct {
	OK bool `json:"ok"`
}

type InboxReplyIn struct {
	ID       int64  `json:"id"`
	BodyText string `json:"body_text,omitempty"`
	BodyHTML string `json:"body_html,omitempty"`
}
type InboxReplyOut struct {
	OK bool `json:"ok"`
}

type InboxDeleteIn struct {
	ID int64 `json:"id"`
}
type InboxDeleteOut struct {
	OK bool `json:"ok"`
}

func (d *Deps) InboxSync(ctx context.Context, req *mcp.CallToolRequest, in InboxSyncIn) (*mcp.CallToolResult, InboxSyncOut, error) {
	if d.Svc.Inbox == nil {
		return mcpserver.Err("inbox_disabled", "inbox is not configured"), InboxSyncOut{}, nil
	}
	res, err := d.Svc.Inbox.Sync(ctx, in.Limit)
	if err != nil {
		return mcpserver.Err("sync_failed", err.Error()), InboxSyncOut{}, nil
	}
	return nil, InboxSyncOut{Fetched: res.Fetched, New: res.New, KnownContacts: res.KnownContacts, Notified: res.Notified}, nil
}

func (d *Deps) InboxList(ctx context.Context, req *mcp.CallToolRequest, in InboxListIn) (*mcp.CallToolResult, InboxListOut, error) {
	if d.Svc.Inbox == nil {
		return mcpserver.Err("inbox_disabled", "inbox is not configured"), InboxListOut{}, nil
	}
	var contactID *int64
	if in.ContactID > 0 {
		contactID = &in.ContactID
	}
	page, err := d.Svc.Inbox.List(ctx, domain.InboxFilter{UnreadOnly: in.Unread, KnownOnly: in.KnownOnly, ContactID: contactID}, port.Paging{Limit: in.Limit, Cursor: in.Cursor})
	if err != nil {
		return mcpserver.Err("list_failed", err.Error()), InboxListOut{}, nil
	}
	out := InboxListOut{NextCursor: page.NextCursor}
	for _, msg := range page.Items {
		out.Items = append(out.Items, inboxItemOut(msg))
	}
	return nil, out, nil
}

func (d *Deps) InboxGet(ctx context.Context, req *mcp.CallToolRequest, in InboxGetIn) (*mcp.CallToolResult, InboxGetOut, error) {
	if d.Svc.Inbox == nil {
		return mcpserver.Err("inbox_disabled", "inbox is not configured"), InboxGetOut{}, nil
	}
	msg, err := d.Svc.Inbox.Get(ctx, in.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return mcpserver.Err("not_found", "inbox message not found"), InboxGetOut{}, nil
	}
	if err != nil {
		return mcpserver.Err("get_failed", err.Error()), InboxGetOut{}, nil
	}
	return nil, inboxGetOut(msg), nil
}

func (d *Deps) InboxMarkRead(ctx context.Context, req *mcp.CallToolRequest, in InboxMarkReadIn) (*mcp.CallToolResult, InboxMarkReadOut, error) {
	if d.Svc.Inbox == nil {
		return mcpserver.Err("inbox_disabled", "inbox is not configured"), InboxMarkReadOut{}, nil
	}
	if err := d.Svc.Inbox.MarkRead(ctx, in.ID, in.Read); err != nil {
		return mcpserver.Err("mark_read_failed", err.Error()), InboxMarkReadOut{}, nil
	}
	return nil, InboxMarkReadOut{OK: true}, nil
}

func (d *Deps) InboxReply(ctx context.Context, req *mcp.CallToolRequest, in InboxReplyIn) (*mcp.CallToolResult, InboxReplyOut, error) {
	if d.Svc.Inbox == nil {
		return mcpserver.Err("inbox_disabled", "inbox is not configured"), InboxReplyOut{}, nil
	}
	if err := d.Svc.Inbox.Reply(ctx, domain.InboxReply{InboundID: in.ID, BodyText: in.BodyText, BodyHTML: in.BodyHTML}); err != nil {
		return mcpserver.Err("reply_failed", err.Error()), InboxReplyOut{}, nil
	}
	return nil, InboxReplyOut{OK: true}, nil
}

func (d *Deps) InboxDelete(ctx context.Context, req *mcp.CallToolRequest, in InboxDeleteIn) (*mcp.CallToolResult, InboxDeleteOut, error) {
	if d.Svc.Inbox == nil {
		return mcpserver.Err("inbox_disabled", "inbox is not configured"), InboxDeleteOut{}, nil
	}
	if err := d.Svc.Inbox.Delete(ctx, in.ID); err != nil {
		return mcpserver.Err("delete_failed", err.Error()), InboxDeleteOut{}, nil
	}
	return nil, InboxDeleteOut{OK: true}, nil
}

func inboxItemOut(msg domain.InboundMessage) InboxItemOut {
	return InboxItemOut{ID: msg.ID, From: msg.FromEmail, FromName: msg.FromName, Subject: msg.Subject, Snippet: msg.Snippet, ReceivedAt: msg.ReceivedAt.Format(time.RFC3339), ContactID: msg.ContactID, Read: msg.ReadAt != nil, Replied: msg.RepliedAt != nil}
}

func inboxGetOut(msg domain.InboundMessage) InboxGetOut {
	return InboxGetOut{ID: msg.ID, Mailbox: msg.Mailbox, UID: msg.UID, MessageID: msg.MessageID, InReplyTo: msg.InReplyTo, ReferencesHeader: msg.ReferencesHeader, From: msg.FromEmail, FromName: msg.FromName, To: msg.ToEmail, Subject: msg.Subject, ReceivedAt: msg.ReceivedAt.Format(time.RFC3339), TextBody: msg.TextBody, HTMLBody: msg.HTMLBody, ContactID: msg.ContactID, Read: msg.ReadAt != nil, Replied: msg.RepliedAt != nil, RawHeadersJSON: msg.RawHeadersJSON}
}
