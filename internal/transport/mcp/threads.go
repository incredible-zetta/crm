package mcptransport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ThreadsLimitCursorIn struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type ThreadsProfileOut struct {
	ID         string `json:"id,omitempty"`
	Username   string `json:"username,omitempty"`
	Name       string `json:"name,omitempty"`
	PictureURL string `json:"picture_url,omitempty"`
	Biography  string `json:"biography,omitempty"`
}

func (d *Deps) ThreadsProfile(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, ThreadsProfileOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsProfileOut{}, nil
	}
	p, err := d.Svc.Threads.Profile(ctx)
	if err != nil {
		return nil, ThreadsProfileOut{}, fmt.Errorf("threads_profile: %w", err)
	}
	return nil, ThreadsProfileOut{ID: p.ID, Username: p.Username, Name: p.Name, PictureURL: p.PictureURL, Biography: p.Biography}, nil
}

type ThreadsListOut struct {
	Items      []ThreadsPostOut `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type ThreadsPostOut struct {
	ID               int64  `json:"id,omitempty"`
	ThreadsID        string `json:"threads_id"`
	MediaProductType string `json:"media_product_type,omitempty"`
	MediaType        string `json:"media_type,omitempty"`
	Text             string `json:"text,omitempty"`
	Permalink        string `json:"permalink,omitempty"`
	Timestamp        string `json:"timestamp,omitempty"`
	Username         string `json:"username,omitempty"`
	IsQuotePost      bool   `json:"is_quote_post,omitempty"`
}

func (d *Deps) ThreadsList(ctx context.Context, req *mcp.CallToolRequest, in ThreadsLimitCursorIn) (*mcp.CallToolResult, ThreadsListOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsListOut{}, nil
	}
	items, next, err := d.Svc.Threads.List(ctx, in.Limit, in.Cursor)
	if err != nil {
		return nil, ThreadsListOut{}, fmt.Errorf("threads_list: %w", err)
	}
	return nil, ThreadsListOut{Items: toThreadsPostOuts(items), NextCursor: next}, nil
}

type ThreadsPublishIn struct {
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	VideoURL string `json:"video_url,omitempty"`
}

type ThreadsPublishOut struct {
	ContainerID string         `json:"container_id"`
	Post        ThreadsPostOut `json:"post"`
}

func (d *Deps) ThreadsPublish(ctx context.Context, req *mcp.CallToolRequest, in ThreadsPublishIn) (*mcp.CallToolResult, ThreadsPublishOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsPublishOut{}, nil
	}
	if in.Text == "" && in.ImageURL == "" && in.VideoURL == "" {
		return mcpserver.Err("validation", "text, image_url, or video_url required"), ThreadsPublishOut{}, nil
	}
	res, err := d.Svc.Threads.Publish(ctx, port.ThreadsPublishInput{Text: in.Text, ImageURL: in.ImageURL, VideoURL: in.VideoURL})
	if err != nil {
		return nil, ThreadsPublishOut{}, fmt.Errorf("threads_publish: %w", err)
	}
	return nil, ThreadsPublishOut{ContainerID: res.ContainerID, Post: toThreadsPostOut(res.Post)}, nil
}

type ThreadsDeleteIn struct {
	ID        int64  `json:"id,omitempty"`
	ThreadsID string `json:"threads_id,omitempty"`
}

type ThreadsOKOut struct {
	OK bool `json:"ok"`
}

func (d *Deps) ThreadsDelete(ctx context.Context, req *mcp.CallToolRequest, in ThreadsDeleteIn) (*mcp.CallToolResult, ThreadsOKOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsOKOut{}, nil
	}
	if in.ID == 0 && in.ThreadsID == "" {
		return mcpserver.Err("validation", "id or threads_id required"), ThreadsOKOut{}, nil
	}
	if err := d.Svc.Threads.Delete(ctx, in.ThreadsID, in.ID); err != nil {
		return nil, ThreadsOKOut{}, fmt.Errorf("threads_delete: %w", err)
	}
	return nil, ThreadsOKOut{OK: true}, nil
}

type ThreadsInsightsIn struct {
	ThreadsID string `json:"threads_id,omitempty"`
}

type ThreadsInsightsOut struct {
	Items []domain.ThreadsInsight `json:"items"`
}

func (d *Deps) ThreadsInsights(ctx context.Context, req *mcp.CallToolRequest, in ThreadsInsightsIn) (*mcp.CallToolResult, ThreadsInsightsOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsInsightsOut{}, nil
	}
	items, err := d.Svc.Threads.Insights(ctx, in.ThreadsID)
	if err != nil {
		return nil, ThreadsInsightsOut{}, fmt.Errorf("threads_insights: %w", err)
	}
	return nil, ThreadsInsightsOut{Items: items}, nil
}

type ThreadsRepliesIn struct {
	ThreadsID string `json:"threads_id"`
	Limit     int    `json:"limit,omitempty"`
	Cursor    string `json:"cursor,omitempty"`
}

type ThreadsReplyItemOut struct {
	ReplyID    string `json:"reply_id"`
	PostID     string `json:"post_id"`
	Text       string `json:"text,omitempty"`
	Username   string `json:"username,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
	HideStatus string `json:"hide_status,omitempty"`
}

