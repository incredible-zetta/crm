package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/template"
)

// EmailService orchestrates the single-email sending pipeline.
type EmailService struct {
	sender       port.EmailSender
	contacts     port.ContactRepo
	templates    port.TemplateRepo
	tracking     port.TrackingRepo
	events       port.EventRepo
	clock        port.Clock
	idgen        port.IDGenerator
	baseURL      string
	openCodeFn   func() (string, error)
	blockInvalid bool // when true, refuse to send to contacts verified invalid
}

// SetBlockInvalid toggles refusing sends to contacts whose persisted email
// verification status is invalid.
func (s *EmailService) SetBlockInvalid(block bool) {
	s.blockInvalid = block
}

// NewEmailService creates a new EmailService.
func NewEmailService(
	sender port.EmailSender,
	contacts port.ContactRepo,
	templates port.TemplateRepo,
	tracking port.TrackingRepo,
	events port.EventRepo,
	clock port.Clock,
	idgen port.IDGenerator,
	baseURL string,
) *EmailService {
	return &EmailService{
		sender:     sender,
		contacts:   contacts,
		templates:  templates,
		tracking:   tracking,
		events:     events,
		clock:      clock,
		idgen:      idgen,
		baseURL:    baseURL,
		openCodeFn: defaultOpenCode,
	}
}

// SendInput defines the parameters for sending a single email.
type SendInput struct {
	ContactID  int64
	CampaignID *int64
	To         string // recipient; resolved from ContactID if empty
	TemplateID int64  // if >0 load template, else raw fields
	Subject    string
	HTML       string
	Text       string
	Vars       map[string]any
}

