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
func (r *whFakeRepo) MarkRead(ctx context.Context, id int64, at *time.Time) error     { return nil }
func (r *whFakeRepo) MarkReplied(ctx context.Context, id int64, at time.Time) error   { return nil }
func (r *whFakeRepo) MarkNotified(ctx context.Context, id int64, at time.Time) error  { return nil }
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
	svc := service.NewWhatsAppService(whFakeGateway{}, repo, whFakeContacts{}, whStubClock{}, nil, port.SmartSendPolicy{})
	return NewWhatsAppWebhookHandler(svc, secret), repo
}

func post(h http.Handler, body, sig string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/wa/webhook", strings.NewReader(body))
	if sig != "" {
		req.Header.Set("X-Webhook-Signature", sig)
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

func TestWebhookMessageIngested(t *testing.T) {
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
	body := `{"action":"message","data":{"id":"wamid-2","from":"628123456789@s.whatsapp.net","from_me":true,"body":"echo","timestamp":"2026-06-09T10:00:00Z"}}`
	rec := post(h, body, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if repo.inserts != 0 {
		t.Errorf("from_me message should be skipped, inserts=%d", repo.inserts)
	}
}

func TestWebhookReceiptUpdatesStatus(t *testing.T) {
	h, repo := newWebhookHandler("")
	body := `{"action":"message.ack","data":{"ids":["m1","m2"],"type":"read","timestamp":"2026-06-09T10:00:00Z"}}`
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

func TestWebhookUnknownActionIgnored(t *testing.T) {
	h, repo := newWebhookHandler("")
	body := `{"action":"presence","data":{}}`
	rec := post(h, body, "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (unknown action ignored)", rec.Code)
	}
	if repo.inserts != 0 || len(repo.statusCalls) != 0 {
		t.Error("unknown action should not touch repo")
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
	body := `{"action":"message","data":{"id":"sig-1","from":"628123456789@s.whatsapp.net","body":"hi","timestamp":"2026-06-09T10:00:00Z"}}`
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
	body := `{"action":"message","data":{"id":"sig-2","from":"628123456789@s.whatsapp.net","body":"hi","timestamp":"2026-06-09T10:00:00Z"}}`
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
	body := `{"action":"message","data":{"id":"sig-3","from":"628123456789@s.whatsapp.net","body":"hi","timestamp":"2026-06-09T10:00:00Z"}}`
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
	original := `{"action":"message","data":{"id":"sig-4","from":"628123456789@s.whatsapp.net","body":"hi","timestamp":"2026-06-09T10:00:00Z"}}`
	tampered := strings.Replace(original, "hi", "HACKED", 1)
	rec := post(h, tampered, sign(secret, original)) // sig for original, body tampered
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (body tampered after signing)", rec.Code)
	}
	if repo.inserts != 0 {
		t.Error("tampered body must not be processed")
	}
}
