package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadRequiresAPIKey(t *testing.T) {
	os.Clearenv()
	err := func() error { _, e := Load(); return e }()
	if err == nil {
		t.Fatal("expected error when MCP_API_KEY missing")
	}
	if !strings.Contains(err.Error(), "MCP_API_KEY") {
		t.Fatalf("error should name MCP_API_KEY, got: %v", err)
	}
}

func TestLoadRequiresDSN(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("BASE_URL", "http://x")
	err := func() error { _, e := Load(); return e }()
	if err == nil {
		t.Fatal("expected error when DB_DSN missing")
	}
	if !strings.Contains(err.Error(), "DB_DSN") {
		t.Fatalf("error should name DB_DSN, got: %v", err)
	}
}

func TestLoadRequiresBaseURL(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	err := func() error { _, e := Load(); return e }()
	if err == nil {
		t.Fatal("expected error when BASE_URL missing")
	}
	if !strings.Contains(err.Error(), "BASE_URL") {
		t.Fatalf("error should name BASE_URL, got: %v", err)
	}
}

func TestLoadDebugLogLevel(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	os.Setenv("LOG_LEVEL", "debug")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.LogLevel != "debug" {
		t.Fatalf("want debug log level, got %q", c.LogLevel)
	}
	if !c.DebugEnabled() {
		t.Fatal("expected debug logging enabled")
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "8080" {
		t.Fatalf("want default port 8080, got %s", c.Port)
	}
	if c.SchedulerIntervalSec != 15 {
		t.Fatalf("want default 15, got %d", c.SchedulerIntervalSec)
	}
}

func TestLoadInvalidInterval(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	os.Setenv("SCHEDULER_INTERVAL_SEC", "notanumber")
	if _, err := Load(); err == nil {
		t.Fatal("expected error on invalid interval")
	}
}

func TestLoadAllValues(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "key1")
	os.Setenv("DB_DSN", "dsn1")
	os.Setenv("BASE_URL", "https://crm.test")
	os.Setenv("PORT", "9090")
	os.Setenv("SCHEDULER_INTERVAL_SEC", "30")
	os.Setenv("SMTP_HOST", "smtp.test")
	os.Setenv("SMTP_PORT", "587")
	os.Setenv("SMTP_USER", "u")
	os.Setenv("SMTP_PASS", "p")
	os.Setenv("SMTP_FROM", "from@test")
	os.Setenv("MAILGUN_DOMAIN", "mg.test")
	os.Setenv("MAILGUN_API_KEY", "mgkey")
	os.Setenv("LOG_LEVEL", "debug")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.MCPAPIKey != "key1" {
		t.Errorf("expected MCPAPIKey to be 'key1', got %q", c.MCPAPIKey)
	}
	if c.DBDSN != "dsn1" {
		t.Errorf("expected DBDSN to be 'dsn1', got %q", c.DBDSN)
	}
	if c.BaseURL != "https://crm.test" {
		t.Errorf("expected BaseURL to be 'https://crm.test', got %q", c.BaseURL)
	}
	if c.Port != "9090" {
		t.Errorf("expected Port to be '9090', got %q", c.Port)
	}
	if c.SchedulerIntervalSec != 30 {
		t.Errorf("expected SchedulerIntervalSec to be 30, got %d", c.SchedulerIntervalSec)
	}
	if c.SMTPHost != "smtp.test" {
		t.Errorf("expected SMTPHost to be 'smtp.test', got %q", c.SMTPHost)
	}
	if c.SMTPPort != "587" {
		t.Errorf("expected SMTPPort to be '587', got %q", c.SMTPPort)
	}
	if c.SMTPUser != "u" {
		t.Errorf("expected SMTPUser to be 'u', got %q", c.SMTPUser)
	}
	if c.SMTPPass != "p" {
		t.Errorf("expected SMTPPass to be 'p', got %q", c.SMTPPass)
	}
	if c.SMTPFrom != "from@test" {
		t.Errorf("expected SMTPFrom to be 'from@test', got %q", c.SMTPFrom)
	}
	if c.MailgunDomain != "mg.test" {
		t.Errorf("expected MailgunDomain to be 'mg.test', got %q", c.MailgunDomain)
	}
	if c.MailgunAPIKey != "mgkey" {
		t.Errorf("expected MailgunAPIKey to be 'mgkey', got %q", c.MailgunAPIKey)
	}
	if c.LogLevel != "debug" {
		t.Errorf("expected LogLevel to be 'debug', got %q", c.LogLevel)
	}
}
