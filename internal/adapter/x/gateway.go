// Package x is the cookie-only x.com (Twitter) adapter. It wraps the native
// GraphQL client ported from x-utils (internal/adapter/x/xclient) behind the
// port.XGateway interface. Auth is a per-call Netscape cookie blob, so a single
// Gateway serves many accounts: the caller supplies the cookies of the account
// to act as on each request. No access token, web-scraped via a logged-in
// session cookie (auth_token + ct0).
package x

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/adapter/x/xclient"
	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// Gateway implements port.XGateway using the native xclient. It is stateless
// beyond an HTTP client for media downloads; per-call cookies build a fresh
// xclient.Client each request.
type Gateway struct {
	http *http.Client
}

var _ port.XGateway = (*Gateway)(nil)

// New returns an x.com gateway.
func New() *Gateway {
	return &Gateway{http: &http.Client{Timeout: 60 * time.Second}}
}

func client(cookies string) (*xclient.Client, error) {
	if strings.TrimSpace(cookies) == "" {
		return nil, fmt.Errorf("cookies required")
	}
	return xclient.NewClientFromCookies(cookies)
}

func (g *Gateway) UserByScreenName(ctx context.Context, cookies, handle string) (domain.XUser, error) {
	c, err := client(cookies)
	if err != nil {
		return domain.XUser{}, err
	}
	u, err := c.UserByScreenName(strings.TrimPrefix(handle, "@"))
	if err != nil {
		return domain.XUser{}, err
	}
	return toXUser(u), nil
}

func (g *Gateway) Post(ctx context.Context, cookies string, in port.XPostInput) (domain.XPostResult, error) {
	c, err := client(cookies)
	if err != nil {
		return domain.XPostResult{}, err
	}
	opts := xclient.TweetOptions{Text: in.Text, ReplyTo: in.ReplyTo, QuoteOf: in.QuoteOf}
	if in.MediaURL != "" {
		id, err := g.uploadFromURL(c, in.MediaURL, false)
		if err != nil {
			return domain.XPostResult{}, fmt.Errorf("upload media: %w", err)
		}
		opts.MediaIDs = []string{id}
	}
	res, err := c.PostTweet(opts)
	if err != nil {
		return domain.XPostResult{}, err
	}
	out := domain.XPostResult{RestID: res.RestID, ScreenName: res.ScreenName}
	if res.ScreenName != "" && res.RestID != "" {
		out.URL = "https://x.com/" + res.ScreenName + "/status/" + res.RestID
	}
	return out, nil
}

func (g *Gateway) Delete(ctx context.Context, cookies, tweetID string) error {
	c, err := client(cookies)
	if err != nil {
		return err
	}
	return c.DeleteTweet(tweetID)
}

func (g *Gateway) Search(ctx context.Context, cookies string, in port.XSearchInput) (domain.XTweetPage, error) {
	c, err := client(cookies)
	if err != nil {
		return domain.XTweetPage{}, err
	}
	res, err := c.Search(xclient.SearchOptions{
		Query:   in.Query,
		Product: xclient.SearchProduct(in.Product),
		Count:   in.Count,
		Cursor:  in.Cursor,
	})
	if err != nil {
		return domain.XTweetPage{}, err
	}
	return toXTweetPage(res.Tweets, res.NextCursor), nil
}

func (g *Gateway) UserTweets(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XTweetPage, error) {
	c, err := client(cookies)
	if err != nil {
		return domain.XTweetPage{}, err
	}
	res, err := c.UserTweets(xclient.UserTweetsOptions{UserID: userID, Count: count, Cursor: cursor})
	if err != nil {
		return domain.XTweetPage{}, err
	}
	return toXTweetPage(res.Tweets, res.NextCursor), nil
}

func (g *Gateway) TweetDetail(ctx context.Context, cookies, tweetID string) (domain.XTweetDetail, error) {
	c, err := client(cookies)
	if err != nil {
		return domain.XTweetDetail{}, err
	}
	d, err := c.TweetDetailByID(tweetID)
	if err != nil {
		return domain.XTweetDetail{}, err
	}
	return domain.XTweetDetail{
		XTweet:         toXTweet(d.TweetSummary),
		Views:          d.Views,
		ViewState:      d.ViewState,
		Quotes:         d.Quotes,
		Bookmarks:      d.Bookmarks,
		ConversationID: d.ConversationID,
	}, nil
}

