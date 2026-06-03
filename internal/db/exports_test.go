package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestExportsRepo(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set; skipping integration test")
	}

	repo := getTestDB(t)
	ctx := context.Background()

	// Cleanup
	t.Cleanup(func() {
		_, _ = repo.db.ExecContext(ctx, "DELETE FROM exports WHERE id LIKE 't4exp%'")
	})

	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	// Let's form exactly a 16-character ID: "t4exp_" is 6 chars, so we need 10 more.
	// We can take the last 10 characters of uniqueSuffix.
	if len(uniqueSuffix) > 10 {
		uniqueSuffix = uniqueSuffix[len(uniqueSuffix)-10:]
	} else if len(uniqueSuffix) < 10 {
		uniqueSuffix = fmt.Sprintf("%010s", uniqueSuffix)
	}
	id := "t4exp_" + uniqueSuffix

	expires := time.Now().Add(1 * time.Hour).Truncate(time.Second)

	exp := Export{
		ID:        id,
		Path:      "/tmp/test_export.csv",
		Rows:      42,
		ExpiresAt: &expires,
	}

	// 1. CreateExport
	err := repo.CreateExport(ctx, exp)
	if err != nil {
		t.Fatalf("CreateExport failed: %v", err)
	}

	// 2. GetExport
	fetched, err := repo.GetExport(ctx, id)
	if err != nil {
		t.Fatalf("GetExport failed: %v", err)
	}

	if fetched.ID != id {
		t.Errorf("expected ID %q, got %q", id, fetched.ID)
	}
	if fetched.Path != "/tmp/test_export.csv" {
		t.Errorf("expected Path '/tmp/test_export.csv', got %q", fetched.Path)
	}
	if fetched.Rows != 42 {
		t.Errorf("expected Rows 42, got %d", fetched.Rows)
	}
	if fetched.ExpiresAt == nil {
		t.Errorf("expected non-nil ExpiresAt")
	} else if !fetched.ExpiresAt.Equal(expires) && !fetched.ExpiresAt.UTC().Equal(expires.UTC()) {
		t.Errorf("expected ExpiresAt %v, got %v", expires, *fetched.ExpiresAt)
	}

	// 3. CreateExport with null expires_at
	idNil := "t4exp_nilexpi00"
	expNil := Export{
		ID:   idNil,
		Path: "/tmp/nil_export.csv",
		Rows: 0,
	}
	err = repo.CreateExport(ctx, expNil)
	if err != nil {
		t.Fatalf("CreateExport with nil expires_at failed: %v", err)
	}

	fetchedNil, err := repo.GetExport(ctx, idNil)
	if err != nil {
		t.Fatalf("GetExport with nil expires_at failed: %v", err)
	}
	if fetchedNil.ExpiresAt != nil {
		t.Errorf("expected nil ExpiresAt, got %v", fetchedNil.ExpiresAt)
	}

	// 4. Get unknown export -> ErrNotFound
	_, err = repo.GetExport(ctx, "t4exp_nonexistnt")
	if err == nil {
		t.Errorf("expected error for non-existent Export, got nil")
	} else if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
