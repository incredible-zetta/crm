package mcptransport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Check ---

type WhatsAppCheckIn struct {
	ID    int64  `json:"id,omitempty"`
	Phone string `json:"phone,omitempty"`
}

type WhatsAppCheckOut struct {
	Phone     string `json:"phone"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	CheckedAt string `json:"checked_at"`
}

func (d *Deps) WhatsAppCheck(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppCheckIn) (*mcp.CallToolResult, WhatsAppCheckOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppCheckOut{}, nil
	}
	if in.ID == 0 && in.Phone == "" {
		return mcpserver.Err("validation", "id or phone required"), WhatsAppCheckOut{}, nil
	}
	var v domain.WhatsAppCheck
	var err error
	if in.ID != 0 {
		_, v, err = d.Svc.WhatsApp.Check(ctx, in.ID)
	} else {
		v, err = d.Svc.WhatsApp.CheckPhone(ctx, in.Phone)
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "contact not found"), WhatsAppCheckOut{}, nil
		}
		return nil, WhatsAppCheckOut{}, fmt.Errorf("whatsapp_check: %w", err)
	}
	return nil, WhatsAppCheckOut{
		Phone:     v.Phone,
		Status:    string(v.Status),
		Reason:    v.Reason,
		CheckedAt: v.CheckedAt.Format(time.RFC3339),
	}, nil
}

// --- Audit ---

type WhatsAppAuditIn struct {
	Stage   string `json:"stage,omitempty"`
	Company string `json:"company,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Q       string `json:"q,omitempty"`
	// OnlyUnchecked skips contacts already checked (default true).
	OnlyUnchecked *bool `json:"only_unchecked,omitempty"`
	Limit         int   `json:"limit,omitempty"`
	Cursor        int64 `json:"cursor,omitempty"`
}

type WhatsAppAuditOut struct {
	Checked       int   `json:"checked"`
	Registered    int   `json:"registered"`
	NotRegistered int   `json:"not_registered"`
	Unknown       int   `json:"unknown"`
	NextCursor    int64 `json:"next_cursor"`
}

func (d *Deps) WhatsAppAudit(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppAuditIn) (*mcp.CallToolResult, WhatsAppAuditOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppAuditOut{}, nil
	}
	only := true
	if in.OnlyUnchecked != nil {
		only = *in.OnlyUnchecked
	}
	f := domain.ContactFilter{Stage: in.Stage, Company: in.Company, Tag: in.Tag, Q: in.Q}
	res, err := d.Svc.WhatsApp.Audit(ctx, f, only, in.Limit, in.Cursor)
	if err != nil {
		return nil, WhatsAppAuditOut{}, fmt.Errorf("whatsapp_audit: %w", err)
	}
	return nil, WhatsAppAuditOut{
		Checked:       res.Checked,
		Registered:    res.Registered,
		NotRegistered: res.NotRegistered,
		Unknown:       res.Unknown,
		NextCursor:    res.NextCursor,
	}, nil
}

// --- Send ---

type WhatsAppSendIn struct {
	// ID or Phone identifies the recipient. ID takes precedence.
	ID    int64  `json:"id,omitempty"`
	Phone string `json:"phone,omitempty"`
	// Body is the message text. Use WhatsApp markdown:
	// *bold*, _italic_, ~strikethrough~, ```code```.
	// See the whatsapp://formatting resource for full reference.
	Body string `json:"body"`
}

type WhatsAppSendOut struct {
	ID        int64  `json:"id"`
	MessageID string `json:"message_id"`
	Phone     string `json:"phone"`
	Status    string `json:"status"`
}

