package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type FakeLinks struct {
	GetLinkFn func(ctx context.Context, code string) (target string, campaignID, contactID *int64, err error)
}

func (f *FakeLinks) GetLink(ctx context.Context, code string) (target string, campaignID, contactID *int64, err error) {
	if f.GetLinkFn != nil {
		return f.GetLinkFn(ctx, code)
	}
	return "", nil, nil, errors.New("not implemented")
}

type LoggedEvent struct {
	ContactID  int64
	CampaignID *int64
	EventType  string
	LinkCode   string
	Meta       map[string]any
}

type FakeEvents struct {
	Events     []LoggedEvent
	LogEventFn func(ctx context.Context, contactID int64, campaignID *int64, eventType, linkCode string, meta map[string]any) error
}

func (f *FakeEvents) LogEvent(ctx context.Context, contactID int64, campaignID *int64, eventType, linkCode string, meta map[string]any) error {
	f.Events = append(f.Events, LoggedEvent{
		ContactID:  contactID,
		CampaignID: campaignID,
		EventType:  eventType,
		LinkCode:   linkCode,
		Meta:       meta,
	})
	if f.LogEventFn != nil {
		return f.LogEventFn(ctx, contactID, campaignID, eventType, linkCode, meta)
	}
	return nil
}

type FakeExports struct {
	GetExportFn func(ctx context.Context, id string) (path string, expiresAt *time.Time, err error)
}

func (f *FakeExports) GetExport(ctx context.Context, id string) (path string, expiresAt *time.Time, err error) {
	if f.GetExportFn != nil {
		return f.GetExportFn(ctx, id)
	}
	return "", nil, errors.New("not implemented")
}

func ptr(i int64) *int64 {
	return &i
}

func TestClickRedirectsAndLogs(t *testing.T) {
	fakeLinks := &FakeLinks{
		GetLinkFn: func(ctx context.Context, code string) (string, *int64, *int64, error) {
			if code == "abc" {
				return "https://dest.example/x", ptr(7), ptr(42), nil
			}
			return "", nil, nil, fmt.Errorf("link not found")
		},
	}
	fakeEvents := &FakeEvents{}
	fakeExports := &FakeExports{}

	h := &Handlers{
		Links:   fakeLinks,
		Events:  fakeEvents,
		Exports: fakeExports,
	}

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/t/abc", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "https://dest.example/x" {
		t.Errorf("expected location 'https://dest.example/x', got '%s'", loc)
	}

	if len(fakeEvents.Events) != 1 {
		t.Fatalf("expected 1 event to be logged, got %d", len(fakeEvents.Events))
	}
	ev := fakeEvents.Events[0]
	if ev.EventType != "click" {
		t.Errorf("expected event type 'click', got '%s'", ev.EventType)
	}
	if ev.LinkCode != "abc" {
		t.Errorf("expected link code 'abc', got '%s'", ev.LinkCode)
	}
	if ev.ContactID != 42 {
		t.Errorf("expected contactID 42, got %d", ev.ContactID)
	}
	if ev.CampaignID == nil || *ev.CampaignID != 7 {
		t.Errorf("expected campaignID 7, got %v", ev.CampaignID)
	}
	if ev.Meta == nil || ev.Meta["ua"] != "Mozilla/5.0" {
		t.Errorf("expected ua meta to be 'Mozilla/5.0', got %v", ev.Meta)
	}
}

func TestClickNotFound(t *testing.T) {
	fakeLinks := &FakeLinks{
		GetLinkFn: func(ctx context.Context, code string) (string, *int64, *int64, error) {
			return "", nil, nil, fmt.Errorf("any database error means not found")
		},
	}
	fakeEvents := &FakeEvents{}
	fakeExports := &FakeExports{}

	h := &Handlers{
		Links:   fakeLinks,
		Events:  fakeEvents,
		Exports: fakeExports,
	}

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/t/zzz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
	if len(fakeEvents.Events) != 0 {
		t.Errorf("expected 0 events to be logged, got %d", len(fakeEvents.Events))
	}
}

func TestClickLogErrorStillRedirects(t *testing.T) {
	fakeLinks := &FakeLinks{
		GetLinkFn: func(ctx context.Context, code string) (string, *int64, *int64, error) {
			return "https://dest.example/x", nil, nil, nil
		},
	}
	fakeEvents := &FakeEvents{
		LogEventFn: func(ctx context.Context, contactID int64, campaignID *int64, eventType, linkCode string, meta map[string]any) error {
			return fmt.Errorf("failed to write event log to db")
		},
	}
	fakeExports := &FakeExports{}

	h := &Handlers{
		Links:   fakeLinks,
		Events:  fakeEvents,
		Exports: fakeExports,
	}

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/t/abc", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected status 302 even when logging fails, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "https://dest.example/x" {
		t.Errorf("expected location 'https://dest.example/x', got '%s'", loc)
	}
}

