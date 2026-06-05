package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

type AdminNotifier struct {
	sender port.EmailSender
	to     string
}

var _ port.AdminNotifier = (*AdminNotifier)(nil)

func NewAdminNotifier(sender port.EmailSender, to string) *AdminNotifier {
	return &AdminNotifier{sender: sender, to: to}
}

func (n *AdminNotifier) NotifyInboundMessage(ctx context.Context, msg domain.InboundMessage, contact domain.Contact) error {
	name := strings.TrimSpace(strings.TrimSpace(contact.FirstName + " " + contact.LastName))
	if name == "" {
		name = msg.FromEmail
	}
	subject := fmt.Sprintf("Zetta CRM: New reply from %s", name)
	body := fmt.Sprintf("New inbound reply from known contact.\n\nContact: %s <%s>\nCompany: %s\nMessage from: %s <%s>\nSubject: %s\nReceived: %s\n\nSnippet:\n%s\n\nSuggested MCP tools:\n- inbox_get {\"id\": %d}\n- inbox_reply {\"id\": %d, \"body_text\": \"...\"}\n- contact_get {\"id\": %d}\n",
		name,
		contact.Email,
		contact.Company,
		msg.FromName,
		msg.FromEmail,
		msg.Subject,
		msg.ReceivedAt.Format("2006-01-02 15:04:05 MST"),
		msg.Snippet,
		msg.ID,
		msg.ID,
		contact.ID,
	)
	return n.sender.Send(ctx, port.OutboundMessage{To: n.to, Subject: subject, Text: body})
}
