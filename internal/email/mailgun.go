package email

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

type MailgunSender struct {
	domain, apiKey, from string
	baseURL              string       // default "https://api.mailgun.net/v3"; tests override to httptest server URL
	client               *http.Client // default http.DefaultClient
}

func (m *MailgunSender) Send(ctx context.Context, msg Message) error {
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
