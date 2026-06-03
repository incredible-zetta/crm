package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Contact struct {
	ID        int64
	Email     string
	FirstName string
	LastName  string
	Company   string
	Phone     string
	Stage     string
	Tags      []string // stored as JSON
	Notes     string
	Custom    map[string]any // stored as JSON
	Source    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var ValidStages = []string{"new", "contacted", "qualified", "proposal", "won", "lost"}

type ContactFilter struct {
	Stage   string // optional exact match
	Company string // optional exact match
	Tag     string // optional: contact has this tag
	Q       string // optional: search email/first/last/company LIKE
}

type ContactPatch struct {
	Email     *string
	FirstName *string
	LastName  *string
	Company   *string
	Phone     *string
	Stage     *string
	Tags      *[]string
	Notes     *string
	Custom    *map[string]any
	Source    *string
}

var ErrNotFound = errors.New("contact not found")

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) UpsertContact(ctx context.Context, c Contact) (Contact, error) {
	originalStage := c.Stage
	if c.Stage == "" {
		c.Stage = "new"
	}

	if !validStage(c.Stage) {
		return Contact{}, fmt.Errorf("invalid stage: %q", c.Stage)
	}

	var tagsJSON []byte
	if c.Tags != nil {
		var err error
		tagsJSON, err = json.Marshal(c.Tags)
		if err != nil {
			return Contact{}, fmt.Errorf("marshal tags: %w", err)
		}
	}

	var customJSON []byte
	if c.Custom != nil {
		var err error
		customJSON, err = json.Marshal(c.Custom)
		if err != nil {
			return Contact{}, fmt.Errorf("marshal custom: %w", err)
		}
	}

	query := `INSERT INTO contacts (email, first_name, last_name, company, phone, stage, tags, notes, custom, source)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  first_name = IF(VALUES(first_name) != '', VALUES(first_name), first_name),
  last_name  = IF(VALUES(last_name)  != '', VALUES(last_name),  last_name),
  company    = IF(VALUES(company)    != '', VALUES(company),    company),
  phone      = IF(VALUES(phone)      != '', VALUES(phone),      phone),
  stage      = IF(? != '', VALUES(stage), stage),
  tags       = IF(VALUES(tags) IS NOT NULL, VALUES(tags), tags),
  notes      = IF(VALUES(notes)      != '', VALUES(notes),      notes),
  custom     = IF(VALUES(custom) IS NOT NULL, VALUES(custom), custom),
  source     = IF(VALUES(source)     != '', VALUES(source),     source)`

	_, err := r.db.ExecContext(ctx, query,
		c.Email,
		c.FirstName,
		c.LastName,
		c.Company,
		c.Phone,
		c.Stage,
		tagsJSON,
		c.Notes,
		customJSON,
		c.Source,
		originalStage,
	)
	if err != nil {
		return Contact{}, fmt.Errorf("upsert contact exec: %w", err)
	}

	return r.GetContactByEmail(ctx, c.Email)
}

func (r *Repo) GetContact(ctx context.Context, id int64) (Contact, error) {
	query := "SELECT id, email, first_name, last_name, company, phone, stage, tags, notes, custom, source, created_at, updated_at FROM contacts WHERE id = ?"
	var c Contact
	var (
		firstName sql.NullString
		lastName  sql.NullString
		company   sql.NullString
		phone     sql.NullString
		notes     sql.NullString
		source    sql.NullString
		tagsBuf   []byte
		customBuf []byte
	)
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID,
		&c.Email,
		&firstName,
		&lastName,
		&company,
		&phone,
		&c.Stage,
		&tagsBuf,
		&notes,
		&customBuf,
		&source,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Contact{}, fmt.Errorf("contact not found: %w", ErrNotFound)
	}
	if err != nil {
		return Contact{}, fmt.Errorf("get contact: %w", err)
	}

	c.FirstName = firstName.String
	c.LastName = lastName.String
	c.Company = company.String
	c.Phone = phone.String
	c.Notes = notes.String
	c.Source = source.String

	if tagsBuf != nil {
		if err := json.Unmarshal(tagsBuf, &c.Tags); err != nil {
			return Contact{}, fmt.Errorf("unmarshal tags: %w", err)
		}
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}

	if customBuf != nil {
		if err := json.Unmarshal(customBuf, &c.Custom); err != nil {
			return Contact{}, fmt.Errorf("unmarshal custom: %w", err)
		}
	}
	if c.Custom == nil {
		c.Custom = make(map[string]any)
	}

	return c, nil
}

