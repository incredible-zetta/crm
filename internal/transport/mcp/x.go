package mcptransport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The x_* tools are a cookie-only x.com (Twitter) channel ported natively from
// x-utils. Auth is per-call: every tool takes a `cookies` field holding a
// Netscape cookie-file blob (auth_token + ct0 + ...) for the account to act as.
// This makes multi-account trivial — the caller supplies whichever account's
// cookies it wants on each request. No access token, no server-side account.

// XCookiesIn is embedded by every x_* input: the per-call account cookies OR a
// stored account label. Exactly one is needed. Account resolves to a persisted
// cookie blob; cookies is a raw Netscape blob for ad-hoc / unsaved accounts.
type XCookiesIn struct {
	Cookies string `json:"cookies,omitempty" jsonschema:"Netscape cookie-file blob for the x.com account to act as (must include auth_token and ct0). Sensitive. Omit when using account."`
	Account string `json:"account,omitempty" jsonschema:"Label of a stored account (see x_account_save/x_account_list) to act as, instead of passing cookies inline."`
}

// resolveCookies returns the cookie blob to use: a stored account label takes
// precedence, else the inline blob. Returns an error envelope when neither is
// usable or the label is unknown.
func (d *Deps) resolveCookies(ctx context.Context, in XCookiesIn) (string, *mcp.CallToolResult) {
	if in.Account != "" {
		ck, err := d.Svc.X.CookiesForLabel(ctx, in.Account)
		if err != nil {
			return "", mcpserver.Err("not_found", fmt.Sprintf("account %q: %v", in.Account, err))
		}
		return ck, nil
	}
	if in.Cookies == "" {
		return "", mcpserver.Err("validation", "cookies or account required")
	}
	return in.Cookies, nil
}

func (d *Deps) xReady() (*mcp.CallToolResult, bool) {
	if d.Svc.X == nil {
		return mcpserver.Err("disabled", "x channel not configured"), false
	}
	return nil, true
}

// --- Shared output DTOs -----------------------------------------------------

type XUserOut struct {
	RestID       string `json:"rest_id"`
	Name         string `json:"name,omitempty"`
	ScreenName   string `json:"screen_name"`
	CreatedAt    string `json:"created_at,omitempty"`
	Followers    int    `json:"followers"`
	Following    int    `json:"following"`
	Tweets       int    `json:"tweets"`
	Verified     bool   `json:"verified"`
	BlueVerified bool   `json:"blue_verified"`
}