func (d *Deps) WhatsAppSend(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppSendIn) (*mcp.CallToolResult, WhatsAppSendOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppSendOut{}, nil
	}
	if in.Body == "" {
		return mcpserver.Err("validation", "body required"), WhatsAppSendOut{}, nil
	}
	var phone string
	if in.ID != 0 {
		c, err := d.Svc.Contact.Get(ctx, in.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return mcpserver.Err("not_found", "contact not found"), WhatsAppSendOut{}, nil
			}
			return nil, WhatsAppSendOut{}, fmt.Errorf("whatsapp_send: get contact: %w", err)
		}
		if c.Phone == "" {
			return mcpserver.Err("validation", "contact has no phone"), WhatsAppSendOut{}, nil
		}
		phone = c.Phone
	} else if in.Phone != "" {
		phone = in.Phone
	} else {
		return mcpserver.Err("validation", "id or phone required"), WhatsAppSendOut{}, nil
	}
	msg, err := d.Svc.WhatsApp.Send(ctx, phone, in.Body)
	if err != nil {
		if strings.Contains(err.Error(), "verified not on WhatsApp") {
			return mcpserver.Err("blocked", err.Error()), WhatsAppSendOut{}, nil
		}
		return nil, WhatsAppSendOut{}, fmt.Errorf("whatsapp_send: %w", err)
	}
	return nil, WhatsAppSendOut{
		ID:        msg.ID,
		MessageID: msg.MessageID,
		Phone:     msg.Phone,
		Status:    string(msg.Status),
	}, nil
}

// --- List ---

