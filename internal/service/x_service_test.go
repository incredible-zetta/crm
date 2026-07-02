package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func (f *fakeXGateway) Me(ctx context.Context, cookies string) (string, error) {
	if err := f.userErr[cookies]; err != nil {
		return "", err
	}
	return "42", nil
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
func (f *fakeXGateway) Mentions(context.Context, string, string, int) (domain.XTweetPage, error) {
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
func (r *fakeXAccountRepo) List(context.Context) ([]domain.XAccount, error) { return nil, nil }
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

// fakeXWatchRepo is an in-memory XWatchRepo for the watch poller test.
type fakeXWatchRepo struct {
	due    []port.XWatchWithTenant
	events []domain.XWatchEvent
	seen   map[string]bool // tweet ids already added
	polled map[int64]string
}

func (r *fakeXWatchRepo) Save(context.Context, port.XWatchSaveInput) (domain.XWatch, error) {
	return domain.XWatch{}, nil
}
func (r *fakeXWatchRepo) List(context.Context) ([]domain.XWatch, error) { return nil, nil }
func (r *fakeXWatchRepo) GetByLabel(context.Context, string) (domain.XWatch, error) {
	return domain.XWatch{}, nil
}
func (r *fakeXWatchRepo) Delete(context.Context, string) error { return nil }
func (r *fakeXWatchRepo) ListDue(context.Context, int) ([]port.XWatchWithTenant, error) {
	return r.due, nil
}
func (r *fakeXWatchRepo) UpdatePollState(ctx context.Context, id int64, lastSeenID, lastErr string) error {
	if r.polled == nil {
		r.polled = map[int64]string{}
	}
	r.polled[id] = lastSeenID
	return nil
}
func (r *fakeXWatchRepo) AddEvent(ctx context.Context, watchID int64, ev domain.XWatchEvent) (bool, int64, error) {
	if r.seen == nil {
		r.seen = map[string]bool{}
	}
	if r.seen[ev.TweetID] {
		return false, 0, nil
	}
	r.seen[ev.TweetID] = true
	ev.WatchID = watchID
	ev.ID = int64(len(r.events) + 1)
	r.events = append(r.events, ev)
	return true, ev.ID, nil
}
func (r *fakeXWatchRepo) MarkDelivered(ctx context.Context, eventID int64, status domain.XWatchDelivery, deliverErr string) error {
	for i := range r.events {
		if r.events[i].ID == eventID {
			r.events[i].Delivery = status
			r.events[i].DeliveryError = deliverErr
		}
	}
	return nil
}
func (r *fakeXWatchRepo) ListEvents(context.Context, int64, string, int) ([]domain.XWatchEvent, error) {
	return r.events, nil
}

// mentionGateway returns two mentions so the poller can store + deliver them.
type mentionGateway struct{ fakeXGateway }

func (g *mentionGateway) Mentions(context.Context, string, string, int) (domain.XTweetPage, error) {
	return domain.XTweetPage{Tweets: []domain.XTweet{
		{RestID: "1002", Text: "@me hi again", ScreenName: "bob"},
		{RestID: "1001", Text: "@me hello", ScreenName: "alice"},
	}}, nil
}

func TestRunWatchesDeliversSignedWebhook(t *testing.T) {
	var (
		gotSig  string
		gotBody []byte
		hits    int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits++
		gotSig = req.Header.Get("x-zetta-signature")
		gotBody, _ = io.ReadAll(req.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := &fakeXWatchRepo{due: []port.XWatchWithTenant{{
		TenantID: "default",
		Watch: domain.XWatch{
			ID: 7, Label: "mentions", Kind: domain.XWatchMention, Query: "me",
			WebhookURL: srv.URL, WebhookSecret: "s3cr3t", Active: true,
		},
	}}}
	xsvc := NewXService(&mentionGateway{}, &fakeXAccountRepo{})
	ws := NewXWatchService(repo, xsvc)

	polled, events := ws.RunWatches(context.Background(), 10)
	if polled != 1 || events != 2 {
		t.Fatalf("polled=%d events=%d, want 1/2", polled, events)
	}
	if hits != 2 {
		t.Fatalf("webhook hits=%d, want 2", hits)
	}
	if !strings.HasPrefix(gotSig, "sha256=") {
		t.Fatalf("missing hmac signature: %q", gotSig)
	}
	mac := hmac.New(sha256.New, []byte("s3cr3t"))
	mac.Write(gotBody)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if gotSig != want {
		t.Fatalf("signature mismatch: got %q want %q", gotSig, want)
	}
	if repo.polled[7] != "1002" {
		t.Fatalf("high-water mark = %q, want 1002", repo.polled[7])
	}
	for _, e := range repo.events {
		if e.Delivery != domain.XDeliveryDelivered {
			t.Fatalf("event %s delivery = %q, want delivered", e.TweetID, e.Delivery)
		}
	}
}

func TestWatchesDisabledWithoutRepo(t *testing.T) {
	ws := NewXWatchService(nil, NewXService(&fakeXGateway{}, nil))
	if ws.Enabled() {
		t.Fatal("expected disabled without repo")
	}
	if _, err := ws.ListWatches(context.Background()); err != ErrWatchesDisabled {
		t.Fatalf("ListWatches err = %v, want ErrWatchesDisabled", err)
	}
}

func TestRunWatchesSendsCustomHeadersAndVerify(t *testing.T) {
	var gotAuth, gotSig string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotAuth = req.Header.Get("Authorization")
		gotSig = req.Header.Get("X-Zetta-Signature")
		gotBody, _ = io.ReadAll(req.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := &fakeXWatchRepo{due: []port.XWatchWithTenant{{
		TenantID: "default",
		Watch: domain.XWatch{
			ID: 9, Label: "w", Kind: domain.XWatchMention, Query: "me",
			WebhookURL: srv.URL, WebhookSecret: "sk", Active: true,
			WebhookHeaders: map[string]string{"Authorization": "Bearer tok123"},
		},
	}}}
	ws := NewXWatchService(repo, NewXService(&mentionGateway{}, &fakeXAccountRepo{}))

	ws.RunWatches(context.Background(), 10)
	if gotAuth != "Bearer tok123" {
		t.Fatalf("custom Authorization header = %q, want Bearer tok123", gotAuth)
	}
	if !VerifyWebhookSignature("sk", gotBody, gotSig) {
		t.Fatalf("VerifyWebhookSignature failed for sig %q", gotSig)
	}
	if VerifyWebhookSignature("wrong", gotBody, gotSig) {
		t.Fatal("VerifyWebhookSignature accepted wrong secret")
	}
}

func TestWebhookBuiltinHeadersWinOverCustom(t *testing.T) {
	// An agent must not be able to override the signature or content-type.
	var gotCT, gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotCT = req.Header.Get("Content-Type")
		gotSig = req.Header.Get("X-Zetta-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := &fakeXWatchRepo{due: []port.XWatchWithTenant{{
		TenantID: "default",
		Watch: domain.XWatch{
			ID: 11, Label: "w", Kind: domain.XWatchMention, Query: "me",
			WebhookURL: srv.URL, WebhookSecret: "sk", Active: true,
			WebhookHeaders: map[string]string{
				"Content-Type":      "text/plain",
				"X-Zetta-Signature": "sha256=forged",
			},
		},
	}}}
	ws := NewXWatchService(repo, NewXService(&mentionGateway{}, &fakeXAccountRepo{}))
	ws.RunWatches(context.Background(), 10)

	if gotCT != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json (builtin must win)", gotCT)
	}
	if gotSig == "sha256=forged" {
		t.Fatal("forged signature was not overridden by builtin")
	}
}
