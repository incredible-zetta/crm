package email

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/smtp"
	"strings"
	"testing"

	"github.com/incredible-zetta/crm/internal/port"
)

func TestNewSMTP(t *testing.T) {
	// 1. Unknown provider
	cfgErr := Config{Provider: "unknown"}
	_, err := New(cfgErr)
	if err == nil {
		t.Error("expected error for unknown provider, got nil")
	}

	// 2. Missing smtp fields
	cfgMissing := Config{Provider: "smtp", SMTPHost: ""}
	_, err = New(cfgMissing)
	if err == nil {
		t.Error("expected error for missing SMTPHost, got nil")
	}

	// 3. Valid config
	cfgValid := Config{
		Provider: "smtp",
		SMTPHost: "smtp.example.com",
		SMTPPort: "587",
		SMTPFrom: "default@example.com",
	}
	sender, err := New(cfgValid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender == nil {
		t.Fatal("expected non-nil sender for valid config")
	}
}

func TestNewMailgun(t *testing.T) {
	// 1. Missing mailgun fields
	cfgMissing := Config{Provider: "mailgun", MailgunDomain: ""}
	_, err := New(cfgMissing)
	if err == nil {
		t.Error("expected error for missing MailgunDomain, got nil")
	}

	// 2. Valid config
	cfgValid := Config{
		Provider:      "mailgun",
		MailgunDomain: "example.com",
		MailgunAPIKey: "key-123",
		DefaultFrom:   "sender@example.com",
	}
	sender, err := New(cfgValid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender == nil {
		t.Fatal("expected non-nil sender for valid config")
	}
}

func TestSMTPSenderSend(t *testing.T) {
	var capturedAddr string
	var capturedAuth smtp.Auth
	var capturedFrom string
	var capturedTo []string
	var capturedMsg []byte
	var callCount int

	fakeSendFunc := func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		capturedAddr = addr
		capturedAuth = a
		capturedFrom = from
		capturedTo = to
		capturedMsg = msg
		callCount++
		return nil
	}

	sender := &smtpSender{
		addr:     "smtp.test.com:587",
		from:     "default@test.com",
		sendFunc: fakeSendFunc,
	}

	// Test case 1: normal send with text and html, explicit From
	msg := port.OutboundMessage{
		To:      "recipient@example.com",
		From:    "sender@example.com",
		Subject: "Test Multipart Message",
		Text:    "This is plain text.",
		HTML:    "<p>This is HTML.</p>",
	}

	ctx := context.Background()
	err := sender.Send(ctx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("expected sendFunc to be called once, called %d times", callCount)
	}

	if capturedAddr != "smtp.test.com:587" {
		t.Errorf("expected addr 'smtp.test.com:587', got %q", capturedAddr)
	}

	if capturedAuth != nil {
		t.Errorf("expected nil auth, got %v", capturedAuth)
	}

	if capturedFrom != "sender@example.com" {
		t.Errorf("expected from 'sender@example.com', got %q", capturedFrom)
	}

	if len(capturedTo) != 1 || capturedTo[0] != "recipient@example.com" {
		t.Errorf("expected to ['recipient@example.com'], got %v", capturedTo)
	}

	msgStr := string(capturedMsg)
	requiredHeaders := []string{
		"To: recipient@example.com",
		"From: sender@example.com",
		"Subject: Test Multipart Message",
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative;",
		"This is plain text.",
		"<p>This is HTML.</p>",
	}

	for _, header := range requiredHeaders {
		if !strings.Contains(msgStr, header) {
			t.Errorf("expected message to contain %q, but it didn't.\nFull message:\n%s", header, msgStr)
		}
	}

	if !bytes.Contains(capturedMsg, []byte("\r\n")) {
		t.Error("expected message to contain CRLF line endings, but it doesn't")
	}

	// Parse with net/mail
	parsed, err := mail.ReadMessage(bytes.NewReader(capturedMsg))
	if err != nil {
		t.Fatalf("failed to parse captured message: %v", err)
	}
	if parsed.Header.Get("From") != "sender@example.com" {
		t.Errorf("expected From header %q, got %q", "sender@example.com", parsed.Header.Get("From"))
	}

	// 2. Parse MediaType
	contentType := parsed.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed to parse Content-Type: %v", err)
	}
	if mediaType != "multipart/alternative" {
		t.Errorf("expected mediaType 'multipart/alternative', got %s", mediaType)
	}
	boundary, ok := params["boundary"]
	if !ok {
		t.Fatal("missing boundary in Content-Type")
	}

	mr := multipart.NewReader(parsed.Body, boundary)
	partCount := 0
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read next part: %v", err)
		}
		partCount++
		partContentType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("failed to parse part Content-Type: %v", err)
		}
		if partCount == 1 && partContentType != "text/plain" {
			t.Errorf("expected first part to be text/plain, got %q", partContentType)
		}
		if partCount == 2 && partContentType != "text/html" {
			t.Errorf("expected second part to be text/html, got %q", partContentType)
		}
	}
	if partCount != 2 {
		t.Errorf("expected exactly 2 parts, got %d", partCount)
	}

	// Test case 2: From fallback when msg.From is empty
	msgEmptyFrom := port.OutboundMessage{
		To:      "recipient@example.com",
		Subject: "Fallback Subject",
		Text:    "Hello fallbacks!",
	}
	callCount = 0
	err = sender.Send(ctx, msgEmptyFrom)
	if err != nil {
		t.Fatalf("unexpected error on fallback send: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected sendFunc to be called once for fallback send")
	}
	if capturedFrom != "default@test.com" {
		t.Errorf("expected from to fall back to 'default@test.com', got %q", capturedFrom)
	}
}

