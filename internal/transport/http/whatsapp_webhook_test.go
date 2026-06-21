package httptransport

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/service"
)

// --- minimal port fakes for webhook-driven service paths -------------------

type whFakeGateway struct{}

func (whFakeGateway) Send(ctx context.Context, m port.WhatsAppMessage) (port.WhatsAppSendResult, error) {
	return port.WhatsAppSendResult{}, nil
}
func (whFakeGateway) Check(ctx context.Context, phone string) (domain.WhatsAppCheck, error) {
	return domain.WhatsAppCheck{}, nil
}
func (whFakeGateway) MarkRead(ctx context.Context, id, phone string) error { return nil }
func (whFakeGateway) DownloadMedia(ctx context.Context, id, phone string) (port.WhatsAppMedia, error) {
	return port.WhatsAppMedia{}, nil
}
func (whFakeGateway) ListGroups(ctx context.Context) ([]port.WhatsAppGroup, error) { return nil, nil }
func (whFakeGateway) ListContacts(ctx context.Context) ([]port.WhatsAppContact, error) {
	return nil, nil
}
func (whFakeGateway) JoinGroup(ctx context.Context, link string) (string, error) { return "", nil }
func (whFakeGateway) LeaveGroup(ctx context.Context, jid string) error           { return nil }
func (whFakeGateway) GroupInfoFromLink(ctx context.Context, link string) (port.WhatsAppGroup, error) {
	return port.WhatsAppGroup{}, nil
}
func (whFakeGateway) SendMedia(ctx context.Context, msg port.WhatsAppMediaMessage) (port.WhatsAppSendResult, error) {
	return port.WhatsAppSendResult{}, nil
}

type whFakeRepo struct {
	inserts     int
	statusCalls []string
}

func (r *whFakeRepo) Insert(ctx context.Context, msg domain.WAMessage) (domain.WAMessage, bool, error) {
	r.inserts++
	msg.ID = int64(r.inserts)
	return msg, true, nil
}
func (r *whFakeRepo) Get(ctx context.Context, id int64) (domain.WAMessage, error) {
	return domain.WAMessage{}, domain.ErrNotFound
}
func (r *whFakeRepo) List(ctx context.Context, f domain.WAInboundFilter, p port.Paging) (port.WAMessagePage, error) {
	return port.WAMessagePage{}, nil
}
func (r *whFakeRepo) UpdateStatus(ctx context.Context, messageID string, status domain.WAMessageStatus, at time.Time) error {
	r.statusCalls = append(r.statusCalls, messageID+":"+string(status))
	return nil
}
func (r *whFakeRepo) MarkRead(ctx context.Context, id int64, at *time.Time) error    { return nil }
func (r *whFakeRepo) MarkReplied(ctx context.Context, id int64, at time.Time) error  { return nil }
func (r *whFakeRepo) MarkNotified(ctx context.Context, id int64, at time.Time) error { return nil }
func (r *whFakeRepo) SetRepliedTo(ctx context.Context, outID int64, inMsgID string) error {
	return nil
}
func (r *whFakeRepo) SoftDelete(ctx context.Context, id int64) error { return nil }
func (r *whFakeRepo) CountSentSince(ctx context.Context, phone string, since time.Time) (int, error) {
	return 0, nil
}
func (r *whFakeRepo) CountSentSinceAll(ctx context.Context, since time.Time) (int, error) {
	return 0, nil
}

type whFakeContacts struct{}

