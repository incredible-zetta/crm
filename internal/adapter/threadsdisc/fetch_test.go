package threadsdisc

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func makeTarGz(t *testing.T, binName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte(content)
	if err := tw.WriteHeader(&tar.Header{Name: binName, Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// fetchServer stubs the two GitHub API calls: release-by-tag (JSON) and
// asset-by-id (octet-stream). It rewrites api.github.com URLs to itself via a
// custom transport on http.DefaultClient.
func TestEnsureBinary_DownloadsAndExtracts(t *testing.T) {
	suffix, _ := assetNameFor(runtime.GOOS, runtime.GOARCH)
	assetName := "threads_0.1.0_" + suffix
	archive := makeTarGz(t, "threads", "#!/bin/sh\necho stub\n")

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/x/releases/tags/v0.1.0", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
			t.Errorf("missing/wrong auth header: %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v0.1.0",
			"assets":   []map[string]any{{"id": 999, "name": assetName}},
		})
	})
	mux.HandleFunc("/repos/acme/x/releases/assets/999", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/octet-stream" {
			t.Errorf("asset download must use octet-stream, got %q", got)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archive)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Redirect api.github.com -> test server.
	orig := http.DefaultClient.Transport
	http.DefaultClient.Transport = rewriteTransport{base: srv.URL, rt: http.DefaultTransport}
	defer func() { http.DefaultClient.Transport = orig }()

	dest := filepath.Join(t.TempDir(), "bin", "threads")
	err := EnsureBinary(context.Background(), FetchConfig{Token: "tok123", Repo: "acme/x", Tag: "v0.1.0", Dest: dest})
	if err != nil {
		t.Fatalf("EnsureBinary: %v", err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if !bytes.Contains(b, []byte("stub")) {
		t.Fatalf("unexpected binary content: %q", b)
	}
	info, _ := os.Stat(dest)
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("binary not executable: %v", info.Mode())
	}
}

func TestEnsureBinary_NoopWhenExists(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "threads")
	if err := os.WriteFile(dest, []byte("existing"), 0o755); err != nil {
		t.Fatal(err)
	}
	// No token, but file exists → must be a no-op, no error.
	if err := EnsureBinary(context.Background(), FetchConfig{Dest: dest}); err != nil {
		t.Fatalf("expected no-op, got %v", err)
	}
}

func TestEnsureBinary_MissingTokenErrors(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "threads")
	err := EnsureBinary(context.Background(), FetchConfig{Dest: dest})
	if err == nil {
		t.Fatal("expected error when binary missing and no token")
	}
}

type rewriteTransport struct {
	base string
	rt   http.RoundTripper
}

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "api.github.com" {
		newURL := fmt.Sprintf("%s%s", rt.base, req.URL.Path)
		r2, _ := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
		r2.Header = req.Header
		return rt.rt.RoundTrip(r2)
	}
	return rt.rt.RoundTrip(req)
}