type ThreadsRepliesOut struct {
	Items      []ThreadsReplyItemOut `json:"items"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

func (d *Deps) ThreadsReplies(ctx context.Context, req *mcp.CallToolRequest, in ThreadsRepliesIn) (*mcp.CallToolResult, ThreadsRepliesOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsRepliesOut{}, nil
	}
	if in.ThreadsID == "" {
		return mcpserver.Err("validation", "threads_id required"), ThreadsRepliesOut{}, nil
	}
	items, next, err := d.Svc.Threads.Replies(ctx, in.ThreadsID, in.Limit, in.Cursor)
	if err != nil {
		return nil, ThreadsRepliesOut{}, fmt.Errorf("threads_replies: %w", err)
	}
	out := make([]ThreadsReplyItemOut, len(items))
	for i, item := range items {
		out[i] = toThreadsReplyOut(item)
	}
	return nil, ThreadsRepliesOut{Items: out, NextCursor: next}, nil
}

type ThreadsReplyIn struct {
	ThreadsID string `json:"threads_id"`
	Text      string `json:"text"`
}

type ThreadsReplyOut struct {
	ReplyID string `json:"reply_id"`
}

func (d *Deps) ThreadsReply(ctx context.Context, req *mcp.CallToolRequest, in ThreadsReplyIn) (*mcp.CallToolResult, ThreadsReplyOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsReplyOut{}, nil
	}
	if in.ThreadsID == "" || in.Text == "" {
		return mcpserver.Err("validation", "threads_id and text required"), ThreadsReplyOut{}, nil
	}
	id, err := d.Svc.Threads.Reply(ctx, in.ThreadsID, in.Text)
	if err != nil {
		return nil, ThreadsReplyOut{}, fmt.Errorf("threads_reply: %w", err)
	}
	return nil, ThreadsReplyOut{ReplyID: id}, nil
}

type ThreadsReplyQuotaOut struct {
	Result map[string]any `json:"result"`
}

func (d *Deps) ThreadsReplyQuota(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, ThreadsReplyQuotaOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsReplyQuotaOut{}, nil
	}
	out, err := d.Svc.Threads.ReplyQuota(ctx)
	if err != nil {
		return nil, ThreadsReplyQuotaOut{}, fmt.Errorf("threads_reply_quota: %w", err)
	}
	return nil, ThreadsReplyQuotaOut{Result: out}, nil
}

type ThreadsMentionsOut struct {
	Items      []domain.ThreadsMention `json:"items"`
	NextCursor string                  `json:"next_cursor,omitempty"`
}

func (d *Deps) ThreadsMentions(ctx context.Context, req *mcp.CallToolRequest, in ThreadsLimitCursorIn) (*mcp.CallToolResult, ThreadsMentionsOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsMentionsOut{}, nil
	}
	items, next, err := d.Svc.Threads.Mentions(ctx, in.Limit, in.Cursor)
	if err != nil {
		return nil, ThreadsMentionsOut{}, fmt.Errorf("threads_mentions: %w", err)
	}
	return nil, ThreadsMentionsOut{Items: items, NextCursor: next}, nil
}

type ThreadsSearchIn struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type ThreadsSearchOut struct {
	Result map[string]any `json:"result"`
}

func (d *Deps) ThreadsSearch(ctx context.Context, req *mcp.CallToolRequest, in ThreadsSearchIn) (*mcp.CallToolResult, ThreadsSearchOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsSearchOut{}, nil
	}
	if in.Query == "" {
		return mcpserver.Err("validation", "query required"), ThreadsSearchOut{}, nil
	}
	out, err := d.Svc.Threads.Search(ctx, in.Query, in.Limit, in.Cursor)
	if err != nil {
		return nil, ThreadsSearchOut{}, fmt.Errorf("threads_search: %w", err)
	}
	return nil, ThreadsSearchOut{Result: out}, nil
}

type ThreadsCachedListIn struct {
	Username string `json:"username,omitempty"`
	Q        string `json:"q,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Cursor   int64  `json:"cursor,omitempty"`
}

type ThreadsCachedListOut struct {
	Items      []ThreadsPostOut `json:"items"`
	NextCursor int64            `json:"next_cursor"`
}

