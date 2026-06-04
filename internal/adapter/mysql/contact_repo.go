package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

type contactRepo struct {
	db *sql.DB
}

var _ port.ContactRepo = (*contactRepo)(nil)

func (r *contactRepo) Upsert(ctx context.Context, c domain.Contact) (domain.Contact, error) {
	originalStage := c.Stage
	if c.Stage == "" {
		c.Stage = domain.StageNew
	}

	if !c.Stage.Valid() {
		return domain.Contact{}, fmt.Errorf("%w: invalid stage: %q", domain.ErrValidation, c.Stage)
	}

	var tagsJSON []byte
	if c.Tags != nil {
		var err error
		tagsJSON, err = marshalJSON(c.Tags, "[]")
		if err != nil {
			return domain.Contact{}, fmt.Errorf("marshal tags: %w", err)
		}
	}

	var customJSON []byte
	if c.Custom != nil {
		var err error
		customJSON, err = marshalJSON(c.Custom, "{}")
		if err != nil {
			return domain.Contact{}, fmt.Errorf("marshal custom: %w", err)
		}
	}

	query := `INSERT INTO contacts (email, first_name, last_name, company, phone, stage, tags, notes, custom, source, unsub_code, unsubscribed_at, deleted_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  first_name = IF(VALUES(first_name) != '', VALUES(first_name), first_name),
  last_name  = IF(VALUES(last_name)  != '', VALUES(last_name),  last_name),
  company    = IF(VALUES(company)    != '', VALUES(company),    company),
  phone      = IF(VALUES(phone)      != '', VALUES(phone),      phone),
  stage      = IF(? != '', VALUES(stage), stage),
  tags       = IF(VALUES(tags) IS NOT NULL, VALUES(tags), tags),
  notes      = IF(VALUES(notes)      != '', VALUES(notes),      notes),
  custom     = IF(VALUES(custom) IS NOT NULL, VALUES(custom), custom),
  source     = IF(VALUES(source)     != '', VALUES(source),     source),
  unsub_code = IF(VALUES(unsub_code) IS NOT NULL AND VALUES(unsub_code) != '', VALUES(unsub_code), unsub_code),
  unsubscribed_at = IF(VALUES(unsubscribed_at) IS NOT NULL, VALUES(unsubscribed_at), unsubscribed_at),
  deleted_at = IF(VALUES(deleted_at) IS NOT NULL, VALUES(deleted_at), deleted_at)`

	_, err := r.db.ExecContext(ctx, query,
		c.Email,
		toNullString(c.FirstName),
		toNullString(c.LastName),
		toNullString(c.Company),
		toNullString(c.Phone),
		string(c.Stage),
		tagsJSON,
		toNullString(c.Notes),
		customJSON,
		toNullString(c.Source),
		toNullString(c.UnsubCode),
		toNullTime(c.UnsubscribedAt),
		toNullTime(c.DeletedAt),
		string(originalStage),
	)
	if err != nil {
		return domain.Contact{}, fmt.Errorf("upsert contact exec: %w", err)
	}

	return r.GetByEmail(ctx, c.Email)
}

