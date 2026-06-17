package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

type ThreadsService struct {
	gateway port.ThreadsGateway
	repo    port.ThreadsRepo
}

func NewThreadsService(gateway port.ThreadsGateway, repo port.ThreadsRepo) *ThreadsService {
	return &ThreadsService{gateway: gateway, repo: repo}
}

func (s *ThreadsService) Profile(ctx context.Context) (domain.ThreadsProfile, error) {
	p, raw, err := s.gateway.Profile(ctx)
	s.audit(ctx, "profile", p.ID, err, raw)
	return p, err
}

func (s *ThreadsService) List(ctx context.Context, limit int, cursor string) ([]domain.ThreadsPost, string, error) {
	posts, next, err := s.gateway.List(ctx, limit, cursor)
	s.audit(ctx, "list", "", err, nil)
	if err != nil {
		return nil, "", err
	}
	for _, p := range posts {
		_, _ = s.repo.UpsertPost(ctx, p)
	}
	return posts, next, nil
}

func (s *ThreadsService) Publish(ctx context.Context, in port.ThreadsPublishInput) (port.ThreadsPublishResult, error) {
	res, err := s.gateway.Publish(ctx, in)
	s.audit(ctx, "publish", res.Post.ThreadsID, err, res.RawJSON)
	if err != nil {
		return port.ThreadsPublishResult{}, err
	}
	stored, storeErr := s.repo.UpsertPost(ctx, res.Post)
	if storeErr == nil {
		res.Post = stored
	}
	return res, nil
}

func (s *ThreadsService) Delete(ctx context.Context, mediaID string, localID int64) error {
	if mediaID != "" {
		err := s.gateway.Delete(ctx, mediaID)
		s.audit(ctx, "delete", mediaID, err, nil)
		if err != nil {
			return err
		}
	}
	if localID != 0 {
		return s.repo.SoftDeletePost(ctx, localID)
	}
	if mediaID != "" {
		post, err := s.repo.GetPostByThreadsID(ctx, mediaID)
		if err == nil {
			return s.repo.SoftDeletePost(ctx, post.ID)
		}
	}
	return nil
}

func (s *ThreadsService) Insights(ctx context.Context, mediaID string) ([]domain.ThreadsInsight, error) {
	items, raw, err := s.gateway.Insights(ctx, mediaID)
	s.audit(ctx, "insights", mediaID, err, raw)
	return items, err
}

func (s *ThreadsService) FollowerDemographics(ctx context.Context, breakdown string) (map[string]any, error) {
	out, raw, err := s.gateway.FollowerDemographics(ctx, breakdown)
	s.audit(ctx, "follower_demographics", "", err, raw)
	return out, err
}

func (s *ThreadsService) Replies(ctx context.Context, mediaID string, limit int, cursor string) ([]domain.ThreadsReply, string, error) {
	items, next, err := s.gateway.Replies(ctx, mediaID, limit, cursor)
	s.audit(ctx, "replies", mediaID, err, nil)
	if err != nil {
		return nil, "", err
	}
	for _, item := range items {
		_, _ = s.repo.UpsertReply(ctx, item)
	}
	return items, next, nil
}

func (s *ThreadsService) Conversation(ctx context.Context, mediaID string, limit int, cursor string) ([]domain.ThreadsReply, string, error) {
	items, next, err := s.gateway.Conversation(ctx, mediaID, limit, cursor)
	s.audit(ctx, "conversation", mediaID, err, nil)
	if err != nil {
		return nil, "", err
	}
	for _, item := range items {
		_, _ = s.repo.UpsertReply(ctx, item)
	}
	return items, next, nil
}

func (s *ThreadsService) Reply(ctx context.Context, mediaID, text string) (string, error) {
	id, raw, err := s.gateway.Reply(ctx, mediaID, text)
	s.audit(ctx, "reply", mediaID, err, raw)
	if err == nil && id != "" {
		_, _ = s.repo.UpsertReply(ctx, domain.ThreadsReply{ReplyID: id, PostID: mediaID, Text: text, RawJSON: raw})
	}
	return id, err
}

func (s *ThreadsService) HideReply(ctx context.Context, replyID string) error {
	return s.manageReply(ctx, replyID, true)
}

func (s *ThreadsService) UnhideReply(ctx context.Context, replyID string) error {
	return s.manageReply(ctx, replyID, false)
}

func (s *ThreadsService) manageReply(ctx context.Context, replyID string, hide bool) error {
	action := "reply_unhide"
	if hide {
		action = "reply_hide"
	}
	raw, err := s.gateway.ManageReply(ctx, replyID, hide)
	s.audit(ctx, action, replyID, err, raw)
	return err
}

func (s *ThreadsService) ReplyQuota(ctx context.Context) (map[string]any, error) {
	out, raw, err := s.gateway.ReplyQuota(ctx)
	s.audit(ctx, "reply_quota", "", err, raw)
	return out, err
}

func (s *ThreadsService) Mentions(ctx context.Context, limit int, cursor string) ([]domain.ThreadsMention, string, error) {
	items, next, err := s.gateway.Mentions(ctx, limit, cursor)
	s.audit(ctx, "mentions", "", err, nil)
	if err != nil {
		return nil, "", err
	}
	for _, item := range items {
		_, _ = s.repo.UpsertMention(ctx, item)
	}
	return items, next, nil
}

func (s *ThreadsService) Search(ctx context.Context, in port.ThreadsSearchInput) (map[string]any, error) {
	out, raw, err := s.gateway.Search(ctx, in)
	s.audit(ctx, "search", in.Query, err, raw)
	return out, err
}

func (s *ThreadsService) ExchangeToken(ctx context.Context, accessToken string) (port.ThreadsTokenResult, error) {
	out, err := s.gateway.ExchangeToken(ctx, accessToken)
	s.audit(ctx, "token_exchange", "", err, nil)
	return out, err
}

func (s *ThreadsService) RefreshToken(ctx context.Context, accessToken string) (port.ThreadsTokenResult, error) {
	out, err := s.gateway.RefreshToken(ctx, accessToken)
	s.audit(ctx, "token_refresh", "", err, nil)
	return out, err
}

func (s *ThreadsService) ListCached(ctx context.Context, f domain.ThreadsListFilter, p port.Paging) (port.ThreadsPostPage, error) {
	return s.repo.ListPosts(ctx, f, p)
}

func (s *ThreadsService) GetCached(ctx context.Context, id int64, threadsID string) (domain.ThreadsPost, error) {
	if id != 0 {
		return s.repo.GetPost(ctx, id)
	}
	if threadsID != "" {
		return s.repo.GetPostByThreadsID(ctx, threadsID)
	}
	return domain.ThreadsPost{}, fmt.Errorf("%w: id or threads_id required", domain.ErrValidation)
}

func (s *ThreadsService) History(ctx context.Context, p port.Paging) (port.ThreadsAuditPage, error) {
	return s.repo.ListAudit(ctx, p)
}

func (s *ThreadsService) audit(ctx context.Context, action, objectID string, callErr error, raw []byte) {
	if s.repo == nil {
		return
	}
	event := domain.ThreadsAuditEvent{Action: action, ObjectID: objectID, OK: callErr == nil, RawJSON: raw}
	if callErr != nil {
		event.Error = callErr.Error()
	}
	if len(event.RawJSON) == 0 {
		event.RawJSON, _ = json.Marshal(map[string]any{"action": action, "object_id": objectID, "ok": event.OK})
	}
	_ = s.repo.InsertAudit(ctx, event)
}
