package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/incredible-zetta/crm/internal/port"
)

type tenantRepo struct {
	db *sql.DB
}

var _ port.TenantRepo = (*tenantRepo)(nil)

// Resolve returns the tenant id for the (apiKey, sessionID) pair, creating a
// tenant row on first contact. The tenant id is a deterministic, opaque hash
// of the pair so the same caller always maps to the same tenant across
// reconnects, and the raw api key is never stored (only its SHA-256 hash).
func (r *tenantRepo) Resolve(ctx context.Context, apiKey, sessionID string) (string, error) {
	keyHash := sha256Hex(apiKey)
	id := tenantID(keyHash, sessionID)

	// Insert-or-touch: create on first contact, otherwise update last_seen_at.
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tenants (id, api_key_hash, session_id, last_seen_at)
		 VALUES (?, ?, ?, NOW())
		 ON DUPLICATE KEY UPDATE last_seen_at = NOW()`,
		id, keyHash, sessionID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant: %w", err)
	}
	return id, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// tenantID derives a short, stable, opaque tenant id from the api key hash and
// session id. Truncated to 32 hex chars to fit the VARCHAR(64) id column with
// room to spare while staying collision-resistant for this scale.
func tenantID(keyHash, sessionID string) string {
	sum := sha256.Sum256([]byte(keyHash + "|" + sessionID))
	return "t_" + hex.EncodeToString(sum[:])[:30]
}
