package mcptransport

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// In & Out structs for contact_create
type ContactCreateIn struct {
	Email     string         `json:"email" jsonschema:"Email address of the contact"`
	FirstName string         `json:"first_name,omitempty" jsonschema:"First name of the contact"`
	LastName  string         `json:"last_name,omitempty" jsonschema:"Last name of the contact"`
	Company   string         `json:"company,omitempty" jsonschema:"Company of the contact"`
	Phone     string         `json:"phone,omitempty" jsonschema:"Phone number of the contact"`
	Stage     string         `json:"stage,omitempty" jsonschema:"Lifecycle stage (new, contacted, qualified, proposal, won, lost)"`
	Tags      []string       `json:"tags,omitempty" jsonschema:"Tags associated with the contact"`
	Notes     string         `json:"notes,omitempty" jsonschema:"Notes or descriptions"`
	Custom    map[string]any `json:"custom,omitempty" jsonschema:"Custom metadata key-value pairs"`
	Source    string         `json:"source,omitempty" jsonschema:"Source channel of the contact"`
}

type ContactCreateOut struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Stage string `json:"stage"`
}

// In & Out structs for contact_update
type ContactUpdateIn struct {
	ID        int64           `json:"id,omitempty" jsonschema:"ID of the contact to update. Either ID or Email must be provided."`
	Email     string          `json:"email,omitempty" jsonschema:"Email of the contact to update (used if ID is 0/omitted)."`
	FirstName *string         `json:"first_name,omitempty" jsonschema:"Updated first name"`
	LastName  *string         `json:"last_name,omitempty" jsonschema:"Updated last name"`
	Company   *string         `json:"company,omitempty" jsonschema:"Updated company"`
	Phone     *string         `json:"phone,omitempty" jsonschema:"Updated phone"`
	Stage     *string         `json:"stage,omitempty" jsonschema:"Updated stage"`
	Tags      *[]string       `json:"tags,omitempty" jsonschema:"Updated tags"`
	Notes     *string         `json:"notes,omitempty" jsonschema:"Updated notes"`
	Custom    *map[string]any `json:"custom,omitempty" jsonschema:"Updated custom properties"`
	Source    *string         `json:"source,omitempty" jsonschema:"Updated source"`
}

type ContactUpdateOut struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Stage string `json:"stage"`
}

// In & Out structs for contact_list
type ContactListIn struct {
	Stage   string   `json:"stage,omitempty" jsonschema:"Filter by stage"`
	Company string   `json:"company,omitempty" jsonschema:"Filter by company"`
	Tag     string   `json:"tag,omitempty" jsonschema:"Filter by tag"`
	Q       string   `json:"q,omitempty" jsonschema:"Filter by search query (name, email, company)"`
	Limit   int      `json:"limit,omitempty" jsonschema:"Max results to return (default 20, cap 100)"`
	Cursor  int64    `json:"cursor,omitempty" jsonschema:"Pagination cursor"`
	Fields  []string `json:"fields,omitempty" jsonschema:"Optional projection fields to return. If empty, a compact default is returned."`
}

type ContactListOut struct {
	Total      int              `json:"total"`
	Count      int              `json:"count"`
	Items      []map[string]any `json:"items"`
	NextCursor int64            `json:"next_cursor"`
}

// In & Out structs for contact_import
type ContactInput struct {
	Email     string         `json:"email"`
	FirstName string         `json:"first_name,omitempty"`
	LastName  string         `json:"last_name,omitempty"`
	Company   string         `json:"company,omitempty"`
	Phone     string         `json:"phone,omitempty"`
	Stage     string         `json:"stage,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	Notes     string         `json:"notes,omitempty"`
	Custom    map[string]any `json:"custom,omitempty"`
	Source    string         `json:"source,omitempty"`
}

type ContactImportIn struct {
	Contacts []ContactInput `json:"contacts,omitempty" jsonschema:"Array of contact input objects to import"`
	CSV      string         `json:"csv,omitempty" jsonschema:"CSV data to import. Header must be email,first_name,last_name,company,phone,stage,tags,source"`
}

type ContactImportOut struct {
	Inserted int      `json:"inserted"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
}

