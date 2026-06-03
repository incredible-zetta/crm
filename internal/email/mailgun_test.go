package email

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMailgunPostsCorrectRequest(t *testing.T) {
	domain := "sandbox-test.mailgun.org"
	apiKey := "key-abcdefg"

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		// Assert HTTP Method and Path
		if r.Method != "POST" {
			t.Errorf("expected Method POST, got %q", r.Method)
		}
		expectedPath := "/v3/" + domain + "/messages"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
		}

		// Assert Basic Auth
		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("expected Basic Auth header, but none was found")
		}
		if username != "api" {
			t.Errorf("expected Basic Auth username 'api', got %q", username)
		}
		if password != apiKey {
			t.Errorf("expected Basic Auth password %q, got %q", apiKey, password)
		}

		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatalf("failed to parse multipart form: %v", err)
		}

		// Assert Form fields
		from := r.FormValue("from")
		if from != "sender@test.com" {
			t.Errorf("expected form value 'from' = 'sender@test.com', got %q", from)
		}

		to := r.FormValue("to")
		if to != "recipient@test.com" {
			t.Errorf("expected form value 'to' = 'recipient@test.com', got %q", to)
		}

		subject := r.FormValue("subject")
		if subject != "Test Mailgun" {
			t.Errorf("expected form value 'subject' = 'Test Mailgun', got %q", subject)
		}

		text := r.FormValue("text")
		if text != "Mailgun text body" {
			t.Errorf("expected form value 'text' = 'Mailgun text body', got %q", text)
		}

		html := r.FormValue("html")
		if html != "<p>Mailgun HTML body</p>" {
			t.Errorf("expected form value 'html' = '<p>Mailgun HTML body</p>', got %q", html)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"<20260603.test@mailgun.org>", "message":"Queued. Thank you."}`))
	}))
	defer server.Close()

	sender := &MailgunSender{
		domain:  domain,
		apiKey:  apiKey,
		from:    "default@test.com",
		baseURL: server.URL,
		client:  server.Client(),
	}

	msg := Message{
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
		t.Error("expected server to be called, but it wasn't")
	}
}

func TestMailgunNon2xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Invalid recipient email address"))
	}))
	defer server.Close()

	sender := &MailgunSender{
		domain:  "test.domain",
		apiKey:  "key",
		from:    "default@test.com",
		baseURL: server.URL,
		client:  server.Client(),
	}

	msg := Message{
		To:      "recipient@test.com",
		Subject: "Subject",
		Text:    "Hello",
	}

	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for non-2xx response, got nil")
	}

	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected error message to contain status '400', got: %v", err)
	}
	if !strings.Contains(err.Error(), "Invalid recipient") {
		t.Errorf("expected error message to contain response body 'Invalid recipient', got: %v", err)
	}
}

func TestMailgunMissingTo(t *testing.T) {
	sender := &MailgunSender{
		domain:  "test.domain",
		apiKey:  "key",
		from:    "default@test.com",
		baseURL: "http://invalid-url-should-not-be-called",
		client:  http.DefaultClient,
	}

	msg := Message{
		To:      "",
		Subject: "Subject",
		Text:    "Hello",
	}

	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Error("expected error for empty To field, got nil")
	}
}

func TestMailgunMissingFrom(t *testing.T) {
	sender := &MailgunSender{
		domain:  "test.domain",
		apiKey:  "key",
		from:    "", // no default
		baseURL: "http://invalid-url-should-not-be-called",
		client:  http.DefaultClient,
	}

	msg := Message{
		To:      "recipient@test.com",
		From:    "", // no message from
		Subject: "Subject",
		Text:    "Hello",
	}

	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Error("expected error for empty resolved From, got nil")
	}
}

func TestMailgunMissingBody(t *testing.T) {
	sender := &MailgunSender{
		domain:  "test.domain",
		apiKey:  "key",
		from:    "default@test.com",
		baseURL: "http://invalid-url-should-not-be-called",
		client:  http.DefaultClient,
	}

	msg := Message{
		To:      "recipient@test.com",
		Subject: "Subject",
	}

	err := sender.Send(context.Background(), msg)
	if err == nil {
		t.Error("expected error for both html/text body empty, got nil")
	}
}

func TestMailgunCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not have been called for canceled context")
	}))
	defer server.Close()

	sender := &MailgunSender{
		domain:  "test.domain",
		apiKey:  "key",
		from:    "default@test.com",
		baseURL: server.URL,
		client:  server.Client(),
	}

	msg := Message{
		To:      "recipient@test.com",
		Subject: "Subject",
		Text:    "Hello",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := sender.Send(ctx, msg)
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}
