package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// --- fakes -----------------------------------------------------------------

type fakeWAGateway struct {
	sends      int
	sentBodies []string
	checkVerd  domain.WhatsAppStatus
	sendErr    error
	checkErr   error
	markedRead []string
	mediaURL   string
}

func (g *fakeWAGateway) Send(ctx context.Context, msg port.WhatsAppMessage) (port.WhatsAppSendResult, error) {
	if g.sendErr != nil {
		return port.WhatsAppSendResult{}, g.sendErr
	}
	g.sends++
	g.sentBodies = append(g.sentBodies, msg.Body)
	return port.WhatsAppSendResult{MessageID: "wamid-out", Status: "sent"}, nil
}

func (g *fakeWAGateway) Check(ctx context.Context, phone string) (domain.WhatsAppCheck, error) {
	if g.checkErr != nil {
		return domain.WhatsAppCheck{}, g.checkErr
	}
	status := g.checkVerd
	if status == "" {
		status = domain.WhatsAppRegistered
	}
	return domain.WhatsAppCheck{Phone: phone, Status: status, CheckedAt: time.Unix(100, 0)}, nil
}

func (g *fakeWAGateway) MarkRead(ctx context.Context, messageID, phone string) error {
	g.markedRead = append(g.markedRead, messageID)
	return nil
}

func (g *fakeWAGateway) DownloadMedia(ctx context.Context, messageID, phone string) (port.WhatsAppMedia, error) {
	return port.WhatsAppMedia{URL: g.mediaURL, MimeType: "image/jpeg"}, nil
}

type fakeWARepo struct {
	msgs        map[int64]domain.WAMessage
	byMessageID map[string]int64
	nextID      int64
	inserted    []domain.WAMessage
	statusCalls []string
	repliedTo   map[int64]string
	markReplied map[int64]bool
}

func newFakeWARepo() *fakeWARepo {
	return &fakeWARepo{
		msgs:        map[int64]domain.WAMessage{},
		byMessageID: map[string]int64{},
		repliedTo:   map[int64]string{},
		markReplied: map[int64]bool{},
	}
}

func (r *fakeWARepo) Insert(ctx context.Context, msg domain.WAMessage) (domain.WAMessage, bool, error) {
	if msg.MessageID != "" {
		if id, ok := r.byMessageID[msg.MessageID]; ok {
			return r.msgs[id], false, nil // idempotent
		}
	}
	r.nextID++
	msg.ID = r.nextID
	r.msgs[msg.ID] = msg
	if msg.MessageID != "" {
		r.byMessageID[msg.MessageID] = msg.ID
	}
	r.inserted = append(r.inserted, msg)
	return msg, true, nil
}

func (r *fakeWARepo) Get(ctx context.Context, id int64) (domain.WAMessage, error) {
	m, ok := r.msgs[id]
	if !ok {
		return domain.WAMessage{}, domain.ErrNotFound
	}
	return m, nil
}

func (r *fakeWARepo) List(ctx context.Context, f domain.WAInboundFilter, p port.Paging) (port.WAMessagePage, error) {
	var items []domain.WAMessage
	for _, m := range r.msgs {
		items = append(items, m)
	}
	return port.WAMessagePage{Items: items, Total: len(items)}, nil
}

func (r *fakeWARepo) UpdateStatus(ctx context.Context, messageID string, status domain.WAMessageStatus, at time.Time) error {
	r.statusCalls = append(r.statusCalls, messageID+":"+string(status))
	if id, ok := r.byMessageID[messageID]; ok {
		m := r.msgs[id]
		m.Status = status
		r.msgs[id] = m
	}
	return nil
}

func (r *fakeWARepo) MarkRead(ctx context.Context, id int64, at *time.Time) error {
	m := r.msgs[id]
	m.ReadAt = at
	r.msgs[id] = m
	return nil
}

func (r *fakeWARepo) MarkReplied(ctx context.Context, id int64, at time.Time) error {
	r.markReplied[id] = true
	return nil
}

