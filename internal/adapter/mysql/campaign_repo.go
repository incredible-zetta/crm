package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
)

type campaignRepo struct {
	db *sql.DB
}

var _ port.CampaignRepo = (*campaignRepo)(nil)

func (r *campaignRepo) Create(ctx context.Context, c domain.Campaign) (domain.Campaign, error) {
	if c.Provider == "" {
		c.Provider = domain.ProviderSMTP
	}
	if c.Status == "" {
		c.Status = domain.CampaignDraft
	}

	if !c.Provider.Valid() {
		return domain.Campaign{}, fmt.Errorf("%w: invalid provider: %q", domain.ErrValidation, c.Provider)
	}
	if !c.Status.Valid() {
		return domain.Campaign{}, fmt.Errorf("%w: invalid status: %q", domain.ErrValidation, c.Status)
	}

	segmentJSON, err := marshalJSON(c.Segment, "{}")
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("marshal segment: %w", err)
	}

	statsJSON, err := marshalJSON(c.Stats, "{}")
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("marshal stats: %w", err)
	}

	query := `INSERT INTO campaigns (name, template_id, provider, segment, status, scheduled_at, stats, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, c.Name, toNullInt64(c.TemplateID), string(c.Provider), segmentJSON, string(c.Status), toNullTime(c.ScheduledAt), statsJSON, toNullTime(c.DeletedAt))
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("create campaign exec: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("last insert id: %w", err)
	}

	return r.Get(ctx, id)
}

func (r *campaignRepo) Get(ctx context.Context, id int64) (domain.Campaign, error) {
	query := `SELECT id, name, template_id, provider, segment, status, scheduled_at, stats, deleted_at, created_at FROM campaigns WHERE id = ? AND deleted_at IS NULL`
	row := r.db.QueryRowContext(ctx, query, id)
	c, err := scanCampaign(row)
	if err == sql.ErrNoRows {
		return domain.Campaign{}, fmt.Errorf("campaign not found: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("get campaign: %w", err)
	}
	return c, nil
}

func (r *campaignRepo) List(ctx context.Context) ([]domain.Campaign, error) {
	query := `SELECT id, name, template_id, provider, segment, status, scheduled_at, stats, deleted_at, created_at FROM campaigns WHERE deleted_at IS NULL ORDER BY id DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []domain.Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, fmt.Errorf("scan campaign: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list campaigns rows: %w", err)
	}
	return campaigns, nil
}

func (r *campaignRepo) UpdateStatus(ctx context.Context, id int64, status domain.CampaignStatus) error {
	if !status.Valid() {
		return fmt.Errorf("%w: invalid status: %q", domain.ErrValidation, status)
	}

	query := `UPDATE campaigns SET status = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, string(status), id)
	if err != nil {
		return fmt.Errorf("update campaign status exec: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		_, err := r.Get(ctx, id)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *campaignRepo) Update(ctx context.Context, id int64, c domain.Campaign) (domain.Campaign, error) {
	if !c.Provider.Valid() {
		return domain.Campaign{}, fmt.Errorf("%w: invalid provider: %q", domain.ErrValidation, c.Provider)
	}

	segmentJSON, err := marshalJSON(c.Segment, "{}")
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("marshal segment: %w", err)
	}

	query := `UPDATE campaigns SET name = ?, template_id = ?, provider = ?, segment = ?, scheduled_at = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, c.Name, toNullInt64(c.TemplateID), string(c.Provider), segmentJSON, toNullTime(c.ScheduledAt), id)
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("update campaign: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		_, err := r.Get(ctx, id)
		if err != nil {
			return domain.Campaign{}, err
		}
	}

	return r.Get(ctx, id)
}

func (r *campaignRepo) SetStats(ctx context.Context, id int64, stats map[string]any) error {
	statsJSON, err := marshalJSON(stats, "{}")
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}

	query := `UPDATE campaigns SET stats = ? WHERE id = ? AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, statsJSON, id)
	if err != nil {
		return fmt.Errorf("set stats: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		_, err := r.Get(ctx, id)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *campaignRepo) SoftDelete(ctx context.Context, id int64) error {
	query := `UPDATE campaigns SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("soft delete campaign: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("campaign not found or already deleted: %w", domain.ErrNotFound)
	}

	return nil
}

func scanCampaign(row interface{ Scan(dest ...any) error }) (domain.Campaign, error) {
	var c domain.Campaign
	var (
		templateID  sql.NullInt64
		segmentBuf  []byte
		statsBuf    []byte
		scheduledAt sql.NullTime
		deletedAt   sql.NullTime
	)

	err := row.Scan(
		&c.ID,
		&c.Name,
		&templateID,
		(*string)(&c.Provider),
		&segmentBuf,
		(*string)(&c.Status),
		&scheduledAt,
		&statsBuf,
		&deletedAt,
		&c.CreatedAt,
	)
	if err != nil {
		return domain.Campaign{}, err
	}

	c.TemplateID = templateID.Int64
	c.ScheduledAt = fromNullTime(scheduledAt)
	c.DeletedAt = fromNullTime(deletedAt)

	if segmentBuf != nil {
		if err := unmarshalJSON(segmentBuf, &c.Segment); err != nil {
			return domain.Campaign{}, fmt.Errorf("unmarshal segment: %w", err)
		}
	}
	if c.Segment == nil {
		c.Segment = make(map[string]any)
	}

	if statsBuf != nil {
		if err := unmarshalJSON(statsBuf, &c.Stats); err != nil {
			return domain.Campaign{}, fmt.Errorf("unmarshal stats: %w", err)
		}
	}
	if c.Stats == nil {
		c.Stats = make(map[string]any)
	}

	return c, nil
}
