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