func (r *fakeWARepo) MarkNotified(ctx context.Context, id int64, at time.Time) error { return nil }

func (r *fakeWARepo) SetRepliedTo(ctx context.Context, outboundID int64, inboundMessageID string) error {
	r.repliedTo[outboundID] = inboundMessageID
	return nil
}

func (r *fakeWARepo) SoftDelete(ctx context.Context, id int64) error {
	delete(r.msgs, id)
	return nil
}

func (r *fakeWARepo) CountSentSince(ctx context.Context, phone string, since time.Time) (int, error) {
	return 0, nil
}

func (r *fakeWARepo) CountSentSinceAll(ctx context.Context, since time.Time) (int, error) {
	return 0, nil
}

// fakeWAContactRepo is a minimal ContactRepo for WhatsApp service tests. Only
// the phone/list/status methods are exercised; the rest satisfy the interface.
type fakeWAContactRepo struct {
	byPhone     map[string]domain.Contact
	byID        map[int64]domain.Contact
	listItems   []domain.Contact
	statusSaved map[int64]domain.WhatsAppCheck
}

func newFakeWAContactRepo() *fakeWAContactRepo {
	return &fakeWAContactRepo{
		byPhone:     map[string]domain.Contact{},
		byID:        map[int64]domain.Contact{},
		statusSaved: map[int64]domain.WhatsAppCheck{},
	}
}

func (r *fakeWAContactRepo) Upsert(ctx context.Context, c domain.Contact) (domain.Contact, error) {
	return c, nil
}
func (r *fakeWAContactRepo) Get(ctx context.Context, id int64) (domain.Contact, error) {
	c, ok := r.byID[id]
	if !ok {
		return domain.Contact{}, domain.ErrNotFound
	}
	return c, nil
}
func (r *fakeWAContactRepo) GetByEmail(ctx context.Context, email string) (domain.Contact, error) {
	return domain.Contact{}, domain.ErrNotFound
}
func (r *fakeWAContactRepo) GetByPhone(ctx context.Context, phone string) (domain.Contact, error) {
	c, ok := r.byPhone[phone]
	if !ok {
		return domain.Contact{}, domain.ErrNotFound
	}
	return c, nil
}
func (r *fakeWAContactRepo) GetByUnsubCode(ctx context.Context, code string) (domain.Contact, error) {
	return domain.Contact{}, domain.ErrNotFound
}
func (r *fakeWAContactRepo) Update(ctx context.Context, id int64, patch domain.ContactPatch) (domain.Contact, error) {
	return domain.Contact{}, nil
}
func (r *fakeWAContactRepo) List(ctx context.Context, f domain.ContactFilter, p port.Paging) (port.ContactPage, error) {
	return port.ContactPage{Items: r.listItems, Total: len(r.listItems)}, nil
}
func (r *fakeWAContactRepo) CountByStage(ctx context.Context) (map[string]int, error) {
	return map[string]int{}, nil
}
func (r *fakeWAContactRepo) SoftDelete(ctx context.Context, id int64) error { return nil }
func (r *fakeWAContactRepo) Purge(ctx context.Context, id int64) error      { return nil }
func (r *fakeWAContactRepo) SetUnsubscribed(ctx context.Context, id int64, t time.Time) error {
	return nil
}
func (r *fakeWAContactRepo) SetUnsubCode(ctx context.Context, id int64, code string) error {
	return nil
}
func (r *fakeWAContactRepo) SetEmailStatus(ctx context.Context, id int64, v domain.EmailVerification) error {
	return nil
}
func (r *fakeWAContactRepo) SetWhatsAppStatus(ctx context.Context, id int64, v domain.WhatsAppCheck) error {
	r.statusSaved[id] = v
	c := r.byID[id]
	c.WhatsAppStatus = v.Status
	c.WhatsAppCheckedAt = &v.CheckedAt
	r.byID[id] = c
	return nil
}

func newWASvc(gw *fakeWAGateway, repo *fakeWARepo, contacts *fakeWAContactRepo, policy port.SmartSendPolicy) *WhatsAppService {
	return NewWhatsAppService(gw, repo, contacts, stubClock{now: time.Unix(1000, 0)}, nil, policy)
}

