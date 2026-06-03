package db

import (
	"os"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set; skipping integration test")
	}
	d, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	tables := []string{"contacts", "email_templates", "campaigns", "tracking_links", "email_events", "scheduled_tasks", "exports"}
	for _, tbl := range tables {
		var n int
		if err := d.QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&n); err != nil {
			t.Errorf("table %s not queryable: %v", tbl, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set")
	}
	d, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(d); err != nil {
		t.Fatalf("second migrate (idempotent) failed: %v", err)
	}
}
