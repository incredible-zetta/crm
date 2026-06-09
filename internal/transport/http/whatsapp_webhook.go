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

// webhookEnvelope is the top-level JSON structure sent by the gateway.
type webhookEnvelope struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// webhookMessage is the payload for action="message".
type webhookMessage struct {
	MessageID    string `json:"id"`
	ChatID       string `json:"chat_id"`
	From         string `json:"from"`
	FromName     string `json:"pushname"`
	IsFromMe     bool   `json:"from_me"`
	Body         string `json:"body"`
	MediaType    string `json:"media_type"`
	MediaURL     string `json:"media_url"`
	MediaCaption string `json:"media_caption"`
	RepliedTo    string `json:"quoted_message_id"`
	Timestamp    string `json:"timestamp"`
}

// webhookReceipt is the payload for action="message.ack".
type webhookReceipt struct {
	MessageIDs  []string `json:"ids"`
	ChatID      string   `json:"chat_id"`
	From        string   `json:"from"`
	ReceiptType string   `json:"type"` // "delivered" | "read"
	Timestamp   string   `json:"timestamp"`
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

	// Validate HMAC signature if secret is configured
	if h.secret != "" {
		sig := r.Header.Get("X-Webhook-Signature")
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
	switch env.Action {
	case "message":
		if err := h.handleMessage(ctx, env.Data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "message.ack":
		if err := h.handleReceipt(ctx, env.Data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		// Ignore unknown actions
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

	evt := domain.WAInboundEvent{
		MessageID:    msg.MessageID,
		ChatID:       msg.ChatID,
		From:         msg.From,
		FromName:     msg.FromName,
		IsFromMe:     msg.IsFromMe,
		Body:         msg.Body,
		MediaType:    domain.WAMediaType(msg.MediaType),
		MediaURL:     msg.MediaURL,
		MediaCaption: msg.MediaCaption,
		RepliedTo:    msg.RepliedTo,
		Timestamp:    ts,
	}

	return h.svc.IngestMessage(ctx, evt)
}

func (h *WhatsAppWebhookHandler) handleReceipt(ctx context.Context, data json.RawMessage) error {
	var receipt webhookReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return fmt.Errorf("parse receipt: %w", err)
	}

	ts, err := time.Parse(time.RFC3339, receipt.Timestamp)
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
