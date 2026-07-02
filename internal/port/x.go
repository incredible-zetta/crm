package port

import (
	"context"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
)

// XGateway is the cookie-only x.com (Twitter) gateway backed by the native
// GraphQL client ported from x-utils. Auth is a per-call Netscape cookie blob
// (auth_token + ct0), so one gateway instance serves many accounts: the caller
// passes the cookies of the account to act as on every request.
type XGateway interface {
	UserByScreenName(ctx context.Context, cookies, handle string) (domain.XUser, error)
	Post(ctx context.Context, cookies string, in XPostInput) (domain.XPostResult, error)
	Delete(ctx context.Context, cookies, tweetID string) error
	Search(ctx context.Context, cookies string, in XSearchInput) (domain.XTweetPage, error)
	UserTweets(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XTweetPage, error)
	TweetDetail(ctx context.Context, cookies, tweetID string) (domain.XTweetDetail, error)
	TweetReplies(ctx context.Context, cookies, tweetID string, count int, cursor string) (domain.XTweetPage, error)
	// Mentions returns recent tweets mentioning handle (newest first), backed
	// by search Latest. Used by watch polling.
	Mentions(ctx context.Context, cookies, handle string, count int) (domain.XTweetPage, error)
	Followers(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XUserPage, error)
	Following(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XUserPage, error)
	SendDM(ctx context.Context, cookies string, in XDMInput) (domain.XDMResult, error)
}

// XAccountRepo persists x.com accounts (cookie blobs + liveness). Save is an
// upsert keyed by (tenant, label). ListForLiveness runs cross-tenant for the
// liveness cron, so it returns each account's TenantID via ctx-free rows —
// callers scope follow-up writes with tenant.With.
type XAccountRepo interface {
	Save(ctx context.Context, in XAccountSaveInput) (domain.XAccount, error)
	List(ctx context.Context) ([]domain.XAccount, error)
	GetByLabel(ctx context.Context, label string) (domain.XAccount, error)
	Delete(ctx context.Context, label string) error
	UpdateLiveness(ctx context.Context, id int64, liveness domain.XLiveness, screenName, userID, lastErr string) error
	// ListStale returns accounts (across all tenants) whose liveness has not
	// been checked since before cutoff, each paired with its tenant id.
	ListStale(ctx context.Context, cutoff time.Time, limit int) ([]XAccountWithTenant, error)
}

// XAccountSaveInput is the upsert payload for a stored account.
type XAccountSaveInput struct {
	Label      string
	Cookies    string
	ScreenName string
	UserID     string
}

// XAccountWithTenant pairs an account with its owning tenant for cross-tenant
// cron scans (ctx carries no tenant during the liveness sweep).
type XAccountWithTenant struct {
	TenantID string
	Account  domain.XAccount
}

// XWatchWithTenant pairs a watch with its owning tenant for the cross-tenant
// watch poller (ctx carries no tenant during the sweep).
type XWatchWithTenant struct {
	TenantID string
	Watch    domain.XWatch
}

// XWatchSaveInput is the upsert payload for a watch (keyed by tenant+label).
// Pointer fields are only applied when non-nil so an AI agent can patch a
// single attribute (e.g. rotate the webhook secret) without clobbering others.
type XWatchSaveInput struct {
	Label          string
	Kind           *domain.XWatchKind
	Query          *string
	AccountLabel   *string
	WebhookURL     *string
	WebhookSecret  *string
	WebhookHeaders *map[string]string
	Active         *bool
}

// XWatchRepo persists watches and their matched events. Save upserts by
// (tenant, label). ListDue runs cross-tenant for the poller. AddEvent is an
// idempotent insert deduped by (tenant, watch, tweet); it reports whether the
// row was newly inserted so the poller only delivers fresh matches.
type XWatchRepo interface {
	Save(ctx context.Context, in XWatchSaveInput) (domain.XWatch, error)
	List(ctx context.Context) ([]domain.XWatch, error)
	GetByLabel(ctx context.Context, label string) (domain.XWatch, error)
	Delete(ctx context.Context, label string) error
	ListDue(ctx context.Context, limit int) ([]XWatchWithTenant, error)
	UpdatePollState(ctx context.Context, id int64, lastSeenID, lastErr string) error
	AddEvent(ctx context.Context, watchID int64, ev domain.XWatchEvent) (inserted bool, id int64, err error)
	MarkDelivered(ctx context.Context, eventID int64, status domain.XWatchDelivery, deliverErr string) error
	ListEvents(ctx context.Context, watchID int64, delivery string, limit int) ([]domain.XWatchEvent, error)
}

// XPostInput configures a tweet/reply/quote. ReplyTo and QuoteOf are mutually
// exclusive. MediaURL, when set, is downloaded and uploaded before posting.
type XPostInput struct {
	Text     string
	ReplyTo  string
	QuoteOf  string
	MediaURL string
}

// XSearchInput configures a SearchTimeline fetch. Product is Top/Latest/People/
// Media (defaults Top).
type XSearchInput struct {
	Query   string
	Product string
	Count   int
	Cursor  string
}

// XDMInput configures a direct message. Exactly one of RecipientID or Handle
// identifies the recipient; Handle is resolved to an id when set.
type XDMInput struct {
	RecipientID string
	Handle      string
	Text        string
	MediaURL    string
}
