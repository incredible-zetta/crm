package mcptransport

import (
	"context"
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

// XCookiesIn is embedded by every x_* input: the per-call account cookies.
type XCookiesIn struct {
	Cookies string `json:"cookies" jsonschema:"Netscape cookie-file blob for the x.com account to act as (must include auth_token and ct0). Sensitive; identifies the acting account."`
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
	u, err := d.Svc.X.User(ctx, in.Cookies, in.Handle)
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
	res, err := d.Svc.X.Post(ctx, in.Cookies, port.XPostInput{
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
	if err := d.Svc.X.Delete(ctx, in.Cookies, in.TweetID); err != nil {
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
	page, err := d.Svc.X.Search(ctx, in.Cookies, port.XSearchInput{
		Query: in.Query, Product: in.Product, Count: in.Count, Cursor: in.Cursor,
	})
	if err != nil {
		return nil, XTweetPageOut{}, fmt.Errorf("x_search: %w", err)
	}
	return nil, XTweetPageOut{Tweets: toXTweetOuts(page.Tweets), NextCursor: page.NextCursor}, nil
}

// --- x_user_tweets ----------------------------------------------------------

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
	userID, errRes := d.resolveXUserID(ctx, in.Cookies, in.UserID, in.Handle)
	if errRes != nil {
		return errRes, XTweetPageOut{}, nil
	}
	page, err := d.Svc.X.UserTweets(ctx, in.Cookies, userID, in.Count, in.Cursor)
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
	userID, errRes := d.resolveXUserID(ctx, in.Cookies, in.UserID, in.Handle)
	if errRes != nil {
		return errRes, XUserPageOut{}, nil
	}
	page, err := d.Svc.X.Followers(ctx, in.Cookies, userID, in.Count, in.Cursor)
	if err != nil {
		return nil, XUserPageOut{}, fmt.Errorf("x_followers: %w", err)
	}
	return nil, toXUserPageOut(page), nil
}

func (d *Deps) XFollowing(ctx context.Context, req *mcp.CallToolRequest, in XSocialIn) (*mcp.CallToolResult, XUserPageOut, error) {
	if errRes, ok := d.xReady(); !ok {
		return errRes, XUserPageOut{}, nil
	}
	userID, errRes := d.resolveXUserID(ctx, in.Cookies, in.UserID, in.Handle)
	if errRes != nil {
		return errRes, XUserPageOut{}, nil
	}
	page, err := d.Svc.X.Following(ctx, in.Cookies, userID, in.Count, in.Cursor)
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
	res, err := d.Svc.X.SendDM(ctx, in.Cookies, port.XDMInput{
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

var _ = time.Now // reserved for future timestamped outputs
