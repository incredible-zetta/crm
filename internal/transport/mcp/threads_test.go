package mcptransport

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeThreadsGateway struct {
	profile      domain.ThreadsProfile
	replies      []domain.ThreadsReply
	conversation []domain.ThreadsReply
}

func (f fakeThreadsGateway) Profile(context.Context) (domain.ThreadsProfile, []byte, error) {
	return f.profile, nil, nil
}
func (fakeThreadsGateway) ProfileLookup(context.Context, string) (domain.ThreadsPublicProfile, []byte, error) {
	return domain.ThreadsPublicProfile{}, nil, nil
}
func (fakeThreadsGateway) List(context.Context, int, string) ([]domain.ThreadsPost, string, error) {
	return nil, "", nil
}
func (fakeThreadsGateway) Publish(context.Context, port.ThreadsPublishInput) (port.ThreadsPublishResult, error) {
	return port.ThreadsPublishResult{}, nil
}
func (fakeThreadsGateway) Delete(context.Context, string) error { return nil }
func (fakeThreadsGateway) Insights(context.Context, string) ([]domain.ThreadsInsight, []byte, error) {
	return nil, nil, nil
}
func (fakeThreadsGateway) FollowerDemographics(context.Context, string) (map[string]any, []byte, error) {
	return nil, nil, nil
}
func (f fakeThreadsGateway) Replies(context.Context, string, int, string) ([]domain.ThreadsReply, string, error) {
	return f.replies, "", nil
}
func (f fakeThreadsGateway) Conversation(context.Context, string, int, string) ([]domain.ThreadsReply, string, error) {
	return f.conversation, "", nil
}
func (fakeThreadsGateway) Reply(context.Context, string, string) (string, []byte, error) {
	return "", nil, nil
}
func (fakeThreadsGateway) ManageReply(context.Context, string, bool) ([]byte, error) {
	return nil, nil
}
func (fakeThreadsGateway) ReplyQuota(context.Context) (map[string]any, []byte, error) {
	return nil, nil, nil
}
func (fakeThreadsGateway) Mentions(context.Context, int, string) ([]domain.ThreadsMention, string, error) {
	return nil, "", nil
}
func (fakeThreadsGateway) Search(context.Context, port.ThreadsSearchInput) (map[string]any, []byte, error) {
	return nil, nil, nil
}
func (fakeThreadsGateway) ExchangeToken(context.Context, string) (port.ThreadsTokenResult, error) {
	return port.ThreadsTokenResult{}, nil
}
func (fakeThreadsGateway) RefreshToken(context.Context, string) (port.ThreadsTokenResult, error) {
	return port.ThreadsTokenResult{}, nil
}

// recordingThreadsGateway captures ManageReply and Reply args for tests.
type recordingThreadsGateway struct {
	fakeThreadsGateway
	calledReplyID string
	calledHide    bool
	calls         int
	replyTarget   string
	replyText     string
	replyCalls    int
}

func (r *recordingThreadsGateway) Reply(_ context.Context, mediaID, text string) (string, []byte, error) {
	r.replyTarget = mediaID
	r.replyText = text
	r.replyCalls++
	return "newreply", nil, nil
}

func (r *recordingThreadsGateway) ManageReply(_ context.Context, replyID string, hide bool) ([]byte, error) {
	r.calledReplyID = replyID
	r.calledHide = hide
	r.calls++
	return []byte(`{"success":true}`), nil
}

type fakeThreadsRepo struct{ audit []domain.ThreadsAuditEvent }

func (f fakeThreadsRepo) UpsertPost(context.Context, domain.ThreadsPost) (domain.ThreadsPost, error) {
	return domain.ThreadsPost{}, nil
}
func (f fakeThreadsRepo) GetPost(context.Context, int64) (domain.ThreadsPost, error) {
	return domain.ThreadsPost{}, nil
}
func (f fakeThreadsRepo) GetPostByThreadsID(context.Context, string) (domain.ThreadsPost, error) {
	return domain.ThreadsPost{}, nil
}
func (f fakeThreadsRepo) ListPosts(context.Context, domain.ThreadsListFilter, port.Paging) (port.ThreadsPostPage, error) {
	return port.ThreadsPostPage{}, nil
}
func (f fakeThreadsRepo) SoftDeletePost(context.Context, int64) error { return nil }
func (f fakeThreadsRepo) UpsertReply(context.Context, domain.ThreadsReply) (domain.ThreadsReply, error) {
	return domain.ThreadsReply{}, nil
}
func (f fakeThreadsRepo) UpsertMention(context.Context, domain.ThreadsMention) (domain.ThreadsMention, error) {
	return domain.ThreadsMention{}, nil
}
func (f fakeThreadsRepo) InsertAudit(context.Context, domain.ThreadsAuditEvent) error { return nil }
func (f fakeThreadsRepo) ListAudit(context.Context, port.Paging) (port.ThreadsAuditPage, error) {
	return port.ThreadsAuditPage{Items: f.audit, NextCursor: 7}, nil
}

