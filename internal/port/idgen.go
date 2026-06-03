package port

// IDGenerator produces opaque IDs and codes.
type IDGenerator interface {
	// ExportID generates a unique identifier for data exports (16 hex chars).
	ExportID() (string, error)
	// UnsubCode generates a unique opt-out/unsubscribe token for a contact (16 hex chars).
	UnsubCode() (string, error)
}