type XTweetOut struct {
	RestID     string `json:"rest_id"`
	Text       string `json:"text"`
	ScreenName string `json:"screen_name"`
	Name       string `json:"name,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	Likes      int    `json:"likes"`
	Retweets   int    `json:"retweets"`
	Replies    int    `json:"replies"`
	HasMedia   bool   `json:"has_media"`
	URL        string `json:"url"`
}

func toXUserOut(u domain.XUser) XUserOut {
	return XUserOut{
		RestID: u.RestID, Name: u.Name, ScreenName: u.ScreenName, CreatedAt: u.CreatedAt,
		Followers: u.Followers, Following: u.Following, Tweets: u.Tweets,
		Verified: u.Verified, BlueVerified: u.BlueVerified,
	}
}

func toXTweetOut(t domain.XTweet) XTweetOut {
	return XTweetOut{
		RestID: t.RestID, Text: t.Text, ScreenName: t.ScreenName, Name: t.Name,
		CreatedAt: t.CreatedAt, Likes: t.Likes, Retweets: t.Retweets, Replies: t.Replies,
		HasMedia: t.HasMedia, URL: t.URL,
	}
}

func toXTweetOuts(items []domain.XTweet) []XTweetOut {
	out := make([]XTweetOut, 0, len(items))
	for _, t := range items {
		out = append(out, toXTweetOut(t))
	}
	return out
}

// --- x_user -----------------------------------------------------------------

type XUserIn struct {
	XCookiesIn
	Handle string `json:"handle" jsonschema:"Screen name / @handle to look up (with or without @)"`
}

func (d *Deps) XUser(ctx context.Context, req *mcp.CallToolRequest, in XUserIn) (*mcp.CallToolResult, XUserOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XUserOut{}, nil
	}
	if in.Handle == "" {
		return mcpserver.Err("validation", "handle required"), XUserOut{}, nil
	}
	cookies, errRes := d.resolveCookies(ctx, in.XCookiesIn)
	if errRes != nil {
		return errRes, XUserOut{}, nil
	}
	u, err := d.Svc.X.User(ctx, cookies, in.Handle)
	if err != nil {
		return nil, XUserOut{}, fmt.Errorf("x_user: %w", err)
	}
	return nil, toXUserOut(u), nil
}

// --- x_post -----------------------------------------------------------------

type XPostIn struct {
	XCookiesIn
	Text     string `json:"text" jsonschema:"Tweet text; include @mentions and #hashtags inline"`
	ReplyTo  string `json:"reply_to,omitempty" jsonschema:"Optional tweet id to reply to (mutually exclusive with quote)"`
	Quote    string `json:"quote,omitempty" jsonschema:"Optional tweet id or URL to quote (mutually exclusive with reply_to)"`
	MediaURL string `json:"media_url,omitempty" jsonschema:"Optional image/video URL to download and attach"`
}

type XPostOut struct {
	RestID     string `json:"rest_id"`
	ScreenName string `json:"screen_name,omitempty"`
	URL        string `json:"url,omitempty"`
}

func (d *Deps) XPost(ctx context.Context, req *mcp.CallToolRequest, in XPostIn) (*mcp.CallToolResult, XPostOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XPostOut{}, nil
	}
	if in.Text == "" && in.MediaURL == "" {
		return mcpserver.Err("validation", "text or media_url required"), XPostOut{}, nil
	}
	if in.ReplyTo != "" && in.Quote != "" {
		return mcpserver.Err("validation", "reply_to and quote are mutually exclusive"), XPostOut{}, nil
	}
	cookies, errRes := d.resolveCookies(ctx, in.XCookiesIn)
	if errRes != nil {
		return errRes, XPostOut{}, nil
	}
	res, err := d.Svc.X.Post(ctx, cookies, port.XPostInput{
		Text: in.Text, ReplyTo: in.ReplyTo, QuoteOf: in.Quote, MediaURL: in.MediaURL,
	})
	if err != nil {
		return nil, XPostOut{}, fmt.Errorf("x_post: %w", err)
	}
	return nil, XPostOut{RestID: res.RestID, ScreenName: res.ScreenName, URL: res.URL}, nil
}

// --- x_delete ---------------------------------------------------------------

type XDeleteIn struct {
	XCookiesIn
	TweetID string `json:"tweet_id" jsonschema:"Tweet id to delete (must be owned by the acting account)"`
}

type XOKOut struct {
	OK bool `json:"ok"`
}

func (d *Deps) XDelete(ctx context.Context, req *mcp.CallToolRequest, in XDeleteIn) (*mcp.CallToolResult, XOKOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XOKOut{}, nil
	}
	if in.TweetID == "" {
		return mcpserver.Err("validation", "tweet_id required"), XOKOut{}, nil
	}
	cookies, errRes := d.resolveCookies(ctx, in.XCookiesIn)
	if errRes != nil {
		return errRes, XOKOut{}, nil
	}
	if err := d.Svc.X.Delete(ctx, cookies, in.TweetID); err != nil {
		return nil, XOKOut{}, fmt.Errorf("x_delete: %w", err)
	}
	return nil, XOKOut{OK: true}, nil
}

// --- x_search ---------------------------------------------------------------

type XSearchIn struct {
	XCookiesIn
	Query   string `json:"query" jsonschema:"Search query; supports #hashtag $cashtag @mention and operators like min_faves:"`
	Product string `json:"product,omitempty" jsonschema:"Tab: Top (default), Latest, People, or Media"`
	Count   int    `json:"count,omitempty" jsonschema:"Results per page (default 20)"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"Pagination cursor"`
}

type XTweetPageOut struct {
	Tweets     []XTweetOut `json:"tweets"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

func (d *Deps) XSearch(ctx context.Context, req *mcp.CallToolRequest, in XSearchIn) (*mcp.CallToolResult, XTweetPageOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XTweetPageOut{}, nil
	}
	if in.Query == "" {
		return mcpserver.Err("validation", "query required"), XTweetPageOut{}, nil
	}
	cookies, errRes := d.resolveCookies(ctx, in.XCookiesIn)
	if errRes != nil {
		return errRes, XTweetPageOut{}, nil
	}
	page, err := d.Svc.X.Search(ctx, cookies, port.XSearchInput{
		Query: in.Query, Product: in.Product, Count: in.Count, Cursor: in.Cursor,
	})
	if err != nil {
		return nil, XTweetPageOut{}, fmt.Errorf("x_search: %w", err)
	}
	return nil, XTweetPageOut{Tweets: toXTweetOuts(page.Tweets), NextCursor: page.NextCursor}, nil
}