type WhatsAppListIn struct {
	Direction string `json:"direction,omitempty"` // "in" | "out" | ""
	Unread    bool   `json:"unread,omitempty"`
	KnownOnly bool   `json:"known_only,omitempty"`
	ContactID int64  `json:"contact_id,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Cursor    int64  `json:"cursor,omitempty"`
}

type WhatsAppItemOut struct {
	ID          int64  `json:"id"`
	MessageID   string `json:"message_id,omitempty"`
	Direction   string `json:"direction"`
	Phone       string `json:"phone"`
	ContactID   *int64 `json:"contact_id,omitempty"`
	Body        string `json:"body,omitempty"`
	MediaType   string `json:"media_type,omitempty"`
	Status      string `json:"status"`
	RepliedTo   string `json:"replied_to,omitempty"`
	SentAt      string `json:"sent_at,omitempty"`
	DeliveredAt string `json:"delivered_at,omitempty"`
	ReadAt      string `json:"read_at,omitempty"`
	ReceivedAt  string `json:"received_at,omitempty"`
	RepliedAt   string `json:"replied_at,omitempty"`
}

type WhatsAppListOut struct {
	Items      []WhatsAppItemOut `json:"items"`
	NextCursor int64             `json:"next_cursor"`
}

func (d *Deps) WhatsAppList(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppListIn) (*mcp.CallToolResult, WhatsAppListOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppListOut{}, nil
	}
	var contactID *int64
	if in.ContactID > 0 {
		contactID = &in.ContactID
	}
	f := domain.WAInboundFilter{
		Direction:  in.Direction,
		UnreadOnly: in.Unread,
		KnownOnly:  in.KnownOnly,
		ContactID:  contactID,
		Phone:      in.Phone,
	}
	page, err := d.Svc.WhatsApp.List(ctx, f, port.Paging{Limit: in.Limit, Cursor: in.Cursor})
	if err != nil {
		return nil, WhatsAppListOut{}, fmt.Errorf("whatsapp_list: %w", err)
	}
	items := make([]WhatsAppItemOut, len(page.Items))
	for i, msg := range page.Items {
		items[i] = toWAItemOut(msg)
	}
	return nil, WhatsAppListOut{Items: items, NextCursor: page.NextCursor}, nil
}

func toWAItemOut(msg domain.WAMessage) WhatsAppItemOut {
	out := WhatsAppItemOut{
		ID:        msg.ID,
		MessageID: msg.MessageID,
		Direction: string(msg.Direction),
		Phone:     msg.Phone,
		ContactID: msg.ContactID,
		Body:      msg.Body,
		MediaType: string(msg.MediaType),
		Status:    string(msg.Status),
		RepliedTo: msg.RepliedTo,
	}
	if msg.SentAt != nil {
		out.SentAt = msg.SentAt.Format(time.RFC3339)
	}
	if msg.DeliveredAt != nil {
		out.DeliveredAt = msg.DeliveredAt.Format(time.RFC3339)
	}
	if msg.ReadAt != nil {
		out.ReadAt = msg.ReadAt.Format(time.RFC3339)
	}
	if msg.ReceivedAt != nil {
		out.ReceivedAt = msg.ReceivedAt.Format(time.RFC3339)
	}
	if msg.RepliedAt != nil {
		out.RepliedAt = msg.RepliedAt.Format(time.RFC3339)
	}
	return out
}

// --- Get ---

type WhatsAppGetIn struct {
	ID int64 `json:"id"`
}

type WhatsAppGetOut struct {
	WhatsAppItemOut
	MediaURL     string `json:"media_url,omitempty"`
	MediaCaption string `json:"media_caption,omitempty"`
	Error        string `json:"error,omitempty"`
}

func (d *Deps) WhatsAppGet(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppGetIn) (*mcp.CallToolResult, WhatsAppGetOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppGetOut{}, nil
	}
	if in.ID == 0 {
		return mcpserver.Err("validation", "id required"), WhatsAppGetOut{}, nil
	}
	msg, err := d.Svc.WhatsApp.Get(ctx, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "message not found"), WhatsAppGetOut{}, nil
		}
		return nil, WhatsAppGetOut{}, fmt.Errorf("whatsapp_get: %w", err)
	}
	item := toWAItemOut(msg)
	return nil, WhatsAppGetOut{
		WhatsAppItemOut: item,
		MediaURL:        msg.MediaURL,
		MediaCaption:    msg.MediaCaption,
		Error:           msg.Error,
	}, nil
}

// --- Reply ---

type WhatsAppReplyIn struct {
	// ID of the inbound message to reply to.
	ID int64 `json:"id"`
	// Body is the reply text. Use WhatsApp markdown:
	// *bold*, _italic_, ~strikethrough~, ```code```.
	// See the whatsapp://formatting resource for full reference.
	Body string `json:"body"`
}

type WhatsAppReplyOut struct {
	ID        int64  `json:"id"`
	MessageID string `json:"message_id"`
	Phone     string `json:"phone"`
	Status    string `json:"status"`
}

func (d *Deps) WhatsAppReply(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppReplyIn) (*mcp.CallToolResult, WhatsAppReplyOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppReplyOut{}, nil
	}
	if in.ID == 0 {
		return mcpserver.Err("validation", "id required"), WhatsAppReplyOut{}, nil
	}
	if in.Body == "" {
		return mcpserver.Err("validation", "body required"), WhatsAppReplyOut{}, nil
	}
	msg, err := d.Svc.WhatsApp.Reply(ctx, in.ID, in.Body)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "message not found"), WhatsAppReplyOut{}, nil
		}
		return nil, WhatsAppReplyOut{}, fmt.Errorf("whatsapp_reply: %w", err)
	}
	return nil, WhatsAppReplyOut{
		ID:        msg.ID,
		MessageID: msg.MessageID,
		Phone:     msg.Phone,
		Status:    string(msg.Status),
	}, nil
}

// --- MarkRead ---

type WhatsAppMarkReadIn struct {
	ID int64 `json:"id"`
}

type WhatsAppMarkReadOut struct {
	OK bool `json:"ok"`
}

func (d *Deps) WhatsAppMarkRead(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppMarkReadIn) (*mcp.CallToolResult, WhatsAppMarkReadOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppMarkReadOut{}, nil
	}
	if in.ID == 0 {
		return mcpserver.Err("validation", "id required"), WhatsAppMarkReadOut{}, nil
	}
	if err := d.Svc.WhatsApp.MarkRead(ctx, in.ID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "message not found"), WhatsAppMarkReadOut{}, nil
		}
		return nil, WhatsAppMarkReadOut{}, fmt.Errorf("whatsapp_mark_read: %w", err)
	}
	return nil, WhatsAppMarkReadOut{OK: true}, nil
}

// --- GetMedia ---

type WhatsAppGetMediaIn struct {
	ID int64 `json:"id"`
}

type WhatsAppGetMediaOut struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type,omitempty"`
}

func (d *Deps) WhatsAppGetMedia(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppGetMediaIn) (*mcp.CallToolResult, WhatsAppGetMediaOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppGetMediaOut{}, nil
	}
	if in.ID == 0 {
		return mcpserver.Err("validation", "id required"), WhatsAppGetMediaOut{}, nil
	}
	url, err := d.Svc.WhatsApp.DownloadMedia(ctx, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "message not found"), WhatsAppGetMediaOut{}, nil
		}
		return nil, WhatsAppGetMediaOut{}, fmt.Errorf("whatsapp_get_media: %w", err)
	}
	return nil, WhatsAppGetMediaOut{URL: url}, nil
}
