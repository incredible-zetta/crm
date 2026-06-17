package threadsdisc

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// FetchConfig configures auto-download of the x-threads-utils release binary.
type FetchConfig struct {
	Token   string // GitHub token (PAT or GITHUB_TOKEN) with read access to the repo
	Repo    string // owner/name, e.g. incredible-zetta/x-threads-utils
	Tag     string // release tag, e.g. v0.1.0 or "latest"
	Dest    string // path to write the extracted `threads` binary to
	Timeout time.Duration
}

const defaultDiscoveryRepo = "incredible-zetta/x-threads-utils"

// EnsureBinary downloads and extracts the x-threads-utils `threads` binary to
// cfg.Dest when it is missing and a token is provided. It is a no-op (nil) when
// the binary already exists. The release archive is selected for the current
// GOOS/GOARCH. Works in a distroless container: pure Go, no shell/curl/tar.
func EnsureBinary(ctx context.Context, cfg FetchConfig) error {
	if cfg.Dest == "" {
		return fmt.Errorf("discovery binary destination required")
	}
	if fileExists(cfg.Dest) {
		return nil // already present
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return fmt.Errorf("binary %q missing and no GH_TOKEN set to download it", cfg.Dest)
	}
	if cfg.Repo == "" {
		cfg.Repo = defaultDiscoveryRepo
	}
	if cfg.Tag == "" {
		cfg.Tag = "latest"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	assetName, isZip := assetNameFor(runtime.GOOS, runtime.GOARCH)
	asset, err := findAsset(ctx, cfg, assetName)
	if err != nil {
		return err
	}
	blob, err := downloadAsset(ctx, cfg, asset.ID)
	if err != nil {
		return err
	}
	if err := extractBinary(blob, isZip, cfg.Dest); err != nil {
		return err
	}
	return nil
}

// assetNameFor returns the goreleaser archive name for an OS/arch and whether
// it is a zip (windows) vs tar.gz. Project name is "threads"; the version
// segment is matched loosely by findAsset using a prefix/suffix check.
func assetNameFor(goos, goarch string) (suffix string, isZip bool) {
	if goos == "windows" {
		return fmt.Sprintf("windows_%s.zip", goarch), true
	}
	return fmt.Sprintf("%s_%s.tar.gz", goos, goarch), false
}

type ghAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func findAsset(ctx context.Context, cfg FetchConfig, suffix string) (ghAsset, error) {
	var path string
	if cfg.Tag == "latest" {
		path = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", cfg.Repo)
	} else {
		path = fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", cfg.Repo, cfg.Tag)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ghAsset{}, fmt.Errorf("list release: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ghAsset{}, fmt.Errorf("list release %s: HTTP %d: %s", cfg.Tag, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var rel struct {
		TagName string    `json:"tag_name"`
		Assets  []ghAsset `json:"assets"`
	}
	if err := json.Unmarshal(raw, &rel); err != nil {
		return ghAsset{}, fmt.Errorf("parse release: %w", err)
	}
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, suffix) {
			return a, nil
		}
	}
	return ghAsset{}, fmt.Errorf("no release asset ending in %q for %s@%s", suffix, cfg.Repo, rel.TagName)
}

func downloadAsset(ctx context.Context, cfg FetchConfig, assetID int64) ([]byte, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/assets/%d", cfg.Repo, assetID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	// octet-stream is required to get the binary blob (not JSON metadata).
	req.Header.Set("Accept", "application/octet-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download asset: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		return nil, fmt.Errorf("download asset: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	return io.ReadAll(io.LimitReader(res.Body, 256<<20))
}

// extractBinary pulls the `threads` entry out of a tar.gz or zip archive and
// writes it to dest with 0755.
func extractBinary(archive []byte, isZip bool, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir for binary: %w", err)
	}
	if isZip {
		return extractZip(archive, dest)
	}
	return extractTarGz(archive, dest)
}

func extractTarGz(archive []byte, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if isThreadsBinary(hdr.Name) {
			return writeBinary(dest, tr)
		}
	}
	return fmt.Errorf("threads binary not found in tar.gz archive")
}

func extractZip(archive []byte, dest string) error {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}
	for _, f := range zr.File {
		if isThreadsBinary(f.Name) {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open zip entry: %w", err)
			}
			defer rc.Close()
			return writeBinary(dest, rc)
		}
	}
	return fmt.Errorf("threads binary not found in zip archive")
}

// isThreadsBinary matches the CLI entry ("threads" or "threads.exe"), ignoring
// any directory prefix in the archive.
func isThreadsBinary(name string) bool {
	base := filepath.Base(filepath.ToSlash(name))
	return base == "threads" || base == "threads.exe"
}

func writeBinary(dest string, r io.Reader) error {
	tmp := dest + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create binary: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write binary: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("install binary: %w", err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