// In & Out structs for contact_export
type ContactExportIn struct {
	Stage   string `json:"stage,omitempty" jsonschema:"Filter by stage"`
	Company string `json:"company,omitempty" jsonschema:"Filter by company"`
	Tag     string `json:"tag,omitempty" jsonschema:"Filter by tag"`
	Q       string `json:"q,omitempty" jsonschema:"Filter by search query"`
}

type ContactExportOut struct {
	URL      string `json:"url"`
	Rows     int    `json:"rows"`
	ExportID string `json:"export_id"`
}

func projectContact(c domain.Contact, fields []string) map[string]any {
	m := make(map[string]any)
	if len(fields) == 0 {
		m["id"] = c.ID
		m["email"] = c.Email
		m["first_name"] = c.FirstName
		m["last_name"] = c.LastName
		m["company"] = c.Company
		m["stage"] = string(c.Stage)
		return m
	}
	for _, f := range fields {
		switch f {
		case "id":
			m["id"] = c.ID
		case "email":
			m["email"] = c.Email
		case "first_name":
			m["first_name"] = c.FirstName
		case "last_name":
			m["last_name"] = c.LastName
		case "company":
			m["company"] = c.Company
		case "phone":
			m["phone"] = c.Phone
		case "stage":
			m["stage"] = string(c.Stage)
		case "tags":
			m["tags"] = c.Tags
		case "notes":
			m["notes"] = c.Notes
		case "source":
			m["source"] = c.Source
		}
	}
	return m
}

func (d *Deps) ContactCreate(ctx context.Context, req *mcp.CallToolRequest, in ContactCreateIn) (*mcp.CallToolResult, ContactCreateOut, error) {
	c, err := d.Svc.Contact.Create(ctx, domain.Contact{
		Email:     in.Email,
		FirstName: in.FirstName,
		LastName:  in.LastName,
		Company:   in.Company,
		Phone:     in.Phone,
		Stage:     domain.Stage(in.Stage),
		Tags:      in.Tags,
		Notes:     in.Notes,
		Custom:    in.Custom,
		Source:    in.Source,
	})
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			code := "invalid_input"
			if strings.Contains(err.Error(), "stage") {
				code = "bad_stage"
			}
			return mcpserver.Err(code, err.Error()), ContactCreateOut{}, nil
		}
		if errors.Is(err, domain.ErrConflict) {
			return mcpserver.Err("conflict", err.Error()), ContactCreateOut{}, nil
		}
		return nil, ContactCreateOut{}, fmt.Errorf("contact_create: %w", err)
	}

	return nil, ContactCreateOut{
		ID:    c.ID,
		Email: c.Email,
		Stage: string(c.Stage),
	}, nil
}

func (d *Deps) ContactUpdate(ctx context.Context, req *mcp.CallToolRequest, in ContactUpdateIn) (*mcp.CallToolResult, ContactUpdateOut, error) {
	var targetID int64 = in.ID

	if targetID == 0 {
		if in.Email == "" {
			return mcpserver.Err("missing_identifier", "either id or email must be provided"), ContactUpdateOut{}, nil
		}
		contact, err := d.Svc.Contact.GetByEmail(ctx, in.Email)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return mcpserver.Err("not_found", "contact not found"), ContactUpdateOut{}, nil
			}
			return nil, ContactUpdateOut{}, fmt.Errorf("contact_update get contact: %w", err)
		}
		targetID = contact.ID
	}

	patch := domain.ContactPatch{
		FirstName: in.FirstName,
		LastName:  in.LastName,
		Company:   in.Company,
		Phone:     in.Phone,
		Stage:     in.Stage,
		Tags:      in.Tags,
		Notes:     in.Notes,
		Custom:    in.Custom,
		Source:    in.Source,
	}

	if in.ID > 0 && in.Email != "" {
		patch.Email = &in.Email
	}

	c, err := d.Svc.Contact.Update(ctx, targetID, patch)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			code := "invalid_input"
			if strings.Contains(err.Error(), "stage") {
				code = "bad_stage"
			}
			return mcpserver.Err(code, err.Error()), ContactUpdateOut{}, nil
		}
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "contact not found"), ContactUpdateOut{}, nil
		}
		if errors.Is(err, domain.ErrConflict) {
			return mcpserver.Err("conflict", err.Error()), ContactUpdateOut{}, nil
		}
		return nil, ContactUpdateOut{}, fmt.Errorf("contact_update update: %w", err)
	}

	return nil, ContactUpdateOut{
		ID:    c.ID,
		Email: c.Email,
		Stage: string(c.Stage),
	}, nil
}

