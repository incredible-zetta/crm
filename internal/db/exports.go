package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Export struct {
	ID        string
	Path      string
	Rows      int
	ExpiresAt *time.Time // nullable
	CreatedAt time.Time
}

func (r *Repo) CreateExport(ctx context.Context, e Export) error {
	query := "INSERT INTO exports (id, path, `rows`, expires_at) VALUES (?, ?, ?, ?)"
	_, err := r.db.ExecContext(ctx, query, e.ID, e.Path, e.Rows, e.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create export: %w", err)
	}
	return nil
}

func (r *Repo) GetExport(ctx context.Context, id string) (Export, error) {
	query := "SELECT id, path, `rows`, expires_at, created_at FROM exports WHERE id = ?"
	var e Export
	var expiresAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID,
		&e.Path,
		&e.Rows,
		&expiresAt,
		&e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return Export{}, fmt.Errorf("export not found: %w", ErrNotFound)
	}
	if err != nil {
		return Export{}, fmt.Errorf("get export: %w", err)
	}

	if expiresAt.Valid {
		e.ExpiresAt = &expiresAt.Time
	}

	return e, nil
}