func (whFakeContacts) Upsert(ctx context.Context, c domain.Contact) (domain.Contact, error) {
	return c, nil
}
func (whFakeContacts) Get(ctx context.Context, id int64) (domain.Contact, error) {
	return domain.Contact{}, domain.ErrNotFound
}
func (whFakeContacts) GetByEmail(ctx context.Context, email string) (domain.Contact, error) {
	return domain.Contact{}, domain.ErrNotFound
}
func (whFakeContacts) GetByPhone(ctx context.Context, phone string) (domain.Contact, error) {
	return domain.Contact{}, domain.ErrNotFound
}
func (whFakeContacts) GetByUnsubCode(ctx context.Context, code string) (domain.Contact, error) {
	return domain.Contact{}, domain.ErrNotFound
}
func (whFakeContacts) Update(ctx context.Context, id int64, patch domain.ContactPatch) (domain.Contact, error) {
	return domain.Contact{}, nil
}
func (whFakeContacts) List(ctx context.Context, f domain.ContactFilter, p port.Paging) (port.ContactPage, error) {
	return port.ContactPage{}, nil
}
func (whFakeContacts) CountByStage(ctx context.Context) (map[string]int, error) {
	return map[string]int{}, nil
}
func (whFakeContacts) SoftDelete(ctx context.Context, id int64) error { return nil }
func (whFakeContacts) Purge(ctx context.Context, id int64) error      { return nil }
func (whFakeContacts) SetUnsubscribed(ctx context.Context, id int64, t time.Time) error {
	return nil
}
func (whFakeContacts) SetUnsubCode(ctx context.Context, id int64, code string) error { return nil }
func (whFakeContacts) SetEmailStatus(ctx context.Context, id int64, v domain.EmailVerification) error {
	return nil
}
func (whFakeContacts) SetWhatsAppStatus(ctx context.Context, id int64, v domain.WhatsAppCheck) error {
	return nil
}

type whStubClock struct{}

func (whStubClock) Now() time.Time { return time.Unix(1000, 0) }

func newWebhookHandler(secret string) (*WhatsAppWebhookHandler, *whFakeRepo) {
	repo := &whFakeRepo{}
	svc := service.NewWhatsAppService(whFakeGateway{}, repo, nil, whFakeContacts{}, whStubClock{}, nil, port.SmartSendPolicy{})
	return NewWhatsAppWebhookHandler(svc, secret), repo
}

