// Package whatsapp provides an HTTP client adapter to a
// go-whatsapp-web-multidevice gateway, implementing port.WhatsAppGateway.
//
// The gateway exposes a REST API guarded by HTTP Basic auth and an x-device-id
// header selecting the connected device. This adapter never logs credentials.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// Config configures the gateway HTTP client.
type Config struct {
	BaseURL   string // e.g. https://notification.dev.lazyindra.online
	BasicAuth string // raw base64 of "user:pass" (the value after "Basic ")
	DeviceID  string // x-device-id header value, e.g. "cds"
	Timeout   time.Duration
}

// Client is an HTTP client for the WhatsApp gateway.
type Client struct {
	cfg  Config
	http *http.Client
	now  func() time.Time
}

var _ port.WhatsAppGateway = (*Client)(nil)

// New constructs a gateway client. BaseURL and DeviceID are required.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("whatsapp: base url required")
	}
	if strings.TrimSpace(cfg.DeviceID) == "" {
		return nil, fmt.Errorf("whatsapp: device id required")
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
		now:  time.Now,
	}, nil
}

// envelope is the gateway's standard response wrapper.
type envelope struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Results json.RawMessage `json:"results"`
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.cfg.BasicAuth != "" {
		req.Header.Set("Authorization", "Basic "+c.cfg.BasicAuth)
	}
	if c.cfg.DeviceID != "" {
		req.Header.Set("x-device-id", c.cfg.DeviceID)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

// do executes the request and decodes the standard envelope. It returns a
// sanitized error (never containing the auth header) on transport or API error.
func (c *Client) do(req *http.Request) (envelope, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return envelope{}, fmt.Errorf("whatsapp: request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var env envelope
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &env)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := env.Message
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return env, fmt.Errorf("whatsapp: gateway status %d: %s", resp.StatusCode, msg)
	}
	return env, nil
}

// Check reports whether a phone number is reachable on WhatsApp.
func (c *Client) Check(ctx context.Context, phone string) (domain.WhatsAppCheck, error) {
	phone = NormalizePhone(phone)
	out := domain.WhatsAppCheck{Phone: phone, CheckedAt: c.now().UTC()}
	if phone == "" {
		out.Status = domain.WhatsAppUnknown
		out.Reason = "empty phone"
		return out, nil
	}
	q := url.Values{}
	q.Set("phone", phone)
	req, err := c.newRequest(ctx, http.MethodGet, "/user/check?"+q.Encode(), nil, "")
	if err != nil {
		return out, err
	}
	env, err := c.do(req)
	if err != nil {
		out.Status = domain.WhatsAppUnknown
		out.Reason = "gateway error"
		return out, err
	}
	var res struct {
		IsOnWhatsApp bool `json:"is_on_whatsapp"`
	}
	if err := json.Unmarshal(env.Results, &res); err != nil {
		out.Status = domain.WhatsAppUnknown
		out.Reason = "unparseable response"
		return out, nil
	}
	if res.IsOnWhatsApp {
		out.Status = domain.WhatsAppRegistered
		out.Reason = "on whatsapp"
	} else {
		out.Status = domain.WhatsAppNotRegistered
		out.Reason = "not on whatsapp"
	}
	return out, nil
}

// Send delivers a text message via POST /send/message.
func (c *Client) Send(ctx context.Context, msg port.WhatsAppMessage) (port.WhatsAppSendResult, error) {
	phone := NormalizePhone(msg.Phone)
	if phone == "" {
		return port.WhatsAppSendResult{}, fmt.Errorf("whatsapp: empty phone")
	}
	form := url.Values{}
	form.Set("phone", phone)
	form.Set("message", msg.Body)
	if msg.ReplyToID != "" {
		form.Set("reply_message_id", msg.ReplyToID)
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/send/message", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return port.WhatsAppSendResult{}, err
	}
	env, err := c.do(req)
	if err != nil {
		return port.WhatsAppSendResult{}, err
	}
	var res struct {
		MessageID string `json:"message_id"`
		Status    string `json:"status"`
	}
	_ = json.Unmarshal(env.Results, &res)
	if res.Status == "" {
		res.Status = env.Message
	}
	return port.WhatsAppSendResult{MessageID: res.MessageID, Status: res.Status}, nil
}

