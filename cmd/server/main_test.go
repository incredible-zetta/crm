package main

import "testing"

func TestRedactDSNHidesPasswordAndQuery(t *testing.T) {
	got := redactDSN("user:secret@tcp(db:3306)/ai?parseTime=true&multiStatements=true")
	want := "user:***@tcp(db:3306)/ai?..."
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
	if got == "user:secret@tcp(db:3306)/ai?parseTime=true&multiStatements=true" {
		t.Fatal("DSN was not redacted")
	}
}
