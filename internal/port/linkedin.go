package port

import "context"

// LinkedIn is the CRM's view of the lingin LinkedIn adapter. Implemented by
// internal/adapter/lingin.Runner. All methods take an account label (the
// per-tenant LinkedIn identity) and return raw JSON payloads from the binary.
type LinkedIn interface {
	SaveAccount(ctx context.Context, label, cookies string) (string, error)
	Me(ctx context.Context, account string) (string, error)
	Profile(ctx context.Context, account, id string) (string, error)
	Company(ctx context.Context, account, name string) (string, error)
	SearchPeople(ctx context.Context, account, keywords string) (string, error)
	CreatePost(ctx context.Context, account, text string) (string, error)
	DeletePost(ctx context.Context, account, shareURN string) (string, error)
	ListComments(ctx context.Context, account, activityURN string) (string, error)
	ListReplies(ctx context.Context, account, commentURN string) (string, error)
	Comment(ctx context.Context, account, activityURN, text string) (string, error)
	Reply(ctx context.Context, account, commentURN, text string) (string, error)
	DeleteComment(ctx context.Context, account, commentURN string) (string, error)
}
