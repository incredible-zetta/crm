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
	Version  string
	BaseURL  string
}

// Register attaches the public routes to the mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.handleHome)
	mux.HandleFunc("GET /t/{code}", h.handleClick)
	mux.HandleFunc("GET /o/{code}", h.handleOpen)
	mux.HandleFunc("GET /export/{id}", h.handleExport)
	mux.HandleFunc("GET /u/{code}", h.handleUnsubscribe)
	mux.HandleFunc("GET /healthz", h.handleHealth)
}

func (h *Handlers) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	version := h.Version
	if version == "" {
		version = "dev"
	}
	baseURL := h.BaseURL
	if baseURL == "" {
		baseURL = "https://your-domain.example"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(homePage(version, baseURL)))
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

func homePage(version, baseURL string) string {
	escapedVersion := html.EscapeString(version)
	escapedBaseURL := html.EscapeString(strings.TrimSuffix(baseURL, "/"))
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Zetta CRM Backend Online</title>
  <meta name="description" content="Zetta CRM MCP backend for AI operators is online.">
  <style>
    :root{color-scheme:dark;--bg:#0B0F1C;--surface:#11162A;--surface2:#171C33;--border:#2A3150;--primary:#7B5CFF;--secondary:#4F8CFF;--cyan:#00D5FF;--pink:#FF3CF7;--text:#F4F6FF;--muted:#A6ACCE;--dim:#6B7096;--ok:#22D3A1}
    *{box-sizing:border-box}body{margin:0;min-height:100vh;font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:var(--text);background:radial-gradient(circle at 18% 12%,rgba(123,92,255,.28),transparent 34rem),radial-gradient(circle at 78% 6%,rgba(0,213,255,.18),transparent 28rem),linear-gradient(180deg,#0B0F1C 0%,#080B16 100%);overflow-x:hidden}.grid{position:fixed;inset:0;pointer-events:none;opacity:.2;background-image:linear-gradient(rgba(166,172,206,.08) 1px,transparent 1px),linear-gradient(90deg,rgba(166,172,206,.08) 1px,transparent 1px);background-size:48px 48px;mask-image:linear-gradient(to bottom,#000,transparent)}main{width:min(1120px,calc(100% - 32px));margin:0 auto;padding:56px 0 40px}.shell{display:grid;gap:24px;grid-template-columns:1.02fr .98fr;align-items:center}.badge{display:inline-flex;gap:8px;align-items:center;border:1px solid rgba(34,211,161,.35);background:rgba(34,211,161,.08);color:var(--ok);border-radius:999px;padding:7px 12px;font-size:12px;font-weight:800;letter-spacing:.14em;text-transform:uppercase}.pulse{width:8px;height:8px;border-radius:999px;background:var(--ok);box-shadow:0 0 24px var(--ok)}h1{margin:18px 0 14px;font-size:clamp(42px,7vw,76px);line-height:.94;letter-spacing:-.06em}.grad{background:linear-gradient(120deg,var(--primary),var(--secondary),var(--cyan));-webkit-background-clip:text;background-clip:text;color:transparent}p{margin:0;color:var(--muted);font-size:18px;line-height:1.65;max-width:62ch}.cards{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;margin-top:28px}.card{border:1px solid var(--border);background:linear-gradient(180deg,rgba(23,28,51,.82),rgba(17,22,42,.72));border-radius:18px;padding:16px;box-shadow:0 24px 80px rgba(0,0,0,.22)}.label{font:700 11px/1.2 ui-monospace,SFMono-Regular,Menlo,monospace;color:var(--dim);letter-spacing:.12em;text-transform:uppercase}.value{margin-top:8px;font:700 14px/1.45 ui-monospace,SFMono-Regular,Menlo,monospace;color:var(--text);word-break:break-all}.terminal{border:1px solid rgba(123,92,255,.38);background:linear-gradient(180deg,rgba(7,10,20,.92),rgba(17,22,42,.82));border-radius:24px;overflow:hidden;box-shadow:0 28px 100px rgba(0,0,0,.38),0 0 80px rgba(123,92,255,.12)}.bar{display:flex;gap:8px;align-items:center;border-bottom:1px solid var(--border);padding:13px 16px;background:rgba(255,255,255,.03)}.dot{width:10px;height:10px;border-radius:999px}.red{background:#FF5C7C}.yellow{background:#F5C542}.green{background:#22D3A1}.path{margin-left:8px;font:12px ui-monospace,SFMono-Regular,Menlo,monospace;color:var(--dim)}pre{margin:0;padding:22px;overflow:auto;color:#B8C0FF;font:12px/1.35 ui-monospace,SFMono-Regular,Menlo,monospace}.ascii{color:var(--cyan);text-shadow:0 0 26px rgba(0,213,255,.28)}.cmd{color:var(--ok)}.routes{margin-top:18px;color:var(--muted)}a{color:inherit}.actions{display:flex;flex-wrap:wrap;gap:12px;margin-top:28px}.btn{display:inline-flex;align-items:center;justify-content:center;border-radius:12px;padding:12px 16px;font-weight:800;text-decoration:none}.primary{background:linear-gradient(135deg,var(--primary),var(--secondary));box-shadow:0 0 34px rgba(123,92,255,.28)}.secondary{border:1px solid var(--border);background:rgba(17,22,42,.74);color:var(--muted)}footer{margin-top:34px;color:var(--dim);font-size:13px}@media(max-width:880px){.shell{grid-template-columns:1fr}.cards{grid-template-columns:1fr}main{padding-top:32px}pre{font-size:10px}}
  </style>
</head>
<body>
  <div class="grid"></div>
  <main>
    <section class="shell">
      <div>
        <span class="badge"><span class="pulse"></span> backend online</span>
        <h1>Zetta CRM <span class="grad">Backend</span></h1>
        <p>Self-hosted MCP CRM for AI operators. Contacts, email, campaigns, tracking, scheduling, exports, analytics, unsubscribe, soft-delete, and purge live behind your infrastructure.</p>
        <div class="actions">
          <a class="btn primary" href="https://github.com/incredible-zetta/crm/wiki">Install wiki</a>
          <a class="btn secondary" href="https://github.com/incredible-zetta/crm/releases/tag/v0.0.1-beta">Release notes</a>
          <a class="btn secondary" href="https://github.com/incredible-zetta/crm">GitHub</a>
        </div>
        <div class="cards">
          <div class="card"><div class="label">Version</div><div class="value">` + escapedVersion + `</div></div>
          <div class="card"><div class="label">MCP endpoint</div><div class="value">POST ` + escapedBaseURL + `/mcp</div></div>
          <div class="card"><div class="label">Health</div><div class="value">GET /healthz</div></div>
          <div class="card"><div class="label">Image</div><div class="value">ghcr.io/incredible-zetta/crm:` + escapedVersion + `</div></div>
        </div>
      </div>
      <div class="terminal" aria-label="Zetta terminal card">
        <div class="bar"><span class="dot red"></span><span class="dot yellow"></span><span class="dot green"></span><span class="path">/usr/local/zetta/crm</span></div>
        <pre><code><span class="ascii">███████╗███████╗████████╗████████╗ █████╗
╚══███╔╝██╔════╝╚══██╔══╝╚══██╔══╝██╔══██╗
  ███╔╝ █████╗     ██║      ██║   ███████║
 ███╔╝  ██╔══╝     ██║      ██║   ██╔══██║
███████╗███████╗   ██║      ██║   ██║  ██║
╚══════╝╚══════╝   ╚═╝      ╚═╝   ╚═╝  ╚═╝</span>

<span class="cmd">$ curl -fsS ` + escapedBaseURL + `/healthz</span>
ok

<span class="routes">Public routes:
GET /healthz
GET /t/{code}
GET /o/{code}.png
GET /export/{id}.csv
GET /u/{code}

Private MCP:
POST /mcp
Authorization: Bearer $MCP_API_KEY</span></code></pre>
      </div>
    </section>
    <footer>Zetta CRM · Incredible Zetta × Ciptadusa · data stays in your infra</footer>
  </main>
</body>
</html>`
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
