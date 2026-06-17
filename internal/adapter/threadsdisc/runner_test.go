package threadsdisc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func runGo(t *testing.T, dir string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// buildFakeBin compiles a tiny Go program that acts as the threads binary: it
// prints a JSON array for `search-posts`, echoes its COOKIES_FILE contents to
// stderr for assertions, and exits non-zero for an "error" query.
func buildFakeBin(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-binary test uses a shell-free Go build; skip on windows path quirks")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	const prog = `package main

import (
	"fmt"
	"os"
)

func main() {
	cf := os.Getenv("COOKIES_FILE")
	b, _ := os.ReadFile(cf)
	fmt.Fprintf(os.Stderr, "COOKIES=%s", string(b))
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "missing args")
		os.Exit(2)
	}
	cmd, arg := os.Args[1], os.Args[2]
	if arg == "boom" {
		fmt.Fprintln(os.Stderr, "error: kaboom")
		os.Exit(1)
	}
	switch cmd {
	case "search-posts":
		fmt.Print(` + "`" + `[{"pk":"123","code":"abc","Caption":"hello","like_count":7,"taken_at":100,"user":{"pk":"u1","username":"zuck","full_name":"M Z"}}]` + "`" + `)
	default:
		fmt.Printf("%s %s\n", cmd, arg)
	}
}
`
	if err := os.WriteFile(src, []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "threads")
	// go build via os/exec
	if out, err := runGo(t, dir, "build", "-o", bin, src); err != nil {
		t.Fatalf("build fake bin: %v\n%s", err, out)
	}
	return bin
}

func TestNew_RequiresBinPath(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected error for empty bin path")
	}
}

func TestSearchPosts_ParsesJSONAndPassesCookies(t *testing.T) {
	bin := buildFakeBin(t)
	r, err := New(Config{BinPath: bin})
	if err != nil {
		t.Fatal(err)
	}
	cookies := ".threads.com\tTRUE\t/\tTRUE\t0\tsessionid\tSECRET"
	posts, raw, err := r.SearchPosts(context.Background(), cookies, "software engineering")
	if err != nil {
		t.Fatalf("SearchPosts: %v (raw=%s)", err, raw)
	}
	if len(posts) != 1 {
		t.Fatalf("want 1 post, got %d", len(posts))
	}
	p := posts[0]
	if p.PK != "123" || p.Code != "abc" || p.Caption != "hello" || p.LikeCount != 7 || p.TakenAt != 100 {
		t.Fatalf("unexpected post: %+v", p)
	}
	if p.User.Username != "zuck" || p.User.PK != "u1" {
		t.Fatalf("unexpected user: %+v", p.User)
	}
}

func TestRun_RequiresCookies(t *testing.T) {
	bin := buildFakeBin(t)
	r, _ := New(Config{BinPath: bin})
	if _, _, err := r.SearchPosts(context.Background(), "", "q"); err == nil {
		t.Fatal("expected error for empty cookies")
	}
}

func TestRun_RequiresQuery(t *testing.T) {
	bin := buildFakeBin(t)
	r, _ := New(Config{BinPath: bin})
	if _, _, err := r.SearchPosts(context.Background(), "cookie", ""); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestRun_PropagatesBinaryError(t *testing.T) {
	bin := buildFakeBin(t)
	r, _ := New(Config{BinPath: bin})
	_, _, err := r.SearchPosts(context.Background(), "cookie", "boom")
	if err == nil || !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("want kaboom error, got %v", err)
	}
}

func TestViralAndLatest_ReturnRaw(t *testing.T) {
	bin := buildFakeBin(t)
	r, _ := New(Config{BinPath: bin})
	out, err := r.Viral(context.Background(), "cookie", "golang")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "viral golang" {
		t.Fatalf("viral raw: %q", got)
	}
	out, err = r.Latest(context.Background(), "cookie", "rust")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "latest rust" {
		t.Fatalf("latest raw: %q", got)
	}
}

func TestCookieFileCleanedUp(t *testing.T) {
	bin := buildFakeBin(t)
	r, _ := New(Config{BinPath: bin})
	if _, _, err := r.SearchPosts(context.Background(), "cookie", "q"); err != nil {
		t.Fatal(err)
	}
	// No reliable handle to the temp name, but ensure no leftover threads-cookies-* in TMPDIR root.
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "threads-cookies-*.txt"))
	if len(matches) != 0 {
		t.Fatalf("leftover cookie temp files: %v", matches)
	}
}