func (d *Deps) ContactList(ctx context.Context, req *mcp.CallToolRequest, in ContactListIn) (*mcp.CallToolResult, ContactListOut, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	f := domain.ContactFilter{
		Stage:   in.Stage,
		Company: in.Company,
		Tag:     in.Tag,
		Q:       in.Q,
	}

	page, err := d.Svc.Contact.List(ctx, f, limit, in.Cursor)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return mcpserver.Err("invalid_input", err.Error()), ContactListOut{}, nil
		}
		return nil, ContactListOut{}, fmt.Errorf("contact_list: %w", err)
	}

	var projected []map[string]any
	for _, c := range page.Items {
		projected = append(projected, projectContact(c, in.Fields))
	}

	return nil, ContactListOut{
		Total:      page.Total,
		Count:      len(projected),
		Items:      projected,
		NextCursor: page.NextCursor,
	}, nil
}

func (d *Deps) ContactImport(ctx context.Context, req *mcp.CallToolRequest, in ContactImportIn) (*mcp.CallToolResult, ContactImportOut, error) {
	var contacts []domain.Contact
	for _, c := range in.Contacts {
		contacts = append(contacts, domain.Contact{
			Email:     c.Email,
			FirstName: c.FirstName,
			LastName:  c.LastName,
			Company:   c.Company,
			Phone:     c.Phone,
			Stage:     domain.Stage(c.Stage),
			Tags:      c.Tags,
			Notes:     c.Notes,
			Custom:    c.Custom,
			Source:    c.Source,
		})
	}

	inserted, updated, skipped, errsList, err := d.Svc.Contact.Import(ctx, contacts, in.CSV)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			code := "invalid_input"
			if strings.Contains(err.Error(), "empty import") {
				code = "empty_import"
			} else if strings.Contains(err.Error(), "csv") || strings.Contains(err.Error(), "CSV") {
				code = "invalid_csv"
			}
			return mcpserver.Err(code, err.Error()), ContactImportOut{}, nil
		}
		return nil, ContactImportOut{}, fmt.Errorf("contact_import: %w", err)
	}

	return nil, ContactImportOut{
		Inserted: inserted,
		Updated:  updated,
		Skipped:  skipped,
		Errors:   errsList,
	}, nil
}

func (d *Deps) ContactExport(ctx context.Context, req *mcp.CallToolRequest, in ContactExportIn) (*mcp.CallToolResult, ContactExportOut, error) {
	f := domain.ContactFilter{
		Stage:   in.Stage,
		Company: in.Company,
		Tag:     in.Tag,
		Q:       in.Q,
	}

	exportID, urlVal, rows, err := d.Svc.Contact.Export(ctx, f)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return mcpserver.Err("invalid_input", err.Error()), ContactExportOut{}, nil
		}
		return nil, ContactExportOut{}, fmt.Errorf("contact_export: %w", err)
	}

	return nil, ContactExportOut{
		URL:      urlVal,
		Rows:     rows,
		ExportID: exportID,
	}, nil
}
