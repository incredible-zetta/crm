package httpx

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LinkResolver interface {
	GetLink(ctx context.Context, code string) (target string, campaignID, contactID *int64, err error)
}

type EventRecorder interface {
	LogEvent(ctx context.Context, contactID int64, campaignID *int64, eventType, linkCode string, meta map[string]any) error
}

type ExportResolver interface {
	GetExport(ctx context.Context, id string) (path string, expiresAt *time.Time, err error)
}

type Handlers struct {
	Links   LinkResolver
	Events  EventRecorder
	Exports ExportResolver
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /t/{code}", h.handleClick)
	mux.HandleFunc("GET /o/{code}", h.handleOpen)
	mux.HandleFunc("GET /export/{id}", h.handleExport)
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

	target, campaignID, contactID, err := h.Links.GetLink(r.Context(), code)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if !isSafeRedirect(target) {
		http.NotFound(w, r)
		return
	}

	var cID int64
	if contactID != nil {
		cID = *contactID
	}

	meta := map[string]any{
		"ua": r.UserAgent(),
	}

	if h.Events != nil {
		_ = h.Events.LogEvent(r.Context(), cID, campaignID, "click", code, meta)
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

	meta := map[string]any{
		"ua": r.UserAgent(),
	}

	if h.Events != nil {
		_ = h.Events.LogEvent(r.Context(), 0, nil, "open", code, meta)
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pixelGIF)
}

func (h *Handlers) handleExport(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("id"), ".csv")

	path, expiresAt, err := h.Exports.GetExport(r.Context(), id)
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

func (h *Handlers) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
