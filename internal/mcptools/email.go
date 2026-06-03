package mcptools

import (
	"context"
	"fmt"

	"github.com/cipta/crm-for-aiagents/internal/email"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EmailSendIn struct {
	ContactID  int64          `json:"contact_id" jsonschema:"description=ID of the contact. Required if To is empty."`
	To         string         `json:"to" jsonschema:"description=Recipient email address. Required if ContactID is 0."`
	CampaignID *int64         `json:"campaign_id" jsonschema:"description=Optional campaign ID association"`
	TemplateID int64          `json:"template_id" jsonschema:"description=ID of email template to use"`
	Subject    string         `json:"subject" jsonschema:"description=Subject of the email (used if TemplateID is 0)"`
	HTML       string         `json:"html" jsonschema:"description=HTML body (used if TemplateID is 0)"`
	Text       string         `json:"text" jsonschema:"description=Plain text body (used if TemplateID is 0)"`
	Vars       map[string]any `json:"vars" jsonschema:"description=Template variables to merge"`
}

type EmailSendOut struct {
	Status string `json:"status"`
	To     string `json:"to"`
}

func (d *Deps) EmailSend(ctx context.Context, req *mcp.CallToolRequest, in EmailSendIn) (*mcp.CallToolResult, EmailSendOut, error) {
	var resolvedTo = in.To
	var resolvedContactID = in.ContactID

	if resolvedTo == "" && resolvedContactID > 0 {
		contact, err := d.Repo.GetContact(ctx, resolvedContactID)
		if err != nil {
			return mcpserver.Err("recipient_not_found", fmt.Sprintf("contact %d not found", resolvedContactID)), EmailSendOut{}, nil
		}
		resolvedTo = contact.Email
	}

	if resolvedTo == "" {
		return mcpserver.Err("missing_recipient", "either to or contact_id is required"), EmailSendOut{}, nil
	}

	if resolvedContactID == 0 {
		contact, err := d.Repo.GetContactByEmail(ctx, resolvedTo)
		if err == nil {
			resolvedContactID = contact.ID
		}
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
		return mcpserver.Err("send_failed", err.Error()), EmailSendOut{}, nil
	}

	return nil, EmailSendOut{
		Status: "sent",
		To:     resolvedTo,
	}, nil
}