// --- x_replies --------------------------------------------------------------

type XRepliesIn struct {
	XCookiesIn
	TweetID string `json:"tweet_id" jsonschema:"Numeric id of the tweet whose replies to fetch"`
	Count   int    `json:"count,omitempty" jsonschema:"Replies per page (default 20)"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"Pagination cursor from a previous next_cursor"`
}

func (d *Deps) XReplies(ctx context.Context, req *mcp.CallToolRequest, in XRepliesIn) (*mcp.CallToolResult, XTweetPageOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XTweetPageOut{}, nil
	}
	if in.TweetID == "" {
		return mcpserver.Err("validation", "tweet_id required"), XTweetPageOut{}, nil
	}
	cookies, errRes := d.resolveCookies(ctx, in.XCookiesIn)
	if errRes != nil {
		return errRes, XTweetPageOut{}, nil
	}
	page, err := d.Svc.X.TweetReplies(ctx, cookies, in.TweetID, in.Count, in.Cursor)
	if err != nil {
		return nil, XTweetPageOut{}, fmt.Errorf("x_replies: %w", err)
	}
	return nil, XTweetPageOut{Tweets: toXTweetOuts(page.Tweets), NextCursor: page.NextCursor}, nil
}

type XUserTweetsIn struct {
	XCookiesIn
	UserID string `json:"user_id,omitempty" jsonschema:"Numeric user id (rest_id). Provide this or handle."`
	Handle string `json:"handle,omitempty" jsonschema:"@handle to resolve to a user id when user_id is not given."`
	Count  int    `json:"count,omitempty" jsonschema:"Tweets per page (default 20)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"Pagination cursor"`
}

func (d *Deps) XUserTweets(ctx context.Context, req *mcp.CallToolRequest, in XUserTweetsIn) (*mcp.CallToolResult, XTweetPageOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XTweetPageOut{}, nil
	}
	cookies, errRes := d.resolveCookies(ctx, in.XCookiesIn)
	if errRes != nil {
		return errRes, XTweetPageOut{}, nil
	}
	userID, errRes := d.resolveXUserID(ctx, cookies, in.UserID, in.Handle)
	if errRes != nil {
		return errRes, XTweetPageOut{}, nil
	}
	page, err := d.Svc.X.UserTweets(ctx, cookies, userID, in.Count, in.Cursor)
	if err != nil {
		return nil, XTweetPageOut{}, fmt.Errorf("x_user_tweets: %w", err)
	}
	return nil, XTweetPageOut{Tweets: toXTweetOuts(page.Tweets), NextCursor: page.NextCursor}, nil
}

// --- x_tweet (detail) -------------------------------------------------------

type XTweetDetailIn struct {
	XCookiesIn
	TweetID string `json:"tweet_id" jsonschema:"Tweet id to fetch detail + engagement analytics for"`
}

type XTweetDetailOut struct {
	XTweetOut
	Views          int    `json:"views"`
	ViewState      string `json:"view_state,omitempty"`
	Quotes         int    `json:"quotes"`
	Bookmarks      int    `json:"bookmarks"`
	ConversationID string `json:"conversation_id,omitempty"`
}

