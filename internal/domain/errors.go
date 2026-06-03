package domain

import "errors"

// Shared sentinel errors wrapped or returned by services.
var (
	// ErrNotFound is returned when an entity is not found in the repository.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned when there is a conflict, e.g. duplicate unique values.
	ErrConflict = errors.New("conflict")
	// ErrValidation is returned when input validation fails.
	ErrValidation = errors.New("validation error")
)
