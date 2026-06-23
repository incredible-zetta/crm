package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
)

type eventRepo struct {
	db *sql.DB
}

var _ port.EventRepo = (*eventRepo)(nil)

func (r *eventRepo) Insert(ctx context.Context, e domain.EmailEvent) error {
	if !e.Type.Valid() {
		return fmt.Errorf("%w: invalid event type: %q", domain.ErrValidation, e.Type)
	}

	metaJSON, err := marshalJSON(e.Meta, "{}")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	var linkCodeVal any
	if e.LinkCode != "" {
		linkCodeVal = e.LinkCode
	} else {
		linkCodeVal = nil
	}

	query := `INSERT INTO email_events (tenant_id, contact_id, campaign_id, type, link_code, meta) VALUES (?, ?, ?, ?, ?, ?)`
	_, err = r.db.ExecContext(ctx, query, tenant.From(ctx), e.ContactID, e.CampaignID, string(e.Type), linkCodeVal, metaJSON)
	if err != nil {
		return fmt.Errorf("insert event exec: %w", err)
	}

	return nil
}

func (r *eventRepo) OverviewCounts(ctx context.Context) (map[string]int, error) {
	query := `SELECT type, COUNT(*) FROM email_events WHERE tenant_id = ? GROUP BY type`
	rows, err := r.db.QueryContext(ctx, query, tenant.From(ctx))
	if err != nil {
		return nil, fmt.Errorf("overview counts query: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	// Initialize all valid event types with 0
	for _, t := range domain.EventTypes {
		counts[string(t)] = 0
	}

	for rows.Next() {
		var t string
		var count int
		if err := rows.Scan(&t, &count); err != nil {
			return nil, fmt.Errorf("overview counts scan: %w", err)
		}
		counts[t] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("overview counts rows: %w", err)
	}

	return counts, nil
}

func (r *eventRepo) CampaignCounts(ctx context.Context, campaignID int64) (map[string]int, error) {
	query := `SELECT type, COUNT(*) FROM email_events WHERE campaign_id = ? AND tenant_id = ? GROUP BY type`
	rows, err := r.db.QueryContext(ctx, query, campaignID, tenant.From(ctx))
	if err != nil {
		return nil, fmt.Errorf("campaign counts query: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	// Initialize all valid event types with 0
	for _, t := range domain.EventTypes {
		counts[string(t)] = 0
	}

	for rows.Next() {
		var t string
		var count int
		if err := rows.Scan(&t, &count); err != nil {
			return nil, fmt.Errorf("campaign counts scan: %w", err)
		}
		counts[t] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("campaign counts rows: %w", err)
	}

	return counts, nil
}

func (r *eventRepo) UniqueOpens(ctx context.Context, campaignID *int64) (int, error) {
	var query string
	var args []any
	if campaignID == nil {
		query = `SELECT COUNT(DISTINCT contact_id) FROM email_events WHERE type = 'open' AND tenant_id = ?`
		args = append(args, tenant.From(ctx))
	} else {
		query = `SELECT COUNT(DISTINCT contact_id) FROM email_events WHERE type = 'open' AND campaign_id = ? AND tenant_id = ?`
		args = append(args, *campaignID, tenant.From(ctx))
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("unique opens count: %w", err)
	}
	return count, nil
}

func (r *eventRepo) CampaignUniqueOpens(ctx context.Context, campaignID int64) (int, error) {
	return r.UniqueOpens(ctx, &campaignID)
}

func (r *eventRepo) TopLinks(ctx context.Context, campaignID int64, limit int) ([]domain.LinkCount, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `SELECT link_code, COUNT(*) as clicks FROM email_events 
WHERE campaign_id = ? AND tenant_id = ? AND type = 'click' AND link_code IS NOT NULL AND link_code != ''
GROUP BY link_code 
ORDER BY clicks DESC, link_code ASC 
LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, campaignID, tenant.From(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("top links query: %w", err)
	}
	defer rows.Close()

	var counts []domain.LinkCount
	for rows.Next() {
		var lc domain.LinkCount
		var linkCode sql.NullString
		if err := rows.Scan(&linkCode, &lc.Clicks); err != nil {
			return nil, fmt.Errorf("top links scan: %w", err)
		}
		lc.LinkCode = linkCode.String
		counts = append(counts, lc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("top links rows: %w", err)
	}

	return counts, nil
}
