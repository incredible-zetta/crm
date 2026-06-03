package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type EmailTemplate struct {
	ID        int64
	Name      string
	Subject   string
	BodyHTML  string
	BodyText  string
	Variables []string // stored as JSON
	CreatedAt time.Time
}

func (r *Repo) CreateTemplate(ctx context.Context, t EmailTemplate) (EmailTemplate, error) {
	var variablesJSON []byte
	if t.Variables != nil {
		var err error
		variablesJSON, err = json.Marshal(t.Variables)
		if err != nil {
			return EmailTemplate{}, fmt.Errorf("marshal variables: %w", err)
		}
	} else {
		variablesJSON = []byte("[]")
	}

	query := `INSERT INTO email_templates (name, subject, body_html, body_text, variables) VALUES (?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, t.Name, t.Subject, t.BodyHTML, t.BodyText, variablesJSON)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
			return EmailTemplate{}, fmt.Errorf("template with name %q already exists: %w", t.Name, err)
		}
		if strings.Contains(err.Error(), "Duplicate entry") {
			return EmailTemplate{}, fmt.Errorf("template with name %q already exists: %w", t.Name, err)
		}
		return EmailTemplate{}, fmt.Errorf("create template: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return EmailTemplate{}, fmt.Errorf("last insert id: %w", err)
	}

	return r.GetTemplate(ctx, id)
}

func (r *Repo) GetTemplate(ctx context.Context, id int64) (EmailTemplate, error) {
	query := `SELECT id, name, subject, body_html, body_text, variables, created_at FROM email_templates WHERE id = ?`
	var t EmailTemplate
	var (
		subject      sql.NullString
		bodyHTML     sql.NullString
		bodyText     sql.NullString
		variablesBuf []byte
	)

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID,
		&t.Name,
		&subject,
		&bodyHTML,
		&bodyText,
		&variablesBuf,
		&t.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return EmailTemplate{}, fmt.Errorf("template not found: %w", ErrNotFound)
	}
	if err != nil {
		return EmailTemplate{}, fmt.Errorf("get template: %w", err)
	}

	t.Subject = subject.String
	t.BodyHTML = bodyHTML.String
	t.BodyText = bodyText.String

	if variablesBuf != nil {
		if err := json.Unmarshal(variablesBuf, &t.Variables); err != nil {
			return EmailTemplate{}, fmt.Errorf("unmarshal variables: %w", err)
		}
	}
	if t.Variables == nil {
		t.Variables = []string{}
	}

	return t, nil
}

func (r *Repo) GetTemplateByName(ctx context.Context, name string) (EmailTemplate, error) {
	query := `SELECT id, name, subject, body_html, body_text, variables, created_at FROM email_templates WHERE name = ?`
	var t EmailTemplate
	var (
		subject      sql.NullString
		bodyHTML     sql.NullString
		bodyText     sql.NullString
		variablesBuf []byte
	)

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&t.ID,
		&t.Name,
		&subject,
		&bodyHTML,
		&bodyText,
		&variablesBuf,
		&t.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return EmailTemplate{}, fmt.Errorf("template not found by name: %w", ErrNotFound)
	}
	if err != nil {
		return EmailTemplate{}, fmt.Errorf("get template by name: %w", err)
	}

	t.Subject = subject.String
	t.BodyHTML = bodyHTML.String
	t.BodyText = bodyText.String

	if variablesBuf != nil {
		if err := json.Unmarshal(variablesBuf, &t.Variables); err != nil {
			return EmailTemplate{}, fmt.Errorf("unmarshal variables: %w", err)
		}
	}
	if t.Variables == nil {
		t.Variables = []string{}
	}

	return t, nil
}

func (r *Repo) ListTemplates(ctx context.Context) ([]EmailTemplate, error) {
	query := `SELECT id, name, subject, body_html, body_text, variables, created_at FROM email_templates ORDER BY name ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list templates query: %w", err)
	}
	defer rows.Close()

	var templates []EmailTemplate
	for rows.Next() {
		var t EmailTemplate
		var (
			subject      sql.NullString
			bodyHTML     sql.NullString
			bodyText     sql.NullString
			variablesBuf []byte
		)
		err := rows.Scan(
			&t.ID,
			&t.Name,
			&subject,
			&bodyHTML,
			&bodyText,
			&variablesBuf,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("list templates scan: %w", err)
		}

		t.Subject = subject.String
		t.BodyHTML = bodyHTML.String
		t.BodyText = bodyText.String

		if variablesBuf != nil {
			if err := json.Unmarshal(variablesBuf, &t.Variables); err != nil {
				return nil, fmt.Errorf("unmarshal variables: %w", err)
			}
		}
		if t.Variables == nil {
			t.Variables = []string{}
		}

		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list templates rows: %w", err)
	}

	return templates, nil
}