func (d *Deps) XTweet(ctx context.Context, req *mcp.CallToolRequest, in XTweetDetailIn) (*mcp.CallToolResult, XTweetDetailOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XTweetDetailOut{}, nil
	}
	if in.TweetID == "" {
		return mcpserver.Err("validation", "tweet_id required"), XTweetDetailOut{}, nil
	}
	t, err := d.Svc.X.TweetDetail(ctx, in.Cookies, in.TweetID)
	if err != nil {
		return nil, XTweetDetailOut{}, fmt.Errorf("x_tweet: %w", err)
	}
	return nil, XTweetDetailOut{
		XTweetOut:      toXTweetOut(t.XTweet),
		Views:          t.Views,
		ViewState:      t.ViewState,
		Quotes:         t.Quotes,
		Bookmarks:      t.Bookmarks,
		ConversationID: t.ConversationID,
	}, nil
}

// --- x_followers / x_following ----------------------------------------------

type XSocialIn struct {
	XCookiesIn
	UserID string `json:"user_id,omitempty" jsonschema:"Numeric user id (rest_id). Provide this or handle."`
	Handle string `json:"handle,omitempty" jsonschema:"@handle to resolve to a user id when user_id is not given."`
	Count  int    `json:"count,omitempty" jsonschema:"Users per page (default 20)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"Pagination cursor"`
}

