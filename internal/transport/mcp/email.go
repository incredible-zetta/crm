package mcptransport

import (
	"context"
	"errors"
	"strings"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/incredible-zetta/crm/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EmailSendIn struct {
	ContactID  int64          `json:"contact_id,omitempty" jsonschema:"ID of the contact. Required if To is empty."`
	To         string         `json:"to,omitempty" jsonschema:"Recipient email address. Required if ContactID is 0."`
	CampaignID *int64         `json:"campaign_id,omitempty" jsonschema:"Optional campaign ID association"`
	TemplateID int64          `json:"template_id,omitempty" jsonschema:"ID of email template to use"`
	Subject    string         `json:"subject,omitempty" jsonschema:"Subject of the email (used if TemplateID is 0)"`
	HTML       string         `json:"html,omitempty" jsonschema:"HTML body (used if TemplateID is 0)"`
	Text       string         `json:"text,omitempty" jsonschema:"Plain text body (used if TemplateID is 0)"`
	Vars       map[string]any `json:"vars,omitempty" jsonschema:"Template variables to merge"`
}

type EmailSendOut struct {
	Status string `json:"status"`
	To     string `json:"to"`
}

func (d *Deps) EmailSend(ctx context.Context, req *mcp.CallToolRequest, in EmailSendIn) (*mcp.CallToolResult, EmailSendOut, error) {
	status, to, err := d.Svc.Email.Send(ctx, service.SendInput{
		ContactID:  in.ContactID,
		CampaignID: in.CampaignID,
		To:         in.To,
		TemplateID: in.TemplateID,
		Subject:    in.Subject,
		HTML:       in.HTML,
		Text:       in.Text,
		Vars:       in.Vars,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "contact not found"), EmailSendOut{}, nil
		}
		if errors.Is(err, domain.ErrValidation) {
			msg := err.Error()
			msg = strings.TrimPrefix(msg, "validation error: ")
			return mcpserver.Err("invalid_input", msg), EmailSendOut{}, nil
		}
		return mcpserver.Err("send_failed", "email send failed"), EmailSendOut{}, nil
	}

	return nil, EmailSendOut{
		Status: status,
		To:     to,
	}, nil
}
