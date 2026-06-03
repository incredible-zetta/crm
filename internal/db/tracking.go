package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type TrackingLink struct {
	ID         int64
	Code       string
	TargetURL  string
	CampaignID *int64
	ContactID  *int64
	CreatedAt  time.Time
}

const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func generateCode() (string, error) {
	b := make([]byte, 12)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

func (r *Repo) CreateLink(ctx context.Context, targetURL string, campaignID, contactID *int64) (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		code, err := generateCode()
		if err != nil {
			return "", fmt.Errorf("generate code: %w", err)
		}

		query := `INSERT INTO tracking_links (code, target_url, campaign_id, contact_id) VALUES (?, ?, ?, ?)`
		_, err = r.db.ExecContext(ctx, query, code, targetURL, campaignID, contactID)
		if err != nil {
			if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
				continue
			}
			if strings.Contains(err.Error(), "Duplicate entry") {
				continue
			}
			return "", fmt.Errorf("insert tracking link: %w", err)
		}

		return code, nil
	}

	return "", fmt.Errorf("failed to generate unique tracking link code after 3 attempts")
}

func (r *Repo) GetLink(ctx context.Context, code string) (TrackingLink, error) {
	query := `SELECT id, code, target_url, campaign_id, contact_id, created_at FROM tracking_links WHERE code = ?`
	var link TrackingLink
	var (
		campaignID sql.NullInt64
		contactID  sql.NullInt64
	)

	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&link.ID,
		&link.Code,
		&link.TargetURL,
		&campaignID,
		&contactID,
		&link.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return TrackingLink{}, fmt.Errorf("tracking link not found: %w", ErrNotFound)
	}
	if err != nil {
		return TrackingLink{}, fmt.Errorf("get tracking link: %w", err)
	}

	if campaignID.Valid {
		link.CampaignID = &campaignID.Int64
	}
	if contactID.Valid {
		link.ContactID = &contactID.Int64
	}

	return link, nil
}