func post(h http.Handler, body, sig string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/wa/webhook", strings.NewReader(body))
	if sig != "" {
		// Gateway sends X-Hub-Signature-256: sha256=<hex>.
		req.Header.Set("X-Hub-Signature-256", "sha256="+sig)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// --- tests -----------------------------------------------------------------

func TestWebhookRejectsNonPost(t *testing.T) {
	h, _ := newWebhookHandler("")
	req := httptest.NewRequest(http.MethodGet, "/wa/webhook", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// Real go-whatsapp-web-multidevice inbound text payload.
func TestWebhookMessageIngested(t *testing.T) {
	h, repo := newWebhookHandler("")
	body := `{"device_id":"6285792071380@s.whatsapp.net","event":"message","payload":{"body":"haii","chat_id":"628123456789@s.whatsapp.net","from":"628123456789@s.whatsapp.net","from_lid":"239959873196218@lid","from_name":"Indra","id":"3EB0F0AAAFA7B83D7DCEC6","is_from_me":false,"timestamp":"2026-06-09T18:50:24Z"}}`
	rec := post(h, body, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if repo.inserts != 1 {
		t.Errorf("inserts = %d, want 1", repo.inserts)
	}
}

// Image message: media nested object, caption folded into body.
func TestWebhookMediaMessageIngested(t *testing.T) {
	h, repo := newWebhookHandler("")
	body := `{"event":"message","payload":{"id":"img-1","chat_id":"628123456789@s.whatsapp.net","from":"628123456789@s.whatsapp.net","from_name":"Indra","is_from_me":false,"timestamp":"2026-06-09T18:50:24Z","body":"Check this out!","image":{"url":"https://mmg.whatsapp.net/x.jpeg","caption":"Check this out!"}}}`
	rec := post(h, body, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if repo.inserts != 1 {
		t.Errorf("inserts = %d, want 1", repo.inserts)
	}
}

// Backward-compat: legacy action/data envelope must still parse.
func TestWebhookLegacyActionDataEnvelope(t *testing.T) {
	h, repo := newWebhookHandler("")
	body := `{"action":"message","data":{"id":"wamid-1","from":"628123456789@s.whatsapp.net","body":"halo","timestamp":"2026-06-09T10:00:00Z"}}`
	rec := post(h, body, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if repo.inserts != 1 {
		t.Errorf("inserts = %d, want 1", repo.inserts)
	}
}

func TestWebhookSkipsFromMe(t *testing.T) {
	h, repo := newWebhookHandler("")
	body := `{"event":"message","payload":{"id":"wamid-2","from":"628123456789@s.whatsapp.net","is_from_me":true,"body":"echo","timestamp":"2026-06-09T10:00:00Z"}}`
	rec := post(h, body, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if repo.inserts != 0 {
		t.Errorf("is_from_me message should be skipped, inserts=%d", repo.inserts)
	}
}

// Real ack payload: receipt_type field, timestamp on envelope (not payload).
func TestWebhookReceiptUpdatesStatus(t *testing.T) {
	h, repo := newWebhookHandler("")
	body := `{"device_id":"6285792071380@s.whatsapp.net","event":"message.ack","payload":{"chat_id":"239959873196218@lid","from":"628123456789@s.whatsapp.net","from_lid":"239959873196218@lid","ids":["m1","m2"],"receipt_type":"read"},"timestamp":"2026-06-09T18:51:23Z"}`
	rec := post(h, body, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(repo.statusCalls) != 2 {
		t.Fatalf("status calls = %d, want 2", len(repo.statusCalls))
	}
	if repo.statusCalls[0] != "m1:read" {
		t.Errorf("got %q, want m1:read", repo.statusCalls[0])
	}
}

// Real delivered ack from gateway.
func TestWebhookReceiptDelivered(t *testing.T) {
	h, repo := newWebhookHandler("")
	body := `{"device_id":"6285792071380@s.whatsapp.net","event":"message.ack","payload":{"chat_id":"239959873196218@lid","from":"628123456789@s.whatsapp.net","ids":["3EB031839DBE61B52FB044"],"receipt_type":"delivered","receipt_type_description":"means the message was delivered to the device (but the user might not have noticed)."},"timestamp":"2026-06-09T18:51:23Z"}`
	rec := post(h, body, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(repo.statusCalls) != 1 || repo.statusCalls[0] != "3EB031839DBE61B52FB044:delivered" {
		t.Errorf("status calls = %v, want [3EB031839DBE61B52FB044:delivered]", repo.statusCalls)
	}
}

func TestWebhookUnknownActionIgnored(t *testing.T) {
	h, repo := newWebhookHandler("")
	body := `{"event":"chat_presence","payload":{"state":"composing"}}`
	rec := post(h, body, "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (unknown event ignored)", rec.Code)
	}
	if repo.inserts != 0 || len(repo.statusCalls) != 0 {
		t.Error("unknown event should not touch repo")
	}
}

func TestWebhookInvalidJSON(t *testing.T) {
	h, _ := newWebhookHandler("")
	rec := post(h, `{not json`, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- HMAC validation (security) --------------------------------------------

func TestWebhookHMACValidSignatureAccepted(t *testing.T) {
	const secret = "topsecret"
	h, repo := newWebhookHandler(secret)
	body := `{"event":"message","payload":{"id":"sig-1","from":"628123456789@s.whatsapp.net","body":"hi","timestamp":"2026-06-09T10:00:00Z"}}`
	rec := post(h, body, sign(secret, body))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid signature rejected: status=%d", rec.Code)
	}
	if repo.inserts != 1 {
		t.Errorf("inserts = %d, want 1", repo.inserts)
	}
}

func TestWebhookHMACMissingSignatureRejected(t *testing.T) {
	h, repo := newWebhookHandler("topsecret")
	body := `{"event":"message","payload":{"id":"sig-2","from":"628123456789@s.whatsapp.net","body":"hi","timestamp":"2026-06-09T10:00:00Z"}}`
	rec := post(h, body, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (missing signature)", rec.Code)
	}
	if repo.inserts != 0 {
		t.Error("unsigned request must not be processed")
	}
}

func TestWebhookHMACInvalidSignatureRejected(t *testing.T) {
	h, repo := newWebhookHandler("topsecret")
	body := `{"event":"message","payload":{"id":"sig-3","from":"628123456789@s.whatsapp.net","body":"hi","timestamp":"2026-06-09T10:00:00Z"}}`
	rec := post(h, body, sign("wrong-secret", body))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (bad signature)", rec.Code)
	}
	if repo.inserts != 0 {
		t.Error("tampered request must not be processed")
	}
}

func TestWebhookHMACTamperedBodyRejected(t *testing.T) {
	const secret = "topsecret"
	h, repo := newWebhookHandler(secret)
	original := `{"event":"message","payload":{"id":"sig-4","from":"628123456789@s.whatsapp.net","body":"hi","timestamp":"2026-06-09T10:00:00Z"}}`
	tampered := strings.Replace(original, "hi", "HACKED", 1)
	rec := post(h, tampered, sign(secret, original)) // sig for original, body tampered
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (body tampered after signing)", rec.Code)
	}
	if repo.inserts != 0 {
		t.Error("tampered body must not be processed")
	}
}
