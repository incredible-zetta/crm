package domain

import "time"

// Template represents an email template containing localized HTML and text with placeholders.
type Template struct {
	ID        int64
	Name      string
	Subject   string
	BodyHTML  string
	BodyText  string
	Variables []string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsDeleted returns true if the template has been soft-deleted.
func (t Template) IsDeleted() bool {
	return t.DeletedAt != nil
}
