package service

import (
	"context"
	"errors"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
)

// XService is the use-case layer for the cookie-only x.com channel. It is a
// thin pass-through to the gateway: auth is a per-call Netscape cookie blob so
// one service instance serves many accounts. Optional; nil when the channel is
// not wired.
//
// When accounts != nil, accounts can also be persisted (label -> cookie blob)
// and a liveness cron can verify them. The MCP layer resolves either a raw
// cookie blob or a stored account label to the cookies used per call.
type XService struct {
	gateway  port.XGateway
	accounts port.XAccountRepo
}

// NewXService builds the x.com service. accounts may be nil to disable account
// persistence (raw per-call cookies only).
func NewXService(gateway port.XGateway, accounts port.XAccountRepo) *XService {
	return &XService{gateway: gateway, accounts: accounts}
}

// ErrAccountsDisabled is returned by account operations when no repo is wired.
var ErrAccountsDisabled = errors.New("x account persistence not configured")

func (s *XService) User(ctx context.Context, cookies, handle string) (domain.XUser, error) {
	return s.gateway.UserByScreenName(ctx, cookies, handle)
}

// Me returns the acting account's own numeric user id from the session cookies.
func (s *XService) Me(ctx context.Context, cookies string) (string, error) {
	return s.gateway.Me(ctx, cookies)
}

// SelfProfile resolves the acting account's own profile from the session
// cookies (twid -> UserByRestId). No handle needed.
func (s *XService) SelfProfile(ctx context.Context, cookies string) (domain.XUser, error) {
	return s.gateway.SelfProfile(ctx, cookies)
}

func (s *XService) Post(ctx context.Context, cookies string, in port.XPostInput) (domain.XPostResult, error) {
	return s.gateway.Post(ctx, cookies, in)
}

func (s *XService) Delete(ctx context.Context, cookies, tweetID string) error {
	return s.gateway.Delete(ctx, cookies, tweetID)
}

func (s *XService) Search(ctx context.Context, cookies string, in port.XSearchInput) (domain.XTweetPage, error) {
	return s.gateway.Search(ctx, cookies, in)
}

func (s *XService) UserTweets(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XTweetPage, error) {
	return s.gateway.UserTweets(ctx, cookies, userID, count, cursor)
}

func (s *XService) TweetDetail(ctx context.Context, cookies, tweetID string) (domain.XTweetDetail, error) {
	return s.gateway.TweetDetail(ctx, cookies, tweetID)
}

func (s *XService) TweetReplies(ctx context.Context, cookies, tweetID string, count int, cursor string) (domain.XTweetPage, error) {
	return s.gateway.TweetReplies(ctx, cookies, tweetID, count, cursor)
}

func (s *XService) Followers(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XUserPage, error) {
	return s.gateway.Followers(ctx, cookies, userID, count, cursor)
}

func (s *XService) Following(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XUserPage, error) {
	return s.gateway.Following(ctx, cookies, userID, count, cursor)
}

func (s *XService) SendDM(ctx context.Context, cookies string, in port.XDMInput) (domain.XDMResult, error) {
	return s.gateway.SendDM(ctx, cookies, in)
}

// --- Account persistence ---------------------------------------------------

// SaveAccount persists (upserts) an account by label and verifies its cookies
// immediately, recording liveness. Returns the stored account.
func (s *XService) SaveAccount(ctx context.Context, label, cookies string) (domain.XAccount, error) {
	if s.accounts == nil {
		return domain.XAccount{}, ErrAccountsDisabled
	}
	acct, err := s.accounts.Save(ctx, port.XAccountSaveInput{Label: label, Cookies: cookies})
	if err != nil {
		return domain.XAccount{}, err
	}
	// Best-effort immediate liveness probe so the saved row is not left
	// "unknown" until the next cron sweep.
	s.probe(ctx, acct)
	return s.accounts.GetByLabel(ctx, label)
}

// ListAccounts returns stored accounts for the current tenant.
func (s *XService) ListAccounts(ctx context.Context) ([]domain.XAccount, error) {
	if s.accounts == nil {
		return nil, ErrAccountsDisabled
	}
	return s.accounts.List(ctx)
}

// DeleteAccount soft-deletes a stored account by label.
func (s *XService) DeleteAccount(ctx context.Context, label string) error {
	if s.accounts == nil {
		return ErrAccountsDisabled
	}
	return s.accounts.Delete(ctx, label)
}

// Account returns a stored account by label (including liveness metadata).
func (s *XService) Account(ctx context.Context, label string) (domain.XAccount, error) {
	if s.accounts == nil {
		return domain.XAccount{}, ErrAccountsDisabled
	}
	return s.accounts.GetByLabel(ctx, label)
}

// CookiesForLabel resolves a stored account label to its cookie blob so the
// MCP tools can act as a saved account without the caller re-sending cookies.
func (s *XService) CookiesForLabel(ctx context.Context, label string) (string, error) {
	if s.accounts == nil {
		return "", ErrAccountsDisabled
	}
	acct, err := s.accounts.GetByLabel(ctx, label)
	if err != nil {
		return "", err
	}
	return acct.Cookies, nil
}

// CheckLiveness runs one cross-tenant sweep of accounts not checked since
// cutoff, verifying each account's cookies and recording live/dead. Returns
// the number of accounts checked. Intended to be driven by a cron loop.
func (s *XService) CheckLiveness(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	if s.accounts == nil {
		return 0, ErrAccountsDisabled
	}
	stale, err := s.accounts.ListStale(ctx, cutoff, limit)
	if err != nil {
		return 0, err
	}
	for _, item := range stale {
		// ListStale is cross-tenant; scope each write to its owning tenant.
		cctx := tenant.With(ctx, item.TenantID)
		s.probe(cctx, item.Account)
	}
	return len(stale), nil
}

// probe verifies an account's cookies via SelfProfile (twid -> UserByRestId)
// and records the resulting liveness. This reflects real functional access and
// needs no stored screen_name. Errors are captured as dead + last_error.
func (s *XService) probe(ctx context.Context, acct domain.XAccount) {
	var screenName, userID, lastErr string
	liveness := domain.XLivenessDead
	u, err := s.gateway.SelfProfile(ctx, acct.Cookies)
	if err != nil {
		lastErr = err.Error()
	} else {
		liveness = domain.XLivenessLive
		screenName, userID = u.ScreenName, u.RestID
	}
	_ = s.accounts.UpdateLiveness(ctx, acct.ID, liveness, screenName, userID, lastErr)
}
