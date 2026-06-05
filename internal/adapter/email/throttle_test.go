package email

import (
	"context"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/port"
)

type countingSender struct{ count int }

func (c *countingSender) Send(ctx context.Context, msg port.OutboundMessage) error {
	c.count++
	return nil
}

func TestNewThrottledSenderDisabledPassthrough(t *testing.T) {
	base := &countingSender{}
	if got := NewThrottledSender(base, 0, time.Second); got != base {
		t.Errorf("expected passthrough when max<=0")
	}
	if got := NewThrottledSender(base, 5, 0); got != base {
		t.Errorf("expected passthrough when window<=0")
	}
}

func TestThrottledSenderBurstThenPaces(t *testing.T) {
	base := &countingSender{}
	// 2 per second; bucket starts full with 2 tokens.
	ts := &throttledSender{
		next:     base,
		max:      2,
		window:   time.Second,
		tokens:   2,
		lastFill: time.Unix(0, 0),
	}
	now := time.Unix(0, 0)
	ts.clock = func() time.Time { return now }

	var slept []time.Duration
	ts.sleep = func(ctx context.Context, d time.Duration) error {
		slept = append(slept, d)
		now = now.Add(d) // advance virtual clock by the wait
		return nil
	}

	ctx := context.Background()
	// First two sends consume the initial burst with no sleeping.
	for i := 0; i < 2; i++ {
		if err := ts.Send(ctx, port.OutboundMessage{To: "a@b.c"}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if len(slept) != 0 {
		t.Fatalf("expected no sleeps in burst, got %v", slept)
	}

	// Third send must wait ~0.5s (refill at 2/sec).
	if err := ts.Send(ctx, port.OutboundMessage{To: "a@b.c"}); err != nil {
		t.Fatalf("third send: %v", err)
	}
	if len(slept) != 1 {
		t.Fatalf("expected one sleep, got %v", slept)
	}
	if slept[0] < 400*time.Millisecond || slept[0] > 600*time.Millisecond {
		t.Errorf("expected ~500ms pace wait, got %v", slept[0])
	}
	if base.count != 3 {
		t.Errorf("expected 3 delivered, got %d", base.count)
	}
}

func TestThrottledSenderRespectsContextCancel(t *testing.T) {
	base := &countingSender{}
	ts := &throttledSender{
		next:     base,
		max:      1,
		window:   time.Hour, // effectively never refills in test
		tokens:   0,
		lastFill: time.Unix(0, 0),
		clock:    func() time.Time { return time.Unix(0, 0) },
	}
	ctx, cancel := context.WithCancel(context.Background())
	ts.sleep = func(ctx context.Context, d time.Duration) error {
		cancel()
		return context.Canceled
	}
	if err := ts.Send(ctx, port.OutboundMessage{To: "a@b.c"}); err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if base.count != 0 {
		t.Errorf("expected no delivery when cancelled, got %d", base.count)
	}
}
