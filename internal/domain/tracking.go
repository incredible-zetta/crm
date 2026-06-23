package domain

import "time"

// TrackingLink represents a redirect link used to track clicks and associate them with campaigns and contacts.
type TrackingLink struct {
	ID         int64
	TenantID   string
	Code       string
	TargetURL  string
	CampaignID *int64
	ContactID  *int64
	CreatedAt  time.Time
}
