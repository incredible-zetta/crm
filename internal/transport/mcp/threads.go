package mcptransport

import (
	"context"
	"encoding/json"
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
	ID             string `json:"id,omitempty"`
	Username       string `json:"username,omitempty"`
	Name           string `json:"name,omitempty"`
	PictureURL     string `json:"picture_url,omitempty"`
	Biography      string `json:"biography,omitempty"`
	FollowersCount *int64 `json:"followers_count,omitempty"`
}

func (d *Deps) ThreadsProfile(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, ThreadsProfileOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsProfileOut{}, nil
	}
	p, err := d.Svc.Threads.Profile(ctx)
	if err != nil {
		return nil, ThreadsProfileOut{}, fmt.Errorf("threads_profile: %w", err)
	}
	return nil, ThreadsProfileOut{ID: p.ID, Username: p.Username, Name: p.Name, PictureURL: p.PictureURL, Biography: p.Biography, FollowersCount: p.FollowersCount}, nil
}

type ThreadsProfileLookupIn struct {
	Username string `json:"username" jsonschema:"Exact Threads handle to look up (with or without @). Public profiles only."`
}

type ThreadsPublicProfileOut struct {
	Username      string `json:"username,omitempty"`
	Name          string `json:"name,omitempty"`
	Biography     string `json:"biography,omitempty"`
	PictureURL    string `json:"picture_url,omitempty"`
	IsVerified    *bool  `json:"is_verified,omitempty"`
	FollowerCount *int64 `json:"follower_count,omitempty"`
	LikesCount    *int64 `json:"likes_count,omitempty"`
	QuotesCount   *int64 `json:"quotes_count,omitempty"`
	RepliesCount  *int64 `json:"replies_count,omitempty"`
	RepostsCount  *int64 `json:"reposts_count,omitempty"`
	ViewsCount    *int64 `json:"views_count,omitempty"`
}

func (d *Deps) ThreadsProfileLookup(ctx context.Context, req *mcp.CallToolRequest, in ThreadsProfileLookupIn) (*mcp.CallToolResult, ThreadsPublicProfileOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsPublicProfileOut{}, nil
	}
	if in.Username == "" {
		return mcpserver.Err("validation", "username required"), ThreadsPublicProfileOut{}, nil
	}
	p, err := d.Svc.Threads.ProfileLookup(ctx, in.Username)
	if err != nil {
		return nil, ThreadsPublicProfileOut{}, fmt.Errorf("threads_profile_lookup: %w", err)
	}
	return nil, ThreadsPublicProfileOut{
		Username:      p.Username,
		Name:          p.Name,
		Biography:     p.Biography,
		PictureURL:    p.PictureURL,
		IsVerified:    p.IsVerified,
		FollowerCount: p.FollowerCount,
		LikesCount:    p.LikesCount,
		QuotesCount:   p.QuotesCount,
		RepliesCount:  p.RepliesCount,
		RepostsCount:  p.RepostsCount,
		ViewsCount:    p.ViewsCount,
	}, nil
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
	TopicTag         string `json:"topic_tag,omitempty"`
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
	TopicTag string `json:"topic_tag,omitempty" jsonschema:"Optional Threads topic tag, 1-50 chars; disallows . and &"`
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
	res, err := d.Svc.Threads.Publish(ctx, port.ThreadsPublishInput{Text: in.Text, ImageURL: in.ImageURL, VideoURL: in.VideoURL, TopicTag: in.TopicTag})
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

type ThreadsFollowerDemographicsIn struct {
	Breakdown string `json:"breakdown,omitempty" jsonschema:"Demographic breakdown dimension: country (default), city, age, or gender. Requires the profile to have at least 100 followers."`
}

type ThreadsFollowerDemographicsOut struct {
	Result map[string]any `json:"result"`
}

func (d *Deps) ThreadsFollowerDemographics(ctx context.Context, req *mcp.CallToolRequest, in ThreadsFollowerDemographicsIn) (*mcp.CallToolResult, ThreadsFollowerDemographicsOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsFollowerDemographicsOut{}, nil
	}
	out, err := d.Svc.Threads.FollowerDemographics(ctx, in.Breakdown)
	if err != nil {
		return nil, ThreadsFollowerDemographicsOut{}, fmt.Errorf("threads_follower_demographics: %w", err)
	}
	return nil, ThreadsFollowerDemographicsOut{Result: out}, nil
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

