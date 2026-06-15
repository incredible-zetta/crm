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
func (f fakeThreadsGateway) Replies(context.Context, string, int, string) ([]domain.ThreadsReply, string, error) {
	return f.replies, "", nil
}
func (f fakeThreadsGateway) Conversation(context.Context, string, int, string) ([]domain.ThreadsReply, string, error) {
	return f.conversation, "", nil
}
func (fakeThreadsGateway) Reply(context.Context, string, string) (string, []byte, error) {
	return "", nil, nil
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
	// top level: aliff + aqil
	if len(out.Items) != 2 {
		t.Fatalf("expected 2 top-level replies, got %d", len(out.Items))
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
	if aliff.Depth != 1 || len(aliff.Children) != 1 || aliff.Children[0].ReplyID != "mine1" {
		t.Fatalf("expected aliff -> mine1, got %+v", aliff)
	}
	mine := aliff.Children[0]
	if !mine.IsMine || mine.Depth != 2 || len(mine.Children) != 1 || mine.Children[0].ReplyID != "bal" {
		t.Fatalf("expected mine1 -> bal nested, got %+v", mine)
	}
	if !mine.Children[0].NeedsReply {
		t.Fatal("expected bal (reply to my reply) flagged needs_reply")
	}
	if !aqil.NeedsReply || aqil.IsMine {
		t.Fatalf("expected aqil needs_reply and not mine, got %+v", aqil)
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
