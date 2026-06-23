package tenant

import (
	"context"
	"testing"
)

func TestFromDefaultsWhenUnset(t *testing.T) {
	if got := From(context.Background()); got != DefaultID {
		t.Fatalf("expected %q, got %q", DefaultID, got)
	}
}

func TestWithAndFromRoundTrip(t *testing.T) {
	ctx := With(context.Background(), "t_abc")
	if got := From(ctx); got != "t_abc" {
		t.Fatalf("expected %q, got %q", "t_abc", got)
	}
}

func TestWithEmptyNormalizesToDefault(t *testing.T) {
	ctx := With(context.Background(), "")
	if got := From(ctx); got != DefaultID {
		t.Fatalf("expected %q, got %q", DefaultID, got)
	}
}

func TestWithOverwrites(t *testing.T) {
	ctx := With(With(context.Background(), "t_a"), "t_b")
	if got := From(ctx); got != "t_b" {
		t.Fatalf("expected %q, got %q", "t_b", got)
	}
}
