package lingin

import "context"

// Typed convenience wrappers over call(). Each maps a CRM operation to a lingin
// MCP tool. The `account` argument is the LinkedIn identity label (per tenant).
// Results are returned as raw JSON strings (the tool text payload); callers
// parse as needed. This keeps the adapter thin and avoids duplicating lingin's
// evolving response schemas in the CRM.

// Me returns the authenticated LinkedIn identity for an account.
func (r *Runner) Me(ctx context.Context, account string) (string, error) {
	return r.call(ctx, account, "linkedin_me", nil)
}

// Profile fetches a member profile by public id or urn fragment.
func (r *Runner) Profile(ctx context.Context, account, id string) (string, error) {
	return r.call(ctx, account, "linkedin_profile", map[string]any{"id": id})
}

// Company fetches a company by universal name.
func (r *Runner) Company(ctx context.Context, account, name string) (string, error) {
	return r.call(ctx, account, "linkedin_company", map[string]any{"name": name})
}

// SearchPeople runs a people search by keywords.
func (r *Runner) SearchPeople(ctx context.Context, account, keywords string) (string, error) {
	return r.call(ctx, account, "linkedin_search_people", map[string]any{"keywords": keywords})
}

// CreatePost publishes a share with the given text. High-risk: writes to the
// live LinkedIn account.
func (r *Runner) CreatePost(ctx context.Context, account, text string) (string, error) {
	return r.call(ctx, account, "linkedin_create_post", map[string]any{"text": text})
}

// DeletePost removes a share by its share urn.
func (r *Runner) DeletePost(ctx context.Context, account, shareURN string) (string, error) {
	return r.call(ctx, account, "linkedin_delete_post", map[string]any{"urn": shareURN})
}

// ListComments lists comments on an activity.
func (r *Runner) ListComments(ctx context.Context, account, activityURN string) (string, error) {
	return r.call(ctx, account, "linkedin_list_comments", map[string]any{"id": activityURN})
}

// ListReplies lists replies under a comment.
func (r *Runner) ListReplies(ctx context.Context, account, commentURN string) (string, error) {
	return r.call(ctx, account, "linkedin_list_replies", map[string]any{"id": commentURN})
}

// Comment posts a top-level comment on an activity. High-risk: writes live.
func (r *Runner) Comment(ctx context.Context, account, activityURN, text string) (string, error) {
	return r.call(ctx, account, "linkedin_comment", map[string]any{"id": activityURN, "text": text})
}

// Reply posts a reply under a comment. High-risk: writes live.
func (r *Runner) Reply(ctx context.Context, account, commentURN, text string) (string, error) {
	return r.call(ctx, account, "linkedin_reply", map[string]any{"id": commentURN, "text": text})
}

// DeleteComment removes a comment by its urn.
func (r *Runner) DeleteComment(ctx context.Context, account, commentURN string) (string, error) {
	return r.call(ctx, account, "linkedin_delete_comment", map[string]any{"id": commentURN})
}
