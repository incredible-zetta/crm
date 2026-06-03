package domain

import "time"

// Export represents an exported data file (e.g. CSV) and its metadata.
type Export struct {
	ID        string
	Path      string
	Rows      int
	ExpiresAt *time.Time
	CreatedAt time.Time
}
