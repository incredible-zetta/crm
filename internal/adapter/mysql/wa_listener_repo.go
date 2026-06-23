package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
)

type waListenerRepo struct{ db *sql.DB }

var _ port.WAListenerRepo = (*waListenerRepo)(nil)

func (r *waListenerRepo) Create(ctx context.Context, l domain.WAListener) (domain.WAListener, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO wa_listeners (tenant_id, chat_jid, name, enabled, summary) VALUES (?, ?, ?, ?, ?)`, tenant.From(ctx), l.ChatJID, toNullString(l.Name), l.Enabled, toNullString(l.Summary))
	if err != nil {
		return domain.WAListener{}, fmt.Errorf("create wa listener: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.WAListener{}, err
	}
	return r.Get(ctx, id)
}

func (r *waListenerRepo) Get(ctx context.Context, id int64) (domain.WAListener, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `SELECT id, chat_jid, name, enabled, summary, deleted_at, created_at, updated_at FROM wa_listeners WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`, id, tenant.From(ctx)))
}

func (r *waListenerRepo) GetByChatJID(ctx context.Context, chatJID string) (domain.WAListener, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `SELECT id, chat_jid, name, enabled, summary, deleted_at, created_at, updated_at FROM wa_listeners WHERE chat_jid = ? AND tenant_id = ? AND deleted_at IS NULL`, chatJID, tenant.From(ctx)))
}

func (r *waListenerRepo) List(ctx context.Context, enabledOnly bool) ([]domain.WAListener, error) {
	query := `SELECT id, chat_jid, name, enabled, summary, deleted_at, created_at, updated_at FROM wa_listeners WHERE deleted_at IS NULL AND tenant_id = ?`
	if enabledOnly {
		query += ` AND enabled = TRUE`
	}
	query += ` ORDER BY id DESC`
	rows, err := r.db.QueryContext(ctx, query, tenant.From(ctx))
	if err != nil {
		return nil, fmt.Errorf("list wa listeners: %w", err)
	}
	defer rows.Close()
	var out []domain.WAListener
	for rows.Next() {
		l, err := scanWAListener(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *waListenerRepo) Update(ctx context.Context, id int64, l domain.WAListener) (domain.WAListener, error) {
	_, err := r.db.ExecContext(ctx, `UPDATE wa_listeners SET chat_jid = ?, name = ?, enabled = ?, summary = ? WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`, l.ChatJID, toNullString(l.Name), l.Enabled, toNullString(l.Summary), id, tenant.From(ctx))
	if err != nil {
		return domain.WAListener{}, fmt.Errorf("update wa listener: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *waListenerRepo) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wa_listeners SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`, id, tenant.From(ctx))
	return err
}

func (r *waListenerRepo) SetSummary(ctx context.Context, id int64, summary string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wa_listeners SET summary = ? WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`, summary, id, tenant.From(ctx))
	return err
}

func (r *waListenerRepo) scanOne(row rowScanner) (domain.WAListener, error) {
	l, err := scanWAListener(row)
	if err == sql.ErrNoRows {
		return domain.WAListener{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.WAListener{}, err
	}
	return l, nil
}

func scanWAListener(scanner rowScanner) (domain.WAListener, error) {
	var l domain.WAListener
	var name, summary sql.NullString
	var deleted sql.NullTime
	if err := scanner.Scan(&l.ID, &l.ChatJID, &name, &l.Enabled, &summary, &deleted, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return domain.WAListener{}, err
	}
	if name.Valid {
		l.Name = name.String
	}
	if summary.Valid {
		l.Summary = summary.String
	}
	l.DeletedAt = fromNullTime(deleted)
	return l, nil
}
