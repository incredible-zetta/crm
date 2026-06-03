package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
)

type exportRepo struct {
	db *sql.DB
}

var _ port.ExportRepo = (*exportRepo)(nil)

func (r *exportRepo) Create(ctx context.Context, e domain.Export) error {
	query := "INSERT INTO exports (id, path, `rows`, expires_at) VALUES (?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, e.ID, e.Path, e.Rows, toNullTime(e.ExpiresAt))
	if err != nil {
		return fmt.Errorf("create export: %w", err)
	}
	return nil
}

func (r *exportRepo) Get(ctx context.Context, id string) (domain.Export, error) {
	query := "SELECT id, path, `rows`, expires_at, created_at FROM exports WHERE id = ?"
	var e domain.Export
	var expiresAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID,
		&e.Path,
		&e.Rows,
		&expiresAt,
		&e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return domain.Export{}, fmt.Errorf("export not found: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Export{}, fmt.Errorf("get export: %w", err)
	}

	e.ExpiresAt = fromNullTime(expiresAt)

	return e, nil
}