type ThreadsReplyTreeItemOut struct {
	ThreadsReplyItemOut
	ParentID string `json:"parent_id,omitempty"`
	Depth    int    `json:"depth"`
	IsMine   bool   `json:"is_mine"`
}

type ThreadsReplyNodeOut struct {
	ThreadsReplyItemOut
	ParentID   string `json:"parent_id,omitempty"`
	Depth      int    `json:"depth"`
	IsMine     bool   `json:"is_mine"`
	NeedsReply bool   `json:"needs_reply"`
}

type ThreadsReplyTreeOut struct {
	ThreadsID       string                    `json:"threads_id"`
	AuthenticatedAs string                    `json:"authenticated_as,omitempty"`
	AlreadyReplied  bool                      `json:"already_replied"`
	NeedsReplyCount int                       `json:"needs_reply_count"`
	MyReplies       []ThreadsReplyTreeItemOut `json:"my_replies"`
	Items           []ThreadsReplyNodeOut     `json:"items"`
	NextCursor      string                    `json:"next_cursor,omitempty"`
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

func (d *Deps) ThreadsReplyTree(ctx context.Context, req *mcp.CallToolRequest, in ThreadsRepliesIn) (*mcp.CallToolResult, ThreadsReplyTreeOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsReplyTreeOut{}, nil
	}
	if in.ThreadsID == "" {
		return mcpserver.Err("validation", "threads_id required"), ThreadsReplyTreeOut{}, nil
	}
	profile, err := d.Svc.Threads.Profile(ctx)
	if err != nil {
		return nil, ThreadsReplyTreeOut{}, fmt.Errorf("threads_reply_tree profile: %w", err)
	}
	items, next, err := d.Svc.Threads.Conversation(ctx, in.ThreadsID, in.Limit, in.Cursor)
	if err != nil {
		return nil, ThreadsReplyTreeOut{}, fmt.Errorf("threads_reply_tree conversation: %w", err)
	}

	me := profile.Username
	out := ThreadsReplyTreeOut{ThreadsID: in.ThreadsID, AuthenticatedAs: me, NextCursor: next}

	// Index replies by id and remember input order.
	type rec struct {
		item     domain.ThreadsReply
		isMine   bool
		parentID string
	}
	byID := make(map[string]rec, len(items))
	order := make([]string, 0, len(items))
	children := make(map[string][]string)
	for _, item := range items {
		isMine := me != "" && item.Username == me
		byID[item.ReplyID] = rec{item: item, isMine: isMine, parentID: item.ParentID}
		order = append(order, item.ReplyID)
		children[item.ParentID] = append(children[item.ParentID], item.ReplyID)
		if isMine {
			out.AlreadyReplied = true
			out.MyReplies = append(out.MyReplies, ThreadsReplyTreeItemOut{ThreadsReplyItemOut: toThreadsReplyOut(item), ParentID: item.ParentID, IsMine: true})
		}
	}

	// A comment is answered if it has a direct child authored by me.
	hasMyChild := make(map[string]bool)
	for _, id := range order {
		r := byID[id]
		if r.isMine && r.parentID != "" {
			hasMyChild[r.parentID] = true
		}
	}

	// Roots: parent is the post itself or not present in the result set.
	var roots []string
	for _, id := range order {
		r := byID[id]
		if _, ok := byID[r.parentID]; ok && r.parentID != "" {
			continue
		}
		roots = append(roots, id)
	}

	// Flatten depth-first into a parent_id/depth ordered list (schema-safe; the
	// caller reconstructs the tree from parent_id + depth). Avoids a self-
	// referential Children field, which the MCP schema generator rejects.
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		r := byID[id]
		node := ThreadsReplyNodeOut{
			ThreadsReplyItemOut: toThreadsReplyOut(r.item),
			ParentID:            r.parentID,
			Depth:               depth,
			IsMine:              r.isMine,
			NeedsReply:          !r.isMine && !hasMyChild[id],
		}
		if node.NeedsReply {
			out.NeedsReplyCount++
		}
		out.Items = append(out.Items, node)
		for _, cid := range children[id] {
			walk(cid, depth+1)
		}
	}
	for _, id := range roots {
		walk(id, 1)
	}
	return nil, out, nil
}

