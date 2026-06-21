package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// WhatsAppService coordinates outbound sends, inbound webhook ingestion, and
// capability audits (WhatsApp registration checks). It mirrors InboxService
// for the inbound side and adds outbound + audit capabilities.
type WhatsAppService struct {
	gateway   port.WhatsAppGateway
	waRepo    port.WAMessageRepo
	listeners port.WAListenerRepo
	contacts  port.ContactRepo
	clock     port.Clock
	notifier  port.AdminNotifier
	policy    port.SmartSendPolicy
}

func NewWhatsAppService(
	gateway port.WhatsAppGateway,
	waRepo port.WAMessageRepo,
	listeners port.WAListenerRepo,
	contacts port.ContactRepo,
	clock port.Clock,
	notifier port.AdminNotifier,
	policy port.SmartSendPolicy,
) *WhatsAppService {
	return &WhatsAppService{
		gateway:   gateway,
		waRepo:    waRepo,
		listeners: listeners,
		contacts:  contacts,
		clock:     clock,
		notifier:  notifier,
		policy:    policy,
	}
}

// Send sends a WhatsApp message. The body is expected to already be in
// WhatsApp-formatted markdown (*bold*, _italic_, ~strike~, ```code```).
// If the contact is known, it is persisted with the contact_id. Returns the
// stored message with its gateway-assigned message_id.
func (s *WhatsAppService) Send(ctx context.Context, phone, body string) (domain.WAMessage, error) {
	if s.gateway == nil {
		return domain.WAMessage{}, fmt.Errorf("whatsapp disabled")
	}
	phone = normalizePhone(phone)
	if phone == "" {
		return domain.WAMessage{}, fmt.Errorf("invalid phone")
	}
	if body == "" {
		return domain.WAMessage{}, fmt.Errorf("empty body")
	}

	// Smart-send policy: block if contact is verified not-registered.
	if s.policy.BlockNotRegistered {
		c, err := s.contacts.GetByPhone(ctx, phone)
		if err == nil && c.WhatsAppStatus == domain.WhatsAppNotRegistered {
			return domain.WAMessage{}, fmt.Errorf("contact verified not on WhatsApp")
		}
	}

	// Look up contact if known.
	var contactID *int64
	if c, err := s.contacts.GetByPhone(ctx, phone); err == nil {
		contactID = &c.ID
	}

	// Send via gateway (smart-sender wrapper applies rate-limiting).
	result, err := s.gateway.Send(ctx, port.WhatsAppMessage{Phone: phone, Body: body})
	if err != nil {
		return domain.WAMessage{}, fmt.Errorf("gateway send: %w", err)
	}

	now := s.clock.Now()
	msg := domain.WAMessage{
		MessageID: result.MessageID,
		Direction: domain.WAOutbound,
		Phone:     phone,
		ContactID: contactID,
		Body:      body,
		Status:    domain.WAStatusSent,
		SentAt:    &now,
		CreatedAt: now,
	}
	stored, _, err := s.waRepo.Insert(ctx, msg)
	if err != nil {
		return domain.WAMessage{}, fmt.Errorf("persist outbound: %w", err)
	}
	return stored, nil
}

// ListGroups returns joined groups from the gateway.
func (s *WhatsAppService) ListGroups(ctx context.Context) ([]port.WhatsAppGroup, error) {
	if s.gateway == nil {
		return nil, fmt.Errorf("whatsapp disabled")
	}
	return s.gateway.ListGroups(ctx)
}

// ListContacts returns contacts known by gateway storage.
func (s *WhatsAppService) ListContacts(ctx context.Context) ([]port.WhatsAppContact, error) {
	if s.gateway == nil {
		return nil, fmt.Errorf("whatsapp disabled")
	}
	return s.gateway.ListContacts(ctx)
}

// CreateListener enables AI-visible listening for a chat/group JID.
func (s *WhatsAppService) CreateListener(ctx context.Context, chatJID, name string) (domain.WAListener, error) {
	if s.listeners == nil {
		return domain.WAListener{}, fmt.Errorf("whatsapp listeners disabled")
	}
	if chatJID == "" {
		return domain.WAListener{}, fmt.Errorf("chat_jid required")
	}
	return s.listeners.Create(ctx, domain.WAListener{ChatJID: chatJID, Name: name, Enabled: true})
}

func (s *WhatsAppService) ListListeners(ctx context.Context, enabledOnly bool) ([]domain.WAListener, error) {
	if s.listeners == nil {
		return nil, fmt.Errorf("whatsapp listeners disabled")
	}
	return s.listeners.List(ctx, enabledOnly)
}

