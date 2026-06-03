package httptransport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeTracking struct {
	target    string
	err       error
	clickedAt []string
}

func (f *fakeTracking) ResolveClick(ctx context.Context, code string) (string, error) {
	f.clickedAt = append(f.clickedAt, code)
	if f.err != nil {
		return "", f.err
	}
	return f.target, nil
}

type fakeOpens struct{ opened []string }

func (f *fakeOpens) ResolveOpen(ctx context.Context, code string) error {
	f.opened = append(f.opened, code)
	return nil
}

type fakeExports struct {
	path      string
	expiresAt *time.Time
	err       error
}

func (f *fakeExports) GetExportFile(ctx context.Context, id string) (string, *time.Time, error) {
	if f.err != nil {
		return "", nil, f.err
	}
	return f.path, f.expiresAt, nil
}

type fakeUnsub struct {
	codes []string
	err   error
}

func (f *fakeUnsub) UnsubscribeByCode(ctx context.Context, code string) error {
	f.codes = append(f.codes, code)
	return f.err
}

func newServer(h *Handlers) *httptest.Server {
	mux := http.NewServeMux()
	h.Register(mux)
	return httptest.NewServer(mux)
}

// get performs a GET and fails the test on transport error.
func get(t *testing.T, u string) *http.Response {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	return resp
}

func TestClickRedirects(t *testing.T) {
	trk := &fakeTracking{target: "https://example.com/page"}
	srv := newServer(&Handlers{Tracking: trk, Opens: &fakeOpens{}, Exports: &fakeExports{}, Unsub: &fakeUnsub{}})
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(srv.URL + "/t/abc123")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://example.com/page" {
		t.Errorf("expected redirect to target, got %q", loc)
	}
	if len(trk.clickedAt) != 1 || trk.clickedAt[0] != "abc123" {
		t.Errorf("expected click resolved for abc123, got %v", trk.clickedAt)
	}
}

func TestClickUnknownCode404(t *testing.T) {
	trk := &fakeTracking{err: errors.New("not found")}
	srv := newServer(&Handlers{Tracking: trk, Opens: &fakeOpens{}, Exports: &fakeExports{}, Unsub: &fakeUnsub{}})
	defer srv.Close()

	resp := get(t, srv.URL+"/t/nope")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestClickRejectsUnsafeScheme(t *testing.T) {
	trk := &fakeTracking{target: "javascript:alert(1)"}
	srv := newServer(&Handlers{Tracking: trk, Opens: &fakeOpens{}, Exports: &fakeExports{}, Unsub: &fakeUnsub{}})
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(srv.URL + "/t/x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unsafe scheme, got %d", resp.StatusCode)
	}
}

func TestOpenPixel(t *testing.T) {
	opens := &fakeOpens{}
	srv := newServer(&Handlers{Tracking: &fakeTracking{}, Opens: opens, Exports: &fakeExports{}, Unsub: &fakeUnsub{}})
	defer srv.Close()

	resp := get(t, srv.URL+"/o/oc1.png")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/gif" {
		t.Errorf("expected image/gif, got %q", ct)
	}
	if len(opens.opened) != 1 || opens.opened[0] != "oc1" {
		t.Errorf("expected open recorded for oc1 (suffix stripped), got %v", opens.opened)
	}
}

func TestExportDownload(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "exp1.csv")
	if err := os.WriteFile(p, []byte("id,email\n1,a@b.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	srv := newServer(&Handlers{Tracking: &fakeTracking{}, Opens: &fakeOpens{}, Exports: &fakeExports{path: p, expiresAt: &future}, Unsub: &fakeUnsub{}})
	defer srv.Close()

	resp := get(t, srv.URL+"/export/exp1.csv")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("expected text/csv, got %q", ct)
	}
}

func TestExportExpired404(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	srv := newServer(&Handlers{Tracking: &fakeTracking{}, Opens: &fakeOpens{}, Exports: &fakeExports{path: "/x", expiresAt: &past}, Unsub: &fakeUnsub{}})
	defer srv.Close()

	resp := get(t, srv.URL+"/export/old.csv")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for expired export, got %d", resp.StatusCode)
	}
}

func TestUnsubscribeSuccess(t *testing.T) {
	unsub := &fakeUnsub{}
	srv := newServer(&Handlers{Tracking: &fakeTracking{}, Opens: &fakeOpens{}, Exports: &fakeExports{}, Unsub: unsub})
	defer srv.Close()

	resp := get(t, srv.URL+"/u/code123")
	defer resp.Body.Close()
	body := readBody(resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "unsubscribed") {
		t.Errorf("expected confirmation page, got %q", body)
	}
	if len(unsub.codes) != 1 || unsub.codes[0] != "code123" {
		t.Errorf("expected unsubscribe for code123, got %v", unsub.codes)
	}
}

func TestUnsubscribeUnknownCodeStillOK(t *testing.T) {
	unsub := &fakeUnsub{err: errors.New("not found")}
	srv := newServer(&Handlers{Tracking: &fakeTracking{}, Opens: &fakeOpens{}, Exports: &fakeExports{}, Unsub: unsub})
	defer srv.Close()

	resp := get(t, srv.URL+"/u/bad")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (no enumeration), got %d", resp.StatusCode)
	}
}

func TestHealth(t *testing.T) {
	srv := newServer(&Handlers{Tracking: &fakeTracking{}, Opens: &fakeOpens{}, Exports: &fakeExports{}, Unsub: &fakeUnsub{}})
	defer srv.Close()

	resp := get(t, srv.URL+"/healthz")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func readBody(resp *http.Response) string {
	b := make([]byte, 4096)
	n, _ := resp.Body.Read(b)
	return string(b[:n])
}
