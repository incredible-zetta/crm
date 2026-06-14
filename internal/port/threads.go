package port

import (
	"context"

	"github.com/incredible-zetta/crm/internal/domain"
)

type ThreadsGateway interface {
	Profile(ctx context.Context) (domain.ThreadsProfile, []byte, error)
	List(ctx context.Context, limit int, cursor string) ([]domain.ThreadsPost, string, error)
	Publish(ctx context.Context, in ThreadsPublishInput) (ThreadsPublishResult, error)
	Delete(ctx context.Context, mediaID string) error
	Insights(ctx context.Context, mediaID string) ([]domain.ThreadsInsight, []byte, error)
	Replies(ctx context.Context, mediaID string, limit int, cursor string) ([]domain.ThreadsReply, string, error)
	Reply(ctx context.Context, mediaID, text string) (string, []byte, error)
	ReplyQuota(ctx context.Context) (map[string]any, []byte, error)
	Mentions(ctx context.Context, limit int, cursor string) ([]domain.ThreadsMention, string, error)
	Search(ctx context.Context, query string, limit int, cursor string) (map[string]any, []byte, error)
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
