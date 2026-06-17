package port

import (
	"context"

	"github.com/incredible-zetta/crm/internal/domain"
)

type ThreadsGateway interface {
	Profile(ctx context.Context) (domain.ThreadsProfile, []byte, error)
	ProfileLookup(ctx context.Context, username string) (domain.ThreadsPublicProfile, []byte, error)
	List(ctx context.Context, limit int, cursor string) ([]domain.ThreadsPost, string, error)
	Publish(ctx context.Context, in ThreadsPublishInput) (ThreadsPublishResult, error)
	Delete(ctx context.Context, mediaID string) error
	Insights(ctx context.Context, mediaID string) ([]domain.ThreadsInsight, []byte, error)
	FollowerDemographics(ctx context.Context, breakdown string) (map[string]any, []byte, error)
	Replies(ctx context.Context, mediaID string, limit int, cursor string) ([]domain.ThreadsReply, string, error)
	Conversation(ctx context.Context, mediaID string, limit int, cursor string) ([]domain.ThreadsReply, string, error)
	Reply(ctx context.Context, mediaID, text string) (string, []byte, error)
	ManageReply(ctx context.Context, replyID string, hide bool) ([]byte, error)
	ReplyQuota(ctx context.Context) (map[string]any, []byte, error)
	Mentions(ctx context.Context, limit int, cursor string) ([]domain.ThreadsMention, string, error)
	Search(ctx context.Context, in ThreadsSearchInput) (map[string]any, []byte, error)
	ExchangeToken(ctx context.Context, accessToken string) (ThreadsTokenResult, error)
	RefreshToken(ctx context.Context, accessToken string) (ThreadsTokenResult, error)
}

// ThreadsDiscovery is the cookie-only discovery path backed by the
// x-threads-utils binary. Auth is a logged-in session cookie blob per account
// (no Graph API token). SearchPosts/Viral/Latest are reliable cookie-only.
type ThreadsDiscovery interface {
	// SearchPosts runs `search-posts` (server-rendered HTML scrape, JSON out).
	SearchPosts(ctx context.Context, cookies, query string) ([]domain.ThreadsDiscoveredPost, []byte, error)
	// Viral runs `viral <topic>` ranked by engagement (text out, returned raw).
	Viral(ctx context.Context, cookies, topic string) ([]byte, error)
	// Latest runs `latest <topic>` newest-first (text out, returned raw).
	Latest(ctx context.Context, cookies, topic string) ([]byte, error)
}

type ThreadsPublishInput struct {
	Text     string
	ImageURL string
	VideoURL string
	TopicTag string
}

type ThreadsPublishResult struct {
	ContainerID string
	Post        domain.ThreadsPost
	RawJSON     []byte
}

type ThreadsSearchInput struct {
	Query          string
	SearchType     string
	SearchMode     string
	MediaType      string
	AuthorUsername string
	Since          string
	Until          string
	Fields         string
	Limit          int
	Cursor         string
}

type ThreadsTokenResult struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`
	RawJSON     []byte `json:"-"`
}

type ThreadsRepo interface {
	UpsertPost(ctx context.Context, post domain.ThreadsPost) (domain.ThreadsPost, error)
	GetPost(ctx context.Context, id int64) (domain.ThreadsPost, error)
	GetPostByThreadsID(ctx context.Context, threadsID string) (domain.ThreadsPost, error)
	ListPosts(ctx context.Context, f domain.ThreadsListFilter, p Paging) (ThreadsPostPage, error)
	SoftDeletePost(ctx context.Context, id int64) error
	UpsertReply(ctx context.Context, reply domain.ThreadsReply) (domain.ThreadsReply, error)
	UpsertMention(ctx context.Context, mention domain.ThreadsMention) (domain.ThreadsMention, error)
	InsertAudit(ctx context.Context, event domain.ThreadsAuditEvent) error
	ListAudit(ctx context.Context, p Paging) (ThreadsAuditPage, error)
}

type ThreadsPostPage struct {
	Items      []domain.ThreadsPost
	Total      int
	NextCursor int64
}

type ThreadsAuditPage struct {
	Items      []domain.ThreadsAuditEvent
	Total      int
	NextCursor int64
}