type ThreadsReplyIn struct {
	ThreadsID string `json:"threads_id,omitempty" jsonschema:"Target id to reply UNDER. To reply to someone's COMMENT, pass that comment's reply_id (from threads_reply_tree/threads_replies items), NOT the root post id. Passing the root post id replies to your own post, not the comment. If both reply_id and threads_id are set, reply_id wins."`
	ReplyID   string `json:"reply_id,omitempty" jsonschema:"The reply/comment id to respond to (from threads_reply_tree/threads_replies items). Prefer this when replying to a comment in a thread. Takes precedence over threads_id when both are provided, so a nested reply lands under the comment instead of at the post root."`
	Text      string `json:"text" jsonschema:"Reply body text"`
}

type ThreadsReplyOut struct {
	ReplyID string `json:"reply_id"`
}

func (d *Deps) ThreadsReply(ctx context.Context, req *mcp.CallToolRequest, in ThreadsReplyIn) (*mcp.CallToolResult, ThreadsReplyOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsReplyOut{}, nil
	}
	// reply_id is the more specific target (a comment). When both are set, it
	// must win so a reply nests under the comment instead of the post root.
	// Passing the root post id in threads_id alongside a comment reply_id was
	// flattening replies to depth 1.
	target := in.ReplyID
	if target == "" {
		target = in.ThreadsID
	}
	if target == "" || in.Text == "" {
		return mcpserver.Err("validation", "threads_id (or reply_id) and text required"), ThreadsReplyOut{}, nil
	}
	id, err := d.Svc.Threads.Reply(ctx, target, in.Text)
	if err != nil {
		return nil, ThreadsReplyOut{}, fmt.Errorf("threads_reply: %w", err)
	}
	return nil, ThreadsReplyOut{ReplyID: id}, nil
}

type ThreadsManageReplyIn struct {
	ReplyID string `json:"reply_id" jsonschema:"The reply/comment id to moderate (from threads_reply_tree/threads_replies items). Must be a reply on YOUR post; you cannot hide replies on other people's posts."`
}

func (d *Deps) ThreadsReplyHide(ctx context.Context, req *mcp.CallToolRequest, in ThreadsManageReplyIn) (*mcp.CallToolResult, ThreadsOKOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsOKOut{}, nil
	}
	if in.ReplyID == "" {
		return mcpserver.Err("validation", "reply_id required"), ThreadsOKOut{}, nil
	}
	if err := d.Svc.Threads.HideReply(ctx, in.ReplyID); err != nil {
		return nil, ThreadsOKOut{}, fmt.Errorf("threads_reply_hide: %w", err)
	}
	return nil, ThreadsOKOut{OK: true}, nil
}

func (d *Deps) ThreadsReplyUnhide(ctx context.Context, req *mcp.CallToolRequest, in ThreadsManageReplyIn) (*mcp.CallToolResult, ThreadsOKOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsOKOut{}, nil
	}
	if in.ReplyID == "" {
		return mcpserver.Err("validation", "reply_id required"), ThreadsOKOut{}, nil
	}
	if err := d.Svc.Threads.UnhideReply(ctx, in.ReplyID); err != nil {
		return nil, ThreadsOKOut{}, fmt.Errorf("threads_reply_unhide: %w", err)
	}
	return nil, ThreadsOKOut{OK: true}, nil
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
	Query          string `json:"query"`
	SearchType     string `json:"search_type,omitempty" jsonschema:"TOP (default) or RECENT"`
	SearchMode     string `json:"search_mode,omitempty" jsonschema:"KEYWORD (default) or TAG for topic tag search"`
	MediaType      string `json:"media_type,omitempty" jsonschema:"TEXT, IMAGE, or VIDEO"`
	AuthorUsername string `json:"author_username,omitempty" jsonschema:"Exact username filter, with or without @"`
	Since          string `json:"since,omitempty" jsonschema:"Unix timestamp or parseable date/time"`
	Until          string `json:"until,omitempty" jsonschema:"Unix timestamp or parseable date/time"`
	Fields         string `json:"fields,omitempty" jsonschema:"Comma-separated fields; default includes id,text,media_type,permalink,timestamp,username,has_replies,is_quote_post,is_reply,topic_tag"`
	Limit          int    `json:"limit,omitempty"`
	Cursor         string `json:"cursor,omitempty"`
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
	out, err := d.Svc.Threads.Search(ctx, port.ThreadsSearchInput{Query: in.Query, SearchType: in.SearchType, SearchMode: in.SearchMode, MediaType: in.MediaType, AuthorUsername: in.AuthorUsername, Since: in.Since, Until: in.Until, Fields: in.Fields, Limit: in.Limit, Cursor: in.Cursor})
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

