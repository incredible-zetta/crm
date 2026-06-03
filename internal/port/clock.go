package port

import "time"

// Clock provides an interface to retrieve the current time, enabling testability.
type Clock interface {
	// Now returns the current local time.
	Now() time.Time
}
