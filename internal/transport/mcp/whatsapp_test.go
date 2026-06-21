package mcptransport_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/service"
	mcptransport "github.com/incredible-zetta/crm/internal/transport/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- compact port fakes for WA happy-path -----------------------------------

type waGwFake struct {
	checkStatus domain.WhatsAppStatus
	sends       int
	mediaSends  int
}

func (g *waGwFake) Send(ctx context.Context, m port.WhatsAppMessage) (port.WhatsAppSendResult, error) {
	g.sends++
	return port.WhatsAppSendResult{MessageID: "wamid-mcp", Status: "sent"}, nil
}
func (g *waGwFake) Check(ctx context.Context, phone string) (domain.WhatsAppCheck, error) {
	s := g.checkStatus
	if s == "" {
		s = domain.WhatsAppRegistered
	}
	return domain.WhatsAppCheck{Phone: phone, Status: s, CheckedAt: time.Unix(100, 0)}, nil
}
func (g *waGwFake) MarkRead(ctx context.Context, id, phone string) error { return nil }
func (g *waGwFake) DownloadMedia(ctx context.Context, id, phone string) (port.WhatsAppMedia, error) {
	return port.WhatsAppMedia{URL: "https://media.test/x.jpg", MimeType: "image/jpeg"}, nil
}
func (g *waGwFake) ListGroups(ctx context.Context) ([]port.WhatsAppGroup, error) {
	return []port.WhatsAppGroup{{JID: "120363@g.us", Name: "Team", Topic: "Ops", Participant: 2}}, nil
}
func (g *waGwFake) ListContacts(ctx context.Context) ([]port.WhatsAppContact, error) {
	return []port.WhatsAppContact{{JID: "628123456789@s.whatsapp.net", Name: "Alice"}}, nil
}
func (g *waGwFake) SendMedia(ctx context.Context, m port.WhatsAppMediaMessage) (port.WhatsAppSendResult, error) {
	g.mediaSends++
	return port.WhatsAppSendResult{MessageID: "wamid-media", Status: "sent"}, nil
}

type waRepoFake struct {
	rows   map[int64]domain.WAMessage
	nextID int64
}

func newWARepoFake() *waRepoFake { return &waRepoFake{rows: map[int64]domain.WAMessage{}} }

func (r *waRepoFake) Insert(ctx context.Context, msg domain.WAMessage) (domain.WAMessage, bool, error) {
	r.nextID++
	msg.ID = r.nextID
	r.rows[msg.ID] = msg
	return msg, true, nil
}
func (r *waRepoFake) Get(ctx context.Context, id int64) (domain.WAMessage, error) {
	m, ok := r.rows[id]
	if !ok {
		return domain.WAMessage{}, domain.ErrNotFound
	}
	return m, nil
}
func (r *waRepoFake) List(ctx context.Context, f domain.WAInboundFilter, p port.Paging) (port.WAMessagePage, error) {
	var items []domain.WAMessage
	for _, m := range r.rows {
		items = append(items, m)
	}
	return port.WAMessagePage{Items: items, Total: len(items)}, nil
}
func (r *waRepoFake) UpdateStatus(ctx context.Context, mid string, s domain.WAMessageStatus, at time.Time) error {
	return nil
}
func (r *waRepoFake) MarkRead(ctx context.Context, id int64, at *time.Time) error {
	m := r.rows[id]
	m.ReadAt = at
	r.rows[id] = m
	return nil
}
func (r *waRepoFake) MarkReplied(ctx context.Context, id int64, at time.Time) error  { return nil }
func (r *waRepoFake) MarkNotified(ctx context.Context, id int64, at time.Time) error { return nil }
func (r *waRepoFake) SetRepliedTo(ctx context.Context, outID int64, inMsgID string) error {
	return nil
}
func (r *waRepoFake) SoftDelete(ctx context.Context, id int64) error { return nil }
func (r *waRepoFake) CountSentSince(ctx context.Context, phone string, since time.Time) (int, error) {
	return 0, nil
}
func (r *waRepoFake) CountSentSinceAll(ctx context.Context, since time.Time) (int, error) {
	return 0, nil
}

type waContactsFake struct{ byPhone map[string]domain.Contact }

