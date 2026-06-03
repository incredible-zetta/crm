package mcptools

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/db"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// In & Out structs for contact_create
type ContactCreateIn struct {
	Email     string         `json:"email" jsonschema:"required,description=Email address of the contact"`
	FirstName string         `json:"first_name" jsonschema:"description=First name of the contact"`
	LastName  string         `json:"last_name" jsonschema:"description=Last name of the contact"`
	Company   string         `json:"company" jsonschema:"description=Company of the contact"`
	Phone     string         `json:"phone" jsonschema:"description=Phone number of the contact"`
	Stage     string         `json:"stage" jsonschema:"description=Lifecycle stage (new, contacted, qualified, proposal, won, lost)"`
	Tags      []string       `json:"tags" jsonschema:"description=Tags associated with the contact"`
	Notes     string         `json:"notes" jsonschema:"description=Notes or descriptions"`
	Custom    map[string]any `json:"custom" jsonschema:"description=Custom metadata key-value pairs"`
	Source    string         `json:"source" jsonschema:"description=Source channel of the contact"`
}

type ContactCreateOut struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Stage string `json:"stage"`
}

// In & Out structs for contact_update
type ContactUpdateIn struct {
	ID        int64           `json:"id" jsonschema:"description=ID of the contact to update. Either ID or Email must be provided."`
	Email     string          `json:"email" jsonschema:"description=Email of the contact to update (used if ID is 0/omitted)."`
	FirstName *string         `json:"first_name" jsonschema:"description=Updated first name"`
	LastName  *string         `json:"last_name" jsonschema:"description=Updated last name"`
	Company   *string         `json:"company" jsonschema:"description=Updated company"`
	Phone     *string         `json:"phone" jsonschema:"description=Updated phone"`
	Stage     *string         `json:"stage" jsonschema:"description=Updated stage"`
	Tags      *[]string       `json:"tags" jsonschema:"description=Updated tags"`
	Notes     *string         `json:"notes" jsonschema:"description=Updated notes"`
	Custom    *map[string]any `json:"custom" jsonschema:"description=Updated custom properties"`
	Source    *string         `json:"source" jsonschema:"description=Updated source"`
}

type ContactUpdateOut struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Stage string `json:"stage"`
}

// In & Out structs for contact_list
type ContactListIn struct {
	Stage   string   `json:"stage" jsonschema:"description=Filter by stage"`
	Company string   `json:"company" jsonschema:"description=Filter by company"`
	Tag     string   `json:"tag" jsonschema:"description=Filter by tag"`
	Q       string   `json:"q" jsonschema:"description=Filter by search query (name, email, company)"`
	Limit   int      `json:"limit" jsonschema:"description=Max results to return (default 20, cap 100)"`
	Cursor  int64    `json:"cursor" jsonschema:"description=Pagination cursor"`
	Fields  []string `json:"fields" jsonschema:"description=Optional projection fields to return. If empty, a compact default is returned."`
}

type ContactListOut struct {
	Total      int              `json:"total"`
	Count      int              `json:"count"`
	Items      []map[string]any `json:"items"`
	NextCursor int64            `json:"next_cursor"`
}

// In & Out structs for contact_import
type ContactInput struct {
	Email     string         `json:"email" jsonschema:"required"`
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	Company   string         `json:"company"`
	Phone     string         `json:"phone"`
	Stage     string         `json:"stage"`
	Tags      []string       `json:"tags"`
	Notes     string         `json:"notes"`
	Custom    map[string]any `json:"custom"`
	Source    string         `json:"source"`
}

type ContactImportIn struct {
	Contacts []ContactInput `json:"contacts" jsonschema:"description=Array of contact input objects to import"`
	CSV      string         `json:"csv" jsonschema:"description=CSV data to import. Header must be email,first_name,last_name,company,phone,stage,tags,source"`
}

