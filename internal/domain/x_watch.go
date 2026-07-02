package domain

import "time"

// XWatchKind selects what a watch polls for.
type XWatchKind string

const (
	// XWatchMention polls for tweets mentioning the watch's own handle
	// (query is the @handle without the @). Reuses x.com search Latest.
	XWatchMention XWatchKind = "mention"
	// XWatchSearch polls an arbitrary search query (raw x.com search
	// operators allowed, e.g. `"my brand" -filter:replies`).
	XWatchSearch XWatchKind = "search"
)

// XWatchDelivery is the delivery status of a watch event.
type XWatchDelivery string

const (
	XDeliveryPending   XWatchDelivery = "pending"   // not yet delivered
	XDeliveryDelivered XWatchDelivery = "delivered" // webhook 2xx
	XDeliveryFailed    XWatchDelivery = "failed"    // webhook error (retryable)
	XDeliverySkipped   XWatchDelivery = "skipped"   // no webhook configured
)

// XWatch is a persisted watch on x.com signals for one account. An AI agent
// creates/updates these to drive auto-reply or external hooks. WebhookSecret
// is a sensitive shared key used to HMAC-sign deliveries; never echo it in
// tool output.
type XWatch struct {
	ID            int64      `json:"id"`
	Label         string     `json:"label"`
	Kind          XWatchKind `json:"kind"`
	Query         string     `json:"query"`
	AccountLabel  string     `json:"account_label,omitempty"`
	WebhookURL    string     `json:"webhook_url,omitempty"`
	WebhookSecret string     `json:"-"`
	// WebhookHeaders are extra HTTP headers sent with each delivery (e.g.
	// Authorization or a bearer token for the receiver). Set by the agent;
	// merged over the built-in content-type/user-agent/signature headers.
	WebhookHeaders map[string]string `json:"webhook_headers,omitempty"`
	Active         bool              `json:"active"`
	LastSeenID     string            `json:"last_seen_id,omitempty"`
	LastPolledAt   *time.Time        `json:"last_polled_at,omitempty"`
	LastError      string            `json:"last_error,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// HasWebhook reports whether a delivery target is configured.
func (w XWatch) HasWebhook() bool { return w.WebhookURL != "" }

// XWatchEvent is a single matched tweet queued/delivered for a watch.
type XWatchEvent struct {
	ID             int64          `json:"id"`
	WatchID        int64          `json:"watch_id"`
	TweetID        string         `json:"tweet_id"`
	Author         string         `json:"author"`
	Text           string         `json:"text"`
	URL            string         `json:"url,omitempty"`
	Likes          int            `json:"likes"`
	Retweets       int            `json:"retweets"`
	Replies        int            `json:"replies"`
	TweetCreatedAt string         `json:"tweet_created_at,omitempty"`
	Delivery       XWatchDelivery `json:"delivery"`
	DeliveryError  string         `json:"delivery_error,omitempty"`
	DeliveredAt    *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}
