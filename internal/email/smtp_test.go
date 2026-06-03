package email

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"strings"
	"testing"
)

func TestSMTPBuildsMessage(t *testing.T) {
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

	sender := &SMTPSender{
		addr:     "smtp.test.com:587",
		from:     "default@test.com",
		sendFunc: fakeSendFunc,
	}

	msg := Message{
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

	// Fix 6: Deepen SMTP MIME tests
	// 1. Assert CRLF line endings
	if !bytes.Contains(capturedMsg, []byte("\r\n")) {
		t.Error("expected message to contain CRLF line endings, but it doesn't")
	}

	// 2. Parse headers with net/mail
	parsed, err := mail.ReadMessage(bytes.NewReader(capturedMsg))
	if err != nil {
		t.Fatalf("failed to parse captured message with net/mail: %v", err)
	}
	if parsed.Header.Get("From") != "sender@example.com" {
		t.Errorf("expected parsed From header %q, got %q", "sender@example.com", parsed.Header.Get("From"))
	}
	if parsed.Header.Get("To") != "recipient@example.com" {
		t.Errorf("expected parsed To header %q, got %q", "recipient@example.com", parsed.Header.Get("To"))
	}
	if parsed.Header.Get("Subject") != "Test Multipart Message" {
		t.Errorf("expected parsed Subject header %q, got %q", "Test Multipart Message", parsed.Header.Get("Subject"))
	}

	// 3. Parse MediaType
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

	// 4. Assert 2 parts (text/plain and text/html)
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
		if partCount == 1 {
			if partContentType != "text/plain" {
				t.Errorf("expected first part to be text/plain, got %q", partContentType)
			}
		} else if partCount == 2 {
			if partContentType != "text/html" {
				t.Errorf("expected second part to be text/html, got %q", partContentType)
			}
		}
	}
	if partCount != 2 {
		t.Errorf("expected exactly 2 parts, got %d", partCount)
	}
}

func TestSMTPTextOnly(t *testing.T) {
	var capturedMsg []byte
	var callCount int

	fakeSendFunc := func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		capturedMsg = msg
		callCount++
		return nil
	}

	sender := &SMTPSender{
		addr:     "smtp.test.com:587",
		from:     "default@test.com",
		sendFunc: fakeSendFunc,
	}

	msg := Message{
		To:      "recipient@example.com",
		Subject: "Test Text-Only",
		Text:    "Just plain text.",
	}

	ctx := context.Background()
	err := sender.Send(ctx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("expected sendFunc to be called once")
	}

	msgStr := string(capturedMsg)
	if !strings.Contains(msgStr, "Content-Type: text/plain;") {
		t.Errorf("expected Content-Type text/plain, got headers in message:\n%s", msgStr)
	}
	if !strings.Contains(msgStr, "Just plain text.") {
		t.Errorf("expected plain text content not found in:\n%s", msgStr)
	}
	if strings.Contains(msgStr, "multipart") {
		t.Errorf("expected plain message, found multipart in:\n%s", msgStr)
	}
}

func TestSMTPHTMLOnly(t *testing.T) {
	var capturedMsg []byte
	var callCount int

	fakeSendFunc := func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		capturedMsg = msg
		callCount++
		return nil
	}

	sender := &SMTPSender{
		addr:     "smtp.test.com:587",
		from:     "default@test.com",
		sendFunc: fakeSendFunc,
	}

	msg := Message{
		To:      "recipient@example.com",
		Subject: "Test HTML-Only",
		HTML:    "<h1>Just HTML</h1>",
	}

	ctx := context.Background()
	err := sender.Send(ctx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("expected sendFunc to be called once")
	}

	msgStr := string(capturedMsg)
	if !strings.Contains(msgStr, "Content-Type: text/html;") {
		t.Errorf("expected Content-Type text/html, got:\n%s", msgStr)
	}
	if !strings.Contains(msgStr, "<h1>Just HTML</h1>") {
		t.Errorf("expected HTML content not found in:\n%s", msgStr)
	}
}

func TestSMTPMissingTo(t *testing.T) {
	sender := &SMTPSender{
		addr: "smtp.test.com:587",
		from: "default@test.com",
		sendFunc: func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			t.Fatal("sendFunc should not have been called")
			return nil
		},
	}

	msg := Message{
		To:      "",
		Subject: "Missing To",
		Text:    "Hello",
	}

	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Error("expected error for empty To field, got nil")
	}
}

func TestSMTPMissingFrom(t *testing.T) {
	sender := &SMTPSender{
		addr: "smtp.test.com:587",
		from: "", // no default from
		sendFunc: func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			t.Fatal("sendFunc should not have been called")
			return nil
		},
	}

	msg := Message{
		To:      "recipient@example.com",
		From:    "", // no message from
		Subject: "Missing From",
		Text:    "Hello",
	}

	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Error("expected error for empty From resolved, got nil")
	}
}

func TestSMTPMissingBody(t *testing.T) {
	sender := &SMTPSender{
		addr: "smtp.test.com:587",
		from: "default@test.com",
		sendFunc: func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			t.Fatal("sendFunc should not have been called")
			return nil
		},
	}

	msg := Message{
		To:      "recipient@example.com",
		Subject: "Missing Body",
	}

	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Error("expected error for empty body fields, got nil")
	}
}

func TestSMTPContextCanceled(t *testing.T) {
	sender := &SMTPSender{
		addr: "smtp.test.com:587",
		from: "default@test.com",
		sendFunc: func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			t.Fatal("sendFunc should not have been called with canceled context")
			return nil
		},
	}

	msg := Message{
		To:      "recipient@example.com",
		Subject: "Canceled context",
		Text:    "Hello",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := sender.Send(ctx, msg)
	if err == nil {
		t.Error("expected context canceled error, got nil")
	}
}

func TestSMTPHeaderInjection(t *testing.T) {
	callCount := 0
	sender := &SMTPSender{
		addr: "smtp.test.com:587",
		from: "default@test.com",
		sendFunc: func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			callCount++
			return nil
		},
	}

	// Subject with CRLF
	msg := Message{
		To:      "recipient@example.com",
		Subject: "ok\r\nBcc: evil@x",
		Text:    "Hello",
	}
	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Error("expected error for subject header injection, got nil")
	} else if !strings.Contains(err.Error(), "invalid header value") {
		t.Errorf("expected 'invalid header value' error, got %v", err)
	}

	// To with newline
	msgTo := Message{
		To:      "recipient@example.com\nBcc: evil@x",
		Subject: "Subject",
		Text:    "Hello",
	}
	err = sender.Send(context.Background(), msgTo)
	if err == nil {
		t.Error("expected error for To header injection, got nil")
	}

	if callCount > 0 {
		t.Error("expected sendFunc NOT to be called in case of header injection")
	}
}
