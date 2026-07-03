package service

import (
	"context"

	"github.com/incredible-zetta/crm/internal/port"
)

// LinkedInService is the CRM's LinkedIn facade over the lingin adapter. It is a
// thin pass-through today (the adapter returns raw JSON); it exists so the MCP
// layer and handlers depend on a service, not the adapter directly, and so
// account selection/tenant policy can grow here later.
type LinkedInService struct {
	li port.LinkedIn
}

// NewLinkedInService wires the service over a LinkedIn port. li may be nil when
// the channel is disabled; callers should check Enabled first.
func NewLinkedInService(li port.LinkedIn) *LinkedInService {
	return &LinkedInService{li: li}
}

// Enabled reports whether a LinkedIn adapter is configured.
func (s *LinkedInService) Enabled() bool { return s != nil && s.li != nil }

// Me returns the authenticated identity for an account.
func (s *LinkedInService) Me(ctx context.Context, account string) (string, error) {
	return s.li.Me(ctx, account)
}

// Profile fetches a member profile.
func (s *LinkedInService) Profile(ctx context.Context, account, id string) (string, error) {
	return s.li.Profile(ctx, account, id)
}

// Company fetches a company.
func (s *LinkedInService) Company(ctx context.Context, account, name string) (string, error) {
	return s.li.Company(ctx, account, name)
}

// SearchPeople runs a people search.
func (s *LinkedInService) SearchPeople(ctx context.Context, account, keywords string) (string, error) {
	return s.li.SearchPeople(ctx, account, keywords)
}

// CreatePost publishes a share (high-risk, writes live).
func (s *LinkedInService) CreatePost(ctx context.Context, account, text string) (string, error) {
	return s.li.CreatePost(ctx, account, text)
}

// DeletePost removes a share.
func (s *LinkedInService) DeletePost(ctx context.Context, account, shareURN string) (string, error) {
	return s.li.DeletePost(ctx, account, shareURN)
}

// ListComments lists comments on an activity.
func (s *LinkedInService) ListComments(ctx context.Context, account, activityURN string) (string, error) {
	return s.li.ListComments(ctx, account, activityURN)
}

// ListReplies lists replies under a comment.
func (s *LinkedInService) ListReplies(ctx context.Context, account, commentURN string) (string, error) {
	return s.li.ListReplies(ctx, account, commentURN)
}

// Comment posts a top-level comment (high-risk, writes live).
func (s *LinkedInService) Comment(ctx context.Context, account, activityURN, text string) (string, error) {
	return s.li.Comment(ctx, account, activityURN, text)
}

// Reply posts a reply under a comment (high-risk, writes live).
func (s *LinkedInService) Reply(ctx context.Context, account, commentURN, text string) (string, error) {
	return s.li.Reply(ctx, account, commentURN, text)
}

// DeleteComment removes a comment.
func (s *LinkedInService) DeleteComment(ctx context.Context, account, commentURN string) (string, error) {
	return s.li.DeleteComment(ctx, account, commentURN)
}
