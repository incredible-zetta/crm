package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeClaimer struct {
	mu           sync.Mutex
	tasksToClaim []Task
	claimed      bool
	claimErr     error
	markDoneErr  func(id int64) error

	doneCalls   []int64
	failedCalls map[int64]string

	claimLimits []int
	lastCtx     context.Context
}

func (fc *fakeClaimer) ClaimDue(ctx context.Context, now time.Time, limit int) ([]Task, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.claimLimits = append(fc.claimLimits, limit)
	fc.lastCtx = ctx

	if fc.claimErr != nil {
		return nil, fc.claimErr
	}
	if fc.claimed {
		return nil, nil // return nil on subsequent calls to avoid infinite loops in Start tests
	}
	fc.claimed = true
	return fc.tasksToClaim, nil
}

func (fc *fakeClaimer) MarkDone(ctx context.Context, id int64) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.doneCalls = append(fc.doneCalls, id)
	if fc.markDoneErr != nil {
		return fc.markDoneErr(id)
	}
	return nil
}

func (fc *fakeClaimer) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fc.failedCalls == nil {
		fc.failedCalls = make(map[int64]string)
	}
	fc.failedCalls[id] = errMsg
	return nil
}

type fakeExecutor struct {
	mu          sync.Mutex
	executeFunc func(Task) error
	executedIDs []int64
	lastCtx     context.Context
}

func (fe *fakeExecutor) Execute(ctx context.Context, t Task) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.executedIDs = append(fe.executedIDs, t.ID)
	fe.lastCtx = ctx
	if fe.executeFunc != nil {
		return fe.executeFunc(t)
	}
	return nil
}

func TestRunOnceMarksDoneOnSuccess(t *testing.T) {
	fc := &fakeClaimer{
		tasksToClaim: []Task{
			{ID: 1, Kind: "email", Payload: map[string]any{"to": "alice@example.com"}},
			{ID: 2, Kind: "campaign", Payload: map[string]any{"id": 42}},
		},
	}
	fe := &fakeExecutor{}

	w := &Worker{
		Claimer: fc,
		Exec:    fe,
	}

	processed, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if processed != 2 {
		t.Errorf("expected processed 2, got %d", processed)
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.doneCalls) != 2 {
		t.Errorf("expected 2 done calls, got %d", len(fc.doneCalls))
	} else {
		if fc.doneCalls[0] != 1 || fc.doneCalls[1] != 2 {
			t.Errorf("expected done calls for [1, 2], got %v", fc.doneCalls)
		}
	}
	if len(fc.failedCalls) != 0 {
		t.Errorf("expected 0 failed calls, got %v", fc.failedCalls)
	}
}

func TestRunOnceMarksFailedOnError(t *testing.T) {
	fc := &fakeClaimer{
		tasksToClaim: []Task{
			{ID: 1, Kind: "email"},
			{ID: 2, Kind: "campaign"},
		},
	}
	fe := &fakeExecutor{
		executeFunc: func(task Task) error {
			if task.ID == 2 {
				return errors.New("campaign send failed")
			}
			return nil
		},
	}

	w := &Worker{
		Claimer: fc,
		Exec:    fe,
	}

	processed, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if processed != 2 {
		t.Errorf("expected processed 2, got %d", processed)
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.doneCalls) != 1 || fc.doneCalls[0] != 1 {
		t.Errorf("expected done call for 1, got %v", fc.doneCalls)
	}
	if len(fc.failedCalls) != 1 || fc.failedCalls[2] != "campaign send failed" {
		t.Errorf("expected failed call for 2 with 'campaign send failed', got %v", fc.failedCalls)
	}
}

func TestRunOnceClaimError(t *testing.T) {
	expectedErr := errors.New("db error")
	fc := &fakeClaimer{
		claimErr: expectedErr,
	}
	fe := &fakeExecutor{}

	w := &Worker{
		Claimer: fc,
		Exec:    fe,
	}

	processed, err := w.RunOnce(context.Background())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if processed != 0 {
		t.Errorf("expected processed 0, got %d", processed)
	}
	if len(fe.executedIDs) != 0 {
		t.Errorf("expected 0 executed, got %v", fe.executedIDs)
	}
}

