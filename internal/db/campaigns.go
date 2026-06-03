package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Campaign struct {
	ID          int64
	Name        string
	TemplateID  int64
	Provider    string
	Segment     map[string]any // stored as JSON
	Status      string
	ScheduledAt *time.Time     // nullable
	Stats       map[string]any // stored as JSON
	CreatedAt   time.Time
}

var ValidProviders = []string{"smtp", "mailgun"}
var ValidStatuses = []string{"draft", "scheduled", "sending", "sent", "failed"}

func validProvider(p string) bool {
	for _, v := range ValidProviders {
		if p == v {
			return true
		}
	}
	return false
}

func validStatus(s string) bool {
	for _, v := range ValidStatuses {
		if s == v {
			return true
		}
	}
	return false
}

func (r *Repo) CreateCampaign(ctx context.Context, c Campaign) (Campaign, error) {
	if c.Provider == "" {
		c.Provider = "smtp"
	}
	if c.Status == "" {
		c.Status = "draft"
	}

	if !validProvider(c.Provider) {
		return Campaign{}, fmt.Errorf("invalid provider: %q", c.Provider)
	}
	if !validStatus(c.Status) {
		return Campaign{}, fmt.Errorf("invalid status: %q", c.Status)
	}

	var segmentJSON []byte
	if c.Segment != nil {
		var err error
		segmentJSON, err = json.Marshal(c.Segment)
		if err != nil {
			return Campaign{}, fmt.Errorf("marshal segment: %w", err)
		}
	} else {
		segmentJSON = []byte("{}")
	}

	var statsJSON []byte
	if c.Stats != nil {
		var err error
		statsJSON, err = json.Marshal(c.Stats)
		if err != nil {
			return Campaign{}, fmt.Errorf("marshal stats: %w", err)
		}
	} else {
		statsJSON = []byte("{}")
	}

	query := `INSERT INTO campaigns (name, template_id, provider, segment, status, scheduled_at, stats) VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, c.Name, c.TemplateID, c.Provider, segmentJSON, c.Status, c.ScheduledAt, statsJSON)
	if err != nil {
		return Campaign{}, fmt.Errorf("create campaign exec: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Campaign{}, fmt.Errorf("last insert id: %w", err)
	}

	return r.GetCampaign(ctx, id)
}

func (r *Repo) GetCampaign(ctx context.Context, id int64) (Campaign, error) {
	query := `SELECT id, name, template_id, provider, segment, status, scheduled_at, stats, created_at FROM campaigns WHERE id = ?`
	var c Campaign
	var (
		templateID  sql.NullInt64
		segmentBuf  []byte
		statsBuf    []byte
		scheduledAt sql.NullTime
	)

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID,
		&c.Name,
		&templateID,
		&c.Provider,
		&segmentBuf,
		&c.Status,
		&scheduledAt,
		&statsBuf,
		&c.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return Campaign{}, fmt.Errorf("campaign not found: %w", ErrNotFound)
	}
	if err != nil {
		return Campaign{}, fmt.Errorf("get campaign: %w", err)
	}

	c.TemplateID = templateID.Int64
	if scheduledAt.Valid {
		c.ScheduledAt = &scheduledAt.Time
	}

	if segmentBuf != nil {
		if err := json.Unmarshal(segmentBuf, &c.Segment); err != nil {
			return Campaign{}, fmt.Errorf("unmarshal segment: %w", err)
		}
	}
	if c.Segment == nil {
		c.Segment = make(map[string]any)
	}

	if statsBuf != nil {
		if err := json.Unmarshal(statsBuf, &c.Stats); err != nil {
			return Campaign{}, fmt.Errorf("unmarshal stats: %w", err)
		}
	}
	if c.Stats == nil {
		c.Stats = make(map[string]any)
	}

	return c, nil
}

func (r *Repo) UpdateCampaignStatus(ctx context.Context, id int64, status string) error {
	if !validStatus(status) {
		return fmt.Errorf("invalid status: %q", status)
	}

	query := `UPDATE campaigns SET status = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update campaign status exec: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		var exists int
		err = r.db.QueryRowContext(ctx, "SELECT 1 FROM campaigns WHERE id = ?", id).Scan(&exists)
		if err == sql.ErrNoRows {
			return fmt.Errorf("campaign not found: %w", ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("check campaign existence: %w", err)
		}
	}

	return nil
}

func (r *Repo) SetCampaignStats(ctx context.Context, id int64, stats map[string]any) error {
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}

	query := `UPDATE campaigns SET stats = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, statsJSON, id)
	if err != nil {
		return fmt.Errorf("set campaign stats exec: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		var exists int
		err = r.db.QueryRowContext(ctx, "SELECT 1 FROM campaigns WHERE id = ?", id).Scan(&exists)
		if err == sql.ErrNoRows {
			return fmt.Errorf("campaign not found: %w", ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("check campaign existence: %w", err)
		}
	}

	return nil
}

func (r *Repo) ListCampaigns(ctx context.Context) ([]Campaign, error) {
	query := `SELECT id, name, template_id, provider, segment, status, scheduled_at, stats, created_at FROM campaigns ORDER BY id DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list campaigns query: %w", err)
	}
	defer rows.Close()

	var campaigns []Campaign
	for rows.Next() {
		var c Campaign
		var (
			templateID  sql.NullInt64
			segmentBuf  []byte
			statsBuf    []byte
			scheduledAt sql.NullTime
		)
		err := rows.Scan(
			&c.ID,
			&c.Name,
			&templateID,
			&c.Provider,
			&segmentBuf,
			&c.Status,
			&scheduledAt,
			&statsBuf,
			&c.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("list campaigns scan: %w", err)
		}

		c.TemplateID = templateID.Int64
		if scheduledAt.Valid {
			c.ScheduledAt = &scheduledAt.Time
		}

		if segmentBuf != nil {
			if err := json.Unmarshal(segmentBuf, &c.Segment); err != nil {
				return nil, fmt.Errorf("unmarshal segment: %w", err)
			}
		}
		if c.Segment == nil {
			c.Segment = make(map[string]any)
		}

		if statsBuf != nil {
			if err := json.Unmarshal(statsBuf, &c.Stats); err != nil {
				return nil, fmt.Errorf("unmarshal stats: %w", err)
			}
		}
		if c.Stats == nil {
			c.Stats = make(map[string]any)
		}

		campaigns = append(campaigns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list campaigns rows: %w", err)
	}

	return campaigns, nil
}
