package mcptools

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

func isValidStage(stage string) bool {
	for _, s := range db.ValidStages {
		if s == stage {
			return true
		}
	}
	return false
}

func rand16Hex() (string, error) {
	b := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
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
		return nil, ContactCreateOut{}, fmt.Errorf("contact_create upsert: %w", err)
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
			if errors.Is(err, db.ErrNotFound) {
				return mcpserver.Err("not_found", "contact not found"), ContactUpdateOut{}, nil
			}
			return nil, ContactUpdateOut{}, fmt.Errorf("contact_update get contact: %w", err)
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
		if errors.Is(err, db.ErrNotFound) {
			return mcpserver.Err("not_found", "contact not found"), ContactUpdateOut{}, nil
		}
		return nil, ContactUpdateOut{}, fmt.Errorf("contact_update update: %w", err)
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
		return nil, ContactListOut{}, fmt.Errorf("contact_list list contacts: %w", err)
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

	var inserted, updated, skipped int
	var errorsList []string

	// 1. Process array contacts
	for idx, c := range in.Contacts {
		if c.Email == "" {
			skipped++
			errorsList = append(errorsList, fmt.Sprintf("contact %d: missing email", idx))
			continue
		}
		if c.Stage != "" && !isValidStage(c.Stage) {
			skipped++
			errorsList = append(errorsList, fmt.Sprintf("contact %d: invalid stage %q", idx, c.Stage))
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
			errorsList = append(errorsList, fmt.Sprintf("contact %d: failed to import %s: %v", idx, c.Email, err))
		} else {
			if isUpdate {
				updated++
			} else {
				inserted++
			}
		}
	}

	// 2. Process streaming CSV
	if in.CSV != "" {
		r := csv.NewReader(strings.NewReader(in.CSV))
		header, err := r.Read()
		if err != nil {
			return mcpserver.Err("invalid_csv", "could not parse csv header"), ContactImportOut{}, nil
		}

		colIdx := make(map[string]int)
		for i, h := range header {
			colIdx[strings.ToLower(strings.TrimSpace(h))] = i
		}

		rowNum := 1
		for {
			rowNum++
			row, err := r.Read()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				skipped++
				errorsList = append(errorsList, fmt.Sprintf("row %d: parse error: %v", rowNum, err))
				continue
			}

			getVal := func(key string) string {
				idx, ok := colIdx[key]
				if !ok || idx >= len(row) {
					return ""
				}
				return strings.TrimSpace(row[idx])
			}

			emailVal := getVal("email")
			if emailVal == "" {
				skipped++
				errorsList = append(errorsList, fmt.Sprintf("row %d: missing email", rowNum))
				continue
			}

			stageVal := getVal("stage")
			if stageVal != "" && !isValidStage(stageVal) {
				skipped++
				errorsList = append(errorsList, fmt.Sprintf("row %d: bad stage %q", rowNum, stageVal))
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

			_, getErr := d.Repo.GetContactByEmail(ctx, emailVal)
			isUpdate := getErr == nil

			_, err = d.Repo.UpsertContact(ctx, db.Contact{
				Email:     emailVal,
				FirstName: getVal("first_name"),
				LastName:  getVal("last_name"),
				Company:   getVal("company"),
				Phone:     getVal("phone"),
				Stage:     stageVal,
				Tags:      tags,
				Source:    getVal("source"),
			})
			if err != nil {
				skipped++
				errorsList = append(errorsList, fmt.Sprintf("row %d: failed to import %s: %v", rowNum, emailVal, err))
			} else {
				if isUpdate {
					updated++
				} else {
					inserted++
				}
			}
		}
	}

	return nil, ContactImportOut{
		Inserted: inserted,
		Updated:  updated,
		Skipped:  skipped,
		Errors:   errorsList,
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
			return nil, ContactExportOut{}, fmt.Errorf("contact_export list contacts: %w", err)
		}
		allContacts = append(allContacts, items...)

		if nextCursor == 0 || len(items) == 0 {
			break
		}
		cursor = nextCursor
	}

	if err := os.MkdirAll(d.ExportDir, 0755); err != nil {
		return nil, ContactExportOut{}, fmt.Errorf("contact_export mkdir: %w", err)
	}

	id, err := rand16Hex()
	if err != nil {
		return nil, ContactExportOut{}, fmt.Errorf("contact_export generate id: %w", err)
	}
	filename := id + ".csv"
	fullPath := filepath.Join(d.ExportDir, filename)
	f, err := os.Create(fullPath)
	if err != nil {
		return nil, ContactExportOut{}, fmt.Errorf("contact_export create file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"email", "first_name", "last_name", "company", "phone", "stage", "tags", "source"}); err != nil {
		return nil, ContactExportOut{}, fmt.Errorf("contact_export write header: %w", err)
	}

	for _, c := range allContacts {
		tagsStr := strings.Join(c.Tags, ";")
		row := []string{c.Email, c.FirstName, c.LastName, c.Company, c.Phone, c.Stage, tagsStr, c.Source}
		if err := w.Write(row); err != nil {
			return nil, ContactExportOut{}, fmt.Errorf("contact_export write row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, ContactExportOut{}, fmt.Errorf("contact_export flush: %w", err)
	}

	expires := time.Now().Add(24 * time.Hour)
	exp := db.Export{
		ID:        id,
		Path:      fullPath,
		Rows:      len(allContacts),
		ExpiresAt: &expires,
	}
	if err := d.Repo.CreateExport(ctx, exp); err != nil {
		return nil, ContactExportOut{}, fmt.Errorf("contact_export db create: %w", err)
	}

	urlVal := strings.TrimSuffix(d.BaseURL, "/") + "/export/" + id + ".csv"

	return nil, ContactExportOut{
		URL:      urlVal,
		Rows:     len(allContacts),
		ExportID: id,
	}, nil
}
