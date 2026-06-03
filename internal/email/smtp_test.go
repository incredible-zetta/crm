package email

import (
	"context"
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