// --- Send ------------------------------------------------------------------

func TestWASendStoresOutboundAndLinksContact(t *testing.T) {
	gw := &fakeWAGateway{}
	repo := newFakeWARepo()
	contacts := newFakeWAContactRepo()
	contacts.byPhone["628123456789"] = domain.Contact{ID: 7, Phone: "08123456789"}
	svc := newWASvc(gw, repo, contacts, port.SmartSendPolicy{})

	msg, err := svc.Send(context.Background(), "08123456789", "*hi*")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if gw.sends != 1 {
		t.Errorf("gateway sends = %d, want 1", gw.sends)
	}
	if msg.Direction != domain.WAOutbound || msg.Status != domain.WAStatusSent {
		t.Errorf("unexpected message: dir=%s status=%s", msg.Direction, msg.Status)
	}
	if msg.ContactID == nil || *msg.ContactID != 7 {
		t.Errorf("contact not linked: %v", msg.ContactID)
	}
	if msg.Phone != "628123456789" {
		t.Errorf("phone not normalized: %q", msg.Phone)
	}
}

func TestWASendBlocksNotRegistered(t *testing.T) {
	gw := &fakeWAGateway{}
	repo := newFakeWARepo()
	contacts := newFakeWAContactRepo()
	contacts.byPhone["628123456789"] = domain.Contact{ID: 7, Phone: "08123456789", WhatsAppStatus: domain.WhatsAppNotRegistered}
	svc := newWASvc(gw, repo, contacts, port.SmartSendPolicy{BlockNotRegistered: true})

	_, err := svc.Send(context.Background(), "08123456789", "hi")
	if err == nil {
		t.Fatal("expected block error for not-registered contact")
	}
	if gw.sends != 0 {
		t.Errorf("gateway should not be called when blocked, sends=%d", gw.sends)
	}
}

func TestWASendRejectsEmptyInputs(t *testing.T) {
	svc := newWASvc(&fakeWAGateway{}, newFakeWARepo(), newFakeWAContactRepo(), port.SmartSendPolicy{})
	if _, err := svc.Send(context.Background(), "", "hi"); err == nil {
		t.Error("expected error for empty phone")
	}
	if _, err := svc.Send(context.Background(), "08123456789", ""); err == nil {
		t.Error("expected error for empty body")
	}
}

func TestWASendGatewayErrorNotPersisted(t *testing.T) {
	gw := &fakeWAGateway{sendErr: errors.New("boom")}
	repo := newFakeWARepo()
	svc := newWASvc(gw, repo, newFakeWAContactRepo(), port.SmartSendPolicy{})
	if _, err := svc.Send(context.Background(), "08123456789", "hi"); err == nil {
		t.Fatal("expected gateway error")
	}
	if len(repo.inserted) != 0 {
		t.Errorf("nothing should be persisted on gateway failure, got %d", len(repo.inserted))
	}
}

// --- Audit / Check ---------------------------------------------------------

func TestWAAuditCountsAndPersists(t *testing.T) {
	gw := &fakeWAGateway{checkVerd: domain.WhatsAppRegistered}
	contacts := newFakeWAContactRepo()
	contacts.listItems = []domain.Contact{
		{ID: 1, Phone: "08123456701"},
		{ID: 2, Phone: "08123456702"},
		{ID: 3, Phone: ""}, // skipped: no phone
	}
	contacts.byID[1] = contacts.listItems[0]
	contacts.byID[2] = contacts.listItems[1]
	svc := newWASvc(gw, newFakeWARepo(), contacts, port.SmartSendPolicy{})

	res, err := svc.Audit(context.Background(), domain.ContactFilter{}, false, 100, 0)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if res.Checked != 2 || res.Registered != 2 {
		t.Errorf("got checked=%d registered=%d, want 2/2", res.Checked, res.Registered)
	}
	if _, ok := contacts.statusSaved[1]; !ok {
		t.Error("status not persisted for contact 1")
	}
}