func (r *contactRepo) Get(ctx context.Context, id int64) (domain.Contact, error) {
	query := `SELECT id, email, first_name, last_name, company, phone, stage, tags, notes, custom, source, unsub_code, unsubscribed_at, deleted_at, created_at, updated_at 
FROM contacts WHERE id = ? AND deleted_at IS NULL`
	row := r.db.QueryRowContext(ctx, query, id)
	c, err := scanContact(row)
	if err == sql.ErrNoRows {
		return domain.Contact{}, fmt.Errorf("contact not found: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Contact{}, fmt.Errorf("get contact: %w", err)
	}
	return c, nil
}

func (r *contactRepo) GetByEmail(ctx context.Context, email string) (domain.Contact, error) {
	query := `SELECT id, email, first_name, last_name, company, phone, stage, tags, notes, custom, source, unsub_code, unsubscribed_at, deleted_at, created_at, updated_at 
FROM contacts WHERE email = ? AND deleted_at IS NULL`
	row := r.db.QueryRowContext(ctx, query, email)
	c, err := scanContact(row)
	if err == sql.ErrNoRows {
		return domain.Contact{}, fmt.Errorf("contact not found: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Contact{}, fmt.Errorf("get contact by email: %w", err)
	}
	return c, nil
}

func (r *contactRepo) GetByUnsubCode(ctx context.Context, code string) (domain.Contact, error) {
	query := `SELECT id, email, first_name, last_name, company, phone, stage, tags, notes, custom, source, unsub_code, unsubscribed_at, deleted_at, created_at, updated_at 
FROM contacts WHERE unsub_code = ?`
	row := r.db.QueryRowContext(ctx, query, code)
	c, err := scanContact(row)
	if err == sql.ErrNoRows {
		return domain.Contact{}, fmt.Errorf("contact not found by unsub code: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Contact{}, fmt.Errorf("get contact by unsub code: %w", err)
	}
	return c, nil
}

func (r *contactRepo) Update(ctx context.Context, id int64, patch domain.ContactPatch) (domain.Contact, error) {
	// Verify contact exists and is not soft deleted
	_, err := r.Get(ctx, id)
	if err != nil {
		return domain.Contact{}, err
	}

	var updateParts []string
	var args []any

	if patch.Email != nil {
		updateParts = append(updateParts, "email = ?")
		args = append(args, *patch.Email)
	}
	if patch.FirstName != nil {
		updateParts = append(updateParts, "first_name = ?")
		args = append(args, toNullString(*patch.FirstName))
	}
	if patch.LastName != nil {
		updateParts = append(updateParts, "last_name = ?")
		args = append(args, toNullString(*patch.LastName))
	}
	if patch.Company != nil {
		updateParts = append(updateParts, "company = ?")
		args = append(args, toNullString(*patch.Company))
	}
	if patch.Phone != nil {
		updateParts = append(updateParts, "phone = ?")
		args = append(args, toNullString(*patch.Phone))
	}
	if patch.Stage != nil {
		s := domain.Stage(*patch.Stage)
		if !s.Valid() {
			return domain.Contact{}, fmt.Errorf("%w: invalid stage: %q", domain.ErrValidation, *patch.Stage)
		}
		updateParts = append(updateParts, "stage = ?")
		args = append(args, *patch.Stage)
	}
	if patch.Tags != nil {
		var tagsJSON []byte
		if *patch.Tags != nil {
			var err error
			tagsJSON, err = marshalJSON(*patch.Tags, "[]")
			if err != nil {
				return domain.Contact{}, fmt.Errorf("marshal tags: %w", err)
			}
		}
		updateParts = append(updateParts, "tags = ?")
		args = append(args, tagsJSON)
	}
	if patch.Notes != nil {
		updateParts = append(updateParts, "notes = ?")
		args = append(args, toNullString(*patch.Notes))
	}
	if patch.Custom != nil {
		var customJSON []byte
		if *patch.Custom != nil {
			var err error
			customJSON, err = marshalJSON(*patch.Custom, "{}")
			if err != nil {
				return domain.Contact{}, fmt.Errorf("marshal custom: %w", err)
			}
		}
		updateParts = append(updateParts, "custom = ?")
		args = append(args, customJSON)
	}
	if patch.Source != nil {
		updateParts = append(updateParts, "source = ?")
		args = append(args, toNullString(*patch.Source))
	}

	if len(updateParts) == 0 {
		return r.Get(ctx, id)
	}

	query := fmt.Sprintf("UPDATE contacts SET %s WHERE id = ? AND deleted_at IS NULL", strings.Join(updateParts, ", "))
	args = append(args, id)

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return domain.Contact{}, fmt.Errorf("update contact: %w", err)
	}

	return r.Get(ctx, id)
}

func (r *contactRepo) List(ctx context.Context, f domain.ContactFilter, p port.Paging) (port.ContactPage, error) {
	var whereParts []string
	var args []any

	whereParts = append(whereParts, "deleted_at IS NULL")

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

	// Get total count
	countQuery := "SELECT COUNT(*) FROM contacts WHERE " + strings.Join(whereParts, " AND ")
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return port.ContactPage{}, fmt.Errorf("list contacts count: %w", err)
	}

	limit := p.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	queryParts := make([]string, len(whereParts))
	copy(queryParts, whereParts)
	queryArgs := make([]any, len(args))
	copy(queryArgs, args)

	if p.Cursor > 0 {
		queryParts = append(queryParts, "id > ?")
		queryArgs = append(queryArgs, p.Cursor)
	}

	listQuery := `SELECT id, email, first_name, last_name, company, phone, stage, tags, notes, custom, source, unsub_code, unsubscribed_at, deleted_at, created_at, updated_at 
FROM contacts WHERE ` + strings.Join(queryParts, " AND ") + " ORDER BY id ASC LIMIT ?"
	queryArgs = append(queryArgs, limit+1)

	rows, err := r.db.QueryContext(ctx, listQuery, queryArgs...)
	if err != nil {
		return port.ContactPage{}, fmt.Errorf("list contacts query: %w", err)
	}
	defer rows.Close()

	var items []domain.Contact
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return port.ContactPage{}, fmt.Errorf("list contacts scan: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return port.ContactPage{}, fmt.Errorf("list contacts rows: %w", err)
	}

	var nextCursor int64
	if len(items) > limit {
		items = items[:limit]
		nextCursor = items[limit-1].ID
	}

	return port.ContactPage{
		Items:      items,
		Total:      total,
		NextCursor: nextCursor,
	}, nil
}

func (r *contactRepo) CountByStage(ctx context.Context) (map[string]int, error) {
	query := "SELECT stage, COUNT(*) FROM contacts WHERE deleted_at IS NULL GROUP BY stage"
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

func (r *contactRepo) SoftDelete(ctx context.Context, id int64) error {
	query := "UPDATE contacts SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL"
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("soft delete contact: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("contact not found or already deleted: %w", domain.ErrNotFound)
	}
	return nil
}

func (r *contactRepo) Purge(ctx context.Context, id int64) error {
	query := "DELETE FROM contacts WHERE id = ?"
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("purge contact: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("contact not found: %w", domain.ErrNotFound)
	}
	return nil
}

func (r *contactRepo) SetUnsubscribed(ctx context.Context, id int64, at time.Time) error {
	query := "UPDATE contacts SET unsubscribed_at = ? WHERE id = ?"
	res, err := r.db.ExecContext(ctx, query, at, id)
	if err != nil {
		return fmt.Errorf("set unsubscribed contact: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		var exists int
		if err := r.db.QueryRowContext(ctx, "SELECT 1 FROM contacts WHERE id = ? AND deleted_at IS NULL", id).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("contact not found: %w", domain.ErrNotFound)
			}
			return fmt.Errorf("check contact exists: %w", err)
		}
	}
	return nil
}

func (r *contactRepo) SetUnsubCode(ctx context.Context, id int64, code string) error {
	query := "UPDATE contacts SET unsub_code = ? WHERE id = ?"
	res, err := r.db.ExecContext(ctx, query, code, id)
	if err != nil {
		return fmt.Errorf("set unsub code contact: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("contact not found: %w", domain.ErrNotFound)
	}
	return nil
}

func scanContact(row interface{ Scan(dest ...any) error }) (domain.Contact, error) {
	var c domain.Contact
	var (
		firstName      sql.NullString
		lastName       sql.NullString
		company        sql.NullString
		phone          sql.NullString
		notes          sql.NullString
		source         sql.NullString
		unsubCode      sql.NullString
		unsubscribedAt sql.NullTime
		deletedAt      sql.NullTime
		tagsBuf        []byte
		customBuf      []byte
	)

	err := row.Scan(
		&c.ID,
		&c.Email,
		&firstName,
		&lastName,
		&company,
		&phone,
		(*string)(&c.Stage),
		&tagsBuf,
		&notes,
		&customBuf,
		&source,
		&unsubCode,
		&unsubscribedAt,
		&deletedAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return domain.Contact{}, err
	}

	c.FirstName = firstName.String
	c.LastName = lastName.String
	c.Company = company.String
	c.Phone = phone.String
	c.Notes = notes.String
	c.Source = source.String
	c.UnsubCode = unsubCode.String
	c.UnsubscribedAt = fromNullTime(unsubscribedAt)
	c.DeletedAt = fromNullTime(deletedAt)

	if tagsBuf != nil {
		if err := unmarshalJSON(tagsBuf, &c.Tags); err != nil {
			return domain.Contact{}, fmt.Errorf("unmarshal tags: %w", err)
		}
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}

	if customBuf != nil {
		if err := unmarshalJSON(customBuf, &c.Custom); err != nil {
			return domain.Contact{}, fmt.Errorf("unmarshal custom: %w", err)
		}
	}
	if c.Custom == nil {
		c.Custom = make(map[string]any)
	}

	return c, nil
}
