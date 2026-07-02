package service

import (
	"context"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// XService is the use-case layer for the cookie-only x.com channel. It is a
// thin pass-through to the gateway: auth is a per-call Netscape cookie blob so
// one service instance serves many accounts. Optional; nil when the channel is
// not wired.
type XService struct {
	gateway port.XGateway
}

// NewXService builds the x.com service from a gateway.
func NewXService(gateway port.XGateway) *XService {
	return &XService{gateway: gateway}
}

func (s *XService) User(ctx context.Context, cookies, handle string) (domain.XUser, error) {
	return s.gateway.UserByScreenName(ctx, cookies, handle)
}

func (s *XService) Post(ctx context.Context, cookies string, in port.XPostInput) (domain.XPostResult, error) {
	return s.gateway.Post(ctx, cookies, in)
}

func (s *XService) Delete(ctx context.Context, cookies, tweetID string) error {
	return s.gateway.Delete(ctx, cookies, tweetID)
}

func (s *XService) Search(ctx context.Context, cookies string, in port.XSearchInput) (domain.XTweetPage, error) {
	return s.gateway.Search(ctx, cookies, in)
}

func (s *XService) UserTweets(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XTweetPage, error) {
	return s.gateway.UserTweets(ctx, cookies, userID, count, cursor)
}

func (s *XService) TweetDetail(ctx context.Context, cookies, tweetID string) (domain.XTweetDetail, error) {
	return s.gateway.TweetDetail(ctx, cookies, tweetID)
}

func (s *XService) Followers(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XUserPage, error) {
	return s.gateway.Followers(ctx, cookies, userID, count, cursor)
}

func (s *XService) Following(ctx context.Context, cookies, userID string, count int, cursor string) (domain.XUserPage, error) {
	return s.gateway.Following(ctx, cookies, userID, count, cursor)
}

func (s *XService) SendDM(ctx context.Context, cookies string, in port.XDMInput) (domain.XDMResult, error) {
	return s.gateway.SendDM(ctx, cookies, in)
}
