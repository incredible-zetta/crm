package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestTasksRepo(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set; skipping integration test")
	}

	repo := getTestDB(t)
	ctx := context.Background()

	var insertedIDs []int64

	// Cleanup tasks
	t.Cleanup(func() {
		if len(insertedIDs) > 0 {
			query := "DELETE FROM scheduled_tasks WHERE id IN ("
			for i, id := range insertedIDs {
				if i > 0 {
					query += ","
				}
				query += fmt.Sprintf("%d", id)
			}
			query += ")"
			_, _ = repo.db.ExecContext(ctx, query)
		}
	})

	now := time.Now().Truncate(time.Second)

	// 1. Insert invalid kind -> error
	_, err := repo.InsertTask(ctx, ScheduledTask{
		Kind:    "invalid_kind",
		Payload: map[string]any{"campaign_id": 1},
		RunAt:   now,
	})
	if err == nil {
		t.Errorf("expected error on invalid kind, got nil")
	}

	// 2. Insert valid tasks
	task1 := ScheduledTask{
		Kind:    "campaign",
		Payload: map[string]any{"campaign_id": 101.0}, // JSON unmarshals floats
		RunAt:   now.Add(-5 * time.Second),
	}
	task2 := ScheduledTask{
		Kind:    "email",
		Payload: map[string]any{"email": "test@test.local"},
		RunAt:   now.Add(-2 * time.Second),
	}
	taskFuture := ScheduledTask{
		Kind:    "campaign",
		Payload: map[string]any{"campaign_id": 102.0},
		RunAt:   now.Add(10 * time.Minute),
	}

	id1, err := repo.InsertTask(ctx, task1)
	if err != nil {
		t.Fatalf("InsertTask 1 failed: %v", err)
	}
	insertedIDs = append(insertedIDs, id1)

	id2, err := repo.InsertTask(ctx, task2)
	if err != nil {
		t.Fatalf("InsertTask 2 failed: %v", err)
	}
	insertedIDs = append(insertedIDs, id2)

	idFuture, err := repo.InsertTask(ctx, taskFuture)
	if err != nil {
		t.Fatalf("InsertTask future failed: %v", err)
	}
	insertedIDs = append(insertedIDs, idFuture)

	// 3. ClaimDue
	claimed, err := repo.ClaimDue(ctx, now, 10)
	if err != nil {
		t.Fatalf("ClaimDue failed: %v", err)
	}

	// Should have claimed task1 and task2, but not taskFuture
	if len(claimed) != 2 {
		t.Errorf("expected 2 claimed tasks, got %d", len(claimed))
	}

	var found1, found2, foundFuture bool
	for _, ct := range claimed {
		if ct.ID == id1 {
			found1 = true
			if ct.Status != "running" {
				t.Errorf("expected status 'running', got %q", ct.Status)
			}
			if ct.Payload["campaign_id"] != 101.0 {
				t.Errorf("expected campaign_id 101.0, got %v", ct.Payload["campaign_id"])
			}
		}
		if ct.ID == id2 {
			found2 = true
			if ct.Status != "running" {
				t.Errorf("expected status 'running', got %q", ct.Status)
			}
			if ct.Payload["email"] != "test@test.local" {
				t.Errorf("expected email 'test@test.local', got %v", ct.Payload["email"])
			}
		}
		if ct.ID == idFuture {
			foundFuture = true
		}
	}

	if !found1 || !found2 {
		t.Errorf("expected task1 and task2 to be claimed")
	}
	if foundFuture {
		t.Errorf("did not expect future task to be claimed")
	}

	// 4. Second ClaimDue should not claim them again
	claimedAgain, err := repo.ClaimDue(ctx, now, 10)
	if err != nil {
		t.Fatalf("Second ClaimDue failed: %v", err)
	}
	if len(claimedAgain) != 0 {
		t.Errorf("expected 0 claimed tasks on second attempt, got %d", len(claimedAgain))
	}

	// 5. MarkDone
	err = repo.MarkDone(ctx, id1)
	if err != nil {
		t.Fatalf("MarkDone failed: %v", err)
	}

	// Let's verify task1 is status='done' via raw select
	var status1 string
	err = repo.db.QueryRowContext(ctx, "SELECT status FROM scheduled_tasks WHERE id = ?", id1).Scan(&status1)
	if err != nil {
		t.Fatalf("failed to query task status: %v", err)
	}
	if status1 != "done" {
		t.Errorf("expected status 'done', got %q", status1)
	}

	// 6. MarkFailed
	err = repo.MarkFailed(ctx, id2, "connection timeout")
	if err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}

	var status2 string
	var attempts2 int
	var lastErr2 string
	err = repo.db.QueryRowContext(ctx, "SELECT status, attempts, last_error FROM scheduled_tasks WHERE id = ?", id2).Scan(&status2, &attempts2, &lastErr2)
	if err != nil {
		t.Fatalf("failed to query task 2: %v", err)
	}
	if status2 != "failed" {
		t.Errorf("expected status 'failed', got %q", status2)
	}
	if attempts2 != 1 {
		t.Errorf("expected attempts 1, got %d", attempts2)
	}
	if lastErr2 != "connection timeout" {
		t.Errorf("expected last_error 'connection timeout', got %q", lastErr2)
	}
}