func (g *Gateway) Followers(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XUserPage, error) {
	return g.socialList(cookies, userID, count, cursor, true)
}

func (g *Gateway) Following(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XUserPage, error) {
	return g.socialList(cookies, userID, count, cursor, false)
}

func (g *Gateway) socialList(cookies, userID string, count int, cursor string, followers bool) (domain.XUserPage, error) {
	c, err := client(cookies)
	if err != nil {
		return domain.XUserPage{}, err
	}
	opts := xclient.SocialListOptions{UserID: userID, Count: count, Cursor: cursor}
	var res *xclient.SocialListResult
	if followers {
		res, err = c.Followers(opts)
	} else {
		res, err = c.Following(opts)
	}
	if err != nil {
		return domain.XUserPage{}, err
	}
	out := domain.XUserPage{NextCursor: res.NextCursor, Users: make([]domain.XUser, 0, len(res.Users))}
	for _, u := range res.Users {
		out.Users = append(out.Users, domain.XUser{
			RestID: u.RestID, Name: u.Name, ScreenName: u.ScreenName,
			Followers: u.Followers, Following: u.Following, Tweets: u.Tweets, Verified: u.Verified,
		})
	}
	return out, nil
}

func (g *Gateway) SendDM(ctx context.Context, cookies string, in port.XDMInput) (domain.XDMResult, error) {
	c, err := client(cookies)
	if err != nil {
		return domain.XDMResult{}, err
	}
	recipient := in.RecipientID
	if recipient == "" && in.Handle != "" {
		u, err := c.UserByScreenName(strings.TrimPrefix(in.Handle, "@"))
		if err != nil {
			return domain.XDMResult{}, fmt.Errorf("resolve handle: %w", err)
		}
		recipient = u.RestID
	}
	if recipient == "" {
		return domain.XDMResult{}, fmt.Errorf("recipient_id or handle required")
	}
	opts := xclient.DMOptions{RecipientID: recipient, Text: in.Text}
	if in.MediaURL != "" {
		id, err := g.uploadFromURL(c, in.MediaURL, true)
		if err != nil {
			return domain.XDMResult{}, fmt.Errorf("upload dm media: %w", err)
		}
		opts.MediaID = id
	}
	res, err := c.SendDM(opts)
	if err != nil {
		return domain.XDMResult{}, err
	}
	return domain.XDMResult{MessageID: res.MessageID, ConversationID: res.ConversationID}, nil
}

// uploadFromURL downloads a remote media URL to a temp file and uploads it via
// the chunked media flow, returning the media_id. dm selects the DM category.
func (g *Gateway) uploadFromURL(c *xclient.Client, mediaURL string, dm bool) (string, error) {
	req, err := http.NewRequest(http.MethodGet, mediaURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download media: http %d", resp.StatusCode)
	}
	ext := filepath.Ext(mediaURL)
	if i := strings.IndexByte(ext, '?'); i >= 0 {
		ext = ext[:i]
	}
	if ext == "" {
		ext = ".bin"
	}
	f, err := os.CreateTemp("", "x-media-*"+ext)
	if err != nil {
		return "", err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	if dm {
		return c.UploadDMMedia(name)
	}
	return c.UploadMedia(name)
}

func toXUser(u *xclient.User) domain.XUser {
	return domain.XUser{
		RestID: u.RestID, Name: u.Name, ScreenName: u.ScreenName, CreatedAt: u.CreatedAt,
		Followers: u.Followers, Following: u.Following, Tweets: u.Tweets,
		Verified: u.Verified, BlueVerified: u.BlueVerified,
	}
}

func toXTweet(t xclient.TweetSummary) domain.XTweet {
	return domain.XTweet{
		RestID: t.RestID, Text: t.Text, ScreenName: t.ScreenName, Name: t.Name,
		CreatedAt: t.CreatedAt, CreatedTime: t.CreatedTime,
		Likes: t.Likes, Retweets: t.Retweets, Replies: t.Replies,
		HasMedia: t.HasMedia, URL: t.URL,
	}
}

func toXTweetPage(tweets []xclient.TweetSummary, next string) domain.XTweetPage {
	out := domain.XTweetPage{NextCursor: next, Tweets: make([]domain.XTweet, 0, len(tweets))}
	for _, t := range tweets {
		out.Tweets = append(out.Tweets, toXTweet(t))
	}
	return out
}
