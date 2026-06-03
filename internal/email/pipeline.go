package email

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/template"
)

type TemplateStore interface {
	GetTemplate(ctx context.Context, id int64) (TemplateData, error)
}

type TemplateData struct {
	ID       int64
	Subject  string
	BodyHTML string
	BodyText string
}

type LinkMaker interface {
	// CreateLink returns a short code for a target URL (records campaign/contact association).
	CreateLink(ctx context.Context, targetURL string, campaignID, contactID *int64) (string, error)
}

type EventLogger interface {
	LogEvent(ctx context.Context, contactID int64, campaignID *int64, eventType, linkCode string, meta map[string]any) error
}

type Pipeline struct {
	Sender   Sender
	Tmpl     TemplateStore
	Links    LinkMaker
	Events   EventLogger
	BaseURL  string
	OpenCode func() string // generates the open-pixel tracking code; injectable for tests, default random
}

type SendInput struct {
	ContactID  int64
	CampaignID *int64 // optional
	To         string // recipient email
	TemplateID int64  // if >0, load template; else use raw fields below
	Subject    string // raw (used if TemplateID==0)
	HTML       string
	Text       string
	Vars       map[string]any // template variables
}

func defaultOpenCode() string {
	b := make([]byte, 6)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		s := fmt.Sprintf("o%011x", time.Now().UnixNano())
		if len(s) < 12 {
			return (s + "000000000000")[:12]
		}
		return s[:12]
	}
	return hex.EncodeToString(b) // 12 chars
}

func (p *Pipeline) Send(ctx context.Context, in SendInput) error {
	if in.To == "" {
		return fmt.Errorf("recipient 'To' address is required")
	}

	var rawSubject, rawHTML, rawText string
	if in.TemplateID > 0 {
		if p.Tmpl == nil {
			return fmt.Errorf("template store is nil, cannot load template with id %d", in.TemplateID)
		}
		tmplData, err := p.Tmpl.GetTemplate(ctx, in.TemplateID)
		if err != nil {
			return fmt.Errorf("failed to load template %d: %w", in.TemplateID, err)
		}
		rawSubject = tmplData.Subject
		rawHTML = tmplData.BodyHTML
		rawText = tmplData.BodyText
	} else {
		rawSubject = in.Subject
		rawHTML = in.HTML
		rawText = in.Text
	}

	var renderedSubject, renderedHTML, renderedText string
	var err error
	if rawSubject != "" {
		renderedSubject, err = template.Render(rawSubject, in.Vars)
		if err != nil {
			return fmt.Errorf("failed to render subject: %w", err)
		}
	}
	if rawHTML != "" {
		renderedHTML, err = template.Render(rawHTML, in.Vars)
		if err != nil {
			return fmt.Errorf("failed to render HTML: %w", err)
		}
	}
	if rawText != "" {
		renderedText, err = template.Render(rawText, in.Vars)
		if err != nil {
			return fmt.Errorf("failed to render text: %w", err)
		}
	}

	openCodeFn := p.OpenCode
	if openCodeFn == nil {
		openCodeFn = defaultOpenCode
	}
	openCode := openCodeFn()

	var rewrittenHTML string
	if renderedHTML != "" {
		makeCode := func(target string) (string, error) {
			if p.Links == nil {
				return "", fmt.Errorf("links maker is nil")
			}
			return p.Links.CreateLink(ctx, target, in.CampaignID, &in.ContactID)
		}
		rewrittenHTML, err = template.RewriteLinks(renderedHTML, p.BaseURL, makeCode)
		if err != nil {
			return fmt.Errorf("failed to rewrite links: %w", err)
		}
		rewrittenHTML = template.InjectPixel(rewrittenHTML, p.BaseURL, openCode)
	}

	msg := Message{
		To:      in.To,
		Subject: renderedSubject,
		HTML:    rewrittenHTML,
		Text:    renderedText,
	}

	if p.Sender == nil {
		return fmt.Errorf("sender is nil")
	}

	sendErr := p.Sender.Send(ctx, msg)
	if sendErr != nil {
		if p.Events != nil {
			_ = p.Events.LogEvent(ctx, in.ContactID, in.CampaignID, "failed", "", map[string]any{"error": sendErr.Error()})
		}
		return fmt.Errorf("failed to send message: %w", sendErr)
	}

	if p.Events != nil {
		err = p.Events.LogEvent(ctx, in.ContactID, in.CampaignID, "sent", openCode, map[string]any{"open_code": openCode})
		if err != nil {
			return fmt.Errorf("failed to log sent event: %w", err)
		}
	}

	return nil
}