func (d *Deps) ThreadsListCached(ctx context.Context, req *mcp.CallToolRequest, in ThreadsCachedListIn) (*mcp.CallToolResult, ThreadsCachedListOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsCachedListOut{}, nil
	}
	page, err := d.Svc.Threads.ListCached(ctx, domain.ThreadsListFilter{Username: in.Username, Q: in.Q}, port.Paging{Limit: in.Limit, Cursor: in.Cursor})
	if err != nil {
		return nil, ThreadsCachedListOut{}, fmt.Errorf("threads_list_cached: %w", err)
	}
	return nil, ThreadsCachedListOut{Items: toThreadsPostOuts(page.Items), NextCursor: page.NextCursor}, nil
}

type ThreadsGetCachedIn struct {
	ID        int64  `json:"id,omitempty"`
	ThreadsID string `json:"threads_id,omitempty"`
}

func (d *Deps) ThreadsGetCached(ctx context.Context, req *mcp.CallToolRequest, in ThreadsGetCachedIn) (*mcp.CallToolResult, ThreadsPostOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsPostOut{}, nil
	}
	post, err := d.Svc.Threads.GetCached(ctx, in.ID, in.ThreadsID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "threads post not found"), ThreadsPostOut{}, nil
		}
		if errors.Is(err, domain.ErrValidation) {
			return mcpserver.Err("validation", err.Error()), ThreadsPostOut{}, nil
		}
		return nil, ThreadsPostOut{}, fmt.Errorf("threads_get_cached: %w", err)
	}
	return nil, toThreadsPostOut(post), nil
}

type ThreadsHistoryIn struct {
	Limit  int   `json:"limit,omitempty"`
	Cursor int64 `json:"cursor,omitempty"`
}

type ThreadsHistoryOut struct {
	Items      []domain.ThreadsAuditEvent `json:"items"`
	NextCursor int64                      `json:"next_cursor"`
}

func (d *Deps) ThreadsHistory(ctx context.Context, req *mcp.CallToolRequest, in ThreadsHistoryIn) (*mcp.CallToolResult, ThreadsHistoryOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsHistoryOut{}, nil
	}
	page, err := d.Svc.Threads.History(ctx, port.Paging{Limit: in.Limit, Cursor: in.Cursor})
	if err != nil {
		return nil, ThreadsHistoryOut{}, fmt.Errorf("threads_history: %w", err)
	}
	return nil, ThreadsHistoryOut{Items: page.Items, NextCursor: page.NextCursor}, nil
}

func (d *Deps) ThreadsDeleteCached(ctx context.Context, req *mcp.CallToolRequest, in ThreadsDeleteIn) (*mcp.CallToolResult, ThreadsOKOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsOKOut{}, nil
	}
	if in.ID == 0 && in.ThreadsID == "" {
		return mcpserver.Err("validation", "id or threads_id required"), ThreadsOKOut{}, nil
	}
	if err := d.Svc.Threads.Delete(ctx, "", in.ID); err != nil {
		return nil, ThreadsOKOut{}, fmt.Errorf("threads_delete_cached: %w", err)
	}
	if in.ID == 0 && in.ThreadsID != "" {
		post, err := d.Svc.Threads.GetCached(ctx, 0, in.ThreadsID)
		if err != nil {
			return nil, ThreadsOKOut{}, fmt.Errorf("threads_delete_cached: %w", err)
		}
		if err := d.Svc.Threads.Delete(ctx, "", post.ID); err != nil {
			return nil, ThreadsOKOut{}, fmt.Errorf("threads_delete_cached: %w", err)
		}
	}
	return nil, ThreadsOKOut{OK: true}, nil
}

func toThreadsPostOuts(items []domain.ThreadsPost) []ThreadsPostOut {
	out := make([]ThreadsPostOut, len(items))
	for i, item := range items {
		out[i] = toThreadsPostOut(item)
	}
	return out
}

func toThreadsPostOut(p domain.ThreadsPost) ThreadsPostOut {
	out := ThreadsPostOut{ID: p.ID, ThreadsID: p.ThreadsID, MediaProductType: p.MediaProductType, MediaType: p.MediaType, Text: p.Text, Permalink: p.Permalink, Username: p.Username, IsQuotePost: p.IsQuotePost}
	if p.Timestamp != nil {
		out.Timestamp = p.Timestamp.Format(time.RFC3339)
	}
	return out
}

func toThreadsReplyOut(r domain.ThreadsReply) ThreadsReplyItemOut {
	out := ThreadsReplyItemOut{ReplyID: r.ReplyID, PostID: r.PostID, Text: r.Text, Username: r.Username, HideStatus: r.HideStatus}
	if r.Timestamp != nil {
		out.Timestamp = r.Timestamp.Format(time.RFC3339)
	}
	return out
}