func TestWAAuditOnlyUncheckedSkipsChecked(t *testing.T) {
	gw := &fakeWAGateway{}
	contacts := newFakeWAContactRepo()
	checked := time.Unix(50, 0)
	contacts.listItems = []domain.Contact{
		{ID: 1, Phone: "08123456701", WhatsAppCheckedAt: &checked},
		{ID: 2, Phone: "08123456702"},
	}
	contacts.byID[1] = contacts.listItems[0]
	contacts.byID[2] = contacts.listItems[1]
	svc := newWASvc(gw, newFakeWARepo(), contacts, port.SmartSendPolicy{})

	res, err := svc.Audit(context.Background(), domain.ContactFilter{}, true, 100, 0)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if res.Checked != 1 {
		t.Errorf("checked = %d, want 1 (already-checked must be skipped)", res.Checked)
	}
}

func TestWACheckPersistsStatus(t *testing.T) {
	gw := &fakeWAGateway{checkVerd: domain.WhatsAppRegistered}
	contacts := newFakeWAContactRepo()
	contacts.byID[5] = domain.Contact{ID: 5, Phone: "08123456789"}
	svc := newWASvc(gw, newFakeWARepo(), contacts, port.SmartSendPolicy{})

	c, v, err := svc.Check(context.Background(), 5)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if v.Status != domain.WhatsAppRegistered {
		t.Errorf("status = %s, want registered", v.Status)
	}
	if c.WhatsAppStatus != domain.WhatsAppRegistered {
		t.Errorf("contact status not updated: %s", c.WhatsAppStatus)
	}
}

// --- IngestMessage ---------------------------------------------------------

func TestWAIngestMessageLinksKnownContact(t *testing.T) {
	repo := newFakeWARepo()
	contacts := newFakeWAContactRepo()
	contacts.byPhone["628123456789"] = domain.Contact{ID: 9, Phone: "08123456789"}
	svc := newWASvc(&fakeWAGateway{}, repo, contacts, port.SmartSendPolicy{})

	evt := domain.WAInboundEvent{
		MessageID: "wamid-in-1",
		From:      "628123456789@s.whatsapp.net",
		Body:      "halo",
		Timestamp: time.Unix(500, 0),
	}
	if err := svc.IngestMessage(context.Background(), evt); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("inserted = %d, want 1", len(repo.inserted))
	}
	got := repo.inserted[0]
	if got.Direction != domain.WAInbound || got.Status != domain.WAStatusReceived {
		t.Errorf("unexpected inbound msg: dir=%s status=%s", got.Direction, got.Status)
	}
	if got.ContactID == nil || *got.ContactID != 9 {
		t.Errorf("contact not linked: %v", got.ContactID)
	}
	if got.Phone != "628123456789" {
		t.Errorf("phone not normalized from JID: %q", got.Phone)
	}
}

func TestWAIngestMessageIdempotent(t *testing.T) {
	repo := newFakeWARepo()
	svc := newWASvc(&fakeWAGateway{}, repo, newFakeWAContactRepo(), port.SmartSendPolicy{})
	evt := domain.WAInboundEvent{MessageID: "dup-1", From: "628123456789@s.whatsapp.net", Body: "x", Timestamp: time.Unix(1, 0)}

	_ = svc.IngestMessage(context.Background(), evt)
	_ = svc.IngestMessage(context.Background(), evt)
	if len(repo.inserted) != 1 {
		t.Errorf("duplicate webhook stored twice: inserted=%d", len(repo.inserted))
	}
}

// --- IngestReceipt ---------------------------------------------------------

