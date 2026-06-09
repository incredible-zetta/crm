package whatsapp

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/incredible-zetta/crm/internal/port"
)

// SmartSendPolicy configures ban-prevention pacing for outbound WhatsApp sends.
//
// WhatsApp aggressively bans numbers that blast messages. The smart sender
// applies several human-like safeguards layered on top of the raw gateway:
//
//   - Token-bucket rate limit: at most RateMax sends per RateWindow.
//   - Jitter: a random delay in [JitterMin, JitterMax] before each send, so the
//     cadence is never perfectly regular.
//   - Per-recipient daily cap: refuse to exceed DailyCapPerRecipient messages
//     to the same number within 24h.
//   - Warmup ramp: a global ceiling of WarmupPerDay sends per 24h, intended to
//     be raised gradually as the number ages.
//
// A zero value disables the corresponding safeguard.
type SmartSendPolicy struct {
	RateMax              int
	RateWindow           time.Duration
	JitterMin            time.Duration
	JitterMax            time.Duration
	DailyCapPerRecipient int
	WarmupPerDay         int
}

// Counter exposes the historical-send lookups the smart sender needs. The
// MySQL WAMessageRepo satisfies it.
type Counter interface {
	CountSentSince(ctx context.Context, phone string, since time.Time) (int, error)
	CountSentSinceAll(ctx context.Context, since time.Time) (int, error)
}

// SmartSender wraps a port.WhatsAppGateway and paces Send calls to reduce the
// risk of the WhatsApp account being banned. All other gateway methods pass
// through unchanged.
type SmartSender struct {
	port.WhatsAppGateway
	policy  SmartSendPolicy
	counter Counter

	mu       sync.Mutex
	tokens   float64
	lastFill time.Time

	clock func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
	jitFn func(min, max time.Duration) time.Duration
}

var _ port.WhatsAppGateway = (*SmartSender)(nil)

// NewSmartSender wraps next with the given policy. counter may be nil to
// disable the daily-cap and warmup safeguards (e.g. in tests).
func NewSmartSender(next port.WhatsAppGateway, policy SmartSendPolicy, c Counter) *SmartSender {
	max := policy.RateMax
	if max < 0 {
		max = 0
	}
	return &SmartSender{
		WhatsAppGateway: next,
		policy:          policy,
		counter:         c,
		tokens:          float64(max),
		lastFill:        time.Now(),
		clock:           time.Now,
		sleep:           sleepCtx,
		jitFn:           randJitter,
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

func randJitter(min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	return min + time.Duration(rand.Int63n(int64(max-min)))
}

// Send applies the policy safeguards then delegates to the wrapped gateway.
func (s *SmartSender) Send(ctx context.Context, msg port.WhatsAppMessage) (port.WhatsAppSendResult, error) {
	phone := NormalizePhone(msg.Phone)

	// Warmup ceiling (global per 24h).
	if s.counter != nil && s.policy.WarmupPerDay > 0 {
		n, err := s.counter.CountSentSinceAll(ctx, s.clock().Add(-24*time.Hour))
		if err == nil && n >= s.policy.WarmupPerDay {
			return port.WhatsAppSendResult{}, fmt.Errorf("whatsapp: warmup limit reached (%d/24h); raise WA_WARMUP_PER_DAY as the number ages", s.policy.WarmupPerDay)
		}
	}

	// Per-recipient daily cap.
	if s.counter != nil && s.policy.DailyCapPerRecipient > 0 {
		n, err := s.counter.CountSentSince(ctx, phone, s.clock().Add(-24*time.Hour))
		if err == nil && n >= s.policy.DailyCapPerRecipient {
			return port.WhatsAppSendResult{}, fmt.Errorf("whatsapp: daily cap reached for %s (%d/24h)", phone, s.policy.DailyCapPerRecipient)
		}
	}

	// Token-bucket rate limit.
	if s.policy.RateMax > 0 && s.policy.RateWindow > 0 {
		for {
			if err := ctx.Err(); err != nil {
				return port.WhatsAppSendResult{}, err
			}
			wait := s.reserve()
			if wait <= 0 {
				break
			}
			if err := s.sleep(ctx, wait); err != nil {
				return port.WhatsAppSendResult{}, err
			}
		}
	}

	// Human-like jitter.
	if s.policy.JitterMax > 0 {
		if err := s.sleep(ctx, s.jitFn(s.policy.JitterMin, s.policy.JitterMax)); err != nil {
			return port.WhatsAppSendResult{}, err
		}
	}

	return s.WhatsAppGateway.Send(ctx, msg)
}

func (s *SmartSender) reserve() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()
	ratePerSec := float64(s.policy.RateMax) / s.policy.RateWindow.Seconds()

	elapsed := now.Sub(s.lastFill).Seconds()
	if elapsed > 0 {
		s.tokens += elapsed * ratePerSec
		if s.tokens > float64(s.policy.RateMax) {
			s.tokens = float64(s.policy.RateMax)
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
