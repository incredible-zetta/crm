package domain

import "time"

// Stage represents the sales pipeline stage of a contact.
type Stage string

const (
	// StageNew is the default stage for a newly added contact.
	StageNew Stage = "new"
	// StageContacted indicates that initial contact has been made.
	StageContacted Stage = "contacted"
	// StageQualified indicates that the contact has been qualified.
	StageQualified Stage = "qualified"
	// StageProposal indicates that a proposal has been sent.
	StageProposal Stage = "proposal"
	// StageWon indicates the deal has been won.
	StageWon Stage = "won"
	// StageLost indicates the deal was lost.
	StageLost Stage = "lost"
)

// Stages contains all stages in canonical pipeline order.
var Stages = []Stage{StageNew, StageContacted, StageQualified, StageProposal, StageWon, StageLost}

// Valid checks if the stage is one of the predefined valid values.
func (s Stage) Valid() bool {
	for _, stage := range Stages {
		if s == stage {
			return true
		}
	}
	return false
}

// Contact represents an individual customer or prospect in the CRM.
type Contact struct {
	ID                int64
	Email             string
	FirstName         string
	LastName          string
	Company           string
	Phone             string
	Stage             Stage
	Tags              []string
	Notes             string
	Custom            map[string]any
	Source            string
	UnsubCode         string         // public opt-out token (16 hex), may be empty until set
	UnsubscribedAt    *time.Time     // nil = subscribed
	DeletedAt         *time.Time     // nil = active (soft delete)
	EmailStatus       EmailStatus    // verification verdict; empty treated as unknown
	EmailReason       string         // short reason for the verdict
	EmailCheckedAt    *time.Time     // nil = never verified
	WhatsAppStatus    WhatsAppStatus // WhatsApp capability verdict; empty treated as unknown
	WhatsAppCheckedAt *time.Time     // nil = never checked
	XUsername         string         // x.com/Twitter @handle (no @), empty if unknown
	ThreadsUsername   string         // threads.com @handle (no @), empty if unknown
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// IsUnsubscribed returns true if the contact has unsubscribed.
func (c Contact) IsUnsubscribed() bool {
	return c.UnsubscribedAt != nil
}

// IsDeleted returns true if the contact has been soft-deleted.
func (c Contact) IsDeleted() bool {
	return c.DeletedAt != nil
}

// ContactFilter is a search filter for listing contacts.
type ContactFilter struct {
	Stage   string
	Company string
	Tag     string
	Q       string
}

// ContactPatch represents fields that can be updated on a contact.
// Nil pointers indicate fields that should remain unchanged.
type ContactPatch struct {
	Email           *string
	FirstName       *string
	LastName        *string
	Company         *string
	Phone           *string
	Stage           *string
	Tags            *[]string
	Notes           *string
	Custom          *map[string]any
	Source          *string
	XUsername       *string
	ThreadsUsername *string
}