type XUserPageOut struct {
	Users      []XUserOut `json:"users"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

func toXUserPageOut(page domain.XUserPage) XUserPageOut {
	out := XUserPageOut{NextCursor: page.NextCursor, Users: make([]XUserOut, 0, len(page.Users))}
	for _, u := range page.Users {
		out.Users = append(out.Users, toXUserOut(u))
	}
	return out
}

func (d *Deps) XFollowers(ctx context.Context, req *mcp.CallToolRequest, in XSocialIn) (*mcp.CallToolResult, XUserPageOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XUserPageOut{}, nil
	}
	cookies, errRes := d.resolveCookies(ctx, in.XCookiesIn)
	if errRes != nil {
		return errRes, XUserPageOut{}, nil
	}
	userID, errRes := d.resolveXUserID(ctx, cookies, in.UserID, in.Handle)
	if errRes != nil {
		return errRes, XUserPageOut{}, nil
	}
	page, err := d.Svc.X.Followers(ctx, cookies, userID, in.Count, in.Cursor)
	if err != nil {
		return nil, XUserPageOut{}, fmt.Errorf("x_followers: %w", err)
	}
	return nil, toXUserPageOut(page), nil
}

func (d *Deps) XFollowing(ctx context.Context, req *mcp.CallToolRequest, in XSocialIn) (*mcp.CallToolResult, XUserPageOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XUserPageOut{}, nil
	}
	cookies, errRes := d.resolveCookies(ctx, in.XCookiesIn)
	if errRes != nil {
		return errRes, XUserPageOut{}, nil
	}
	userID, errRes := d.resolveXUserID(ctx, cookies, in.UserID, in.Handle)
	if errRes != nil {
		return errRes, XUserPageOut{}, nil
	}
	page, err := d.Svc.X.Following(ctx, cookies, userID, in.Count, in.Cursor)
	if err != nil {
		return nil, XUserPageOut{}, fmt.Errorf("x_following: %w", err)
	}
	return nil, toXUserPageOut(page), nil
}

// --- x_dm -------------------------------------------------------------------

type XDMIn struct {
	XCookiesIn
	RecipientID string `json:"recipient_id,omitempty" jsonschema:"Numeric recipient user id. Provide this or handle."`
	Handle      string `json:"handle,omitempty" jsonschema:"@handle to resolve to a recipient id when recipient_id is not given."`
	Text        string `json:"text,omitempty" jsonschema:"Message text (text or media_url required)"`
	MediaURL    string `json:"media_url,omitempty" jsonschema:"Optional image URL to download and attach"`
}

type XDMOut struct {
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
}

func (d *Deps) XDM(ctx context.Context, req *mcp.CallToolRequest, in XDMIn) (*mcp.CallToolResult, XDMOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XDMOut{}, nil
	}
	if in.RecipientID == "" && in.Handle == "" {
		return mcpserver.Err("validation", "recipient_id or handle required"), XDMOut{}, nil
	}
	if in.Text == "" && in.MediaURL == "" {
		return mcpserver.Err("validation", "text or media_url required"), XDMOut{}, nil
	}
	cookies, errRes := d.resolveCookies(ctx, in.XCookiesIn)
	if errRes != nil {
		return errRes, XDMOut{}, nil
	}
	res, err := d.Svc.X.SendDM(ctx, cookies, port.XDMInput{
		RecipientID: in.RecipientID, Handle: in.Handle, Text: in.Text, MediaURL: in.MediaURL,
	})
	if err != nil {
		return nil, XDMOut{}, fmt.Errorf("x_dm: %w", err)
	}
	return nil, XDMOut{MessageID: res.MessageID, ConversationID: res.ConversationID}, nil
}

// resolveXUserID returns userID directly when set, else resolves handle to a
// numeric id. Returns an error envelope when neither is usable.
func (d *Deps) resolveXUserID(ctx context.Context, cookies, userID, handle string) (string, *mcp.CallToolResult) {
	if userID != "" {
		return userID, nil
	}
	if handle == "" {
		return "", mcpserver.Err("validation", "user_id or handle required")
	}
	u, err := d.Svc.X.User(ctx, cookies, handle)
	if err != nil {
		return "", mcpserver.Err("upstream", fmt.Sprintf("resolve handle: %v", err))
	}
	if u.RestID == "" {
		return "", mcpserver.Err("not_found", "handle not found")
	}
	return u.RestID, nil
}

// --- x_account_save ---------------------------------------------------------

type XAccountSaveIn struct {
	Label   string `json:"label" jsonschema:"Unique label to reference this account by in later x_* calls (via the account field)"`
	Cookies string `json:"cookies" jsonschema:"Netscape cookie-file blob (auth_token + ct0) for the account. Sensitive; stored server-side."`
}

type XAccountOut struct {
	ID            int64  `json:"id"`
	Label         string `json:"label"`
	ScreenName    string `json:"screen_name,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	Liveness      string `json:"liveness"`
	LastCheckedAt string `json:"last_checked_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
}

func toXAccountOut(a domain.XAccount) XAccountOut {
	out := XAccountOut{
		ID: a.ID, Label: a.Label, ScreenName: a.ScreenName, UserID: a.UserID,
		Liveness: string(a.Liveness), LastError: a.LastError,
	}
	if a.LastCheckedAt != nil {
		out.LastCheckedAt = a.LastCheckedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func (d *Deps) XAccountSave(ctx context.Context, req *mcp.CallToolRequest, in XAccountSaveIn) (*mcp.CallToolResult, XAccountOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XAccountOut{}, nil
	}
	if in.Label == "" || in.Cookies == "" {
		return mcpserver.Err("validation", "label and cookies required"), XAccountOut{}, nil
	}
	acct, err := d.Svc.X.SaveAccount(ctx, in.Label, in.Cookies)
	if err != nil {
		return nil, XAccountOut{}, fmt.Errorf("x_account_save: %w", err)
	}
	return nil, toXAccountOut(acct), nil
}

// --- x_account_list ---------------------------------------------------------

type XAccountListOut struct {
	Accounts []XAccountOut `json:"accounts"`
}

func (d *Deps) XAccountList(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, XAccountListOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XAccountListOut{}, nil
	}
	accts, err := d.Svc.X.ListAccounts(ctx)
	if err != nil {
		return nil, XAccountListOut{}, fmt.Errorf("x_account_list: %w", err)
	}
	out := XAccountListOut{Accounts: make([]XAccountOut, 0, len(accts))}
	for _, a := range accts {
		out.Accounts = append(out.Accounts, toXAccountOut(a))
	}
	return nil, out, nil
}

// --- x_account_delete -------------------------------------------------------

type XAccountDeleteIn struct {
	Label string `json:"label" jsonschema:"Label of the stored account to delete"`
}

func (d *Deps) XAccountDelete(ctx context.Context, req *mcp.CallToolRequest, in XAccountDeleteIn) (*mcp.CallToolResult, XOKOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XOKOut{}, nil
	}
	if in.Label == "" {
		return mcpserver.Err("validation", "label required"), XOKOut{}, nil
	}
	if err := d.Svc.X.DeleteAccount(ctx, in.Label); err != nil {
		return nil, XOKOut{}, fmt.Errorf("x_account_delete: %w", err)
	}
	return nil, XOKOut{OK: true}, nil
}

// --- x_watch_save -----------------------------------------------------------

type XWatchSaveIn struct {
	Label          string             `json:"label" jsonschema:"Unique label for this watch (upsert key)"`
	Kind           *string            `json:"kind,omitempty" jsonschema:"What to watch: 'mention' (tweets mentioning your handle; query = handle without @) or 'search' (raw x.com search query). Defaults to mention."`
	Query          *string            `json:"query,omitempty" jsonschema:"For mention: the @handle (without @) to watch mentions of. For search: the x.com search query."`
	Account        *string            `json:"account,omitempty" jsonschema:"Stored account label whose cookies poll this watch (see x_account_save)."`
	WebhookURL     *string            `json:"webhook_url,omitempty" jsonschema:"HTTPS URL to POST each new match to. Body is JSON; if webhook_secret is set an X-Zetta-Signature: sha256=<hmac> header is added."`
	WebhookSecret  *string            `json:"webhook_secret,omitempty" jsonschema:"Shared secret used to HMAC-SHA256 sign webhook bodies. Write-only; never returned."`
	WebhookHeaders *map[string]string `json:"webhook_headers,omitempty" jsonschema:"Extra HTTP headers to send with each delivery (e.g. an Authorization or bearer token for your receiver). Merged under the built-in content-type/signature headers. Pass {} to clear."`
	Active         *bool              `json:"active,omitempty" jsonschema:"Whether the poller runs this watch. Defaults true on create."`
}

type XWatchOut struct {
	ID             int64             `json:"id"`
	Label          string            `json:"label"`
	Kind           string            `json:"kind"`
	Query          string            `json:"query"`
	Account        string            `json:"account,omitempty"`
	WebhookURL     string            `json:"webhook_url,omitempty"`
	WebhookHeaders map[string]string `json:"webhook_headers,omitempty"`
	HasSecret      bool              `json:"has_secret"`
	Active         bool              `json:"active"`
	LastSeenID     string            `json:"last_seen_id,omitempty"`
	LastError      string            `json:"last_error,omitempty"`
}

func toXWatchOut(w domain.XWatch) XWatchOut {
	return XWatchOut{
		ID:             w.ID,
		Label:          w.Label,
		Kind:           string(w.Kind),
		Query:          w.Query,
		Account:        w.AccountLabel,
		WebhookURL:     w.WebhookURL,
		WebhookHeaders: w.WebhookHeaders,
		HasSecret:      w.WebhookSecret != "",
		Active:         w.Active,
		LastSeenID:     w.LastSeenID,
		LastError:      w.LastError,
	}
}

func (d *Deps) XWatchSave(ctx context.Context, req *mcp.CallToolRequest, in XWatchSaveIn) (*mcp.CallToolResult, XWatchOut, error) {
	if d.Svc.XWatch == nil || !d.Svc.XWatch.Enabled() {
		return mcpserver.Err("disabled", "x watch persistence not configured"), XWatchOut{}, nil
	}
	if in.Label == "" {
		return mcpserver.Err("validation", "label required"), XWatchOut{}, nil
	}
	save := port.XWatchSaveInput{
		Label:          in.Label,
		Query:          in.Query,
		AccountLabel:   in.Account,
		WebhookURL:     in.WebhookURL,
		WebhookSecret:  in.WebhookSecret,
		WebhookHeaders: in.WebhookHeaders,
		Active:         in.Active,
	}
	if in.Kind != nil {
		k := domain.XWatchKind(*in.Kind)
		save.Kind = &k
	}
	w, err := d.Svc.XWatch.SaveWatch(ctx, save)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return mcpserver.Err("validation", err.Error()), XWatchOut{}, nil
		}
		return nil, XWatchOut{}, fmt.Errorf("x_watch_save: %w", err)
	}
	return nil, toXWatchOut(w), nil
}

// --- x_watch_list -----------------------------------------------------------

type XWatchListOut struct {
	Watches []XWatchOut `json:"watches"`
}

func (d *Deps) XWatchList(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, XWatchListOut, error) {
	if d.Svc.XWatch == nil || !d.Svc.XWatch.Enabled() {
		return mcpserver.Err("disabled", "x watch persistence not configured"), XWatchListOut{}, nil
	}
	ws, err := d.Svc.XWatch.ListWatches(ctx)
	if err != nil {
		return nil, XWatchListOut{}, fmt.Errorf("x_watch_list: %w", err)
	}
	out := XWatchListOut{Watches: make([]XWatchOut, 0, len(ws))}
	for _, w := range ws {
		out.Watches = append(out.Watches, toXWatchOut(w))
	}
	return nil, out, nil
}

// --- x_watch_delete ---------------------------------------------------------

type XWatchDeleteIn struct {
	Label string `json:"label" jsonschema:"Label of the watch to delete"`
}

func (d *Deps) XWatchDelete(ctx context.Context, req *mcp.CallToolRequest, in XWatchDeleteIn) (*mcp.CallToolResult, XOKOut, error) {
	if d.Svc.XWatch == nil || !d.Svc.XWatch.Enabled() {
		return mcpserver.Err("disabled", "x watch persistence not configured"), XOKOut{}, nil
	}
	if in.Label == "" {
		return mcpserver.Err("validation", "label required"), XOKOut{}, nil
	}
	if err := d.Svc.XWatch.DeleteWatch(ctx, in.Label); err != nil {
		return nil, XOKOut{}, fmt.Errorf("x_watch_delete: %w", err)
	}
	return nil, XOKOut{OK: true}, nil
}

// --- x_watch_events ---------------------------------------------------------

type XWatchEventsIn struct {
	Label    string `json:"label" jsonschema:"Label of the watch to read events for"`
	Delivery string `json:"delivery,omitempty" jsonschema:"Filter by delivery status: pending, delivered, failed, skipped. Empty = all."`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max events (default 50, newest first)"`
}