func TestOpenReturnsGifAndLogs(t *testing.T) {
	fakeLinks := &FakeLinks{}
	fakeEvents := &FakeEvents{}
	fakeExports := &FakeExports{}

	h := &Handlers{
		Links:   fakeLinks,
		Events:  fakeEvents,
		Exports: fakeExports,
	}

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/o/openXYZ", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	contentType := rec.Header().Get("Content-Type")
	if contentType != "image/gif" {
		t.Errorf("expected Content-Type 'image/gif', got '%s'", contentType)
	}
	cacheControl := rec.Header().Get("Cache-Control")
	if cacheControl != "no-store" {
		t.Errorf("expected Cache-Control 'no-store', got '%s'", cacheControl)
	}

	// Verify gif pixel body matches expected
	expectedGIF := []byte{
		0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00,
		0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00,
		0x00, 0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02,
		0x44, 0x01, 0x00, 0x3b,
	}
	if string(rec.Body.Bytes()) != string(expectedGIF) {
		t.Errorf("response body does not match the 1x1 transparent gif bytes")
	}

	if len(fakeEvents.Events) != 1 {
		t.Fatalf("expected 1 event to be logged, got %d", len(fakeEvents.Events))
	}
	ev := fakeEvents.Events[0]
	if ev.EventType != "open" {
		t.Errorf("expected event type 'open', got '%s'", ev.EventType)
	}
	if ev.LinkCode != "openXYZ" {
		t.Errorf("expected link code 'openXYZ', got '%s'", ev.LinkCode)
	}
	if ev.ContactID != 0 {
		t.Errorf("expected contactID 0, got %d", ev.ContactID)
	}
	if ev.CampaignID != nil {
		t.Errorf("expected campaignID nil, got %v", ev.CampaignID)
	}
	if ev.Meta == nil || ev.Meta["ua"] != "Mozilla/5.0" {
		t.Errorf("expected ua meta to be 'Mozilla/5.0', got %v", ev.Meta)
	}
}

func TestOpenStripsPngSuffix(t *testing.T) {
	fakeLinks := &FakeLinks{}
	fakeEvents := &FakeEvents{}
	fakeExports := &FakeExports{}

	h := &Handlers{
		Links:   fakeLinks,
		Events:  fakeEvents,
		Exports: fakeExports,
	}

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/o/code123.png", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if len(fakeEvents.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(fakeEvents.Events))
	}
	if fakeEvents.Events[0].LinkCode != "code123" {
		t.Errorf("expected stripped linkCode 'code123', got '%s'", fakeEvents.Events[0].LinkCode)
	}
}

func TestExportServesCSV(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "exp1.csv")
	csvContent := "a,b\n1,2\n"
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	if err != nil {
		t.Fatalf("failed to write temp csv file: %v", err)
	}

	fakeLinks := &FakeLinks{}
	fakeEvents := &FakeEvents{}
	fakeExports := &FakeExports{
		GetExportFn: func(ctx context.Context, id string) (string, *time.Time, error) {
			if id == "exp1" {
				return csvPath, nil, nil
			}
			return "", nil, fmt.Errorf("export not found")
		},
	}

	h := &Handlers{
		Links:   fakeLinks,
		Events:  fakeEvents,
		Exports: fakeExports,
	}

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/export/exp1.csv", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/csv") {
		t.Errorf("expected Content-Type containing 'text/csv', got '%s'", contentType)
	}
	contentDisposition := rec.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "attachment") || !strings.Contains(contentDisposition, "exp1.csv") {
		t.Errorf("expected Content-Disposition to be attachment containing 'exp1.csv', got '%s'", contentDisposition)
	}
	if rec.Body.String() != csvContent {
		t.Errorf("expected body '%s', got '%s'", csvContent, rec.Body.String())
	}
}

func TestExportNotFound(t *testing.T) {
	fakeLinks := &FakeLinks{}
	fakeEvents := &FakeEvents{}
	fakeExports := &FakeExports{
		GetExportFn: func(ctx context.Context, id string) (string, *time.Time, error) {
			return "", nil, fmt.Errorf("any DB/resolver error is 404")
		},
	}

	h := &Handlers{
		Links:   fakeLinks,
		Events:  fakeEvents,
		Exports: fakeExports,
	}

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/export/exp99.csv", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing export, got %d", rec.Code)
	}
}

func TestExportExpired(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "expired.csv")
	err := os.WriteFile(csvPath, []byte("some,data\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	pastTime := time.Now().Add(-10 * time.Minute)

	fakeLinks := &FakeLinks{}
	fakeEvents := &FakeEvents{}
	fakeExports := &FakeExports{
		GetExportFn: func(ctx context.Context, id string) (string, *time.Time, error) {
			return csvPath, &pastTime, nil
		},
	}

	h := &Handlers{
		Links:   fakeLinks,
		Events:  fakeEvents,
		Exports: fakeExports,
	}

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/export/expired.csv", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for expired export, got %d", rec.Code)
	}
}

func TestHealth(t *testing.T) {
	h := &Handlers{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got '%s'", rec.Body.String())
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected Content-Type containing 'text/plain', got '%s'", contentType)
	}
}