func TestThreadsReplyTreeNestedHierarchy(t *testing.T) {
	// post p1: aliff replies to post -> callmelords replies to aliff -> bal replies to callmelords.
	// aqil replies to post (no answer from me => needs attention).
	d := &Deps{Svc: &service.Services{Threads: service.NewThreadsService(fakeThreadsGateway{
		profile: domain.ThreadsProfile{Username: "callmelords"},
		conversation: []domain.ThreadsReply{
			{ReplyID: "aliff", PostID: "p1", ParentID: "p1", Username: "aliff", Text: "q1", HasReplies: true},
			{ReplyID: "mine1", PostID: "p1", ParentID: "aliff", Username: "callmelords", Text: "my answer", HasReplies: true},
			{ReplyID: "bal", PostID: "p1", ParentID: "mine1", Username: "bal", Text: "thanks"},
			{ReplyID: "aqil", PostID: "p1", ParentID: "p1", Username: "aqil", Text: "q2"},
		},
	}, fakeThreadsRepo{})}}

	_, out, err := d.ThreadsReplyTree(context.Background(), &mcp.CallToolRequest{}, ThreadsRepliesIn{ThreadsID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.AlreadyReplied || len(out.MyReplies) != 1 || out.MyReplies[0].ReplyID != "mine1" {
		t.Fatalf("expected my reply mine1, got %+v", out.MyReplies)
	}
	// Flat depth-first list: aliff, mine1, bal, aqil.
	if len(out.Items) != 4 {
		t.Fatalf("expected 4 flat reply nodes, got %d", len(out.Items))
	}
	var aliff, aqil *ThreadsReplyNodeOut
	for i := range out.Items {
		switch out.Items[i].ReplyID {
		case "aliff":
			aliff = &out.Items[i]
		case "aqil":
			aqil = &out.Items[i]
		}
	}
	if aliff == nil || aqil == nil {
		t.Fatalf("missing top-level nodes: %+v", out.Items)
	}
	// Flat depth-first order: aliff(1) -> mine1(2) -> bal(3), then aqil(1).
	depth := make(map[string]int)
	node := map[string]ThreadsReplyNodeOut{}
	for _, n := range out.Items {
		depth[n.ReplyID] = n.Depth
		node[n.ReplyID] = n
	}
	if depth["aliff"] != 1 || depth["mine1"] != 2 || depth["bal"] != 3 || depth["aqil"] != 1 {
		t.Fatalf("unexpected depths: %+v", depth)
	}
	if node["mine1"].ParentID != "aliff" || node["bal"].ParentID != "mine1" {
		t.Fatalf("unexpected parent links: %+v", out.Items)
	}
	if !node["mine1"].IsMine {
		t.Fatal("expected mine1 is_mine")
	}
	if !node["bal"].NeedsReply {
		t.Fatal("expected bal (reply to my reply) flagged needs_reply")
	}
	if !node["aqil"].NeedsReply || node["aqil"].IsMine {
		t.Fatalf("expected aqil needs_reply and not mine, got %+v", node["aqil"])
	}
	if out.NeedsReplyCount != 2 {
		t.Fatalf("expected needs_reply_count=2 (aqil + bal), got %d", out.NeedsReplyCount)
	}
}

func TestThreadsHistoryRawJSONReturnsJSONObjectNotByteArray(t *testing.T) {
	created := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	d := &Deps{Svc: &service.Services{Threads: service.NewThreadsService(fakeThreadsGateway{}, fakeThreadsRepo{audit: []domain.ThreadsAuditEvent{{
		ID:        1,
		Action:    "replies",
		ObjectID:  "18593061424004322",
		OK:        true,
		RawJSON:   []byte(`{"ok":true,"action":"replies","object_id":"18593061424004322"}`),
		CreatedAt: created,
	}}})}}

	_, out, err := d.ThreadsHistory(context.Background(), &mcp.CallToolRequest{}, ThreadsHistoryIn{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal output: %v", err)
	}
	var got struct {
		Items []struct {
			RawJSON map[string]any `json:"raw_json"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].RawJSON["action"] != "replies" {
		t.Fatalf("expected raw_json object action=replies, got %s", string(b))
	}
}

// TestRegisterBuildsAllToolSchemas mirrors server boot: AddTool generates an
// output schema per tool and panics on unsupported shapes (e.g. self-
// referential structs). This guards against the threads_reply_tree schema
// cycle regression.
func TestRegisterBuildsAllToolSchemas(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	Register(srv, &Deps{Svc: &service.Services{}})
}

func TestThreadsReplyHideUnhideCallGatewayWithFlag(t *testing.T) {
	gw := &recordingThreadsGateway{}
	d := &Deps{Svc: &service.Services{Threads: service.NewThreadsService(gw, fakeThreadsRepo{})}}

	if _, out, err := d.ThreadsReplyHide(context.Background(), &mcp.CallToolRequest{}, ThreadsManageReplyIn{ReplyID: "r1"}); err != nil || !out.OK {
		t.Fatalf("hide: err=%v ok=%v", err, out.OK)
	}
	if gw.calledReplyID != "r1" || !gw.calledHide {
		t.Fatalf("hide: expected (r1,true), got (%q,%v)", gw.calledReplyID, gw.calledHide)
	}

	if _, out, err := d.ThreadsReplyUnhide(context.Background(), &mcp.CallToolRequest{}, ThreadsManageReplyIn{ReplyID: "r2"}); err != nil || !out.OK {
		t.Fatalf("unhide: err=%v ok=%v", err, out.OK)
	}
	if gw.calledReplyID != "r2" || gw.calledHide {
		t.Fatalf("unhide: expected (r2,false), got (%q,%v)", gw.calledReplyID, gw.calledHide)
	}
	if gw.calls != 2 {
		t.Fatalf("expected 2 gateway calls, got %d", gw.calls)
	}
}

func TestThreadsReplyHideRequiresReplyID(t *testing.T) {
	gw := &recordingThreadsGateway{}
	d := &Deps{Svc: &service.Services{Threads: service.NewThreadsService(gw, fakeThreadsRepo{})}}
	res, _, err := d.ThreadsReplyHide(context.Background(), &mcp.CallToolRequest{}, ThreadsManageReplyIn{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected validation error result, got %+v", res)
	}
	if gw.calls != 0 {
		t.Fatalf("gateway should not be called on validation failure, got %d", gw.calls)
	}
}

func TestThreadsReplyPrefersReplyIDOverThreadsID(t *testing.T) {
	gw := &recordingThreadsGateway{}
	d := &Deps{Svc: &service.Services{Threads: service.NewThreadsService(gw, fakeThreadsRepo{})}}

	// AI passes both the root post id and the target comment id. The reply must
	// nest under the comment (reply_id), not the post root (threads_id).
	_, out, err := d.ThreadsReply(context.Background(), &mcp.CallToolRequest{}, ThreadsReplyIn{
		ThreadsID: "post_root_id",
		ReplyID:   "comment_id",
		Text:      "nested answer",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ReplyID != "newreply" {
		t.Fatalf("expected reply id newreply, got %q", out.ReplyID)
	}
	if gw.replyTarget != "comment_id" {
		t.Fatalf("expected reply target comment_id, got %q", gw.replyTarget)
	}
}

func TestThreadsReplyFallsBackToThreadsID(t *testing.T) {
	gw := &recordingThreadsGateway{}
	d := &Deps{Svc: &service.Services{Threads: service.NewThreadsService(gw, fakeThreadsRepo{})}}

	_, _, err := d.ThreadsReply(context.Background(), &mcp.CallToolRequest{}, ThreadsReplyIn{
		ThreadsID: "post_root_id",
		Text:      "top-level reply",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gw.replyTarget != "post_root_id" {
		t.Fatalf("expected reply target post_root_id, got %q", gw.replyTarget)
	}
}

// summaryGateway returns canned posts/insights/conversation for the daily
// summary test. Insights are keyed by media id so different posts can carry
// different metric values.
type summaryGateway struct {
	fakeThreadsGateway
	posts        []domain.ThreadsPost
	insights     map[string][]domain.ThreadsInsight
	conversation map[string][]domain.ThreadsReply
}

func (g summaryGateway) List(context.Context, int, string) ([]domain.ThreadsPost, string, error) {
	return g.posts, "", nil
}
func (g summaryGateway) Insights(_ context.Context, mediaID string) ([]domain.ThreadsInsight, []byte, error) {
	return g.insights[mediaID], nil, nil
}
func (g summaryGateway) Conversation(_ context.Context, mediaID string, _ int, _ string) ([]domain.ThreadsReply, string, error) {
	return g.conversation[mediaID], "", nil
}

func mediaInsight(name string, value float64) domain.ThreadsInsight {
	return domain.ThreadsInsight{Name: name, RawValue: map[string]any{
		"name":   name,
		"values": []any{map[string]any{"value": value}},
	}}
}

func TestThreadsDailySummaryAggregatesInsightsAndReplies(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Date(2026, 6, 27, 10, 0, 0, 0, loc)
	yesterday := today.AddDate(0, 0, -1)

	gw := summaryGateway{
		fakeThreadsGateway: fakeThreadsGateway{profile: domain.ThreadsProfile{
			Username:       "callmelords",
			FollowersCount: ptrInt64(105),
		}},
		posts: []domain.ThreadsPost{
			{ThreadsID: "p1", Text: "today post", MediaType: "TEXT_POST", Timestamp: &today},
			{ThreadsID: "old", Text: "yesterday post", Timestamp: &yesterday}, // out of window
		},
		insights: map[string][]domain.ThreadsInsight{
			"p1": {
				mediaInsight("views", 962),
				mediaInsight("likes", 3),
				mediaInsight("reposts", 1),
				mediaInsight("quotes", 0),
				mediaInsight("replies", 9),
			},
		},
		conversation: map[string][]domain.ThreadsReply{
			// 2 others' comments; 1 answered by me, 1 not. Plus my own reply.
			"p1": {
				{ReplyID: "c1", Username: "alice", ParentID: "p1"},
				{ReplyID: "c2", Username: "bob", ParentID: "p1"},
				{ReplyID: "m1", Username: "callmelords", ParentID: "c1"},
			},
		},
	}

	d := &Deps{Svc: &service.Services{Threads: service.NewThreadsService(gw, fakeThreadsRepo{})}}
	_, out, err := d.ThreadsDailySummary(context.Background(), &mcp.CallToolRequest{},
		ThreadsDailySummaryIn{Date: "2026-06-27", Timezone: "Asia/Jakarta"})
	if err != nil {
		t.Fatalf("daily summary: %v", err)
	}

	if out.AuthenticatedAs != "callmelords" || out.FollowersCount == nil || *out.FollowersCount != 105 {
		t.Fatalf("identity/followers wrong: %+v", out)
	}
	if len(out.Posts) != 1 {
		t.Fatalf("expected 1 post in window, got %d", len(out.Posts))
	}
	p := out.Posts[0]
	if p.Views != 962 || p.Likes != 3 || p.Reposts != 1 || p.RepliesMetric != 9 {
		t.Fatalf("insights wrong: %+v", p)
	}
	if p.TotalReplies != 3 || p.MyReplies != 1 || p.OtherReplies != 2 {
		t.Fatalf("reply breakdown wrong: total=%d mine=%d others=%d", p.TotalReplies, p.MyReplies, p.OtherReplies)
	}
	// c1 answered by me, c2 not -> needs_reply=1.
	if p.NeedsReply != 1 {
		t.Fatalf("needs_reply expected 1, got %d", p.NeedsReply)
	}
	// engagement = likes+reposts+quotes+other_replies = 3+1+0+2 = 6.
	if p.Engagement != 6 {
		t.Fatalf("engagement expected 6, got %d", p.Engagement)
	}
	if out.Totals.Posts != 1 || out.Totals.Engagement != 6 || out.Totals.Views != 962 {
		t.Fatalf("totals wrong: %+v", out.Totals)
	}
}

func ptrInt64(v int64) *int64 { return &v }