type XWatchEventOut struct {
	ID             int64  `json:"id"`
	TweetID        string `json:"tweet_id"`
	Author         string `json:"author"`
	Text           string `json:"text"`
	URL            string `json:"url,omitempty"`
	Likes          int    `json:"likes"`
	Retweets       int    `json:"retweets"`
	Replies        int    `json:"replies"`
	TweetCreatedAt string `json:"tweet_created_at,omitempty"`
	Delivery       string `json:"delivery"`
	DeliveryError  string `json:"delivery_error,omitempty"`
}

type XWatchEventsOut struct {
	Events []XWatchEventOut `json:"events"`
}

func (d *Deps) XWatchEvents(ctx context.Context, req *mcp.CallToolRequest, in XWatchEventsIn) (*mcp.CallToolResult, XWatchEventsOut, error) {
	if d.Svc.XWatch == nil || !d.Svc.XWatch.Enabled() {
		return mcpserver.Err("disabled", "x watch persistence not configured"), XWatchEventsOut{}, nil
	}
	if in.Label == "" {
		return mcpserver.Err("validation", "label required"), XWatchEventsOut{}, nil
	}
	evs, err := d.Svc.XWatch.ListEvents(ctx, in.Label, in.Delivery, in.Limit)
	if err != nil {
		return nil, XWatchEventsOut{}, fmt.Errorf("x_watch_events: %w", err)
	}
	out := XWatchEventsOut{Events: make([]XWatchEventOut, 0, len(evs))}
	for _, e := range evs {
		out.Events = append(out.Events, XWatchEventOut{
			ID:             e.ID,
			TweetID:        e.TweetID,
			Author:         e.Author,
			Text:           e.Text,
			URL:            e.URL,
			Likes:          e.Likes,
			Retweets:       e.Retweets,
			Replies:        e.Replies,
			TweetCreatedAt: e.TweetCreatedAt,
			Delivery:       string(e.Delivery),
			DeliveryError:  e.DeliveryError,
		})
	}
	return nil, out, nil
}

var _ = time.Now // reserved for future timestamped outputs
