package httptransport

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/service"
)

// WhatsAppWebhookHandler handles inbound webhook events from the WhatsApp gateway.
type WhatsAppWebhookHandler struct {
	svc    *service.WhatsAppService
	secret string // HMAC-SHA256 secret for webhook validation (empty = no validation)
}

// NewWhatsAppWebhookHandler creates a new webhook handler.
func NewWhatsAppWebhookHandler(svc *service.WhatsAppService, secret string) *WhatsAppWebhookHandler {
	return &WhatsAppWebhookHandler{svc: svc, secret: secret}
}

// webhookEnvelope is the top-level JSON structure sent by the gateway
// (go-whatsapp-web-multidevice). It uses "event"/"payload"; "action"/"data"
// are accepted as aliases for forward/backward compatibility. For receipt
// (message.ack) events the timestamp lives at this top level, not in payload.
type webhookEnvelope struct {
	Event     string          `json:"event"`
	Payload   json.RawMessage `json:"payload"`
	DeviceID  string          `json:"device_id"`
	Timestamp string          `json:"timestamp"`
	Action    string          `json:"action"`
	Data      json.RawMessage `json:"data"`
}

// kind returns the normalized event name (event preferred, action fallback).
func (e webhookEnvelope) kind() string {
	if e.Event != "" {
		return e.Event
	}
	return e.Action
}

// body returns the inner payload (payload preferred, data fallback).
func (e webhookEnvelope) body() json.RawMessage {
	if len(e.Payload) > 0 {
		return e.Payload
	}
	return e.Data
}

// mediaPayload is a nested media object (image/video/audio/document/sticker).
// With auto-download the gateway may emit a bare string path instead of an
// object; UnmarshalJSON handles both shapes.
type mediaPayload struct {
	URL      string `json:"url"`
	Path     string `json:"path"`
	Caption  string `json:"caption"`
	Filename string `json:"filename"`
}

func (m *mediaPayload) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		m.Path = s
		return nil
	}
	type alias mediaPayload
	return json.Unmarshal(b, (*alias)(m))
}

func (m mediaPayload) ref() string {
	if m.URL != "" {
		return m.URL
	}
	return m.Path
}

// webhookMessage is the payload for event="message". Media arrives as a typed
// nested object keyed by kind (image/video/...), not flat media_type/media_url.
type webhookMessage struct {
	MessageID string        `json:"id"`
	ChatID    string        `json:"chat_id"`
	From      string        `json:"from"`
	FromName  string        `json:"from_name"`
	IsFromMe  bool          `json:"is_from_me"`
	Body      string        `json:"body"`
	RepliedTo string        `json:"replied_to_id"`
	Timestamp string        `json:"timestamp"`
	Image     *mediaPayload `json:"image"`
	Video     *mediaPayload `json:"video"`
	Audio     *mediaPayload `json:"audio"`
	Document  *mediaPayload `json:"document"`
	Sticker   *mediaPayload `json:"sticker"`
}

// media returns the message's media kind + reference, if any.
func (m webhookMessage) media() (kind, ref string) {
	switch {
	case m.Image != nil:
		return "image", m.Image.ref()
	case m.Video != nil:
		return "video", m.Video.ref()
	case m.Audio != nil:
		return "audio", m.Audio.ref()
	case m.Document != nil:
		return "document", m.Document.ref()
	case m.Sticker != nil:
		return "sticker", m.Sticker.ref()
	}
	return "", ""
}

// webhookReceipt is the payload for event="message.ack". The receipt timestamp
// is on the envelope, not here.
type webhookReceipt struct {
	MessageIDs  []string `json:"ids"`
	ChatID      string   `json:"chat_id"`
	From        string   `json:"from"`
	ReceiptType string   `json:"receipt_type"` // "delivered" | "read"
}

// ServeHTTP handles POST /wa/webhook
func (h *WhatsAppWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body first for HMAC validation
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Validate HMAC signature if secret is configured. The gateway
	// (go-whatsapp-web-multidevice) sends header X-Hub-Signature-256 with value
	// "sha256=<hex>"; the legacy X-Webhook-Signature (raw hex) is also accepted.
	if h.secret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if sig == "" {
			sig = r.Header.Get("X-Webhook-Signature")
		}
		sig = strings.TrimPrefix(sig, "sha256=")
		if sig == "" {
			http.Error(w, "missing signature", http.StatusUnauthorized)
			return
		}
		if !hmac.Equal([]byte(sig), []byte(h.computeSignature(body))) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse envelope
	var env webhookEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	switch env.kind() {
	case "message":
		if err := h.handleMessage(ctx, env.body()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "message.ack":
		if err := h.handleReceipt(ctx, env.body(), env.Timestamp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		// Ignore unhandled events (reactions, presence, group, newsletter, ...)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *WhatsAppWebhookHandler) handleMessage(ctx context.Context, data json.RawMessage) error {
	var msg webhookMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse message: %w", err)
	}

	// Skip outbound messages (we already store them via Send)
	if msg.IsFromMe {
		return nil
	}

	ts, err := time.Parse(time.RFC3339, msg.Timestamp)
	if err != nil {
		ts = time.Now()
	}

	mediaKind, mediaURL := msg.media()

	evt := domain.WAInboundEvent{
		MessageID:    msg.MessageID,
		ChatID:       msg.ChatID,
		From:         msg.From,
		FromName:     msg.FromName,
		IsFromMe:     msg.IsFromMe,
		Body:         msg.Body,
		MediaType:    domain.WAMediaType(mediaKind),
		MediaURL:     mediaURL,
		MediaCaption: msg.Body, // gateway folds media caption into top-level body
		RepliedTo:    msg.RepliedTo,
		Timestamp:    ts,
	}

	return h.svc.IngestMessage(ctx, evt)
}

func (h *WhatsAppWebhookHandler) handleReceipt(ctx context.Context, data json.RawMessage, envTimestamp string) error {
	var receipt webhookReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return fmt.Errorf("parse receipt: %w", err)
	}

	// Receipt timestamp is on the envelope, not the payload.
	ts, err := time.Parse(time.RFC3339, envTimestamp)
	if err != nil {
		ts = time.Now()
	}

	evt := domain.WAReceiptEvent{
		MessageIDs:  receipt.MessageIDs,
		ChatID:      receipt.ChatID,
		From:        receipt.From,
		ReceiptType: receipt.ReceiptType,
		Timestamp:   ts,
	}

	return h.svc.IngestReceipt(ctx, evt)
}

func (h *WhatsAppWebhookHandler) computeSignature(body []byte) string {
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