type ContactImportOut struct {
	Inserted int      `json:"inserted"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
}

// In & Out structs for contact_export
type ContactExportIn struct {
	Stage   string `json:"stage" jsonschema:"description=Filter by stage"`
	Company string `json:"company" jsonschema:"description=Filter by company"`
	Tag     string `json:"tag" jsonschema:"description=Filter by tag"`
	Q       string `json:"q" jsonschema:"description=Filter by search query"`
}

type ContactExportOut struct {
	URL      string `json:"url"`
	Rows     int    `json:"rows"`
	ExportID string `json:"export_id"`
}

func isValidStage(stage string) bool {
	for _, s := range db.ValidStages {
		if s == stage {
			return true
		}
	}
	return false
}

func rand16Hex() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func projectContact(c db.Contact, fields []string) map[string]any {
	m := make(map[string]any)
	if len(fields) == 0 {
		m["id"] = c.ID
		m["email"] = c.Email
		m["first_name"] = c.FirstName
		m["last_name"] = c.LastName
		m["company"] = c.Company
		m["stage"] = c.Stage
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
			m["stage"] = c.Stage
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
	if in.Stage != "" && !isValidStage(in.Stage) {
		return mcpserver.Err("bad_stage", "invalid stage"), ContactCreateOut{}, nil
	}

	c, err := d.Repo.UpsertContact(ctx, db.Contact{
		Email:     in.Email,
		FirstName: in.FirstName,
		LastName:  in.LastName,
		Company:   in.Company,
		Phone:     in.Phone,
		Stage:     in.Stage,
		Tags:      in.Tags,
		Notes:     in.Notes,
		Custom:    in.Custom,
		Source:    in.Source,
	})
	if err != nil {
		return mcpserver.Err("upsert_failed", err.Error()), ContactCreateOut{}, nil
	}

	return nil, ContactCreateOut{
		ID:    c.ID,
		Email: c.Email,
		Stage: c.Stage,
	}, nil
}

func (d *Deps) ContactUpdate(ctx context.Context, req *mcp.CallToolRequest, in ContactUpdateIn) (*mcp.CallToolResult, ContactUpdateOut, error) {
	var targetID int64 = in.ID
	if targetID == 0 {
		if in.Email == "" {
			return mcpserver.Err("missing_identifier", "either id or email must be provided"), ContactUpdateOut{}, nil
		}
		contact, err := d.Repo.GetContactByEmail(ctx, in.Email)
		if err != nil {
			return mcpserver.Err("not_found", "contact not found"), ContactUpdateOut{}, nil
		}
		targetID = contact.ID
	}

	if in.Stage != nil && !isValidStage(*in.Stage) {
		return mcpserver.Err("bad_stage", "invalid stage"), ContactUpdateOut{}, nil
	}

	patch := db.ContactPatch{
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

	c, err := d.Repo.UpdateContact(ctx, targetID, patch)
	if err != nil {
		return mcpserver.Err("update_failed", err.Error()), ContactUpdateOut{}, nil
	}

	return nil, ContactUpdateOut{
		ID:    c.ID,
		Email: c.Email,
		Stage: c.Stage,
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

	f := db.ContactFilter{
		Stage:   in.Stage,
		Company: in.Company,
		Tag:     in.Tag,
		Q:       in.Q,
	}

	items, total, nextCursor, err := d.Repo.ListContacts(ctx, f, limit, in.Cursor)
	if err != nil {
		return mcpserver.Err("list_failed", err.Error()), ContactListOut{}, nil
	}

	var projected []map[string]any
	for _, c := range items {
		projected = append(projected, projectContact(c, in.Fields))
	}

	return nil, ContactListOut{
		Total:      total,
		Count:      len(projected),
		Items:      projected,
		NextCursor: nextCursor,
	}, nil
}

func (d *Deps) ContactImport(ctx context.Context, req *mcp.CallToolRequest, in ContactImportIn) (*mcp.CallToolResult, ContactImportOut, error) {
	if len(in.Contacts) == 0 && in.CSV == "" {
		return mcpserver.Err("empty_import", "either contacts array or csv must be provided"), ContactImportOut{}, nil
	}

	var contactsToImport []ContactInput
	contactsToImport = append(contactsToImport, in.Contacts...)

	if in.CSV != "" {
		r := csv.NewReader(strings.NewReader(in.CSV))
		records, err := r.ReadAll()
		if err != nil {
			return mcpserver.Err("invalid_csv", err.Error()), ContactImportOut{}, nil
		}
		if len(records) > 0 {
			header := records[0]
			colIdx := make(map[string]int)
			for i, h := range header {
				colIdx[strings.ToLower(strings.TrimSpace(h))] = i
			}
			for i := 1; i < len(records); i++ {
				row := records[i]
				getVal := func(key string) string {
					idx, ok := colIdx[key]
					if !ok || idx >= len(row) {
						return ""
					}
					return strings.TrimSpace(row[idx])
				}
				emailVal := getVal("email")
				if emailVal == "" {
					continue
				}
				tagsStr := getVal("tags")
				var tags []string
				if tagsStr != "" {
					for _, t := range strings.Split(tagsStr, ";") {
						tTrimmed := strings.TrimSpace(t)
						if tTrimmed != "" {
							tags = append(tags, tTrimmed)
						}
					}
				}
				ci := ContactInput{
					Email:     emailVal,
					FirstName: getVal("first_name"),
					LastName:  getVal("last_name"),
					Company:   getVal("company"),
					Phone:     getVal("phone"),
					Stage:     getVal("stage"),
					Tags:      tags,
					Source:    getVal("source"),
				}
				contactsToImport = append(contactsToImport, ci)
			}
		}
	}

	var inserted, updated, skipped int
	var errors []string

	for _, c := range contactsToImport {
		if c.Email == "" {
			skipped++
			errors = append(errors, "missing email field")
			continue
		}
		if c.Stage != "" && !isValidStage(c.Stage) {
			skipped++
			errors = append(errors, fmt.Sprintf("invalid stage %q for email %s", c.Stage, c.Email))
			continue
		}

		_, getErr := d.Repo.GetContactByEmail(ctx, c.Email)
		isUpdate := getErr == nil

		_, err := d.Repo.UpsertContact(ctx, db.Contact{
			Email:     c.Email,
			FirstName: c.FirstName,
			LastName:  c.LastName,
			Company:   c.Company,
			Phone:     c.Phone,
			Stage:     c.Stage,
			Tags:      c.Tags,
			Notes:     c.Notes,
			Custom:    c.Custom,
			Source:    c.Source,
		})
		if err != nil {
			skipped++
			errors = append(errors, fmt.Sprintf("failed to import %s: %s", c.Email, err.Error()))
		} else {
			if isUpdate {
				updated++
			} else {
				inserted++
			}
		}
	}

	return nil, ContactImportOut{
		Inserted: inserted,
		Updated:  updated,
		Skipped:  skipped,
		Errors:   errors,
	}, nil
}

func (d *Deps) ContactExport(ctx context.Context, req *mcp.CallToolRequest, in ContactExportIn) (*mcp.CallToolResult, ContactExportOut, error) {
	var allContacts []db.Contact
	var cursor int64
	filter := db.ContactFilter{
		Stage:   in.Stage,
		Company: in.Company,
		Tag:     in.Tag,
		Q:       in.Q,
	}
	for {
		items, _, nextCursor, err := d.Repo.ListContacts(ctx, filter, 100, cursor)
		if err != nil {
			return mcpserver.Err("export_query_failed", err.Error()), ContactExportOut{}, nil
		}
		allContacts = append(allContacts, items...)
		if nextCursor == 0 || len(items) == 0 {
			break
		}
		cursor = nextCursor
	}

	if err := os.MkdirAll(d.ExportDir, 0755); err != nil {
		return mcpserver.Err("io_error", err.Error()), ContactExportOut{}, nil
	}

	id := rand16Hex()
	filename := id + ".csv"
	fullPath := filepath.Join(d.ExportDir, filename)
	f, err := os.Create(fullPath)
	if err != nil {
		return mcpserver.Err("io_error", err.Error()), ContactExportOut{}, nil
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"email", "first_name", "last_name", "company", "phone", "stage", "tags", "source"}); err != nil {
		return mcpserver.Err("io_error", err.Error()), ContactExportOut{}, nil
	}

	for _, c := range allContacts {
		tagsStr := strings.Join(c.Tags, ";")
		row := []string{c.Email, c.FirstName, c.LastName, c.Company, c.Phone, c.Stage, tagsStr, c.Source}
		if err := w.Write(row); err != nil {
			return mcpserver.Err("io_error", err.Error()), ContactExportOut{}, nil
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return mcpserver.Err("io_error", err.Error()), ContactExportOut{}, nil
	}

	expires := time.Now().Add(24 * time.Hour)
	exp := db.Export{
		ID:        id,
		Path:      fullPath,
		Rows:      len(allContacts),
		ExpiresAt: &expires,
	}
	if err := d.Repo.CreateExport(ctx, exp); err != nil {
		return mcpserver.Err("export_db_failed", err.Error()), ContactExportOut{}, nil
	}

	urlVal := strings.TrimSuffix(d.BaseURL, "/") + "/export/" + id + ".csv"

	return nil, ContactExportOut{
		URL:      urlVal,
		Rows:     len(allContacts),
		ExportID: id,
	}, nil
}