func TestRunOnceEmpty(t *testing.T) {
	fc := &fakeClaimer{
		tasksToClaim: []Task{},
	}
	fe := &fakeExecutor{}

	w := &Worker{
		Claimer: fc,
		Exec:    fe,
	}

	processed, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if processed != 0 {
		t.Errorf("expected processed 0, got %d", processed)
	}
}

func TestRunOnceDefaults(t *testing.T) {
	fc := &fakeClaimer{}
	fe := &fakeExecutor{}

	w := &Worker{
		Claimer: fc,
		Exec:    fe,
	}

	_, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.claimLimits) == 0 {
		t.Fatalf("expected ClaimDue to be called")
	}
	if fc.claimLimits[0] != 10 {
		t.Errorf("expected batch limit to default to 10, got %d", fc.claimLimits[0])
	}
}

func TestRunOnceMarkErrorsBestEffort(t *testing.T) {
	fc := &fakeClaimer{
		tasksToClaim: []Task{
			{ID: 1, Kind: "email"},
			{ID: 2, Kind: "campaign"},
		},
		markDoneErr: func(id int64) error {
			if id == 1 {
				return errors.New("mark done failed")
			}
			return nil
		},
	}
	fe := &fakeExecutor{}

	w := &Worker{
		Claimer: fc,
		Exec:    fe,
	}

	processed, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("expected no error (errors from MarkDone should be swallowed/best-effort), got %v", err)
	}
	if processed != 2 {
		t.Errorf("expected processed 2, got %d", processed)
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.doneCalls) != 2 {
		t.Errorf("expected both tasks to be processed and have MarkDone called, got %d", len(fc.doneCalls))
	}
}

func TestRunOncePropagatesContext(t *testing.T) {
	type contextKey string
	key := contextKey("sentinel-key")
	expectedVal := "sentinel-value"

	ctx := context.WithValue(context.Background(), key, expectedVal)

	fc := &fakeClaimer{
		tasksToClaim: []Task{
			{ID: 42, Kind: "email"},
		},
	}
	fe := &fakeExecutor{}

	w := &Worker{
		Claimer: fc,
		Exec:    fe,
	}

	_, err := w.RunOnce(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	fc.mu.Lock()
	claimCtx := fc.lastCtx
	fc.mu.Unlock()

	if claimCtx == nil {
		t.Fatal("ClaimDue context was nil")
	}
	if val := claimCtx.Value(key); val != expectedVal {
		t.Errorf("expected ClaimDue context value to be %q, got %q", expectedVal, val)
	}

	fe.mu.Lock()
	execCtx := fe.lastCtx
	fe.mu.Unlock()

	if execCtx == nil {
		t.Fatal("Execute context was nil")
	}
	if val := execCtx.Value(key); val != expectedVal {
		t.Errorf("expected Execute context value to be %q, got %q", expectedVal, val)
	}
}

func TestStartStopsOnContext(t *testing.T) {
	runSignal := make(chan struct{})
	var once sync.Once

	fc := &fakeClaimer{
		tasksToClaim: []Task{
			{ID: 101, Kind: "email"},
		},
	}
	fe := &fakeExecutor{
		executeFunc: func(task Task) error {
			once.Do(func() {
				close(runSignal)
			})
			return nil
		},
	}

	w := &Worker{
		Claimer: fc,
		Exec:    fe,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Start(ctx, 10*time.Second)
		close(done)
	}()

	// Wait for the first task to start executing
	select {
	case <-runSignal:
		// Now cancel the context
		cancel()
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not execute the first task within timeout")
	}

	// Assert Start stops promptly after cancellation
	select {
	case <-done:
		// success: returned promptly after cancellation
	case <-time.After(1 * time.Second):
		t.Fatal("Start did not exit promptly after context cancellation")
	}

	fe.mu.Lock()
	defer fe.mu.Unlock()
	if len(fe.executedIDs) == 0 {
		t.Errorf("expected executor to have run at least once")
	}
}

func TestStartIntervalGuard(t *testing.T) {
	fc := &fakeClaimer{}
	fe := &fakeExecutor{}

	w := &Worker{
		Claimer: fc,
		Exec:    fe,
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so Start returns after the initial RunOnce without waiting
	cancel()

	// Should not panic when interval <= 0
	w.Start(ctx, 0)
}