// MarkRead marks an inbound message as read via POST /message/{id}/read.
func (c *Client) MarkRead(ctx context.Context, messageID, phone string) error {
	if messageID == "" {
		return fmt.Errorf("whatsapp: message id required")
	}
	form := url.Values{}
	if p := NormalizePhone(phone); p != "" {
		form.Set("phone", p)
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/message/"+url.PathEscape(messageID)+"/read", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}

// DownloadMedia retrieves a media file URL via GET /message/{id}/download.
func (c *Client) DownloadMedia(ctx context.Context, messageID, phone string) (port.WhatsAppMedia, error) {
	if messageID == "" {
		return port.WhatsAppMedia{}, fmt.Errorf("whatsapp: message id required")
	}
	q := url.Values{}
	if p := NormalizePhone(phone); p != "" {
		q.Set("phone", p)
	}
	path := "/message/" + url.PathEscape(messageID) + "/download"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	req, err := c.newRequest(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return port.WhatsAppMedia{}, err
	}
	env, err := c.do(req)
	if err != nil {
		return port.WhatsAppMedia{}, err
	}
	var res struct {
		FileURL  string `json:"file_url"`
		FilePath string `json:"file_path"`
		MimeType string `json:"mime_type"`
		Status   string `json:"status"`
	}
	_ = json.Unmarshal(env.Results, &res)
	return port.WhatsAppMedia{URL: res.FileURL, FilePath: res.FilePath, MimeType: res.MimeType, Status: res.Status}, nil
}

// ListGroups returns groups joined by the authenticated WhatsApp device.
func (c *Client) ListGroups(ctx context.Context) ([]port.WhatsAppGroup, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/user/my/groups", nil, "")
	if err != nil {
		return nil, err
	}
	env, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var raw []struct {
		JID          string            `json:"jid"`
		ID           string            `json:"id"`
		Name         string            `json:"name"`
		Topic        string            `json:"topic"`
		Participants []json.RawMessage `json:"participants"`
	}
	data := unwrapData(env.Results)
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("whatsapp: unparseable groups response: %w", err)
	}
	out := make([]port.WhatsAppGroup, 0, len(raw))
	for _, g := range raw {
		jid := g.JID
		if jid == "" {
			jid = g.ID
		}
		out = append(out, port.WhatsAppGroup{JID: jid, Name: g.Name, Topic: g.Topic, Participant: len(g.Participants)})
	}
	return out, nil
}

// ListContacts returns WhatsApp contacts known by the authenticated device.
func (c *Client) ListContacts(ctx context.Context) ([]port.WhatsAppContact, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/user/my/contacts", nil, "")
	if err != nil {
		return nil, err
	}
	env, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var raw []struct {
		JID  string `json:"jid"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(unwrapData(env.Results), &raw); err != nil {
		return nil, fmt.Errorf("whatsapp: unparseable contacts response: %w", err)
	}
	out := make([]port.WhatsAppContact, len(raw))
	for i, item := range raw {
		out[i] = port.WhatsAppContact{JID: item.JID, Name: item.Name}
	}
	return out, nil
}

func unwrapData(raw json.RawMessage) json.RawMessage {
	var wrapped struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Data) > 0 {
		return wrapped.Data
	}
	return raw
}

// JoinGroup joins a group via invite link. Returns the joined group JID if present.
func (c *Client) JoinGroup(ctx context.Context, inviteLink string) (string, error) {
	if strings.TrimSpace(inviteLink) == "" {
		return "", fmt.Errorf("whatsapp: invite link required")
	}
	body, _ := json.Marshal(map[string]string{"link": inviteLink})
	req, err := c.newRequest(ctx, http.MethodPost, "/group/join-with-link", bytes.NewReader(body), "application/json")
	if err != nil {
		return "", err
	}
	env, err := c.do(req)
	if err != nil {
		return "", err
	}
	var res struct {
		GroupID string `json:"group_id"`
		JID     string `json:"jid"`
	}
	_ = json.Unmarshal(env.Results, &res)
	if res.GroupID != "" {
		return res.GroupID, nil
	}
	return res.JID, nil
}

// LeaveGroup leaves a group by JID.
func (c *Client) LeaveGroup(ctx context.Context, groupJID string) error {
	if strings.TrimSpace(groupJID) == "" {
		return fmt.Errorf("whatsapp: group jid required")
	}
	body, _ := json.Marshal(map[string]string{"group_id": groupJID})
	req, err := c.newRequest(ctx, http.MethodPost, "/group/leave", bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}

// GroupInfoFromLink fetches group metadata from an invite link without joining.
func (c *Client) GroupInfoFromLink(ctx context.Context, inviteLink string) (port.WhatsAppGroup, error) {
	if strings.TrimSpace(inviteLink) == "" {
		return port.WhatsAppGroup{}, fmt.Errorf("whatsapp: invite link required")
	}
	q := url.Values{}
	q.Set("link", inviteLink)
	req, err := c.newRequest(ctx, http.MethodGet, "/group/info-from-link?"+q.Encode(), nil, "")
	if err != nil {
		return port.WhatsAppGroup{}, err
	}
	env, err := c.do(req)
	if err != nil {
		return port.WhatsAppGroup{}, err
	}
	var res struct {
		JID          string            `json:"jid"`
		ID           string            `json:"id"`
		GroupID      string            `json:"group_id"`
		Name         string            `json:"name"`
		Topic        string            `json:"topic"`
		Participants []json.RawMessage `json:"participants"`
	}
	if err := json.Unmarshal(unwrapData(env.Results), &res); err != nil {
		return port.WhatsAppGroup{}, fmt.Errorf("whatsapp: unparseable group info response: %w", err)
	}
	jid := res.JID
	if jid == "" {
		jid = res.GroupID
	}
	if jid == "" {
		jid = res.ID
	}
	return port.WhatsAppGroup{JID: jid, Name: res.Name, Topic: res.Topic, Participant: len(res.Participants)}, nil
}

// SendMedia sends image/video/file media via go-whatsapp-web-multidevice multipart endpoints.
func (c *Client) SendMedia(ctx context.Context, msg port.WhatsAppMediaMessage) (port.WhatsAppSendResult, error) {
	phone := NormalizePhone(msg.Phone)
	if phone == "" {
		return port.WhatsAppSendResult{}, fmt.Errorf("whatsapp: empty phone")
	}
	kind := strings.ToLower(strings.TrimSpace(msg.Kind))
	field, urlField := kind, kind+"_url"
	switch kind {
	case "image", "video":
	case "file", "document":
		kind, field, urlField = "file", "file", "file_url"
	default:
		return port.WhatsAppSendResult{}, fmt.Errorf("whatsapp: unsupported media kind %q", msg.Kind)
	}
	if msg.URL == "" && msg.FilePath == "" {
		return port.WhatsAppSendResult{}, fmt.Errorf("whatsapp: media url or file path required")
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("phone", phone)
	if msg.Caption != "" {
		_ = mw.WriteField("caption", msg.Caption)
	}
	if msg.ReplyToID != "" {
		_ = mw.WriteField("reply_message_id", msg.ReplyToID)
	}
	if msg.URL != "" {
		_ = mw.WriteField(urlField, msg.URL)
	}
	if msg.FilePath != "" {
		f, err := os.Open(msg.FilePath)
		if err != nil {
			return port.WhatsAppSendResult{}, err
		}
		defer f.Close()
		part, err := mw.CreateFormFile(field, filepath.Base(msg.FilePath))
		if err != nil {
			return port.WhatsAppSendResult{}, err
		}
		if _, err := io.Copy(part, f); err != nil {
			return port.WhatsAppSendResult{}, err
		}
	}
	if err := mw.Close(); err != nil {
		return port.WhatsAppSendResult{}, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/send/"+kind, &body, mw.FormDataContentType())
	if err != nil {
		return port.WhatsAppSendResult{}, err
	}
	env, err := c.do(req)
	if err != nil {
		return port.WhatsAppSendResult{}, err
	}
	var res struct {
		MessageID string `json:"message_id"`
		Status    string `json:"status"`
	}
	_ = json.Unmarshal(env.Results, &res)
	if res.Status == "" {
		res.Status = env.Message
	}
	return port.WhatsAppSendResult{MessageID: res.MessageID, Status: res.Status}, nil
}

var _ port.WhatsAppManageGateway = (*Client)(nil)

// CreateGroup creates a group with the given title and participant phones.
func (c *Client) CreateGroup(ctx context.Context, title string, participants []string) (string, error) {
	body, _ := json.Marshal(map[string]any{"title": title, "participants": participants})
	req, err := c.newRequest(ctx, http.MethodPost, "/group", bytes.NewReader(body), "application/json")
	if err != nil {
		return "", err
	}
	env, err := c.do(req)
	if err != nil {
		return "", err
	}
	var res struct {
		GroupID string `json:"group_id"`
		JID     string `json:"jid"`
	}
	_ = json.Unmarshal(env.Results, &res)
	if res.GroupID != "" {
		return res.GroupID, nil
	}
	return res.JID, nil
}

// GroupInfo fetches detailed metadata for a group.
func (c *Client) GroupInfo(ctx context.Context, groupJID string) (port.WhatsAppGroupInfo, error) {
	if strings.TrimSpace(groupJID) == "" {
		return port.WhatsAppGroupInfo{}, fmt.Errorf("whatsapp: group jid required")
	}
	q := url.Values{}
	q.Set("group_id", groupJID)
	req, err := c.newRequest(ctx, http.MethodGet, "/group/info?"+q.Encode(), nil, "")
	if err != nil {
		return port.WhatsAppGroupInfo{}, err
	}
	env, err := c.do(req)
	if err != nil {
		return port.WhatsAppGroupInfo{}, err
	}
	var res struct {
		JID              string `json:"JID"`
		GroupID          string `json:"group_id"`
		Name             string `json:"Name"`
		Topic            string `json:"Topic"`
		OwnerJID         string `json:"OwnerJID"`
		ParticipantCount int    `json:"ParticipantCount"`
		IsLocked         bool   `json:"IsLocked"`
		IsAnnounce       bool   `json:"IsAnnounce"`
		Participants     []struct {
			JID          string `json:"JID"`
			PhoneNumber  string `json:"PhoneNumber"`
			LID          string `json:"LID"`
			DisplayName  string `json:"DisplayName"`
			IsAdmin      bool   `json:"IsAdmin"`
			IsSuperAdmin bool   `json:"IsSuperAdmin"`
		} `json:"Participants"`
	}
	_ = json.Unmarshal(env.Results, &res)
	jid := res.JID
	if jid == "" {
		jid = res.GroupID
	}
	info := port.WhatsAppGroupInfo{
		JID: jid, Name: res.Name, Topic: res.Topic, OwnerJID: res.OwnerJID,
		ParticipantCount: res.ParticipantCount, IsLocked: res.IsLocked, IsAnnounce: res.IsAnnounce,
	}
	for _, p := range res.Participants {
		info.Participants = append(info.Participants, port.WhatsAppParticipant{
			JID: p.JID, Phone: p.PhoneNumber, LID: p.LID, DisplayName: p.DisplayName,
			IsAdmin: p.IsAdmin, IsSuperAdmin: p.IsSuperAdmin,
		})
	}
	if info.ParticipantCount == 0 {
		info.ParticipantCount = len(info.Participants)
	}
	return info, nil
}

// GroupParticipants lists members of a group.
func (c *Client) GroupParticipants(ctx context.Context, groupJID string) ([]port.WhatsAppParticipant, error) {
	if strings.TrimSpace(groupJID) == "" {
		return nil, fmt.Errorf("whatsapp: group jid required")
	}
	q := url.Values{}
	q.Set("group_id", groupJID)
	req, err := c.newRequest(ctx, http.MethodGet, "/group/participants?"+q.Encode(), nil, "")
	if err != nil {
		return nil, err
	}
	env, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var res struct {
		Participants []struct {
			JID          string `json:"jid"`
			PhoneNumber  string `json:"phone_number"`
			LID          string `json:"lid"`
			DisplayName  string `json:"display_name"`
			IsAdmin      bool   `json:"is_admin"`
			IsSuperAdmin bool   `json:"is_super_admin"`
		} `json:"participants"`
	}
	_ = json.Unmarshal(env.Results, &res)
	out := make([]port.WhatsAppParticipant, len(res.Participants))
	for i, p := range res.Participants {
		out[i] = port.WhatsAppParticipant{JID: p.JID, Phone: p.PhoneNumber, LID: p.LID, DisplayName: p.DisplayName, IsAdmin: p.IsAdmin, IsSuperAdmin: p.IsSuperAdmin}
	}
	return out, nil
}

// manageParticipants posts a ManageParticipantRequest to the given path.
func (c *Client) manageParticipants(ctx context.Context, path, groupJID string, participants []string) ([]port.WhatsAppParticipantResult, error) {
	if strings.TrimSpace(groupJID) == "" {
		return nil, fmt.Errorf("whatsapp: group jid required")
	}
	body, _ := json.Marshal(map[string]any{"group_id": groupJID, "participants": participants})
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(body), "application/json")
	if err != nil {
		return nil, err
	}
	env, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var res []struct {
		Participant string `json:"participant"`
		Status      string `json:"status"`
		Message     string `json:"message"`
	}
	_ = json.Unmarshal(env.Results, &res)
	out := make([]port.WhatsAppParticipantResult, len(res))
	for i, r := range res {
		out[i] = port.WhatsAppParticipantResult{Participant: r.Participant, Status: r.Status, Message: r.Message}
	}
	return out, nil
}

// ManageParticipants adds, removes, promotes, or demotes participants.
// action: add | remove | promote | demote.
func (c *Client) ManageParticipants(ctx context.Context, groupJID, action string, participants []string) ([]port.WhatsAppParticipantResult, error) {
	var path string
	switch action {
	case "add":
		path = "/group/participants"
	case "remove":
		path = "/group/participants/remove"
	case "promote":
		path = "/group/participants/promote"
	case "demote":
		path = "/group/participants/demote"
	default:
		return nil, fmt.Errorf("whatsapp: unsupported participant action %q", action)
	}
	return c.manageParticipants(ctx, path, groupJID, participants)
}

// GroupParticipantRequests lists pending join requests for a group.
func (c *Client) GroupParticipantRequests(ctx context.Context, groupJID string) ([]port.WhatsAppParticipant, error) {
	if strings.TrimSpace(groupJID) == "" {
		return nil, fmt.Errorf("whatsapp: group jid required")
	}
	q := url.Values{}
	q.Set("group_id", groupJID)
	req, err := c.newRequest(ctx, http.MethodGet, "/group/participant-requests?"+q.Encode(), nil, "")
	if err != nil {
		return nil, err
	}
	env, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var res []struct {
		JID         string `json:"jid"`
		PhoneNumber string `json:"phone_number"`
		DisplayName string `json:"display_name"`
	}
	_ = json.Unmarshal(unwrapData(env.Results), &res)
	out := make([]port.WhatsAppParticipant, len(res))
	for i, r := range res {
		out[i] = port.WhatsAppParticipant{JID: r.JID, Phone: r.PhoneNumber, DisplayName: r.DisplayName}
	}
	return out, nil
}

// ReviewParticipantRequests approves or rejects pending join requests.
// action: approve | reject.
func (c *Client) ReviewParticipantRequests(ctx context.Context, groupJID, action string, participants []string) error {
	var path string
	switch action {
	case "approve":
		path = "/group/participant-requests/approve"
	case "reject":
		path = "/group/participant-requests/reject"
	default:
		return fmt.Errorf("whatsapp: unsupported review action %q", action)
	}
	if strings.TrimSpace(groupJID) == "" {
		return fmt.Errorf("whatsapp: group jid required")
	}
	body, _ := json.Marshal(map[string]any{"group_id": groupJID, "participants": participants})
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}

func (c *Client) groupSetString(ctx context.Context, path, groupJID, field, value string) error {
	if strings.TrimSpace(groupJID) == "" {
		return fmt.Errorf("whatsapp: group jid required")
	}
	body, _ := json.Marshal(map[string]string{"group_id": groupJID, field: value})
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}

func (c *Client) groupSetBool(ctx context.Context, path, groupJID, field string, value bool) error {
	if strings.TrimSpace(groupJID) == "" {
		return fmt.Errorf("whatsapp: group jid required")
	}
	body, _ := json.Marshal(map[string]any{"group_id": groupJID, field: value})
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}

// SetGroupName sets the group subject.
func (c *Client) SetGroupName(ctx context.Context, groupJID, name string) error {
	return c.groupSetString(ctx, "/group/name", groupJID, "name", name)
}

// SetGroupTopic sets or clears the group topic/description.
func (c *Client) SetGroupTopic(ctx context.Context, groupJID, topic string) error {
	return c.groupSetString(ctx, "/group/topic", groupJID, "topic", topic)
}

// SetGroupLocked locks/unlocks group info editing to admins.
func (c *Client) SetGroupLocked(ctx context.Context, groupJID string, locked bool) error {
	return c.groupSetBool(ctx, "/group/locked", groupJID, "locked", locked)
}

// SetGroupAnnounce enables/disables admin-only messaging.
func (c *Client) SetGroupAnnounce(ctx context.Context, groupJID string, announce bool) error {
	return c.groupSetBool(ctx, "/group/announce", groupJID, "announce", announce)
}

// GroupInviteLink returns (or resets) the group invite link.
func (c *Client) GroupInviteLink(ctx context.Context, groupJID string, reset bool) (string, error) {
	if strings.TrimSpace(groupJID) == "" {
		return "", fmt.Errorf("whatsapp: group jid required")
	}
	q := url.Values{}
	q.Set("group_id", groupJID)
	if reset {
		q.Set("reset", "true")
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/group/invite-link?"+q.Encode(), nil, "")
	if err != nil {
		return "", err
	}
	env, err := c.do(req)
	if err != nil {
		return "", err
	}
	var res struct {
		InviteLink string `json:"invite_link"`
	}
	_ = json.Unmarshal(env.Results, &res)
	return res.InviteLink, nil
}

// ConnectionStatus reports gateway connection/login state.
func (c *Client) ConnectionStatus(ctx context.Context) (port.WhatsAppConnection, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/app/status", nil, "")
	if err != nil {
		return port.WhatsAppConnection{}, err
	}
	env, err := c.do(req)
	if err != nil {
		return port.WhatsAppConnection{}, err
	}
	var res struct {
		IsConnected bool   `json:"is_connected"`
		IsLoggedIn  bool   `json:"is_logged_in"`
		DeviceID    string `json:"device_id"`
		JID         string `json:"jid"`
	}
	_ = json.Unmarshal(env.Results, &res)
	return port.WhatsAppConnection{Connected: res.IsConnected, LoggedIn: res.IsLoggedIn, DeviceID: res.DeviceID, JID: res.JID}, nil
}

// ListDevices lists linked devices.
func (c *Client) ListDevices(ctx context.Context) ([]port.WhatsAppDevice, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/app/devices", nil, "")
	if err != nil {
		return nil, err
	}
	env, err := c.do(req)
	if err != nil {
		return nil, err
	}
	var res []struct {
		Name string `json:"name"`
		JID  string `json:"device"`
	}
	_ = json.Unmarshal(unwrapData(env.Results), &res)
	out := make([]port.WhatsAppDevice, len(res))
	for i, d := range res {
		out[i] = port.WhatsAppDevice{Name: d.Name, JID: d.JID}
	}
	return out, nil
}

// Logout logs the device out of WhatsApp.
func (c *Client) Logout(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/app/logout", nil, "")
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}

// Reconnect forces the gateway to reconnect.
func (c *Client) Reconnect(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/app/reconnect", nil, "")
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}

// UserInfo looks up a WhatsApp user's profile by phone.
func (c *Client) UserInfo(ctx context.Context, phone string) (port.WhatsAppUserInfo, error) {
	p := NormalizePhone(phone)
	if p == "" {
		return port.WhatsAppUserInfo{}, fmt.Errorf("whatsapp: phone required")
	}
	q := url.Values{}
	q.Set("phone", p)
	req, err := c.newRequest(ctx, http.MethodGet, "/user/info?"+q.Encode(), nil, "")
	if err != nil {
		return port.WhatsAppUserInfo{}, err
	}
	env, err := c.do(req)
	if err != nil {
		return port.WhatsAppUserInfo{}, err
	}
	var res struct {
		VerifiedName string   `json:"verified_name"`
		Status       string   `json:"status"`
		PictureID    string   `json:"picture_id"`
		Devices      []string `json:"devices"`
	}
	_ = json.Unmarshal(env.Results, &res)
	return port.WhatsAppUserInfo{VerifiedName: res.VerifiedName, Status: res.Status, PictureID: res.PictureID, Devices: res.Devices}, nil
}

// SetPushName changes the account display (push) name.
func (c *Client) SetPushName(ctx context.Context, name string) error {
	body, _ := json.Marshal(map[string]string{"push_name": name})
	req, err := c.newRequest(ctx, http.MethodPost, "/user/pushname", bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}
