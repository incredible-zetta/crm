package threads

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	c, err := New(Config{BaseURL: srv.URL, AccessToken: "tok", UserID: "me", APIVersion: "v1.0"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, srv
}

func TestProfileIncludesFollowersCount(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/threads_insights"):
			if got := r.URL.Query().Get("metric"); got != "followers_count" {
				t.Errorf("metric = %q, want followers_count", got)
			}
			_, _ = w.Write([]byte(`{"data":[{"name":"followers_count","total_value":{"value":1234}}]}`))
		default:
			_, _ = w.Write([]byte(`{"id":"1","username":"callmelords","name":"Zee"}`))
		}
	})
	defer srv.Close()

	p, _, err := c.Profile(context.Background())
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if p.Username != "callmelords" {
		t.Errorf("username = %q", p.Username)
	}
	if p.FollowersCount == nil || *p.FollowersCount != 1234 {
		t.Fatalf("followers_count = %v, want 1234", p.FollowersCount)
	}
}

func TestProfileFollowersCountBestEffortOmittedOnError(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/threads_insights") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"no insights"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"1","username":"callmelords"}`))
	})
	defer srv.Close()

	p, _, err := c.Profile(context.Background())
	if err != nil {
		t.Fatalf("Profile should not fail when insights errors: %v", err)
	}
	if p.FollowersCount != nil {
		t.Fatalf("followers_count = %v, want nil", *p.FollowersCount)
	}
}

func TestFollowerDemographicsBreakdown(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("metric"); got != "follower_demographics" {
			t.Errorf("metric = %q", got)
		}
		if got := r.URL.Query().Get("breakdown"); got != "country" {
			t.Errorf("breakdown = %q, want country", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"name":"follower_demographics","total_value":{"breakdowns":[{"results":[{"dimension_values":["ID"],"value":900}]}]}}]}`))
	})
	defer srv.Close()

	out, _, err := c.FollowerDemographics(context.Background(), "country")
	if err != nil {
		t.Fatalf("FollowerDemographics: %v", err)
	}
	if out["data"] == nil {
		t.Fatalf("expected data in result, got %v", out)
	}
}

func TestFollowerDemographicsDefaultsToCountry(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("breakdown"); got != "country" {
			t.Errorf("breakdown = %q, want country default", got)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	if _, _, err := c.FollowerDemographics(context.Background(), ""); err != nil {
		t.Fatalf("FollowerDemographics: %v", err)
	}
}

func TestFollowerDemographicsRejectsBadBreakdown(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for invalid breakdown")
	})
	defer srv.Close()

	if _, _, err := c.FollowerDemographics(context.Background(), "planet"); err == nil {
		t.Fatal("expected error for invalid breakdown")
	}
}

func TestErrorPrefersUserMessage(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid operation","error_user_msg":"You cannot load follower demographics for a user with fewer than 100 followers."}}`))
	})
	defer srv.Close()

	_, _, err := c.FollowerDemographics(context.Background(), "country")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fewer than 100 followers") {
		t.Fatalf("expected human-readable reason, got: %v", err)
	}
}
