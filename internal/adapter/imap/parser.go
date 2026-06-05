package imap

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
)

// ParseMessage converts raw RFC 5322 email into an inbound domain message.
func ParseMessage(r io.Reader) (domain.InboundMessage, error) {
	mr, err := mail.ReadMessage(r)
	if err != nil {
		return domain.InboundMessage{}, err
	}

	msg := domain.InboundMessage{
		Subject:          mr.Header.Get("Subject"),
		MessageID:        strings.TrimSpace(mr.Header.Get("Message-Id")),
		InReplyTo:        strings.TrimSpace(mr.Header.Get("In-Reply-To")),
		ReferencesHeader: strings.TrimSpace(mr.Header.Get("References")),
		ReceivedAt:       time.Now().UTC(),
	}
	if date, err := mr.Header.Date(); err == nil {
		msg.ReceivedAt = date.UTC()
	}
	if from, err := mail.ParseAddress(mr.Header.Get("From")); err == nil {
		msg.FromName = from.Name
		msg.FromEmail = strings.ToLower(strings.TrimSpace(from.Address))
	}
	if to, err := mail.ParseAddress(mr.Header.Get("To")); err == nil {
		msg.ToEmail = strings.ToLower(strings.TrimSpace(to.Address))
	}

	headers := map[string][]string(mr.Header)
	if data, err := json.Marshal(headers); err == nil {
		msg.RawHeadersJSON = string(data)
	}

	body, _ := io.ReadAll(mr.Body)
	mediaType, params, err := mime.ParseMediaType(mr.Header.Get("Content-Type"))
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		parseMultipart(&msg, body, params["boundary"])
	} else if strings.EqualFold(mediaType, "text/html") {
		msg.HTMLBody = strings.TrimSpace(string(body))
	} else {
		msg.TextBody = strings.TrimSpace(string(body))
	}
	if msg.Snippet == "" {
		msg.Snippet = makeSnippet(msg.TextBody)
	}
	if msg.Snippet == "" {
		msg.Snippet = makeSnippet(stripHTML(msg.HTMLBody))
	}
	return msg, nil
}

func parseMultipart(msg *domain.InboundMessage, body []byte, boundary string) {
	if boundary == "" {
		return
	}
	reader := multipart.NewReader(strings.NewReader(string(body)), boundary)
	for {
		part, err := reader.NextPart()
		if err != nil {
			return
		}
		content, _ := io.ReadAll(part)
		mediaType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		switch strings.ToLower(mediaType) {
		case "text/plain":
			if msg.TextBody == "" {
				msg.TextBody = strings.TrimSpace(string(content))
			}
		case "text/html":
			if msg.HTMLBody == "" {
				msg.HTMLBody = strings.TrimSpace(string(content))
			}
		}
	}
}

func makeSnippet(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= 240 {
		return s
	}
	return s[:240]
}

var tagRE = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	return strings.TrimSpace(tagRE.ReplaceAllString(s, " "))
}
