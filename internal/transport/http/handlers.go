// Package httptransport exposes the public, unauthenticated HTTP routes:
// click tracking, the open pixel, CSV export downloads, the unsubscribe
// landing page, and the health probe. It is a thin transport over the service
// layer.
package httptransport

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClickResolver resolves a tracking code to a redirect target, recording the
// click as a side effect.
type ClickResolver interface {
	ResolveClick(ctx context.Context, code string) (targetURL string, err error)
}

// OpenRecorder records an email open from the tracking pixel.
type OpenRecorder interface {
	ResolveOpen(ctx context.Context, code string) error
}

// ExportFileResolver resolves an export id to its on-disk path and expiry.
type ExportFileResolver interface {
	GetExportFile(ctx context.Context, id string) (path string, expiresAt *time.Time, err error)
}

// Unsubscriber unsubscribes a contact by their public unsubscribe code.
type Unsubscriber interface {
	UnsubscribeByCode(ctx context.Context, code string) error
}

// Handlers serves the public routes.
type Handlers struct {
	Tracking ClickResolver
	Opens    OpenRecorder
	Exports  ExportFileResolver
	Unsub    Unsubscriber
}

// Register attaches the public routes to the mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /t/{code}", h.handleClick)
	mux.HandleFunc("GET /o/{code}", h.handleOpen)
	mux.HandleFunc("GET /export/{id}", h.handleExport)
	mux.HandleFunc("GET /u/{code}", h.handleUnsubscribe)
	mux.HandleFunc("GET /healthz", h.handleHealth)
}

func isSafeRedirect(target string) bool {
	u, err := url.Parse(target)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (h *Handlers) handleClick(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	target, err := h.Tracking.ResolveClick(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !isSafeRedirect(target) {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

var pixelGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00,
	0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00,
	0x00, 0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02,
	0x44, 0x01, 0x00, 0x3b,
}

func (h *Handlers) handleOpen(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSuffix(r.PathValue("code"), ".png")

	// Best-effort: always return the pixel even if logging fails.
	_ = h.Opens.ResolveOpen(r.Context(), code)

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pixelGIF)
}

func (h *Handlers) handleExport(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("id"), ".csv")

	path, expiresAt, err := h.Exports.GetExportFile(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if expiresAt != nil && time.Now().After(*expiresAt) {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, id+".csv"))
	http.ServeFile(w, r, path)
}

func (h *Handlers) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	err := h.Unsub.UnsubscribeByCode(r.Context(), code)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		// Do not reveal whether the code existed; show a generic page.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(unsubPage("If this link was valid, you have been unsubscribed.")))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(unsubPage("You have been unsubscribed. You will no longer receive emails from us.")))
}

func unsubPage(msg string) string {
	return `<!doctype html><html><head><meta charset="utf-8"><title>Unsubscribe</title>` +
		`<meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<style>body{font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem;color:#11162A}` +
		`h1{font-size:1.25rem}p{color:#444}</style></head>` +
		`<body><h1>Zetta CRM</h1><p>` + html.EscapeString(msg) + `</p></body></html>`
}

func (h *Handlers) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
