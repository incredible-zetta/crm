package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type EmailEvent struct {
	ID         int64
	ContactID  int64
	CampaignID *int64
	Type       string
	LinkCode   string
	Meta       map[string]any // stored as JSON
}

type LinkCount struct {
	LinkCode string
	Clicks   int
}

var ValidEventTypes = []string{"sent", "delivered", "open", "click", "bounce", "failed"}

func validEventType(t string) bool {
	for _, v := range ValidEventTypes {
		if t == v {
			return true
		}
	}
	return false
}

func (r *Repo) InsertEvent(ctx context.Context, e EmailEvent) error {
	if !validEventType(e.Type) {
		return fmt.Errorf("invalid event type: %q", e.Type)
	}

	var metaJSON []byte
	if e.Meta != nil {
		var err error
		metaJSON, err = json.Marshal(e.Meta)
		if err != nil {
			return fmt.Errorf("marshal meta: %w", err)
		}
	}

	var linkCodeVal any
	if e.LinkCode != "" {
		linkCodeVal = e.LinkCode
	} else {
		linkCodeVal = nil
	}

	query := `INSERT INTO email_events (contact_id, campaign_id, type, link_code, meta) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, e.ContactID, e.CampaignID, e.Type, linkCodeVal, metaJSON)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return nil
}

func (r *Repo) OverviewCounts(ctx context.Context) (map[string]int, error) {
	query := `SELECT type, COUNT(*) FROM email_events GROUP BY type`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("overview counts query: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	// Initialize all valid event types with 0
	for _, t := range ValidEventTypes {
		counts[t] = 0
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

func (r *Repo) CampaignCounts(ctx context.Context, campaignID int64) (map[string]int, error) {
	query := `SELECT type, COUNT(*) FROM email_events WHERE campaign_id = ? GROUP BY type`
	rows, err := r.db.QueryContext(ctx, query, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign counts query: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	// Initialize all valid event types with 0
	for _, t := range ValidEventTypes {
		counts[t] = 0
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

func (r *Repo) TopLinks(ctx context.Context, campaignID int64, limit int) ([]LinkCount, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `SELECT link_code, COUNT(*) as clicks FROM email_events 
WHERE campaign_id = ? AND type = 'click' AND link_code IS NOT NULL AND link_code != ''
GROUP BY link_code 
ORDER BY clicks DESC, link_code ASC 
LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, campaignID, limit)
	if err != nil {
		return nil, fmt.Errorf("top links query: %w", err)
	}
	defer rows.Close()

	var counts []LinkCount
	for rows.Next() {
		var lc LinkCount
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
