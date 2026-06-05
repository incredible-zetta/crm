package email

import (
	"context"
	"sync"
	"time"

	"github.com/incredible-zetta/crm/internal/port"
)

// throttledSender wraps a port.EmailSender and paces deliveries to stay under a
// provider rate limit (e.g. Larksuite allows 200 messages per 100 seconds).
//
// It uses a simple token bucket: up to `max` tokens refill evenly across
// `window`. Send blocks until a token is available or the context is cancelled,
// so a campaign loop is naturally paced without bursting the SMTP provider.
type throttledSender struct {
	next   port.EmailSender
	max    int
	window time.Duration

	mu       sync.Mutex
	tokens   float64
	lastFill time.Time
	clock    func() time.Time
	sleep    func(ctx context.Context, d time.Duration) error
}

// NewThrottledSender wraps next so that no more than max sends occur per window.
// If max <= 0 or window <= 0, next is returned unwrapped (throttling disabled).
func NewThrottledSender(next port.EmailSender, max int, window time.Duration) port.EmailSender {
	if max <= 0 || window <= 0 {
		return next
	}
	return &throttledSender{
		next:     next,
		max:      max,
		window:   window,
		tokens:   float64(max),
		lastFill: time.Now(),
		clock:    time.Now,
		sleep:    sleepCtx,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (s *throttledSender) Send(ctx context.Context, msg port.OutboundMessage) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		wait := s.reserve()
		if wait <= 0 {
			break
		}
		if err := s.sleep(ctx, wait); err != nil {
			return err
		}
	}
	return s.next.Send(ctx, msg)
}

// reserve refills the bucket based on elapsed time and either consumes a token
// (returning 0) or returns how long to wait before a token will be available.
func (s *throttledSender) reserve() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()
	ratePerSec := float64(s.max) / s.window.Seconds()

	elapsed := now.Sub(s.lastFill).Seconds()
	if elapsed > 0 {
		s.tokens += elapsed * ratePerSec
		if s.tokens > float64(s.max) {
			s.tokens = float64(s.max)
		}
		s.lastFill = now
	}

	if s.tokens >= 1 {
		s.tokens -= 1
		return 0
	}

	needed := 1 - s.tokens
	return time.Duration(needed / ratePerSec * float64(time.Second))
}
