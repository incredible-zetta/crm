package domain

import "time"

// XLiveness is the last-known auth state of a stored x.com account.
type XLiveness string

const (
	XLivenessUnknown XLiveness = "unknown"
	XLivenessLive    XLiveness = "live"
	XLivenessDead    XLiveness = "dead"
)

// XAccount is a persisted x.com account: a Netscape cookie blob plus liveness
// bookkeeping. The cookies are a live session credential (auth_token + ct0);
// treat as sensitive. Label is the tenant-unique handle callers reference.
type XAccount struct {
	ID            int64
	Label         string
	ScreenName    string
	UserID        string
	Cookies       string
	Liveness      XLiveness
	LastCheckedAt *time.Time
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
