package email

import (
	"testing"
)

func TestNewUnknownProvider(t *testing.T) {
	cfg := Config{Provider: "unknown"}
	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for unknown provider, got nil")
	}
}

func TestNewSMTP(t *testing.T) {
	// Success case
	cfg := Config{
		Provider: "smtp",
		SMTPHost: "localhost",
		SMTPPort: "25",
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := s.(*SMTPSender); !ok {
		t.Errorf("expected *SMTPSender, got %T", s)
	}

	// Missing required fields (host or port)
	cfgMissingHost := Config{
		Provider: "smtp",
		SMTPPort: "25",
	}
	_, err = New(cfgMissingHost)
	if err == nil {
		t.Error("expected error for missing SMTPHost, got nil")
	}

	cfgMissingPort := Config{
		Provider: "smtp",
		SMTPHost: "localhost",
	}
	_, err = New(cfgMissingPort)
	if err == nil {
		t.Error("expected error for missing SMTPPort, got nil")
	}
}

func TestNewMailgun(t *testing.T) {
	// Success case
	cfg := Config{
		Provider:      "mailgun",
		MailgunDomain: "example.com",
		MailgunAPIKey: "key-12345",
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := s.(*MailgunSender); !ok {
		t.Errorf("expected *MailgunSender, got %T", s)
	}

	// Missing required fields
	cfgMissingDomain := Config{
		Provider:      "mailgun",
		MailgunAPIKey: "key-12345",
	}
	_, err = New(cfgMissingDomain)
	if err == nil {
		t.Error("expected error for missing MailgunDomain, got nil")
	}

	cfgMissingKey := Config{
		Provider:      "mailgun",
		MailgunDomain: "example.com",
	}
	_, err = New(cfgMissingKey)
	if err == nil {
		t.Error("expected error for missing MailgunAPIKey, got nil")
	}
}

func TestMailgunDefaultURL(t *testing.T) {
	cfg := Config{
		Provider:      "mailgun",
		MailgunDomain: "d",
		MailgunAPIKey: "k",
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ms, ok := s.(*MailgunSender)
	if !ok {
		t.Fatalf("expected *MailgunSender, got %T", s)
	}
	if ms.baseURL != "https://api.mailgun.net/v3" {
		t.Errorf("expected baseURL 'https://api.mailgun.net/v3', got %q", ms.baseURL)
	}
}
