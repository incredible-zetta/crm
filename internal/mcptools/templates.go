package mcptools

import (
	"context"
	"errors"
	"fmt"

	"github.com/cipta/crm-for-aiagents/internal/db"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
	"github.com/cipta/crm-for-aiagents/internal/template"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type TemplateCreateIn struct {
	Name      string   `json:"name" jsonschema:"Unique template name"`
	Subject   string   `json:"subject" jsonschema:"Subject template with optional merge fields"`
	BodyHTML  string   `json:"body_html" jsonschema:"HTML body template with optional merge fields"`
	BodyText  string   `json:"body_text" jsonschema:"Plain text body template with optional merge fields"`
	Variables []string `json:"variables" jsonschema:"List of variables used in the template"`
}

type TemplateCreateOut struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type TemplateListIn struct{}

type TemplateListOut struct {
	Count int              `json:"count"`
	Items []map[string]any `json:"items"`
}

type TemplateRenderIn struct {
	TemplateID int64          `json:"template_id" jsonschema:"ID of the email template to load and render"`
	Subject    string         `json:"subject" jsonschema:"Raw subject template (used if TemplateID is 0)"`
	BodyHTML   string         `json:"body_html" jsonschema:"Raw HTML body template (used if TemplateID is 0)"`
	BodyText   string         `json:"body_text" jsonschema:"Raw Plain text body template (used if TemplateID is 0)"`
	Vars       map[string]any `json:"vars" jsonschema:"Variable values to merge into the template"`
	HTML       bool           `json:"html" jsonschema:"If true, also include the rendered HTML in the output"`
}

type TemplateRenderOut struct {
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html,omitempty"`
}

func (d *Deps) TemplateCreate(ctx context.Context, req *mcp.CallToolRequest, in TemplateCreateIn) (*mcp.CallToolResult, TemplateCreateOut, error) {
	t, err := d.Repo.CreateTemplate(ctx, db.EmailTemplate{
		Name:      in.Name,
		Subject:   in.Subject,
		BodyHTML:  in.BodyHTML,
		BodyText:  in.BodyText,
		Variables: in.Variables,
	})
	if err != nil {
		return nil, TemplateCreateOut{}, fmt.Errorf("template_create db: %w", err)
	}

	return nil, TemplateCreateOut{
		ID:   t.ID,
		Name: t.Name,
	}, nil
}

func (d *Deps) TemplateList(ctx context.Context, req *mcp.CallToolRequest, in TemplateListIn) (*mcp.CallToolResult, TemplateListOut, error) {
	list, err := d.Repo.ListTemplates(ctx)
	if err != nil {
		return nil, TemplateListOut{}, fmt.Errorf("template_list db: %w", err)
	}

	var items []map[string]any
	for _, t := range list {
		items = append(items, map[string]any{
			"id":        t.ID,
			"name":      t.Name,
			"variables": t.Variables,
		})
	}

	return nil, TemplateListOut{
		Count: len(items),
		Items: items,
	}, nil
}

func (d *Deps) TemplateRender(ctx context.Context, req *mcp.CallToolRequest, in TemplateRenderIn) (*mcp.CallToolResult, TemplateRenderOut, error) {
	var rawSubject, rawHTML, rawText string

	if in.TemplateID > 0 {
		t, err := d.Repo.GetTemplate(ctx, in.TemplateID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return mcpserver.Err("not_found", "template not found"), TemplateRenderOut{}, nil
			}
			return nil, TemplateRenderOut{}, fmt.Errorf("template_render get: %w", err)
		}
		rawSubject = t.Subject
		rawHTML = t.BodyHTML
		rawText = t.BodyText
	} else {
		rawSubject = in.Subject
		rawHTML = in.BodyHTML
		rawText = in.BodyText
	}

	rendered, err := template.RenderEmail(rawSubject, rawHTML, rawText, in.Vars)
	if err != nil {
		return mcpserver.Err("render_failed", "template rendering failed"), TemplateRenderOut{}, nil
	}

	out := TemplateRenderOut{
		Subject: rendered.Subject,
		Text:    rendered.Text,
	}
	if in.HTML {
		out.HTML = rendered.HTML
	}

	return nil, out, nil
}