func (s *WhatsAppService) UpdateListener(ctx context.Context, id int64, chatJID, name string, enabled bool) (domain.WAListener, error) {
	if s.listeners == nil {
		return domain.WAListener{}, fmt.Errorf("whatsapp listeners disabled")
	}
	existing, err := s.listeners.Get(ctx, id)
	if err != nil {
		return domain.WAListener{}, err
	}
	if chatJID != "" {
		existing.ChatJID = chatJID
	}
	existing.Name = name
	existing.Enabled = enabled
	return s.listeners.Update(ctx, id, existing)
}

func (s *WhatsAppService) DeleteListener(ctx context.Context, id int64) error {
	if s.listeners == nil {
		return fmt.Errorf("whatsapp listeners disabled")
	}
	return s.listeners.SoftDelete(ctx, id)
}

func (s *WhatsAppService) ListenerSummary(ctx context.Context, id int64, limit int) (domain.WAListener, []domain.WAMessage, error) {
	if s.listeners == nil {
		return domain.WAListener{}, nil, fmt.Errorf("whatsapp listeners disabled")
	}
	l, err := s.listeners.Get(ctx, id)
	if err != nil {
		return domain.WAListener{}, nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	page, err := s.waRepo.List(ctx, domain.WAInboundFilter{Direction: "in", ChatID: l.ChatJID}, port.Paging{Limit: limit})
	if err != nil {
		return domain.WAListener{}, nil, err
	}
	summary := buildListenerSummary(page.Items)
	if err := s.listeners.SetSummary(ctx, id, summary); err == nil {
		l.Summary = summary
	}
	return l, page.Items, nil
}

func buildListenerSummary(items []domain.WAMessage) string {
	if len(items) == 0 {
		return "No recent messages."
	}
	var b strings.Builder
	b.WriteString("Recent WhatsApp listener messages:\n")
	for i := len(items) - 1; i >= 0; i-- {
		msg := items[i]
		if msg.Body == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(msg.Body)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// SendMedia sends image/video/file media. Caption is persisted as body.
func (s *WhatsAppService) SendMedia(ctx context.Context, in port.WhatsAppMediaMessage) (domain.WAMessage, error) {
	if s.gateway == nil {
		return domain.WAMessage{}, fmt.Errorf("whatsapp disabled")
	}
	phone := normalizePhone(in.Phone)
	if phone == "" {
		return domain.WAMessage{}, fmt.Errorf("invalid phone")
	}
	in.Phone = phone
	result, err := s.gateway.SendMedia(ctx, in)
	if err != nil {
		return domain.WAMessage{}, fmt.Errorf("gateway send media: %w", err)
	}
	var contactID *int64
	if c, err := s.contacts.GetByPhone(ctx, phone); err == nil {
		contactID = &c.ID
	}
	now := s.clock.Now()
	msg := domain.WAMessage{
		MessageID: result.MessageID,
		Direction: domain.WAOutbound,
		Phone:     phone,
		ContactID: contactID,
		Body:      in.Caption,
		MediaURL:  in.URL,
		Status:    domain.WAStatusSent,
		SentAt:    &now,
		CreatedAt: now,
	}
	switch in.Kind {
	case "image":
		msg.MediaType = domain.WAMediaImage
	case "video":
		msg.MediaType = domain.WAMediaVideo
	case "file", "document":
		msg.MediaType = domain.WAMediaDocument
	}
	stored, _, err := s.waRepo.Insert(ctx, msg)
	if err != nil {
		return domain.WAMessage{}, fmt.Errorf("persist outbound media: %w", err)
	}
	return stored, nil
}

// Audit verifies a page of contacts and persists their WhatsApp registration
// status. Returns summary counts and the next cursor for pagination.
func (s *WhatsAppService) Audit(ctx context.Context, f domain.ContactFilter, onlyUnchecked bool, limit int, cursor int64) (domain.WAAuditResult, error) {
	if s.gateway == nil {
		return domain.WAAuditResult{}, fmt.Errorf("whatsapp disabled")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	page, err := s.contacts.List(ctx, f, port.Paging{Limit: limit, Cursor: cursor})
	if err != nil {
		return domain.WAAuditResult{}, fmt.Errorf("list contacts: %w", err)
	}
	var res domain.WAAuditResult
	for _, c := range page.Items {
		if onlyUnchecked && c.WhatsAppCheckedAt != nil {
			continue
		}
		phone := normalizePhone(c.Phone)
		if phone == "" {
			continue
		}
		v, err := s.gateway.Check(ctx, phone)
		if err != nil {
			continue
		}
		if err := s.contacts.SetWhatsAppStatus(ctx, c.ID, v); err != nil {
			continue
		}
		res.Checked++
		switch v.Status {
		case domain.WhatsAppRegistered:
			res.Registered++
		case domain.WhatsAppNotRegistered:
			res.NotRegistered++
		default:
			res.Unknown++
		}
	}
	res.NextCursor = page.NextCursor
	return res, nil
}

// Check verifies a single contact's WhatsApp registration and persists it.
func (s *WhatsAppService) Check(ctx context.Context, id int64) (domain.Contact, domain.WhatsAppCheck, error) {
	if s.gateway == nil {
		return domain.Contact{}, domain.WhatsAppCheck{}, fmt.Errorf("whatsapp disabled")
	}
	c, err := s.contacts.Get(ctx, id)
	if err != nil {
		return domain.Contact{}, domain.WhatsAppCheck{}, err
	}
	phone := normalizePhone(c.Phone)
	if phone == "" {
		return c, domain.WhatsAppCheck{}, fmt.Errorf("contact has no phone")
	}
	v, err := s.gateway.Check(ctx, phone)
	if err != nil {
		return c, domain.WhatsAppCheck{}, fmt.Errorf("gateway check: %w", err)
	}
	if err := s.contacts.SetWhatsAppStatus(ctx, c.ID, v); err != nil {
		return c, v, err
	}
	c.WhatsAppStatus = v.Status
	c.WhatsAppCheckedAt = &v.CheckedAt
	return c, v, nil
}

// CheckPhone verifies an arbitrary phone number's WhatsApp registration.
// Does not persist to any contact (use Check for contact-persisted checks).
func (s *WhatsAppService) CheckPhone(ctx context.Context, phone string) (domain.WhatsAppCheck, error) {
	if s.gateway == nil {
		return domain.WhatsAppCheck{}, fmt.Errorf("whatsapp disabled")
	}
	phone = normalizePhone(phone)
	if phone == "" {
		return domain.WhatsAppCheck{}, fmt.Errorf("invalid phone")
	}
	return s.gateway.Check(ctx, phone)
}

// IngestMessage processes an inbound webhook message. If the sender is a known
// contact, it is linked and an admin notification is sent.
func (s *WhatsAppService) IngestMessage(ctx context.Context, evt domain.WAInboundEvent) error {
	if s.gateway == nil {
		return fmt.Errorf("whatsapp disabled")
	}
	phone := normalizePhone(extractPhoneFromJID(evt.From))
	if phone == "" {
		return fmt.Errorf("invalid sender phone")
	}

	// Look up contact if known.
	var contactID *int64
	if c, err := s.contacts.GetByPhone(ctx, phone); err == nil {
		contactID = &c.ID
	}

	// Detect media type and URL.
	var mediaType domain.WAMediaType
	var mediaURL string
	if evt.MediaType != "" {
		mediaType = evt.MediaType
		mediaURL = evt.MediaURL
	}

	now := s.clock.Now()
	msg := domain.WAMessage{
		MessageID:  evt.MessageID,
		ChatID:     evt.ChatID,
		Direction:  domain.WAInbound,
		Phone:      phone,
		ContactID:  contactID,
		Body:       evt.Body,
		MediaType:  mediaType,
		MediaURL:   mediaURL,
		RepliedTo:  evt.RepliedTo,
		Status:     domain.WAStatusReceived,
		ReceivedAt: &now,
		CreatedAt:  now,
	}
	stored, isNew, err := s.waRepo.Insert(ctx, msg)
	if err != nil {
		return fmt.Errorf("persist inbound: %w", err)
	}
	if !isNew {
		return nil // idempotent
	}

	// TODO: Add WhatsApp-specific admin notifier
	// For now, we just store the message without notifying
	_ = stored
	return nil
}

// IngestReceipt processes a delivered/read receipt webhook.
func (s *WhatsAppService) IngestReceipt(ctx context.Context, evt domain.WAReceiptEvent) error {
	if s.gateway == nil {
		return fmt.Errorf("whatsapp disabled")
	}
	var status domain.WAMessageStatus
	switch evt.ReceiptType {
	case "delivered":
		status = domain.WAStatusDelivered
	case "read":
		status = domain.WAStatusRead
	default:
		return nil // ignore other receipt types
	}
	for _, msgID := range evt.MessageIDs {
		_ = s.waRepo.UpdateStatus(ctx, msgID, status, evt.Timestamp)
	}
	return nil
}

// List returns a page of WhatsApp messages (inbound + outbound).
func (s *WhatsAppService) List(ctx context.Context, f domain.WAInboundFilter, p port.Paging) (port.WAMessagePage, error) {
	if s.waRepo == nil {
		return port.WAMessagePage{}, fmt.Errorf("whatsapp disabled")
	}
	return s.waRepo.List(ctx, f, p)
}

// Get retrieves a single message by ID.
func (s *WhatsAppService) Get(ctx context.Context, id int64) (domain.WAMessage, error) {
	if s.waRepo == nil {
		return domain.WAMessage{}, fmt.Errorf("whatsapp disabled")
	}
	return s.waRepo.Get(ctx, id)
}

// MarkRead marks an inbound message as read (both locally and on the gateway).
func (s *WhatsAppService) MarkRead(ctx context.Context, id int64) error {
	if s.gateway == nil || s.waRepo == nil {
		return fmt.Errorf("whatsapp disabled")
	}
	msg, err := s.waRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	if msg.Direction != domain.WAInbound {
		return fmt.Errorf("only inbound messages can be marked read")
	}
	if err := s.gateway.MarkRead(ctx, msg.MessageID, msg.Phone); err != nil {
		return fmt.Errorf("gateway mark read: %w", err)
	}
	now := s.clock.Now()
	return s.waRepo.MarkRead(ctx, id, &now)
}

// Reply sends a reply to an inbound message, linking it via replied_to.
func (s *WhatsAppService) Reply(ctx context.Context, inboundID int64, body string) (domain.WAMessage, error) {
	if s.gateway == nil || s.waRepo == nil {
		return domain.WAMessage{}, fmt.Errorf("whatsapp disabled")
	}
	inbound, err := s.waRepo.Get(ctx, inboundID)
	if err != nil {
		return domain.WAMessage{}, err
	}
	if inbound.Direction != domain.WAInbound {
		return domain.WAMessage{}, fmt.Errorf("can only reply to inbound messages")
	}
	outbound, err := s.Send(ctx, inbound.Phone, body)
	if err != nil {
		return domain.WAMessage{}, err
	}
	// Link the reply.
	_ = s.waRepo.SetRepliedTo(ctx, outbound.ID, inbound.MessageID)
	// Mark the inbound as replied.
	_ = s.waRepo.MarkReplied(ctx, inbound.ID, s.clock.Now())
	outbound.RepliedTo = inbound.MessageID
	return outbound, nil
}

// Delete soft-deletes a message.
func (s *WhatsAppService) Delete(ctx context.Context, id int64) error {
	if s.waRepo == nil {
		return fmt.Errorf("whatsapp disabled")
	}
	return s.waRepo.SoftDelete(ctx, id)
}

// DownloadMedia fetches the media URL for a message with an attachment.
func (s *WhatsAppService) DownloadMedia(ctx context.Context, id int64) (string, error) {
	if s.gateway == nil || s.waRepo == nil {
		return "", fmt.Errorf("whatsapp disabled")
	}
	msg, err := s.waRepo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if msg.MessageID == "" {
		return "", fmt.Errorf("message has no gateway ID")
	}
	media, err := s.gateway.DownloadMedia(ctx, msg.MessageID, msg.Phone)
	if err != nil {
		return "", err
	}
	return media.URL, nil
}

// normalizePhone is a service-layer helper that delegates to the adapter's
// phone normalization logic. We import it indirectly via the gateway port, but
// for service-layer use we duplicate the minimal logic here.
func normalizePhone(raw string) string {
	// Strip non-digits.
	var b []byte
	for i := 0; i < len(raw); i++ {
		if raw[i] >= '0' && raw[i] <= '9' {
			b = append(b, raw[i])
		}
	}
	digits := string(b)
	if digits == "" {
		return ""
	}
	// Replace leading 0 with 62 (Indonesian national prefix).
	if len(digits) > 0 && digits[0] == '0' {
		digits = "62" + digits[1:]
	}
	return digits
}

// extractPhoneFromJID strips the @s.whatsapp.net suffix from a WhatsApp JID.
func extractPhoneFromJID(jid string) string {
	for i := 0; i < len(jid); i++ {
		if jid[i] == '@' {
			return jid[:i]
		}
	}
	return jid
}
