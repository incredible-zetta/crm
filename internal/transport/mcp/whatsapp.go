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

// --- Groups ---

type WhatsAppGroupsIn struct{}

type WhatsAppContactsIn struct{}

type WhatsAppGroupOut struct {
	JID         string `json:"jid"`
	Name        string `json:"name,omitempty"`
	Topic       string `json:"topic,omitempty"`
	Participant int    `json:"participant_count,omitempty"`
}

type WhatsAppGroupsOut struct {
	Items []WhatsAppGroupOut `json:"items"`
	Count int                `json:"count"`
}

type WhatsAppContactOut struct {
	JID  string `json:"jid"`
	Name string `json:"name,omitempty"`
}
type WhatsAppContactsOut struct {
	Items []WhatsAppContactOut `json:"items"`
	Count int                  `json:"count"`
}

func (d *Deps) WhatsAppGroups(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppGroupsIn) (*mcp.CallToolResult, WhatsAppGroupsOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppGroupsOut{}, nil
	}
	groups, err := d.Svc.WhatsApp.ListGroups(ctx)
	if err != nil {
		return nil, WhatsAppGroupsOut{}, fmt.Errorf("whatsapp_groups: %w", err)
	}
	items := make([]WhatsAppGroupOut, len(groups))
	for i, g := range groups {
		items[i] = WhatsAppGroupOut{JID: g.JID, Name: g.Name, Topic: g.Topic, Participant: g.Participant}
	}
	return nil, WhatsAppGroupsOut{Items: items, Count: len(items)}, nil
}

func (d *Deps) WhatsAppContacts(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppContactsIn) (*mcp.CallToolResult, WhatsAppContactsOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppContactsOut{}, nil
	}
	contacts, err := d.Svc.WhatsApp.ListContacts(ctx)
	if err != nil {
		return nil, WhatsAppContactsOut{}, fmt.Errorf("whatsapp_contacts: %w", err)
	}
	items := make([]WhatsAppContactOut, len(contacts))
	for i, c := range contacts {
		items[i] = WhatsAppContactOut{JID: c.JID, Name: c.Name}
	}
	return nil, WhatsAppContactsOut{Items: items, Count: len(items)}, nil
}

// --- Send media ---

type WhatsAppSendMediaIn struct {
	ID        int64  `json:"id,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Kind      string `json:"kind"` // image | video | file
	URL       string `json:"url,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	Caption   string `json:"caption,omitempty"`
	ReplyToID string `json:"reply_to_id,omitempty"`
}

type WhatsAppSendMediaOut struct {
	ID        int64  `json:"id"`
	MessageID string `json:"message_id"`
	Phone     string `json:"phone"`
	Status    string `json:"status"`
}

func (d *Deps) WhatsAppSendMedia(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppSendMediaIn) (*mcp.CallToolResult, WhatsAppSendMediaOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppSendMediaOut{}, nil
	}
	if in.Kind == "" {
		return mcpserver.Err("validation", "kind required"), WhatsAppSendMediaOut{}, nil
	}
	if in.URL == "" && in.FilePath == "" {
		return mcpserver.Err("validation", "url or file_path required"), WhatsAppSendMediaOut{}, nil
	}
	phone := in.Phone
	if in.ID != 0 {
		c, err := d.Svc.Contact.Get(ctx, in.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return mcpserver.Err("not_found", "contact not found"), WhatsAppSendMediaOut{}, nil
			}
			return nil, WhatsAppSendMediaOut{}, fmt.Errorf("whatsapp_send_media: get contact: %w", err)
		}
		if c.Phone == "" {
			return mcpserver.Err("validation", "contact has no phone"), WhatsAppSendMediaOut{}, nil
		}
		phone = c.Phone
	}
	if phone == "" {
		return mcpserver.Err("validation", "id or phone required"), WhatsAppSendMediaOut{}, nil
	}
	msg, err := d.Svc.WhatsApp.SendMedia(ctx, port.WhatsAppMediaMessage{Phone: phone, Kind: in.Kind, URL: in.URL, FilePath: in.FilePath, Caption: in.Caption, ReplyToID: in.ReplyToID})
	if err != nil {
		return nil, WhatsAppSendMediaOut{}, fmt.Errorf("whatsapp_send_media: %w", err)
	}
	return nil, WhatsAppSendMediaOut{ID: msg.ID, MessageID: msg.MessageID, Phone: msg.Phone, Status: string(msg.Status)}, nil
}

// --- Listeners ---

type WhatsAppListenerCreateIn struct {
	ChatJID string `json:"chat_jid"`
	Name    string `json:"name,omitempty"`
}
type WhatsAppListenerUpdateIn struct {
	ID      int64  `json:"id"`
	ChatJID string `json:"chat_jid,omitempty"`
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled"`
}
type WhatsAppListenerDeleteIn struct {
	ID int64 `json:"id"`
}
type WhatsAppListenerListIn struct {
	EnabledOnly bool `json:"enabled_only,omitempty"`
}
type WhatsAppListenerSummaryIn struct {
	ID    int64 `json:"id"`
	Limit int   `json:"limit,omitempty"`
}

