package mcptransport

import (
	"context"
	"errors"
	"fmt"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/incredible-zetta/crm/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type TemplateCreateIn struct {
	Name      string   `json:"name" jsonschema:"Unique template name"`
	Subject   string   `json:"subject" jsonschema:"Subject template with optional merge fields"`
	BodyHTML  string   `json:"body_html,omitempty" jsonschema:"HTML body template with optional merge fields"`
	BodyText  string   `json:"body_text,omitempty" jsonschema:"Plain text body template with optional merge fields"`
	Variables []string `json:"variables,omitempty" jsonschema:"List of variables used in the template"`
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
	TemplateID int64          `json:"template_id,omitempty" jsonschema:"ID of the email template to load and render"`
	Subject    string         `json:"subject,omitempty" jsonschema:"Raw subject template (used if TemplateID is 0)"`
	BodyHTML   string         `json:"body_html,omitempty" jsonschema:"HTML body template (used if TemplateID is 0)"`
	BodyText   string         `json:"body_text,omitempty" jsonschema:"Raw Plain text body template (used if TemplateID is 0)"`
	Vars       map[string]any `json:"vars,omitempty" jsonschema:"Variable values to merge into the template"`
	HTML       bool           `json:"html,omitempty" jsonschema:"If true, also include the rendered HTML in the output"`
}

type TemplateRenderOut struct {
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html,omitempty"`
}

type TemplateGetIn struct {
	ID   int64  `json:"id,omitempty" jsonschema:"ID of the template"`
	Name string `json:"name,omitempty" jsonschema:"Name of the template"`
}

type TemplateGetOut struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Subject   string   `json:"subject"`
	BodyHTML  string   `json:"body_html"`
	BodyText  string   `json:"body_text"`
	Variables []string `json:"variables"`
}

type TemplateUpdateIn struct {
	ID        int64     `json:"id" jsonschema:"ID of the template to update"`
	Name      *string   `json:"name,omitempty" jsonschema:"New name of the template"`
	Subject   *string   `json:"subject,omitempty" jsonschema:"New subject template"`
	BodyHTML  *string   `json:"body_html,omitempty" jsonschema:"New HTML body template"`
	BodyText  *string   `json:"body_text,omitempty" jsonschema:"New plain text body template"`
	Variables *[]string `json:"variables,omitempty" jsonschema:"New merge variables list"`
}

type TemplateUpdateOut struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type TemplateDeleteIn struct {
	ID int64 `json:"id" jsonschema:"ID of the template to delete"`
}

type TemplateDeleteOut struct {
	ID      int64 `json:"id"`
	Deleted bool  `json:"deleted"`
}

func (d *Deps) TemplateCreate(ctx context.Context, req *mcp.CallToolRequest, in TemplateCreateIn) (*mcp.CallToolResult, TemplateCreateOut, error) {
	t, err := d.Svc.Template.Create(ctx, domain.Template{
		Name:      in.Name,
		Subject:   in.Subject,
		BodyHTML:  in.BodyHTML,
		BodyText:  in.BodyText,
		Variables: in.Variables,
	})
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return mcpserver.Err("invalid_input", err.Error()), TemplateCreateOut{}, nil
		}
		if errors.Is(err, domain.ErrConflict) {
			return mcpserver.Err("conflict", err.Error()), TemplateCreateOut{}, nil
		}
		return nil, TemplateCreateOut{}, fmt.Errorf("template_create: %w", err)
	}

	return nil, TemplateCreateOut{
		ID:   t.ID,
		Name: t.Name,
	}, nil
}

func (d *Deps) TemplateList(ctx context.Context, req *mcp.CallToolRequest, in TemplateListIn) (*mcp.CallToolResult, TemplateListOut, error) {
	list, err := d.Svc.Template.List(ctx)
	if err != nil {
		return nil, TemplateListOut{}, fmt.Errorf("template_list: %w", err)
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
	res, err := d.Svc.Template.Render(ctx, service.RenderInput{
		TemplateID: in.TemplateID,
		Subject:    in.Subject,
		BodyHTML:   in.BodyHTML,
		BodyText:   in.BodyText,
		Vars:       in.Vars,
		WantHTML:   in.HTML,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "template not found"), TemplateRenderOut{}, nil
		}
		if errors.Is(err, domain.ErrValidation) {
			return mcpserver.Err("invalid_input", err.Error()), TemplateRenderOut{}, nil
		}
		return mcpserver.Err("render_failed", err.Error()), TemplateRenderOut{}, nil
	}

	out := TemplateRenderOut{
		Subject: res.Subject,
		Text:    res.Text,
	}
	if in.HTML {
		out.HTML = res.HTML
	}

	return nil, out, nil
}

func (d *Deps) TemplateGet(ctx context.Context, req *mcp.CallToolRequest, in TemplateGetIn) (*mcp.CallToolResult, TemplateGetOut, error) {
	var t domain.Template
	var err error

	if in.ID > 0 {
		t, err = d.Svc.Template.Get(ctx, in.ID)
	} else if in.Name != "" {
		t, err = d.Svc.Template.GetByName(ctx, in.Name)
	} else {
		return mcpserver.Err("invalid_input", "id or name required"), TemplateGetOut{}, nil
	}

	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "template not found"), TemplateGetOut{}, nil
		}
		return nil, TemplateGetOut{}, fmt.Errorf("template_get: %w", err)
	}

	return nil, TemplateGetOut{
		ID:        t.ID,
		Name:      t.Name,
		Subject:   t.Subject,
		BodyHTML:  t.BodyHTML,
		BodyText:  t.BodyText,
		Variables: t.Variables,
	}, nil
}

func (d *Deps) TemplateUpdate(ctx context.Context, req *mcp.CallToolRequest, in TemplateUpdateIn) (*mcp.CallToolResult, TemplateUpdateOut, error) {
	existing, err := d.Svc.Template.Get(ctx, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "template not found"), TemplateUpdateOut{}, nil
		}
		return nil, TemplateUpdateOut{}, fmt.Errorf("template_update get: %w", err)
	}

	t := existing
	if in.Name != nil {
		t.Name = *in.Name
	}
	if in.Subject != nil {
		t.Subject = *in.Subject
	}
	if in.BodyHTML != nil {
		t.BodyHTML = *in.BodyHTML
	}
	if in.BodyText != nil {
		t.BodyText = *in.BodyText
	}
	if in.Variables != nil {
		t.Variables = *in.Variables
	}

	updated, err := d.Svc.Template.Update(ctx, in.ID, t)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return mcpserver.Err("invalid_input", err.Error()), TemplateUpdateOut{}, nil
		}
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "template not found"), TemplateUpdateOut{}, nil
		}
		return nil, TemplateUpdateOut{}, fmt.Errorf("template_update: %w", err)
	}

	return nil, TemplateUpdateOut{
		ID:   updated.ID,
		Name: updated.Name,
	}, nil
}

func (d *Deps) TemplateDelete(ctx context.Context, req *mcp.CallToolRequest, in TemplateDeleteIn) (*mcp.CallToolResult, TemplateDeleteOut, error) {
	err := d.Svc.Template.Delete(ctx, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "template not found"), TemplateDeleteOut{}, nil
		}
		return nil, TemplateDeleteOut{}, fmt.Errorf("template_delete: %w", err)
	}

	return nil, TemplateDeleteOut{
		ID:      in.ID,
		Deleted: true,
	}, nil
}
