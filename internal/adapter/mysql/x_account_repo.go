package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
)

type xAccountRepo struct{ db *sql.DB }

var _ port.XAccountRepo = (*xAccountRepo)(nil)

func xAccountSelectSQL() string {
	return `SELECT id, label, screen_name, user_id, cookies, liveness, last_checked_at, last_error, created_at, updated_at FROM x_accounts`
}

func (r *xAccountRepo) Save(ctx context.Context, in port.XAccountSaveInput) (domain.XAccount, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO x_accounts
		(tenant_id, label, screen_name, user_id, cookies, liveness)
		VALUES (?, ?, ?, ?, ?, 'unknown')
		ON DUPLICATE KEY UPDATE screen_name=VALUES(screen_name), user_id=VALUES(user_id), cookies=VALUES(cookies), liveness='unknown', last_error=NULL, deleted_at=NULL`,
		tenant.From(ctx), in.Label, toNullString(in.ScreenName), toNullString(in.UserID), in.Cookies)
	if err != nil {
		return domain.XAccount{}, fmt.Errorf("save x account: %w", err)
	}
	return r.GetByLabel(ctx, in.Label)
}

func (r *xAccountRepo) GetByLabel(ctx context.Context, label string) (domain.XAccount, error) {
	return r.scan(r.db.QueryRowContext(ctx, xAccountSelectSQL()+` WHERE label = ? AND tenant_id = ? AND deleted_at IS NULL`, label, tenant.From(ctx)))
}

func (r *xAccountRepo) List(ctx context.Context) ([]domain.XAccount, error) {
	rows, err := r.db.QueryContext(ctx, xAccountSelectSQL()+` WHERE tenant_id = ? AND deleted_at IS NULL ORDER BY label`, tenant.From(ctx))
	if err != nil {
		return nil, fmt.Errorf("list x accounts: %w", err)
	}
	defer rows.Close()
	var out []domain.XAccount
	for rows.Next() {
		a, err := r.scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *xAccountRepo) Delete(ctx context.Context, label string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE x_accounts SET deleted_at = NOW() WHERE label = ? AND tenant_id = ? AND deleted_at IS NULL`, label, tenant.From(ctx))
	if err != nil {
		return fmt.Errorf("delete x account: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *xAccountRepo) UpdateLiveness(ctx context.Context, id int64, liveness domain.XLiveness, screenName, userID, lastErr string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE x_accounts
		SET liveness = ?, last_checked_at = NOW(),
		    screen_name = COALESCE(NULLIF(?, ''), screen_name),
		    user_id = COALESCE(NULLIF(?, ''), user_id),
		    last_error = ?
		WHERE id = ? AND tenant_id = ?`,
		string(liveness), screenName, userID, toNullString(lastErr), id, tenant.From(ctx))
	if err != nil {
		return fmt.Errorf("update x account liveness: %w", err)
	}
	return nil
}

func (r *xAccountRepo) ListStale(ctx context.Context, cutoff time.Time, limit int) ([]port.XAccountWithTenant, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT tenant_id, id, label, screen_name, user_id, cookies, liveness, last_checked_at, last_error, created_at, updated_at
		FROM x_accounts
		WHERE deleted_at IS NULL AND (last_checked_at IS NULL OR last_checked_at < ?)
		ORDER BY last_checked_at IS NOT NULL, last_checked_at ASC
		LIMIT ?`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list stale x accounts: %w", err)
	}
	defer rows.Close()
	var out []port.XAccountWithTenant
	for rows.Next() {
		var (
			tenantID  string
			a         domain.XAccount
			screen    sql.NullString
			userID    sql.NullString
			liveness  string
			lastCheck sql.NullTime
			lastErr   sql.NullString
		)
		if err := rows.Scan(&tenantID, &a.ID, &a.Label, &screen, &userID, &a.Cookies, &liveness, &lastCheck, &lastErr, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan stale x account: %w", err)
		}
		a.ScreenName = fromNullString(screen)
		a.UserID = fromNullString(userID)
		a.Liveness = domain.XLiveness(liveness)
		a.LastCheckedAt = fromNullTime(lastCheck)
		a.LastError = fromNullString(lastErr)
		out = append(out, port.XAccountWithTenant{TenantID: tenantID, Account: a})
	}
	return out, rows.Err()
}

func (r *xAccountRepo) scan(row *sql.Row) (domain.XAccount, error) {
	var (
		a         domain.XAccount
		screen    sql.NullString
		userID    sql.NullString
		liveness  string
		lastCheck sql.NullTime
		lastErr   sql.NullString
	)
	err := row.Scan(&a.ID, &a.Label, &screen, &userID, &a.Cookies, &liveness, &lastCheck, &lastErr, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return domain.XAccount{}, err
	}
	a.ScreenName = fromNullString(screen)
	a.UserID = fromNullString(userID)
	a.Liveness = domain.XLiveness(liveness)
	a.LastCheckedAt = fromNullTime(lastCheck)
	a.LastError = fromNullString(lastErr)
	return a, nil
}

func (r *xAccountRepo) scanRows(rows *sql.Rows) (domain.XAccount, error) {
	var (
		a         domain.XAccount
		screen    sql.NullString
		userID    sql.NullString
		liveness  string
		lastCheck sql.NullTime
		lastErr   sql.NullString
	)
	err := rows.Scan(&a.ID, &a.Label, &screen, &userID, &a.Cookies, &liveness, &lastCheck, &lastErr, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return domain.XAccount{}, err
	}
	a.ScreenName = fromNullString(screen)
	a.UserID = fromNullString(userID)
	a.Liveness = domain.XLiveness(liveness)
	a.LastCheckedAt = fromNullTime(lastCheck)
	a.LastError = fromNullString(lastErr)
	return a, nil
}
