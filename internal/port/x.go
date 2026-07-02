package port

import (
	"context"

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
	Followers(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XUserPage, error)
	Following(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XUserPage, error)
	SendDM(ctx context.Context, cookies string, in XDMInput) (domain.XDMResult, error)
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