func (c waContactsFake) Upsert(ctx context.Context, x domain.Contact) (domain.Contact, error) {
	return x, nil
}
func (c waContactsFake) Get(ctx context.Context, id int64) (domain.Contact, error) {
	return domain.Contact{}, domain.ErrNotFound
}
func (c waContactsFake) GetByEmail(ctx context.Context, e string) (domain.Contact, error) {
	return domain.Contact{}, domain.ErrNotFound
}
func (c waContactsFake) GetByPhone(ctx context.Context, p string) (domain.Contact, error) {
	x, ok := c.byPhone[p]
	if !ok {
		return domain.Contact{}, domain.ErrNotFound
	}
	return x, nil
}
func (c waContactsFake) GetByUnsubCode(ctx context.Context, code string) (domain.Contact, error) {
	return domain.Contact{}, domain.ErrNotFound
}
func (c waContactsFake) Update(ctx context.Context, id int64, p domain.ContactPatch) (domain.Contact, error) {
	return domain.Contact{}, nil
}
func (c waContactsFake) List(ctx context.Context, f domain.ContactFilter, p port.Paging) (port.ContactPage, error) {
	return port.ContactPage{}, nil
}
func (c waContactsFake) CountByStage(ctx context.Context) (map[string]int, error) {
	return map[string]int{}, nil
}
func (c waContactsFake) SoftDelete(ctx context.Context, id int64) error { return nil }
func (c waContactsFake) Purge(ctx context.Context, id int64) error      { return nil }
func (c waContactsFake) SetUnsubscribed(ctx context.Context, id int64, t time.Time) error {
	return nil
}
func (c waContactsFake) SetUnsubCode(ctx context.Context, id int64, code string) error { return nil }
func (c waContactsFake) SetEmailStatus(ctx context.Context, id int64, v domain.EmailVerification) error {
	return nil
}
func (c waContactsFake) SetWhatsAppStatus(ctx context.Context, id int64, v domain.WhatsAppCheck) error {
	return nil
}

type waClockFake struct{}

func (waClockFake) Now() time.Time { return time.Unix(1000, 0) }

// wireWA attaches a working WhatsAppService to the harness deps.
func wireWA(h *testHarness, gw *waGwFake) *waRepoFake {
	repo := newWARepoFake()
	contacts := waContactsFake{byPhone: map[string]domain.Contact{}}
	h.deps.Svc.WhatsApp = service.NewWhatsAppService(gw, repo, nil, contacts, waClockFake{}, nil, port.SmartSendPolicy{})
	return repo
}

func waErrCode(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || !res.IsError {
		t.Fatalf("expected error envelope, got %+v", res)
	}
	var env mcpserver.ErrEnvelope
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &env); err != nil {
		t.Fatalf("unmarshal err: %v", err)
	}
	return env.Error
}

// --- disabled contract: every WA tool returns "disabled" when WA not wired ---

func TestWAToolsDisabledContract(t *testing.T) {
	h := setupTestDeps(t) // service.New does not wire WhatsApp -> nil
	ctx := context.Background()

	checks := []struct {
		name string
		call func() *mcp.CallToolResult
	}{
		{"check", func() *mcp.CallToolResult {
			r, _, _ := h.deps.WhatsAppCheck(ctx, nil, mcptransport.WhatsAppCheckIn{Phone: "628123456789"})
			return r
		}},
		{"audit", func() *mcp.CallToolResult {
			r, _, _ := h.deps.WhatsAppAudit(ctx, nil, mcptransport.WhatsAppAuditIn{})
			return r
		}},
		{"send", func() *mcp.CallToolResult {
			r, _, _ := h.deps.WhatsAppSend(ctx, nil, mcptransport.WhatsAppSendIn{Phone: "628123456789", Body: "hi"})
			return r
		}},
		{"list", func() *mcp.CallToolResult {
			r, _, _ := h.deps.WhatsAppList(ctx, nil, mcptransport.WhatsAppListIn{})
			return r
		}},
		{"get", func() *mcp.CallToolResult {
			r, _, _ := h.deps.WhatsAppGet(ctx, nil, mcptransport.WhatsAppGetIn{ID: 1})
			return r
		}},
		{"reply", func() *mcp.CallToolResult {
			r, _, _ := h.deps.WhatsAppReply(ctx, nil, mcptransport.WhatsAppReplyIn{ID: 1, Body: "x"})
			return r
		}},
		{"mark_read", func() *mcp.CallToolResult {
			r, _, _ := h.deps.WhatsAppMarkRead(ctx, nil, mcptransport.WhatsAppMarkReadIn{ID: 1})
			return r
		}},
		{"get_media", func() *mcp.CallToolResult {
			r, _, _ := h.deps.WhatsAppGetMedia(ctx, nil, mcptransport.WhatsAppGetMediaIn{ID: 1})
			return r
		}},
		{"groups", func() *mcp.CallToolResult {
			r, _, _ := h.deps.WhatsAppGroups(ctx, nil, mcptransport.WhatsAppGroupsIn{})
			return r
		}},
		{"send_media", func() *mcp.CallToolResult {
			r, _, _ := h.deps.WhatsAppSendMedia(ctx, nil, mcptransport.WhatsAppSendMediaIn{Phone: "628123456789", Kind: "image", URL: "https://example.com/x.jpg"})
			return r
		}},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if code := waErrCode(t, c.call()); code != "disabled" {
				t.Errorf("tool %s: error code = %q, want disabled", c.name, code)
			}
		})
	}
}

