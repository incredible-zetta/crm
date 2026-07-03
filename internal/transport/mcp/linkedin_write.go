package mcptransport

import (
	"context"

	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Write-side LinkedIn tools. These mutate the live LinkedIn account (publish
// posts, comments, replies, deletes) and should be gated by agent policy.

type LinkedInCreatePostIn struct {
	Account string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
	Text    string `json:"text" jsonschema:"Post body (<=3000 chars). Live-published to the account."`
}

func (d *Deps) LinkedInCreatePost(ctx context.Context, req *mcp.CallToolRequest, in LinkedInCreatePostIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.CreatePost(ctx, in.Account, in.Text)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}

type LinkedInDeletePostIn struct {
	Account string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
	URN     string `json:"urn" jsonschema:"Share urn to delete (e.g. urn:li:share:...)"`
}

func (d *Deps) LinkedInDeletePost(ctx context.Context, req *mcp.CallToolRequest, in LinkedInDeletePostIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.DeletePost(ctx, in.Account, in.URN)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}

type LinkedInListCommentsIn struct {
	Account string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
	ID      string `json:"id" jsonschema:"Activity urn to list comments for"`
}

func (d *Deps) LinkedInListComments(ctx context.Context, req *mcp.CallToolRequest, in LinkedInListCommentsIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.ListComments(ctx, in.Account, in.ID)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}

type LinkedInListRepliesIn struct {
	Account string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
	ID      string `json:"id" jsonschema:"Comment urn to list replies for"`
}

func (d *Deps) LinkedInListReplies(ctx context.Context, req *mcp.CallToolRequest, in LinkedInListRepliesIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.ListReplies(ctx, in.Account, in.ID)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}

type LinkedInCommentIn struct {
	Account string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
	ID      string `json:"id" jsonschema:"Activity urn to comment on"`
	Text    string `json:"text" jsonschema:"Comment body. Live-published."`
}

func (d *Deps) LinkedInComment(ctx context.Context, req *mcp.CallToolRequest, in LinkedInCommentIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.Comment(ctx, in.Account, in.ID, in.Text)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}

type LinkedInReplyIn struct {
	Account string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
	ID      string `json:"id" jsonschema:"Comment urn to reply under"`
	Text    string `json:"text" jsonschema:"Reply body. Live-published."`
}

func (d *Deps) LinkedInReply(ctx context.Context, req *mcp.CallToolRequest, in LinkedInReplyIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.Reply(ctx, in.Account, in.ID, in.Text)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}

type LinkedInDeleteCommentIn struct {
	Account string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
	ID      string `json:"id" jsonschema:"Comment urn to delete"`
}

func (d *Deps) LinkedInDeleteComment(ctx context.Context, req *mcp.CallToolRequest, in LinkedInDeleteCommentIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.DeleteComment(ctx, in.Account, in.ID)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}
