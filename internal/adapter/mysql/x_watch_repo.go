package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
)

type xWatchRepo struct{ db *sql.DB }

var _ port.XWatchRepo = (*xWatchRepo)(nil)

func xWatchSelectSQL() string {
	return `SELECT id, label, kind, query, account_label, webhook_url, webhook_secret, webhook_headers,
		active, last_seen_id, last_polled_at, last_error, created_at, updated_at
		FROM x_watches`
}

func (r *xWatchRepo) Save(ctx context.Context, in port.XWatchSaveInput) (domain.XWatch, error) {
	// Defaults for a brand-new row; ON DUPLICATE only overwrites columns whose
	// pointer input is non-nil (COALESCE against VALUES sentinel is awkward, so
	// we read-modify-write for existing rows instead).
	existing, err := r.getByLabelTx(ctx, in.Label)
	switch {
	case err == sql.ErrNoRows:
		w := domain.XWatch{
			Label:  in.Label,
			Kind:   domain.XWatchMention,
			Active: true,
		}
		applyWatchPatch(&w, in)
		if _, err := r.db.ExecContext(ctx, `INSERT INTO x_watches
			(tenant_id, label, kind, query, account_label, webhook_url, webhook_secret, webhook_headers, active)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			tenant.From(ctx), w.Label, string(w.Kind), w.Query,
			toNullString(w.AccountLabel), toNullString(w.WebhookURL), toNullString(w.WebhookSecret),
			webhookHeadersJSON(w.WebhookHeaders), boolToInt(w.Active)); err != nil {
			return domain.XWatch{}, fmt.Errorf("insert x watch: %w", err)
		}
		return r.GetByLabel(ctx, in.Label)
	case err != nil:
		return domain.XWatch{}, err
	}

	applyWatchPatch(&existing, in)
	if _, err := r.db.ExecContext(ctx, `UPDATE x_watches
		SET kind=?, query=?, account_label=?, webhook_url=?, webhook_secret=?, webhook_headers=?, active=?, deleted_at=NULL
		WHERE id=? AND tenant_id=?`,
		string(existing.Kind), existing.Query, toNullString(existing.AccountLabel),
		toNullString(existing.WebhookURL), toNullString(existing.WebhookSecret),
		webhookHeadersJSON(existing.WebhookHeaders),
		boolToInt(existing.Active), existing.ID, tenant.From(ctx)); err != nil {
		return domain.XWatch{}, fmt.Errorf("update x watch: %w", err)
	}
	return r.GetByLabel(ctx, in.Label)
}

func applyWatchPatch(w *domain.XWatch, in port.XWatchSaveInput) {
	if in.Kind != nil {
		w.Kind = *in.Kind
	}
	if in.Query != nil {
		w.Query = *in.Query
	}
	if in.AccountLabel != nil {
		w.AccountLabel = *in.AccountLabel
	}
	if in.WebhookURL != nil {
		w.WebhookURL = *in.WebhookURL
	}
	if in.WebhookSecret != nil {
		w.WebhookSecret = *in.WebhookSecret
	}
	if in.WebhookHeaders != nil {
		w.WebhookHeaders = *in.WebhookHeaders
	}
	if in.Active != nil {
		w.Active = *in.Active
	}
}

func (r *xWatchRepo) getByLabelTx(ctx context.Context, label string) (domain.XWatch, error) {
	return scanXWatch(r.db.QueryRowContext(ctx,
		xWatchSelectSQL()+` WHERE label=? AND tenant_id=? AND deleted_at IS NULL`,
		label, tenant.From(ctx)))
}

func (r *xWatchRepo) GetByLabel(ctx context.Context, label string) (domain.XWatch, error) {
	return r.getByLabelTx(ctx, label)
}

func (r *xWatchRepo) List(ctx context.Context) ([]domain.XWatch, error) {
	rows, err := r.db.QueryContext(ctx,
		xWatchSelectSQL()+` WHERE tenant_id=? AND deleted_at IS NULL ORDER BY label`,
		tenant.From(ctx))
	if err != nil {
		return nil, fmt.Errorf("list x watches: %w", err)
	}
	defer rows.Close()
	var out []domain.XWatch
	for rows.Next() {
		w, err := scanXWatchRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *xWatchRepo) Delete(ctx context.Context, label string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE x_watches SET deleted_at=NOW() WHERE label=? AND tenant_id=? AND deleted_at IS NULL`,
		label, tenant.From(ctx))
	if err != nil {
		return fmt.Errorf("delete x watch: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *xWatchRepo) ListDue(ctx context.Context, limit int) ([]port.XWatchWithTenant, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT tenant_id, id, label, kind, query, account_label,
		webhook_url, webhook_secret, active, last_seen_id, last_polled_at, last_error, created_at, updated_at
		FROM x_watches
		WHERE deleted_at IS NULL AND active = 1
		ORDER BY last_polled_at IS NOT NULL, last_polled_at ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list due x watches: %w", err)
	}
	defer rows.Close()
	var out []port.XWatchWithTenant
	for rows.Next() {
		var (
			tenantID string
			w        domain.XWatch
			kind     string
			acct     sql.NullString
			hook     sql.NullString
			secret   sql.NullString
			headers  sql.NullString
			active   int
			lastSeen sql.NullString
			polled   sql.NullTime
			lastErr  sql.NullString
		)
		if err := rows.Scan(&tenantID, &w.ID, &w.Label, &kind, &w.Query, &acct,
			&hook, &secret, &headers, &active, &lastSeen, &polled, &lastErr, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan due x watch: %w", err)
		}
		w.Kind = domain.XWatchKind(kind)
		w.AccountLabel = fromNullString(acct)
		w.WebhookURL = fromNullString(hook)
		w.WebhookSecret = fromNullString(secret)
		w.WebhookHeaders = parseWebhookHeaders(headers)
		w.Active = active != 0
		w.LastSeenID = fromNullString(lastSeen)
		w.LastPolledAt = fromNullTime(polled)
		w.LastError = fromNullString(lastErr)
		out = append(out, port.XWatchWithTenant{TenantID: tenantID, Watch: w})
	}
	return out, rows.Err()
}

func (r *xWatchRepo) UpdatePollState(ctx context.Context, id int64, lastSeenID, lastErr string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE x_watches
		SET last_polled_at=NOW(),
		    last_seen_id=COALESCE(NULLIF(?, ''), last_seen_id),
		    last_error=?
		WHERE id=? AND tenant_id=?`,
		lastSeenID, toNullString(lastErr), id, tenant.From(ctx))
	if err != nil {
		return fmt.Errorf("update x watch poll state: %w", err)
	}
	return nil
}

func (r *xWatchRepo) AddEvent(ctx context.Context, watchID int64, ev domain.XWatchEvent) (bool, int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT IGNORE INTO x_watch_events
		(tenant_id, watch_id, tweet_id, author, text, url, likes, retweets, replies, tweet_created_at, delivery)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tenant.From(ctx), watchID, ev.TweetID, ev.Author, ev.Text, ev.URL,
		ev.Likes, ev.Retweets, ev.Replies, toNullString(ev.TweetCreatedAt), string(ev.Delivery))
	if err != nil {
		return false, 0, fmt.Errorf("add x watch event: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, 0, nil // duplicate (already seen)
	}
	id, _ := res.LastInsertId()
	return true, id, nil
}

func (r *xWatchRepo) MarkDelivered(ctx context.Context, eventID int64, status domain.XWatchDelivery, deliverErr string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE x_watch_events
		SET delivery=?, delivery_error=?, delivered_at=NOW()
		WHERE id=? AND tenant_id=?`,
		string(status), toNullString(deliverErr), eventID, tenant.From(ctx))
	if err != nil {
		return fmt.Errorf("mark x watch event delivered: %w", err)
	}
	return nil
}

func (r *xWatchRepo) ListEvents(ctx context.Context, watchID int64, delivery string, limit int) ([]domain.XWatchEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT id, watch_id, tweet_id, author, text, url, likes, retweets, replies,
		tweet_created_at, delivery, delivery_error, delivered_at, created_at
		FROM x_watch_events WHERE watch_id=? AND tenant_id=?`
	args := []any{watchID, tenant.From(ctx)}
	if delivery != "" {
		q += ` AND delivery=?`
		args = append(args, delivery)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list x watch events: %w", err)
	}
	defer rows.Close()
	var out []domain.XWatchEvent
	for rows.Next() {
		ev, err := scanXWatchEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func scanXWatch(row *sql.Row) (domain.XWatch, error)       { return scanXWatchGeneric(row) }
func scanXWatchRows(rows *sql.Rows) (domain.XWatch, error) { return scanXWatchGeneric(rows) }

func scanXWatchGeneric(s rowScanner) (domain.XWatch, error) {
	var (
		w        domain.XWatch
		kind     string
		acct     sql.NullString
		hook     sql.NullString
		secret   sql.NullString
		headers  sql.NullString
		active   int
		lastSeen sql.NullString
		polled   sql.NullTime
		lastErr  sql.NullString
	)
	if err := s.Scan(&w.ID, &w.Label, &kind, &w.Query, &acct, &hook, &secret,
		&headers, &active, &lastSeen, &polled, &lastErr, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return domain.XWatch{}, err
	}
	w.Kind = domain.XWatchKind(kind)
	w.AccountLabel = fromNullString(acct)
	w.WebhookURL = fromNullString(hook)
	w.WebhookSecret = fromNullString(secret)
	w.WebhookHeaders = parseWebhookHeaders(headers)
	w.Active = active != 0
	w.LastSeenID = fromNullString(lastSeen)
	w.LastPolledAt = fromNullTime(polled)
	w.LastError = fromNullString(lastErr)
	return w, nil
}

func scanXWatchEvent(rows *sql.Rows) (domain.XWatchEvent, error) {
	var (
		ev        domain.XWatchEvent
		tweetTime sql.NullString
		delivery  string
		delErr    sql.NullString
		delivered sql.NullTime
	)
	if err := rows.Scan(&ev.ID, &ev.WatchID, &ev.TweetID, &ev.Author, &ev.Text, &ev.URL,
		&ev.Likes, &ev.Retweets, &ev.Replies, &tweetTime, &delivery, &delErr, &delivered, &ev.CreatedAt); err != nil {
		return domain.XWatchEvent{}, err
	}
	ev.TweetCreatedAt = fromNullString(tweetTime)
	ev.Delivery = domain.XWatchDelivery(delivery)
	ev.DeliveryError = fromNullString(delErr)
	ev.DeliveredAt = fromNullTime(delivered)
	return ev, nil
}

// webhookHeadersJSON encodes custom webhook headers for storage. Empty/nil maps
// store as NULL so the column stays clean.
func webhookHeadersJSON(h map[string]string) any {
	if len(h) == 0 {
		return nil
	}
	b, err := json.Marshal(h)
	if err != nil {
		return nil
	}
	return string(b)
}

// parseWebhookHeaders decodes the stored JSON header map (NULL/empty -> nil).
func parseWebhookHeaders(v sql.NullString) map[string]string {
	if !v.Valid || v.String == "" {
		return nil
	}
	var h map[string]string
	if err := json.Unmarshal([]byte(v.String), &h); err != nil {
		return nil
	}
	return h
}