func (r *Repo) GetContactByEmail(ctx context.Context, email string) (Contact, error) {
	query := "SELECT id, email, first_name, last_name, company, phone, stage, tags, notes, custom, source, created_at, updated_at FROM contacts WHERE email = ?"
	var c Contact
	var (
		firstName sql.NullString
		lastName  sql.NullString
		company   sql.NullString
		phone     sql.NullString
		notes     sql.NullString
		source    sql.NullString
		tagsBuf   []byte
		customBuf []byte
	)
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&c.ID,
		&c.Email,
		&firstName,
		&lastName,
		&company,
		&phone,
		&c.Stage,
		&tagsBuf,
		&notes,
		&customBuf,
		&source,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Contact{}, fmt.Errorf("contact not found: %w", ErrNotFound)
	}
	if err != nil {
		return Contact{}, fmt.Errorf("get contact by email: %w", err)
	}

	c.FirstName = firstName.String
	c.LastName = lastName.String
	c.Company = company.String
	c.Phone = phone.String
	c.Notes = notes.String
	c.Source = source.String

	if tagsBuf != nil {
		if err := json.Unmarshal(tagsBuf, &c.Tags); err != nil {
			return Contact{}, fmt.Errorf("unmarshal tags: %w", err)
		}
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}

	if customBuf != nil {
		if err := json.Unmarshal(customBuf, &c.Custom); err != nil {
			return Contact{}, fmt.Errorf("unmarshal custom: %w", err)
		}
	}
	if c.Custom == nil {
		c.Custom = make(map[string]any)
	}

	return c, nil
}

func (r *Repo) UpdateContact(ctx context.Context, id int64, patch ContactPatch) (Contact, error) {
	var updateParts []string
	var args []any

	if patch.Email != nil {
		updateParts = append(updateParts, "email = ?")
		args = append(args, *patch.Email)
	}
	if patch.FirstName != nil {
		updateParts = append(updateParts, "first_name = ?")
		args = append(args, *patch.FirstName)
	}
	if patch.LastName != nil {
		updateParts = append(updateParts, "last_name = ?")
		args = append(args, *patch.LastName)
	}
	if patch.Company != nil {
		updateParts = append(updateParts, "company = ?")
		args = append(args, *patch.Company)
	}
	if patch.Phone != nil {
		updateParts = append(updateParts, "phone = ?")
		args = append(args, *patch.Phone)
	}
	if patch.Stage != nil {
		if !validStage(*patch.Stage) {
			return Contact{}, fmt.Errorf("invalid stage: %q", *patch.Stage)
		}
		updateParts = append(updateParts, "stage = ?")
		args = append(args, *patch.Stage)
	}
	if patch.Tags != nil {
		var tagsJSON []byte
		if *patch.Tags != nil {
			var err error
			tagsJSON, err = json.Marshal(*patch.Tags)
			if err != nil {
				return Contact{}, fmt.Errorf("marshal tags: %w", err)
			}
		}
		updateParts = append(updateParts, "tags = ?")
		args = append(args, tagsJSON)
	}
	if patch.Notes != nil {
		updateParts = append(updateParts, "notes = ?")
		args = append(args, *patch.Notes)
	}
	if patch.Custom != nil {
		var customJSON []byte
		if *patch.Custom != nil {
			var err error
			customJSON, err = json.Marshal(*patch.Custom)
			if err != nil {
				return Contact{}, fmt.Errorf("marshal custom: %w", err)
			}
		}
		updateParts = append(updateParts, "custom = ?")
		args = append(args, customJSON)
	}
	if patch.Source != nil {
		updateParts = append(updateParts, "source = ?")
		args = append(args, *patch.Source)
	}

	if len(updateParts) == 0 {
		return r.GetContact(ctx, id)
	}

	query := fmt.Sprintf("UPDATE contacts SET %s WHERE id = ?", strings.Join(updateParts, ", "))
	args = append(args, id)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return Contact{}, fmt.Errorf("update contact: %w", err)
	}

	return r.GetContact(ctx, id)
}

