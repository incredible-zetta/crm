// Package threadsdisc is the cookie-only Threads discovery adapter. It shells
// out to the x-threads-utils binary (https://github.com/incredible-zetta/x-threads-utils),
// which embeds the query catalog and scrapes dynamic tokens from threads.com
// using only a logged-in session cookie. The CRM stores the cookie blob per
// account; this adapter writes it to a 0600 temp file, points COOKIES_FILE at
// it, runs the binary, and parses the output.
//
// This is intentionally separate from the Graph API ThreadsGateway: no access
// token, web-scraped, and only the reliable cookie-only commands are exposed
// (search-posts, viral, latest).
package threadsdisc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// Config configures the discovery runner.
type Config struct {
	// BinPath is the path to the x-threads-utils `threads` binary.
	BinPath string
	// Timeout bounds a single binary invocation. Defaults to 60s.
	Timeout time.Duration
}

// Runner executes the discovery binary. It holds no session state; cookies are
// passed per call so one runner can serve many accounts.
type Runner struct {
	cfg Config
}

var _ port.ThreadsDiscovery = (*Runner)(nil)

// New returns a discovery runner. BinPath is required.
func New(cfg Config) (*Runner, error) {
	if strings.TrimSpace(cfg.BinPath) == "" {
		return nil, fmt.Errorf("threads discovery binary path required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &Runner{cfg: cfg}, nil
}

// SearchPosts runs `search-posts <query>`. Output is a JSON array of posts.
func (r *Runner) SearchPosts(ctx context.Context, cookies, query string) ([]domain.ThreadsDiscoveredPost, []byte, error) {
	raw, err := r.run(ctx, cookies, "search-posts", query)
	if err != nil {
		return nil, raw, err
	}
	var posts []domain.ThreadsDiscoveredPost
	if err := json.Unmarshal(raw, &posts); err != nil {
		return nil, raw, fmt.Errorf("parse search-posts output: %w", err)
	}
	return posts, raw, nil
}

// Viral runs `viral <topic>`. Output is human-readable text, returned raw.
func (r *Runner) Viral(ctx context.Context, cookies, topic string) ([]byte, error) {
	return r.run(ctx, cookies, "viral", topic)
}

// Latest runs `latest <topic>`. Output is human-readable text, returned raw.
func (r *Runner) Latest(ctx context.Context, cookies, topic string) ([]byte, error) {
	return r.run(ctx, cookies, "latest", topic)
}

// run writes the cookie blob to a private temp file, invokes the binary with
// COOKIES_FILE pointing at it, and returns stdout. The temp file is always
// removed before returning.
func (r *Runner) run(ctx context.Context, cookies string, args ...string) ([]byte, error) {
	if strings.TrimSpace(cookies) == "" {
		return nil, fmt.Errorf("cookies required")
	}
	if len(args) == 0 || strings.TrimSpace(args[len(args)-1]) == "" {
		return nil, fmt.Errorf("query/topic required")
	}

	cookieFile, err := writeCookieFile(cookies)
	if err != nil {
		return nil, err
	}
	defer os.Remove(cookieFile)

	ctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.cfg.BinPath, args...)
	cmd.Env = append(os.Environ(), "COOKIES_FILE="+cookieFile)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), fmt.Errorf("threads discovery %v: %s", args, msg)
	}
	return stdout.Bytes(), nil
}

// writeCookieFile writes the cookie blob to a 0600 temp file and returns its
// path. The blob must already be in Netscape cookie-file format.
func writeCookieFile(cookies string) (string, error) {
	f, err := os.CreateTemp("", "threads-cookies-*.txt")
	if err != nil {
		return "", fmt.Errorf("create cookie temp file: %w", err)
	}
	name := f.Name()
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(name)
		return "", fmt.Errorf("chmod cookie temp file: %w", err)
	}
	if _, err := f.WriteString(cookies); err != nil {
		f.Close()
		os.Remove(name)
		return "", fmt.Errorf("write cookie temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("close cookie temp file: %w", err)
	}
	return name, nil
}
