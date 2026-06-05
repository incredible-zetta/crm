package port

import "context"

// OutboundMessage is a fully-rendered email ready to send.
type OutboundMessage struct {
	To      string
	Subject string
	HTML    string
	Text    string
	From    string
	// Headers carries optional extra RFC 5322 headers (e.g. List-Unsubscribe).
	// Keys and values must not contain CR or LF.
	Headers map[string]string
}

// EmailSender defines the interface for delivering outbound emails.
type EmailSender interface {
	// Send sends a fully-rendered email message.
	Send(ctx context.Context, msg OutboundMessage) error
}
