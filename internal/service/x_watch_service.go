package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
)

// ErrWatchesDisabled is returned when watch persistence is not wired.
var ErrWatchesDisabled = errors.New("x watch persistence not configured")

// XWatchService drives watching x.com signals (mentions/search) per account,
// persisting matches and delivering them to a per-watch HMAC-signed webhook.
// It reuses XService for cookie resolution (stored account label -> cookies).
//
// Optional; watches is nil when the feature is not wired. The HTTP client is
// shared and has a short timeout so a slow webhook can't stall the poller.
type XWatchService struct {
	watches port.XWatchRepo
	x       *XService
	http    *http.Client
}

// NewXWatchService builds the watch service. watches may be nil to disable the
// feature. x is required to resolve stored account cookies for polling.
func NewXWatchService(watches port.XWatchRepo, x *XService) *XWatchService {
	return &XWatchService{
		watches: watches,
		x:       x,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled reports whether watch persistence is available.
func (s *XWatchService) Enabled() bool { return s != nil && s.watches != nil }

func (s *XWatchService) ensure() error {
	if !s.Enabled() {
		return ErrWatchesDisabled
	}
	return nil
}

// SaveWatch upserts a watch (keyed by tenant+label). Pointer fields on in are
// applied only when non-nil so an agent can patch one attribute at a time.
func (s *XWatchService) SaveWatch(ctx context.Context, in port.XWatchSaveInput) (domain.XWatch, error) {
	if err := s.ensure(); err != nil {
		return domain.XWatch{}, err
	}
	if in.Label == "" {
		return domain.XWatch{}, fmt.Errorf("%w: label required", domain.ErrValidation)
	}
	return s.watches.Save(ctx, in)
}

// ListWatches returns all watches for the current tenant.
func (s *XWatchService) ListWatches(ctx context.Context) ([]domain.XWatch, error) {
	if err := s.ensure(); err != nil {
		return nil, err
	}
	return s.watches.List(ctx)
}

// DeleteWatch soft-deletes a watch by label.
func (s *XWatchService) DeleteWatch(ctx context.Context, label string) error {
	if err := s.ensure(); err != nil {
		return err
	}
	return s.watches.Delete(ctx, label)
}

// ListEvents returns matched events for a watch, optionally filtered by
// delivery status ("" = all). Newest first.
func (s *XWatchService) ListEvents(ctx context.Context, label, delivery string, limit int) ([]domain.XWatchEvent, error) {
	if err := s.ensure(); err != nil {
		return nil, err
	}
	w, err := s.watches.GetByLabel(ctx, label)
	if err != nil {
		return nil, err
	}
	return s.watches.ListEvents(ctx, w.ID, delivery, limit)
}

// RunWatches polls all active watches (across tenants) up to limit, persists
// new matched tweets, and delivers them to each watch's webhook. Intended to
// be called on a ticker. It never returns an error for a single watch failure;
// per-watch errors are recorded on the watch/event rows so an agent can
// inspect them.
func (s *XWatchService) RunWatches(ctx context.Context, limit int) (polled, newEvents int) {
	if err := s.ensure(); err != nil {
		return 0, 0
	}
	due, err := s.watches.ListDue(ctx, limit)
	if err != nil {
		return 0, 0
	}
	for _, dw := range due {
		tctx := tenant.With(ctx, dw.TenantID)
		n := s.runOne(tctx, dw.Watch)
		polled++
		newEvents += n
	}
	return polled, newEvents
}

// runOne polls a single watch and returns the count of newly stored events.
func (s *XWatchService) runOne(ctx context.Context, w domain.XWatch) int {
	cookies, err := s.x.CookiesForLabel(ctx, w.AccountLabel)
	if err != nil {
		_ = s.watches.UpdatePollState(ctx, w.ID, "", "resolve account: "+err.Error())
		return 0
	}

	var page domain.XTweetPage
	switch w.Kind {
	case domain.XWatchSearch:
		page, err = s.x.Search(ctx, cookies, port.XSearchInput{Query: w.Query, Product: "Latest", Count: 40})
	default: // mention
		page, err = s.x.gateway.Mentions(ctx, cookies, w.Query, 40)
	}
	if err != nil {
		_ = s.watches.UpdatePollState(ctx, w.ID, "", "poll: "+err.Error())
		return 0
	}

	// Tweets come newest-first. Store all unseen; track the max id as the new
	// high-water mark. Dedup is enforced by the unique (tenant,watch,tweet) key.
	var newest string
	stored := 0
	for _, t := range page.Tweets {
		if idNewer(t.RestID, newest) {
			newest = t.RestID
		}
		// Skip anything not strictly newer than the last high-water mark to
		// avoid re-emitting old tweets on the first poll after a restart.
		if w.LastSeenID != "" && !idNewer(t.RestID, w.LastSeenID) {
			continue
		}
		ev := domain.XWatchEvent{
			TweetID:        t.RestID,
			Author:         t.ScreenName,
			Text:           t.Text,
			URL:            t.URL,
			Likes:          t.Likes,
			Retweets:       t.Retweets,
			Replies:        t.Replies,
			TweetCreatedAt: t.CreatedAt,
			Delivery:       domain.XDeliveryPending,
		}
		inserted, id, err := s.watches.AddEvent(ctx, w.ID, ev)
		if err != nil || !inserted {
			continue
		}
		stored++
		ev.ID = id
		s.deliver(ctx, w, ev)
	}
	_ = s.watches.UpdatePollState(ctx, w.ID, newest, "")
	return stored
}

// deliver posts the event to the watch's webhook (if any) with an HMAC-SHA256
// signature over the JSON body, and records the delivery status.
func (s *XWatchService) deliver(ctx context.Context, w domain.XWatch, ev domain.XWatchEvent) {
	if !w.HasWebhook() {
		_ = s.watches.MarkDelivered(ctx, ev.ID, domain.XDeliverySkipped, "")
		return
	}
	body, err := json.Marshal(watchWebhookPayload{
		WatchLabel: w.Label,
		Kind:       string(w.Kind),
		Query:      w.Query,
		Account:    w.AccountLabel,
		Event:      ev,
	})
	if err != nil {
		_ = s.watches.MarkDelivered(ctx, ev.ID, domain.XDeliveryFailed, "marshal: "+err.Error())
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.WebhookURL, bytes.NewReader(body))
	if err != nil {
		_ = s.watches.MarkDelivered(ctx, ev.ID, domain.XDeliveryFailed, err.Error())
		return
	}
	// Custom headers first so built-ins (content-type, signature) always win
	// and can't be clobbered by the agent's config.
	for k, v := range w.WebhookHeaders {
		req.Header.Set(k, v)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("user-agent", "zetta-crm-xwatch/1")
	if w.WebhookSecret != "" {
		req.Header.Set("x-zetta-signature", SignWebhook(w.WebhookSecret, body))
	}
	resp, err := s.http.Do(req)
	if err != nil {
		_ = s.watches.MarkDelivered(ctx, ev.ID, domain.XDeliveryFailed, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = s.watches.MarkDelivered(ctx, ev.ID, domain.XDeliveryFailed, "webhook http "+strconv.Itoa(resp.StatusCode))
		return
	}
	_ = s.watches.MarkDelivered(ctx, ev.ID, domain.XDeliveryDelivered, "")
}

// watchWebhookPayload is the JSON body POSTed to a watch webhook.
type watchWebhookPayload struct {
	WatchLabel string             `json:"watch_label"`
	Kind       string             `json:"kind"`
	Query      string             `json:"query"`
	Account    string             `json:"account,omitempty"`
	Event      domain.XWatchEvent `json:"event"`
}

// idNewer reports whether tweet id a is numerically greater than b (x.com
// snowflake ids are monotonic). Non-numeric or shorter ids fall back to string
// comparison. b == "" is treated as "a is newer".
func idNewer(a, b string) bool {
	if b == "" {
		return a != ""
	}
	if len(a) != len(b) {
		return len(a) > len(b)
	}
	return a > b
}

// WebhookSignatureHeader is the HTTP header carrying the HMAC signature on
// watch webhook deliveries.
const WebhookSignatureHeader = "X-Zetta-Signature"

// SignWebhook returns the value for the X-Zetta-Signature header: an
// HMAC-SHA256 of body keyed by secret, hex-encoded and prefixed "sha256=".
// A zetta webhook receiver recomputes this over the raw request body and
// compares it (constant-time) to authenticate the delivery.
func SignWebhook(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature reports whether sig (the X-Zetta-Signature header
// value) is a valid signature of body under secret, using a constant-time
// comparison. Intended for the receiving side (a zetta service) to trust an
// inbound watch delivery.
func VerifyWebhookSignature(secret string, body []byte, sig string) bool {
	if secret == "" || sig == "" {
		return false
	}
	want := SignWebhook(secret, body)
	return hmac.Equal([]byte(want), []byte(sig))
}