func (r *Repo) ListContacts(ctx context.Context, f ContactFilter, limit int, cursor int64) (items []Contact, total int, nextCursor int64, err error) {
	var whereParts []string
	var args []any

	if f.Stage != "" {
		whereParts = append(whereParts, "stage = ?")
		args = append(args, f.Stage)
	}
	if f.Company != "" {
		whereParts = append(whereParts, "company = ?")
		args = append(args, f.Company)
	}
	if f.Tag != "" {
		whereParts = append(whereParts, "JSON_CONTAINS(tags, JSON_QUOTE(?))")
		args = append(args, f.Tag)
	}
	if f.Q != "" {
		whereParts = append(whereParts, "(email LIKE ? OR first_name LIKE ? OR last_name LIKE ? OR company LIKE ?)")
		likeVal := "%" + f.Q + "%"
		args = append(args, likeVal, likeVal, likeVal, likeVal)
	}

	// 1. Get total count (ignoring cursor)
	countQuery := "SELECT COUNT(*) FROM contacts"
	if len(whereParts) > 0 {
		countQuery += " WHERE " + strings.Join(whereParts, " AND ")
	}
	err = r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list contacts count: %w", err)
	}

	// 2. Limit defaults/caps
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	// 3. Keyset pagination: id > cursor
	queryParts := make([]string, len(whereParts))
	copy(queryParts, whereParts)
	queryArgs := make([]any, len(args))
	copy(queryArgs, args)

	if cursor > 0 {
		queryParts = append(queryParts, "id > ?")
		queryArgs = append(queryArgs, cursor)
	}

	listQuery := "SELECT id, email, first_name, last_name, company, phone, stage, tags, notes, custom, source, created_at, updated_at FROM contacts"
	if len(queryParts) > 0 {
		listQuery += " WHERE " + strings.Join(queryParts, " AND ")
	}
	listQuery += " ORDER BY id ASC LIMIT ?"
	queryArgs = append(queryArgs, limit)

	rows, err := r.db.QueryContext(ctx, listQuery, queryArgs...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list contacts query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c Contact
		var (
			firstName sql.NullString
			lastName  sql.NullString
			company   sql.NullString
			phone     sql.NullString
			notes     sql.NullString
			source    sql.NullString
			tagsBuf   []byte
			customBuf []byte
		)
		err = rows.Scan(
			&c.ID,
			&c.Email,
			&firstName,
			&lastName,
			&company,
			&phone,
			&c.Stage,
			&tagsBuf,
			&notes,
			&customBuf,
			&source,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("list contacts scan: %w", err)
		}

		c.FirstName = firstName.String
		c.LastName = lastName.String
		c.Company = company.String
		c.Phone = phone.String
		c.Notes = notes.String
		c.Source = source.String

		if tagsBuf != nil {
			if err := json.Unmarshal(tagsBuf, &c.Tags); err != nil {
				return nil, 0, 0, fmt.Errorf("unmarshal tags: %w", err)
			}
		}
		if c.Tags == nil {
			c.Tags = []string{}
		}

		if customBuf != nil {
			if err := json.Unmarshal(customBuf, &c.Custom); err != nil {
				return nil, 0, 0, fmt.Errorf("unmarshal custom: %w", err)
			}
		}
		if c.Custom == nil {
			c.Custom = make(map[string]any)
		}

		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, fmt.Errorf("list contacts rows: %w", err)
	}

	if len(items) == limit {
		nextCursor = items[len(items)-1].ID
	}

	return items, total, nextCursor, nil
}

func (r *Repo) CountByStage(ctx context.Context) (map[string]int, error) {
	query := "SELECT stage, COUNT(*) FROM contacts GROUP BY stage"
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count by stage: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var stage string
		var count int
		if err := rows.Scan(&stage, &count); err != nil {
			return nil, fmt.Errorf("scan count by stage: %w", err)
		}
		counts[stage] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows count by stage: %w", err)
	}
	return counts, nil
}

func validStage(s string) bool {
	for _, v := range ValidStages {
		if s == v {
			return true
		}
	}
	return false
}
