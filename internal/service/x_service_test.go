package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// fakeXGateway records calls and returns canned results/errors keyed by the
// cookie blob so a probe can be steered to live or dead.
type fakeXGateway struct {
	userErr map[string]error
}

func (f *fakeXGateway) UserByScreenName(ctx context.Context, cookies, handle string) (domain.XUser, error) {
	if err := f.userErr[cookies]; err != nil {
		return domain.XUser{}, err
	}
	return domain.XUser{RestID: "42", ScreenName: handle}, nil
}
func (f *fakeXGateway) Post(context.Context, string, port.XPostInput) (domain.XPostResult, error) {
	return domain.XPostResult{}, nil
}
func (f *fakeXGateway) Delete(context.Context, string, string) error { return nil }
func (f *fakeXGateway) Search(ctx context.Context, cookies string, in port.XSearchInput) (domain.XTweetPage, error) {
	if err := f.userErr[cookies]; err != nil {
		return domain.XTweetPage{}, err
	}
	return domain.XTweetPage{}, nil
}
func (f *fakeXGateway) UserTweets(context.Context, string, string, int, string) (domain.XTweetPage, error) {
	return domain.XTweetPage{}, nil
}
func (f *fakeXGateway) TweetDetail(context.Context, string, string) (domain.XTweetDetail, error) {
	return domain.XTweetDetail{}, nil
}
func (f *fakeXGateway) TweetReplies(context.Context, string, string, int, string) (domain.XTweetPage, error) {
	return domain.XTweetPage{}, nil
}
func (f *fakeXGateway) Followers(context.Context, string, string, int, string) (domain.XUserPage, error) {
	return domain.XUserPage{}, nil
}
func (f *fakeXGateway) Following(context.Context, string, string, int, string) (domain.XUserPage, error) {
	return domain.XUserPage{}, nil
}
func (f *fakeXGateway) SendDM(context.Context, string, port.XDMInput) (domain.XDMResult, error) {
	return domain.XDMResult{}, nil
}

// fakeXAccountRepo is an in-memory XAccountRepo capturing liveness updates.
type fakeXAccountRepo struct {
	stale    []port.XAccountWithTenant
	liveness map[int64]domain.XLiveness
	lastErr  map[int64]string
}

func (r *fakeXAccountRepo) Save(context.Context, port.XAccountSaveInput) (domain.XAccount, error) {
	return domain.XAccount{}, nil
}
func (r *fakeXAccountRepo) List(context.Context) ([]domain.XAccount, error)       { return nil, nil }
func (r *fakeXAccountRepo) GetByLabel(context.Context, string) (domain.XAccount, error) {
	return domain.XAccount{}, nil
}
func (r *fakeXAccountRepo) Delete(context.Context, string) error { return nil }
func (r *fakeXAccountRepo) UpdateLiveness(ctx context.Context, id int64, liveness domain.XLiveness, screenName, userID, lastErr string) error {
	if r.liveness == nil {
		r.liveness = map[int64]domain.XLiveness{}
		r.lastErr = map[int64]string{}
	}
	r.liveness[id] = liveness
	r.lastErr[id] = lastErr
	return nil
}
func (r *fakeXAccountRepo) ListStale(context.Context, time.Time, int) ([]port.XAccountWithTenant, error) {
	return r.stale, nil
}

func TestXCheckLivenessMarksLiveAndDead(t *testing.T) {
	repo := &fakeXAccountRepo{
		stale: []port.XAccountWithTenant{
			{TenantID: "default", Account: domain.XAccount{ID: 1, ScreenName: "good", Cookies: "ck-good"}},
			{TenantID: "default", Account: domain.XAccount{ID: 2, ScreenName: "bad", Cookies: "ck-bad"}},
		},
	}
	gw := &fakeXGateway{userErr: map[string]error{"ck-bad": errors.New("401 unauthorized")}}
	svc := NewXService(gw, repo)

	n, err := svc.CheckLiveness(context.Background(), time.Now(), 10)
	if err != nil {
		t.Fatalf("CheckLiveness: %v", err)
	}
	if n != 2 {
		t.Fatalf("checked = %d, want 2", n)
	}
	if repo.liveness[1] != domain.XLivenessLive {
		t.Fatalf("account 1 liveness = %q, want live", repo.liveness[1])
	}
	if repo.liveness[2] != domain.XLivenessDead {
		t.Fatalf("account 2 liveness = %q, want dead", repo.liveness[2])
	}
	if repo.lastErr[2] == "" {
		t.Fatalf("account 2 expected last_error recorded")
	}
}

func TestXAccountsDisabledWithoutRepo(t *testing.T) {
	svc := NewXService(&fakeXGateway{}, nil)
	if _, err := svc.ListAccounts(context.Background()); err != ErrAccountsDisabled {
		t.Fatalf("ListAccounts err = %v, want ErrAccountsDisabled", err)
	}
	if _, err := svc.CheckLiveness(context.Background(), time.Now(), 10); err != ErrAccountsDisabled {
		t.Fatalf("CheckLiveness err = %v, want ErrAccountsDisabled", err)
	}
}