func TestWAIngestReceiptUpdatesStatus(t *testing.T) {
	repo := newFakeWARepo()
	svc := newWASvc(&fakeWAGateway{}, repo, newFakeWAContactRepo(), port.SmartSendPolicy{})

	cases := []struct {
		rtype string
		want  domain.WAMessageStatus
	}{
		{"delivered", domain.WAStatusDelivered},
		{"read", domain.WAStatusRead},
	}
	for _, c := range cases {
		repo.statusCalls = nil
		evt := domain.WAReceiptEvent{MessageIDs: []string{"m1", "m2"}, ReceiptType: c.rtype, Timestamp: time.Unix(2, 0)}
		if err := svc.IngestReceipt(context.Background(), evt); err != nil {
			t.Fatalf("%s: %v", c.rtype, err)
		}
		if len(repo.statusCalls) != 2 {
			t.Errorf("%s: status calls = %d, want 2", c.rtype, len(repo.statusCalls))
		}
		if repo.statusCalls[0] != "m1:"+string(c.want) {
			t.Errorf("%s: got %q", c.rtype, repo.statusCalls[0])
		}
	}
}

func TestWAIngestReceiptIgnoresUnknownType(t *testing.T) {
	repo := newFakeWARepo()
	svc := newWASvc(&fakeWAGateway{}, repo, newFakeWAContactRepo(), port.SmartSendPolicy{})
	evt := domain.WAReceiptEvent{MessageIDs: []string{"m1"}, ReceiptType: "played", Timestamp: time.Unix(2, 0)}
	if err := svc.IngestReceipt(context.Background(), evt); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(repo.statusCalls) != 0 {
		t.Errorf("unknown receipt type should be ignored, calls=%d", len(repo.statusCalls))
	}
}

// --- MarkRead / Reply ------------------------------------------------------

func TestWAMarkReadRejectsOutbound(t *testing.T) {
	repo := newFakeWARepo()
	repo.nextID = 0
	out, _, _ := repo.Insert(context.Background(), domain.WAMessage{MessageID: "o1", Direction: domain.WAOutbound, Phone: "628123456789"})
	svc := newWASvc(&fakeWAGateway{}, repo, newFakeWAContactRepo(), port.SmartSendPolicy{})
	if err := svc.MarkRead(context.Background(), out.ID); err == nil {
		t.Error("expected error marking outbound as read")
	}
}

func TestWAReplyLinksAndMarks(t *testing.T) {
	repo := newFakeWARepo()
	gw := &fakeWAGateway{}
	contacts := newFakeWAContactRepo()
	svc := newWASvc(gw, repo, contacts, port.SmartSendPolicy{})

	inbound, _, _ := repo.Insert(context.Background(), domain.WAMessage{
		MessageID: "in-1", Direction: domain.WAInbound, Phone: "628123456789",
	})

	out, err := svc.Reply(context.Background(), inbound.ID, "thanks")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if gw.sends != 1 {
		t.Errorf("gateway sends = %d, want 1", gw.sends)
	}
	if out.RepliedTo != "in-1" {
		t.Errorf("reply not linked to inbound wamid: %q", out.RepliedTo)
	}
	if !repo.markReplied[inbound.ID] {
		t.Error("inbound not marked replied")
	}
	if repo.repliedTo[out.ID] != "in-1" {
		t.Error("SetRepliedTo not called with inbound wamid")
	}
}

func TestWAReplyRejectsOutboundTarget(t *testing.T) {
	repo := newFakeWARepo()
	out, _, _ := repo.Insert(context.Background(), domain.WAMessage{MessageID: "o1", Direction: domain.WAOutbound, Phone: "628123456789"})
	svc := newWASvc(&fakeWAGateway{}, repo, newFakeWAContactRepo(), port.SmartSendPolicy{})
	if _, err := svc.Reply(context.Background(), out.ID, "x"); err == nil {
		t.Error("expected error replying to an outbound message")
	}
}

// --- disabled guard --------------------------------------------------------

func TestWADisabledWhenGatewayNil(t *testing.T) {
	svc := NewWhatsAppService(nil, newFakeWARepo(), newFakeWAContactRepo(), stubClock{}, nil, port.SmartSendPolicy{})
	if _, err := svc.Send(context.Background(), "08123456789", "hi"); err == nil {
		t.Error("expected disabled error from Send")
	}
	if _, err := svc.CheckPhone(context.Background(), "08123456789"); err == nil {
		t.Error("expected disabled error from CheckPhone")
	}
}
