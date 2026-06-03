package domain

import "time"

// EventType represents the kind of an email tracking event.
type EventType string

const (
	// EventSent indicates the email was sent to the provider.
	EventSent EventType = "sent"
	// EventDelivered indicates the email was delivered successfully.
	EventDelivered EventType = "delivered"
	// EventOpen indicates the email was opened by the recipient.
	EventOpen EventType = "open"
	// EventClick indicates a link in the email was clicked.
	EventClick EventType = "click"
	// EventBounce indicates the email bounced and was not delivered.
	EventBounce EventType = "bounce"
	// EventFailed indicates email sending failed.
	EventFailed EventType = "failed"
	// EventUnsubscribe indicates the recipient unsubscribed from emails.
	EventUnsubscribe EventType = "unsubscribe"
)

// EventTypes contains all valid event types.
var EventTypes = []EventType{
	EventSent,
	EventDelivered,
	EventOpen,
	EventClick,
	EventBounce,
	EventFailed,
	EventUnsubscribe,
}

// Valid checks if the event type is valid.
func (t EventType) Valid() bool {
	for _, eventType := range EventTypes {
		if t == eventType {
			return true
		}
	}
	return false
}

// EmailEvent represents a single event in the email tracking and analytics pipeline.
type EmailEvent struct {
	ID         int64
	ContactID  int64
	CampaignID *int64
	Type       EventType
	LinkCode   string
	Meta       map[string]any
	TS         time.Time
}

// LinkCount tracks click counts for a tracking link.
type LinkCount struct {
	LinkCode string
	Clicks   int
}