func defaultOpenCode() (string, error) {
	b := make([]byte, 6)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Send executes the render -> link-rewrite -> open-pixel -> send -> log event pipeline.
func (s *EmailService) Send(ctx context.Context, in SendInput) (status string, to string, err error) {
	var resolvedTo string

	if in.ContactID > 0 {
		c, err := s.contacts.Get(ctx, in.ContactID)
		if err != nil {
			return "", "", err // ErrNotFound passes through
		}
		if c.IsUnsubscribed() {
			return "", "", fmt.Errorf("%w: contact is unsubscribed", domain.ErrValidation)
		}
		if s.blockInvalid && c.EmailStatus == domain.EmailInvalid {
			return "", "", fmt.Errorf("%w: contact email is verified invalid", domain.ErrValidation)
		}
		if in.To != "" && !strings.EqualFold(in.To, c.Email) {
			return "", "", fmt.Errorf("%w: to does not match contact email", domain.ErrValidation)
		}
		resolvedTo = c.Email
	} else {
		if in.To == "" {
			return "", "", fmt.Errorf("%w: recipient required", domain.ErrValidation)
		}
		resolvedTo = in.To
	}

	var rawSubject, rawHTML, rawText string
	if in.TemplateID > 0 {
		t, err := s.templates.Get(ctx, in.TemplateID)
		if err != nil {
			return "", resolvedTo, err // ErrNotFound passes through
		}
		rawSubject = t.Subject
		rawHTML = t.BodyHTML
		rawText = t.BodyText
	} else {
		rawSubject = in.Subject
		rawHTML = in.HTML
		rawText = in.Text
	}

	var renderedSubject, renderedHTML, renderedText string
	if rawSubject != "" {
		renderedSubject, err = template.Render(rawSubject, in.Vars)
		if err != nil {
			return "", resolvedTo, fmt.Errorf("failed to render subject: %w", err)
		}
	}
	if rawHTML != "" {
		renderedHTML, err = template.Render(rawHTML, in.Vars)
		if err != nil {
			return "", resolvedTo, fmt.Errorf("failed to render HTML: %w", err)
		}
	}
	if rawText != "" {
		renderedText, err = template.Render(rawText, in.Vars)
		if err != nil {
			return "", resolvedTo, fmt.Errorf("failed to render text: %w", err)
		}
	}

	openCode, err := s.openCodeFn()
	if err != nil {
		return "", resolvedTo, fmt.Errorf("failed to generate open code: %w", err)
	}

	var rewrittenHTML string
	if renderedHTML != "" {
		makeCode := func(target string) (string, error) {
			var contactIDPtr *int64
			if in.ContactID > 0 {
				contactIDPtr = &in.ContactID
			}
			return s.tracking.CreateLink(ctx, target, in.CampaignID, contactIDPtr)
		}
		rewrittenHTML, err = template.RewriteLinks(renderedHTML, s.baseURL, makeCode)
		if err != nil {
			return "", resolvedTo, fmt.Errorf("failed to rewrite links: %w", err)
		}
		rewrittenHTML = template.InjectPixel(rewrittenHTML, s.baseURL, openCode)
	}

	// Compliance: build per-contact unsubscribe URL once. Used for both the
	// List-Unsubscribe headers (RFC 2369 / 8058) and the visible HTML footer.
	// Best-effort — a failure to mint the code must not block the send.
	var headers map[string]string
	if in.ContactID > 0 {
		if code, codeErr := s.ensureUnsubCode(ctx, in.ContactID); codeErr == nil && code != "" {
			unsubURL := strings.TrimSuffix(s.baseURL, "/") + "/u/" + code
			headers = map[string]string{
				"List-Unsubscribe":      "<" + unsubURL + ">",
				"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			}
			if rewrittenHTML != "" {
				rewrittenHTML = template.InjectUnsubscribeFooter(rewrittenHTML, s.baseURL, code)
			}
		}
	}

	sendErr := s.sender.Send(ctx, port.OutboundMessage{
		To:      resolvedTo,
		Subject: renderedSubject,
		HTML:    rewrittenHTML,
		Text:    renderedText,
		Headers: headers,
	})
	if sendErr != nil {
		// best-effort event insertion
		_ = s.events.Insert(ctx, domain.EmailEvent{
			ContactID:  in.ContactID,
			CampaignID: in.CampaignID,
			Type:       domain.EventFailed,
			Meta:       map[string]any{"error": sendErr.Error()},
			TS:         s.clock.Now(),
		})
		return "", resolvedTo, fmt.Errorf("send failed: %w", sendErr)
	}

	// log success event
	err = s.events.Insert(ctx, domain.EmailEvent{
		ContactID:  in.ContactID,
		CampaignID: in.CampaignID,
		Type:       domain.EventSent,
		LinkCode:   openCode,
		Meta:       map[string]any{"open_code": openCode},
		TS:         s.clock.Now(),
	})
	if err != nil {
		return "sent", resolvedTo, fmt.Errorf("failed to log sent event: %w", err)
	}

	return "sent", resolvedTo, nil
}

// SendToContact satisfies the CampaignMailer interface.
func (s *EmailService) SendToContact(ctx context.Context, c domain.Contact, templateID int64, campaignID int64) error {
	_, _, err := s.Send(ctx, SendInput{
		ContactID:  c.ID,
		CampaignID: &campaignID,
		To:         c.Email,
		TemplateID: templateID,
		Vars: map[string]any{
			"email":      c.Email,
			"first_name": c.FirstName,
			"last_name":  c.LastName,
			"company":    c.Company,
		},
	})
	return err
}

var _ CampaignMailer = (*EmailService)(nil) // compile-time assert

// ensureUnsubCode returns the contact's existing unsubscribe code or mints and
// persists a new one. Used to build the unsubscribe footer link.
func (s *EmailService) ensureUnsubCode(ctx context.Context, contactID int64) (string, error) {
	c, err := s.contacts.Get(ctx, contactID)
	if err != nil {
		return "", err
	}
	if c.UnsubCode != "" {
		return c.UnsubCode, nil
	}
	code, err := s.idgen.UnsubCode()
	if err != nil {
		return "", err
	}
	if err := s.contacts.SetUnsubCode(ctx, contactID, code); err != nil {
		return "", err
	}
	return code, nil
}
