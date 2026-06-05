package domain

import "time"

// EmailStatus is the verification verdict for a contact email address.
type EmailStatus string

const (
	// EmailUnknown means the address has not been verified yet.
	EmailUnknown EmailStatus = "unknown"
	// EmailValid means syntax is correct and the domain can receive mail.
	EmailValid EmailStatus = "valid"
	// EmailInvalid means syntax is malformed or the domain cannot receive mail.
	EmailInvalid EmailStatus = "invalid"
	// EmailRisky means deliverable but low quality (disposable/role address).
	EmailRisky EmailStatus = "risky"
)

// EmailStatuses lists all valid verification verdicts.
var EmailStatuses = []EmailStatus{EmailUnknown, EmailValid, EmailInvalid, EmailRisky}

// Valid reports whether the status is a recognized value.
func (s EmailStatus) Valid() bool {
	for _, st := range EmailStatuses {
		if s == st {
			return true
		}
	}
	return false
}

// EmailVerification is the result of verifying a single email address.
type EmailVerification struct {
	Email     string
	Status    EmailStatus
	Reason    string
	CheckedAt time.Time
}
