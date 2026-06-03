package mcptools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cipta/crm-for-aiagents/internal/db"
	"github.com/cipta/crm-for-aiagents/internal/email"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EmailSendIn struct {
	ContactID  int64          `json:"contact_id" jsonschema:"ID of the contact. Required if To is empty."`
	To         string         `json:"to" jsonschema:"Recipient email address. Required if ContactID is 0."`
	CampaignID *int64         `json:"campaign_id" jsonschema:"Optional campaign ID association"`
	TemplateID int64          `json:"template_id" jsonschema:"ID of email template to use"`
	Subject    string         `json:"subject" jsonschema:"Subject of the email (used if TemplateID is 0)"`
	HTML       string         `json:"html" jsonschema:"HTML body (used if TemplateID is 0)"`
	Text       string         `json:"text" jsonschema:"Plain text body (used if TemplateID is 0)"`
	Vars       map[string]any `json:"vars" jsonschema:"Template variables to merge"`
}

type EmailSendOut struct {
	Status string `json:"status"`
	To     string `json:"to"`
}

func (d *Deps) EmailSend(ctx context.Context, req *mcp.CallToolRequest, in EmailSendIn) (*mcp.CallToolResult, EmailSendOut, error) {
	var resolvedTo string
	var resolvedContactID int64

	if in.ContactID > 0 {
		contact, err := d.Repo.GetContact(ctx, in.ContactID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return mcpserver.Err("not_found", "contact not found"), EmailSendOut{}, nil
			}
			return nil, EmailSendOut{}, fmt.Errorf("email_send load contact: %w", err)
		}
		resolvedTo = contact.Email
		resolvedContactID = in.ContactID

		if in.To != "" {
			if !strings.EqualFold(in.To, contact.Email) {
				return mcpserver.Err("recipient_mismatch", "to does not match contact email"), EmailSendOut{}, nil
			}
		}
	} else {
		if in.To == "" {
			return mcpserver.Err("missing_recipient", "either to or contact_id is required"), EmailSendOut{}, nil
		}
		resolvedTo = in.To
		resolvedContactID = 0
	}

	sendInput := email.SendInput{
		ContactID:  resolvedContactID,
		CampaignID: in.CampaignID,
		To:         resolvedTo,
		TemplateID: in.TemplateID,
		Subject:    in.Subject,
		HTML:       in.HTML,
		Text:       in.Text,
		Vars:       in.Vars,
	}

	if err := d.Pipeline.Send(ctx, sendInput); err != nil {
		return mcpserver.Err("send_failed", "email send failed"), EmailSendOut{}, nil
	}

	return nil, EmailSendOut{
		Status: "sent",
		To:     resolvedTo,
	}, nil
}
