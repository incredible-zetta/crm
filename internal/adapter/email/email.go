package email

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/port"
)

type Config struct {
	Provider                                         string // "smtp" | "mailgun"
	SMTPHost, SMTPPort, SMTPUser, SMTPPass, SMTPFrom string
	MailgunDomain, MailgunAPIKey                     string
	DefaultFrom                                      string
}

func hasHeaderInjection(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

// New returns a port.EmailSender for the configured provider.
func New(cfg Config) (port.EmailSender, error) {
	switch cfg.Provider {
	case "smtp":
		if cfg.SMTPHost == "" || cfg.SMTPPort == "" {
			return nil, errors.New("smtp host and port are required")
		}
		from := cfg.DefaultFrom
		if from == "" {
			from = cfg.SMTPFrom
		}
		var auth smtp.Auth
		if cfg.SMTPUser != "" {
			auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
		}
		return &smtpSender{
			addr:     cfg.SMTPHost + ":" + cfg.SMTPPort,
			auth:     auth,
			from:     from,
			sendFunc: smtp.SendMail,
		}, nil
	case "mailgun":
		if cfg.MailgunDomain == "" || cfg.MailgunAPIKey == "" {
			return nil, errors.New("mailgun domain and apiKey are required")
		}
		from := cfg.DefaultFrom
		if from == "" {
			from = cfg.SMTPFrom
		}
		return &mailgunSender{
			domain:  cfg.MailgunDomain,
			apiKey:  cfg.MailgunAPIKey,
			from:    from,
			baseURL: "https://api.mailgun.net/v3",
			client:  http.DefaultClient,
		}, nil
	default:
		return nil, errors.New("unknown provider: " + cfg.Provider)
	}
}

type smtpSender struct {
	addr     string
	auth     smtp.Auth
	from     string
	sendFunc func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

func (s *smtpSender) Send(ctx context.Context, m port.OutboundMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Validate fields
	if m.To == "" {
		return errors.New("recipient address (To) is required")
	}

	from := m.From
	if from == "" {
		from = s.from
	}
	if from == "" {
		return errors.New("sender address (From) is required")
	}

	if hasHeaderInjection(m.To) || hasHeaderInjection(from) || hasHeaderInjection(m.Subject) {
		return fmt.Errorf("invalid header value (contains newline)")
	}
	for k, v := range m.Headers {
		if hasHeaderInjection(k) || hasHeaderInjection(v) {
			return fmt.Errorf("invalid extra header value (contains newline)")
		}
	}

	if m.Text == "" && m.HTML == "" {
		return errors.New("at least one of HTML or Text body must be provided")
	}

	// Format RFC 5322 / MIME message
	var buf bytes.Buffer

	// Standard Headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", m.To))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", m.Subject))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	for k, v := range m.Headers {
		buf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	buf.WriteString("MIME-Version: 1.0\r\n")

	if m.Text != "" && m.HTML != "" {
		// multipart/alternative
		mpWriter := multipart.NewWriter(&buf)
		boundary := mpWriter.Boundary()
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary))

		// Part 1: text/plain
		partTextHeaders := make(textproto.MIMEHeader)
		partTextHeaders.Set("Content-Type", "text/plain; charset=UTF-8")
		partTextHeaders.Set("Content-Transfer-Encoding", "7bit")
		partText, err := mpWriter.CreatePart(partTextHeaders)
		if err != nil {
			return fmt.Errorf("failed to create text part: %w", err)
		}
		if _, err := partText.Write([]byte(m.Text)); err != nil {
			return fmt.Errorf("failed to write text part: %w", err)
		}

		// Part 2: text/html
		partHTMLHeaders := make(textproto.MIMEHeader)
		partHTMLHeaders.Set("Content-Type", "text/html; charset=UTF-8")
		partHTMLHeaders.Set("Content-Transfer-Encoding", "7bit")
		partHTML, err := mpWriter.CreatePart(partHTMLHeaders)
		if err != nil {
			return fmt.Errorf("failed to create html part: %w", err)
		}
		if _, err := partHTML.Write([]byte(m.HTML)); err != nil {
			return fmt.Errorf("failed to write html part: %w", err)
		}

		if err := mpWriter.Close(); err != nil {
			return fmt.Errorf("failed to close multipart writer: %w", err)
		}
	} else if m.Text != "" {
		// text/plain
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		buf.WriteString(m.Text)
	} else {
		// text/html
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		buf.WriteString(m.HTML)
	}

	// Again, check context before the network call
	if err := ctx.Err(); err != nil {
		return err
	}

	sendFn := s.sendFunc
	if sendFn == nil {
		sendFn = smtp.SendMail
	}

	to := []string{m.To}
	if err := sendFn(s.addr, s.auth, from, to, buf.Bytes()); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

type mailgunSender struct {
	domain, apiKey, from, baseURL string
	client                        *http.Client
}

func (m *mailgunSender) Send(ctx context.Context, msg port.OutboundMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Validate fields
	if msg.To == "" {
		return errors.New("recipient address (To) is required")
	}

	from := msg.From
	if from == "" {
		from = m.from
	}
	if from == "" {
		return errors.New("sender address (From) is required")
	}

	if hasHeaderInjection(msg.To) || hasHeaderInjection(from) || hasHeaderInjection(msg.Subject) {
		return fmt.Errorf("invalid header value (contains newline)")
	}
	for k, v := range msg.Headers {
		if hasHeaderInjection(k) || hasHeaderInjection(v) {
			return fmt.Errorf("invalid extra header value (contains newline)")
		}
	}

	if msg.Text == "" && msg.HTML == "" {
		return errors.New("at least one of HTML or Text body must be provided")
	}

	// Construct multipart/form-data request body
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("from", from); err != nil {
		return fmt.Errorf("failed to write form field from: %w", err)
	}
	if err := writer.WriteField("to", msg.To); err != nil {
		return fmt.Errorf("failed to write form field to: %w", err)
	}
	if err := writer.WriteField("subject", msg.Subject); err != nil {
		return fmt.Errorf("failed to write form field subject: %w", err)
	}
	if msg.Text != "" {
		if err := writer.WriteField("text", msg.Text); err != nil {
			return fmt.Errorf("failed to write form field text: %w", err)
		}
	}
	if msg.HTML != "" {
		if err := writer.WriteField("html", msg.HTML); err != nil {
			return fmt.Errorf("failed to write form field html: %w", err)
		}
	}
	for k, v := range msg.Headers {
		if err := writer.WriteField("h:"+k, v); err != nil {
			return fmt.Errorf("failed to write custom header %s: %w", k, err)
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Build HTTP request
	url := fmt.Sprintf("%s/%s/messages", m.baseURL, m.domain)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &body)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetBasicAuth("api", m.apiKey)

	client := m.client
	if client == nil {
		client = http.DefaultClient
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read a snippet of the body for error reporting
		limitReader := io.LimitReader(resp.Body, 512)
		bodyBytes, _ := io.ReadAll(limitReader)
		bodySnippet := string(bodyBytes)
		if bodySnippet == "" {
			bodySnippet = "(empty response body)"
		} else {
			bodySnippet = strings.TrimSpace(bodySnippet)
		}
		return fmt.Errorf("mailgun API returned status %d %s: %s", resp.StatusCode, resp.Status, bodySnippet)
	}

	return nil
}

var _ port.EmailSender = (*smtpSender)(nil)
var _ port.EmailSender = (*mailgunSender)(nil)
