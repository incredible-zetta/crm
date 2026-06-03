package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

type templateRepo struct {
	db *sql.DB
}

var _ port.TemplateRepo = (*templateRepo)(nil)

func (r *templateRepo) Create(ctx context.Context, t domain.Template) (domain.Template, error) {
	variablesJSON, err := marshalJSON(t.Variables, "[]")
	if err != nil {
		return domain.Template{}, fmt.Errorf("marshal variables: %w", err)
	}

	query := `INSERT INTO email_templates (name, subject, body_html, body_text, variables, deleted_at) VALUES (?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, t.Name, t.Subject, t.BodyHTML, t.BodyText, variablesJSON, toNullTime(t.DeletedAt))
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			return domain.Template{}, fmt.Errorf("template with name %q already exists: %w", t.Name, domain.ErrConflict)
		}
		return domain.Template{}, fmt.Errorf("create template: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return domain.Template{}, fmt.Errorf("last insert id: %w", err)
	}

	return r.Get(ctx, id)
}

func (r *templateRepo) Get(ctx context.Context, id int64) (domain.Template, error) {
	query := `SELECT id, name, subject, body_html, body_text, variables, deleted_at, created_at, updated_at FROM email_templates WHERE id = ? AND deleted_at IS NULL`
	row := r.db.QueryRowContext(ctx, query, id)
	t, err := scanTemplate(row)
	if err == sql.ErrNoRows {
		return domain.Template{}, fmt.Errorf("template not found: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Template{}, fmt.Errorf("get template: %w", err)
	}
	return t, nil
}

func (r *templateRepo) GetByName(ctx context.Context, name string) (domain.Template, error) {
	query := `SELECT id, name, subject, body_html, body_text, variables, deleted_at, created_at, updated_at FROM email_templates WHERE name = ? AND deleted_at IS NULL`
	row := r.db.QueryRowContext(ctx, query, name)
	t, err := scanTemplate(row)
	if err == sql.ErrNoRows {
		return domain.Template{}, fmt.Errorf("template not found by name: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Template{}, fmt.Errorf("get template by name: %w", err)
	}
	return t, nil
}

func (r *templateRepo) List(ctx context.Context) ([]domain.Template, error) {
	query := `SELECT id, name, subject, body_html, body_text, variables, deleted_at, created_at, updated_at FROM email_templates WHERE deleted_at IS NULL ORDER BY name ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var templates []domain.Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list templates rows: %w", err)
	}
	return templates, nil
}

func (r *templateRepo) Update(ctx context.Context, id int64, t domain.Template) (domain.Template, error) {
	variablesJSON, err := marshalJSON(t.Variables, "[]")
	if err != nil {
		return domain.Template{}, fmt.Errorf("marshal variables: %w", err)
	}

	query := `UPDATE email_templates SET name = ?, subject = ?, body_html = ?, body_text = ?, variables = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, t.Name, t.Subject, t.BodyHTML, t.BodyText, variablesJSON, id)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			return domain.Template{}, fmt.Errorf("template name conflict: %w", domain.ErrConflict)
		}
		return domain.Template{}, fmt.Errorf("update template: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return domain.Template{}, fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		_, err := r.Get(ctx, id)
		if err != nil {
			return domain.Template{}, err
		}
	}

	return r.Get(ctx, id)
}

func (r *templateRepo) SoftDelete(ctx context.Context, id int64) error {
	query := `UPDATE email_templates SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("soft delete template: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("template not found or already deleted: %w", domain.ErrNotFound)
	}

	return nil
}

func scanTemplate(row interface{ Scan(dest ...any) error }) (domain.Template, error) {
	var t domain.Template
	var (
		subject      sql.NullString
		bodyHTML     sql.NullString
		bodyText     sql.NullString
		variablesBuf []byte
		deletedAt    sql.NullTime
	)

	err := row.Scan(
		&t.ID,
		&t.Name,
		&subject,
		&bodyHTML,
		&bodyText,
		&variablesBuf,
		&deletedAt,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		return domain.Template{}, err
	}

	t.Subject = subject.String
	t.BodyHTML = bodyHTML.String
	t.BodyText = bodyText.String
	t.DeletedAt = fromNullTime(deletedAt)

	if variablesBuf != nil {
		if err := unmarshalJSON(variablesBuf, &t.Variables); err != nil {
			return domain.Template{}, fmt.Errorf("unmarshal variables: %w", err)
		}
	}
	if t.Variables == nil {
		t.Variables = []string{}
	}

	return t, nil
}