// --- happy paths -----------------------------------------------------------

func TestWACheckPhoneTool(t *testing.T) {
	h := setupTestDeps(t)
	wireWA(h, &waGwFake{checkStatus: domain.WhatsAppRegistered})
	res, out, err := h.deps.WhatsAppCheck(context.Background(), nil, mcptransport.WhatsAppCheckIn{Phone: "628123456789"})
	if err != nil {
		t.Fatalf("go error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("unexpected error envelope")
	}
	if out.Status != string(domain.WhatsAppRegistered) {
		t.Errorf("status = %q, want registered", out.Status)
	}
}

func TestWACheckValidationNoArgs(t *testing.T) {
	h := setupTestDeps(t)
	wireWA(h, &waGwFake{})
	res, _, _ := h.deps.WhatsAppCheck(context.Background(), nil, mcptransport.WhatsAppCheckIn{})
	if code := waErrCode(t, res); code != "validation" {
		t.Errorf("error code = %q, want validation", code)
	}
}

func TestWASendTool(t *testing.T) {
	h := setupTestDeps(t)
	gw := &waGwFake{}
	wireWA(h, gw)
	res, out, err := h.deps.WhatsAppSend(context.Background(), nil, mcptransport.WhatsAppSendIn{Phone: "628123456789", Body: "halo"})
	if err != nil {
		t.Fatalf("go error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("unexpected error envelope")
	}
	if gw.sends != 1 {
		t.Errorf("gateway sends = %d, want 1", gw.sends)
	}
	if out.MessageID == "" {
		t.Errorf("expected message id in output")
	}
}

func TestWAGroupsTool(t *testing.T) {
	h := setupTestDeps(t)
	wireWA(h, &waGwFake{})
	res, out, err := h.deps.WhatsAppGroups(context.Background(), nil, mcptransport.WhatsAppGroupsIn{})
	if err != nil {
		t.Fatalf("go error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("unexpected error envelope")
	}
	if len(out.Items) != 1 || out.Items[0].JID != "120363@g.us" || out.Items[0].Participant != 2 {
		t.Fatalf("groups = %+v", out.Items)
	}
}

func TestWASendMediaTool(t *testing.T) {
	h := setupTestDeps(t)
	gw := &waGwFake{}
	wireWA(h, gw)
	res, out, err := h.deps.WhatsAppSendMedia(context.Background(), nil, mcptransport.WhatsAppSendMediaIn{Phone: "628123456789", Kind: "image", URL: "https://example.com/x.jpg", Caption: "caption"})
	if err != nil {
		t.Fatalf("go error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("unexpected error envelope")
	}
	if gw.mediaSends != 1 || out.MessageID != "wamid-media" {
		t.Fatalf("mediaSends=%d out=%+v", gw.mediaSends, out)
	}
}

func TestWAGetNotFound(t *testing.T) {
	h := setupTestDeps(t)
	wireWA(h, &waGwFake{})
	res, _, err := h.deps.WhatsAppGet(context.Background(), nil, mcptransport.WhatsAppGetIn{ID: 999})
	if err != nil {
		t.Fatalf("go error: %v", err)
	}
	if code := waErrCode(t, res); code != "not_found" {
		t.Errorf("error code = %q, want not_found", code)
	}
}

func TestWAListTool(t *testing.T) {
	h := setupTestDeps(t)
	repo := wireWA(h, &waGwFake{})
	_, _, _ = repo.Insert(context.Background(), domain.WAMessage{MessageID: "m1", Direction: domain.WAInbound, Phone: "628123456789", Body: "hi"})
	res, out, err := h.deps.WhatsAppList(context.Background(), nil, mcptransport.WhatsAppListIn{})
	if err != nil {
		t.Fatalf("go error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("unexpected error envelope")
	}
	if len(out.Items) != 1 {
		t.Errorf("items = %d, want 1", len(out.Items))
	}
}