type ThreadsTokenIn struct {
	AccessToken string `json:"access_token,omitempty" jsonschema:"Optional token override. Defaults to THREADS_ACCESS_TOKEN. Returned token is sensitive."`
}

type ThreadsTokenOut struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

func (d *Deps) ThreadsTokenExchange(ctx context.Context, req *mcp.CallToolRequest, in ThreadsTokenIn) (*mcp.CallToolResult, ThreadsTokenOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsTokenOut{}, nil
	}
	out, err := d.Svc.Threads.ExchangeToken(ctx, in.AccessToken)
	if err != nil {
		return nil, ThreadsTokenOut{}, fmt.Errorf("threads_token_exchange: %w", err)
	}
	return nil, toThreadsTokenOut(out), nil
}

func (d *Deps) ThreadsTokenRefresh(ctx context.Context, req *mcp.CallToolRequest, in ThreadsTokenIn) (*mcp.CallToolResult, ThreadsTokenOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsTokenOut{}, nil
	}
	out, err := d.Svc.Threads.RefreshToken(ctx, in.AccessToken)
	if err != nil {
		return nil, ThreadsTokenOut{}, fmt.Errorf("threads_token_refresh: %w", err)
	}
	return nil, toThreadsTokenOut(out), nil
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

type ThreadsAuditEventOut struct {
	ID        int64           `json:"id"`
	Action    string          `json:"action"`
	ObjectID  string          `json:"object_id,omitempty"`
	OK        bool            `json:"ok"`
	Error     string          `json:"error,omitempty"`
	RawJSON   json.RawMessage `json:"raw_json,omitempty"`
	CreatedAt string          `json:"created_at,omitempty"`
}

type ThreadsHistoryOut struct {
	Items      []ThreadsAuditEventOut `json:"items"`
	NextCursor int64                  `json:"next_cursor"`
}

func (d *Deps) ThreadsHistory(ctx context.Context, req *mcp.CallToolRequest, in ThreadsHistoryIn) (*mcp.CallToolResult, ThreadsHistoryOut, error) {
	if d.Svc.Threads == nil {
		return mcpserver.Err("disabled", "threads channel not configured"), ThreadsHistoryOut{}, nil
	}
	page, err := d.Svc.Threads.History(ctx, port.Paging{Limit: in.Limit, Cursor: in.Cursor})
	if err != nil {
		return nil, ThreadsHistoryOut{}, fmt.Errorf("threads_history: %w", err)
	}
	return nil, ThreadsHistoryOut{Items: toThreadsAuditEventOuts(page.Items), NextCursor: page.NextCursor}, nil
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
	out := ThreadsPostOut{ID: p.ID, ThreadsID: p.ThreadsID, MediaProductType: p.MediaProductType, MediaType: p.MediaType, Text: p.Text, Permalink: p.Permalink, Username: p.Username, TopicTag: p.TopicTag, IsQuotePost: p.IsQuotePost}
	if p.Timestamp != nil {
		out.Timestamp = p.Timestamp.Format(time.RFC3339)
	}
	return out
}

func toThreadsTokenOut(t port.ThreadsTokenResult) ThreadsTokenOut {
	out := ThreadsTokenOut{AccessToken: t.AccessToken, TokenType: t.TokenType, ExpiresIn: t.ExpiresIn}
	if t.ExpiresIn > 0 {
		out.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	return out
}

func toThreadsAuditEventOuts(items []domain.ThreadsAuditEvent) []ThreadsAuditEventOut {
	out := make([]ThreadsAuditEventOut, len(items))
	for i, item := range items {
		out[i] = toThreadsAuditEventOut(item)
	}
	return out
}

func toThreadsAuditEventOut(e domain.ThreadsAuditEvent) ThreadsAuditEventOut {
	out := ThreadsAuditEventOut{ID: e.ID, Action: e.Action, ObjectID: e.ObjectID, OK: e.OK, Error: e.Error}
	if !e.CreatedAt.IsZero() {
		out.CreatedAt = e.CreatedAt.Format(time.RFC3339)
	}
	if json.Valid(e.RawJSON) {
		out.RawJSON = json.RawMessage(e.RawJSON)
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