type WhatsAppListenerOut struct {
	ID        int64  `json:"id"`
	ChatJID   string `json:"chat_jid"`
	Name      string `json:"name,omitempty"`
	Enabled   bool   `json:"enabled"`
	Summary   string `json:"summary,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}
type WhatsAppListenerListOut struct {
	Items []WhatsAppListenerOut `json:"items"`
	Count int                   `json:"count"`
}
type WhatsAppListenerDeleteOut struct {
	OK bool `json:"ok"`
}
type WhatsAppListenerSummaryOut struct {
	Listener WhatsAppListenerOut `json:"listener"`
	Messages []WhatsAppItemOut   `json:"messages"`
}

func toWAListenerOut(l domain.WAListener) WhatsAppListenerOut {
	return WhatsAppListenerOut{ID: l.ID, ChatJID: l.ChatJID, Name: l.Name, Enabled: l.Enabled, Summary: l.Summary, CreatedAt: l.CreatedAt.Format(time.RFC3339), UpdatedAt: l.UpdatedAt.Format(time.RFC3339)}
}

func (d *Deps) WhatsAppListenerCreate(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppListenerCreateIn) (*mcp.CallToolResult, WhatsAppListenerOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppListenerOut{}, nil
	}
	if in.ChatJID == "" {
		return mcpserver.Err("validation", "chat_jid required"), WhatsAppListenerOut{}, nil
	}
	l, err := d.Svc.WhatsApp.CreateListener(ctx, in.ChatJID, in.Name)
	if err != nil {
		return nil, WhatsAppListenerOut{}, fmt.Errorf("whatsapp_listener_create: %w", err)
	}
	return nil, toWAListenerOut(l), nil
}

func (d *Deps) WhatsAppListenerList(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppListenerListIn) (*mcp.CallToolResult, WhatsAppListenerListOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppListenerListOut{}, nil
	}
	items, err := d.Svc.WhatsApp.ListListeners(ctx, in.EnabledOnly)
	if err != nil {
		return nil, WhatsAppListenerListOut{}, fmt.Errorf("whatsapp_listener_list: %w", err)
	}
	out := make([]WhatsAppListenerOut, len(items))
	for i, item := range items {
		out[i] = toWAListenerOut(item)
	}
	return nil, WhatsAppListenerListOut{Items: out, Count: len(out)}, nil
}

func (d *Deps) WhatsAppListenerUpdate(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppListenerUpdateIn) (*mcp.CallToolResult, WhatsAppListenerOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppListenerOut{}, nil
	}
	if in.ID == 0 {
		return mcpserver.Err("validation", "id required"), WhatsAppListenerOut{}, nil
	}
	l, err := d.Svc.WhatsApp.UpdateListener(ctx, in.ID, in.ChatJID, in.Name, in.Enabled)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "listener not found"), WhatsAppListenerOut{}, nil
		}
		return nil, WhatsAppListenerOut{}, fmt.Errorf("whatsapp_listener_update: %w", err)
	}
	return nil, toWAListenerOut(l), nil
}

func (d *Deps) WhatsAppListenerDelete(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppListenerDeleteIn) (*mcp.CallToolResult, WhatsAppListenerDeleteOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppListenerDeleteOut{}, nil
	}
	if in.ID == 0 {
		return mcpserver.Err("validation", "id required"), WhatsAppListenerDeleteOut{}, nil
	}
	if err := d.Svc.WhatsApp.DeleteListener(ctx, in.ID); err != nil {
		return nil, WhatsAppListenerDeleteOut{}, fmt.Errorf("whatsapp_listener_delete: %w", err)
	}
	return nil, WhatsAppListenerDeleteOut{OK: true}, nil
}

func (d *Deps) WhatsAppListenerSummary(ctx context.Context, req *mcp.CallToolRequest, in WhatsAppListenerSummaryIn) (*mcp.CallToolResult, WhatsAppListenerSummaryOut, error) {
	if d.Svc.WhatsApp == nil {
		return mcpserver.Err("disabled", "whatsapp channel not configured"), WhatsAppListenerSummaryOut{}, nil
	}
	if in.ID == 0 {
		return mcpserver.Err("validation", "id required"), WhatsAppListenerSummaryOut{}, nil
	}
	l, msgs, err := d.Svc.WhatsApp.ListenerSummary(ctx, in.ID, in.Limit)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "listener not found"), WhatsAppListenerSummaryOut{}, nil
		}
		return nil, WhatsAppListenerSummaryOut{}, fmt.Errorf("whatsapp_listener_summary: %w", err)
	}
	outMsgs := make([]WhatsAppItemOut, len(msgs))
	for i, msg := range msgs {
		outMsgs[i] = toWAItemOut(msg)
	}
	return nil, WhatsAppListenerSummaryOut{Listener: toWAListenerOut(l), Messages: outMsgs}, nil
}