func TestSMTPHeaderInjectionRejected(t *testing.T) {
	callCount := 0
	fakeSendFunc := func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		callCount++
		return nil
	}

	sender := &smtpSender{
		addr:     "smtp.test.com:587",
		from:     "default@test.com",
		sendFunc: fakeSendFunc,
	}

	// Subject containing \n
	msgBadSubject := port.OutboundMessage{
		To:      "recipient@example.com",
		Subject: "Bad\nSubject",
		Text:    "Hello",
	}

	err := sender.Send(context.Background(), msgBadSubject)
	if err == nil {
		t.Error("expected error for header injection in subject, got nil")
	}
	if !strings.Contains(err.Error(), "invalid header value") {
		t.Errorf("expected error message to mention 'invalid header value', got: %v", err)
	}

	// To containing \r
	msgBadTo := port.OutboundMessage{
		To:      "recipient@example.com\rBcc: evil@example.com",
		Subject: "Subject",
		Text:    "Hello",
	}

	err = sender.Send(context.Background(), msgBadTo)
	if err == nil {
		t.Error("expected error for header injection in To, got nil")
	}

	if callCount > 0 {
		t.Error("expected sendFunc NOT to be called in case of header injection")
	}
}

func TestMailgunSenderSend(t *testing.T) {
	domain := "sandbox-test.mailgun.org"
	apiKey := "key-abcdefg"

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		if r.Method != "POST" {
			t.Errorf("expected Method POST, got %q", r.Method)
		}
		expectedPath := "/" + domain + "/messages"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
		}

		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("expected Basic Auth header")
		}
		if username != "api" || password != apiKey {
			t.Errorf("expected api:%s, got %s:%s", apiKey, username, password)
		}

		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatalf("failed to parse multipart form: %v", err)
		}

		if r.FormValue("from") != "sender@test.com" {
			t.Errorf("expected from 'sender@test.com', got %q", r.FormValue("from"))
		}
		if r.FormValue("to") != "recipient@test.com" {
			t.Errorf("expected to 'recipient@test.com', got %q", r.FormValue("to"))
		}
		if r.FormValue("subject") != "Test Mailgun" {
			t.Errorf("expected subject 'Test Mailgun', got %q", r.FormValue("subject"))
		}
		if r.FormValue("text") != "Mailgun text body" {
			t.Errorf("expected text 'Mailgun text body', got %q", r.FormValue("text"))
		}
		if r.FormValue("html") != "<p>Mailgun HTML body</p>" {
			t.Errorf("expected html '<p>Mailgun HTML body</p>', got %q", r.FormValue("html"))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"<123>", "message":"Queued."}`))
	}))
	defer server.Close()

	sender := &mailgunSender{
		domain:  domain,
		apiKey:  apiKey,
		from:    "default@test.com",
		baseURL: server.URL,
		client:  server.Client(),
	}

	msg := port.OutboundMessage{
		To:      "recipient@test.com",
		From:    "sender@test.com",
		Subject: "Test Mailgun",
		Text:    "Mailgun text body",
		HTML:    "<p>Mailgun HTML body</p>",
	}

	err := sender.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Error("expected server to be called")
	}

	// Test case: non-2xx status code
	serverNon2xx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Invalid address"))
	}))
	defer serverNon2xx.Close()

	senderNon2xx := &mailgunSender{
		domain:  domain,
		apiKey:  apiKey,
		from:    "default@test.com",
		baseURL: serverNon2xx.URL,
		client:  serverNon2xx.Client(),
	}

	err = senderNon2xx.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error on 400 Bad Request, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid address") {
		t.Errorf("expected error message to contain body, got: %v", err)
	}
}
