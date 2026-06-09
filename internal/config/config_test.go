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

func TestLoadInboxDefaultsDisabled(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.IMAPPort != "993" {
		t.Fatalf("want default IMAP port 993, got %q", c.IMAPPort)
	}
	if c.IMAPMailbox != "INBOX" {
		t.Fatalf("want default IMAP mailbox INBOX, got %q", c.IMAPMailbox)
	}
	if c.IMAPPollIntervalSec != 60 {
		t.Fatalf("want default IMAP poll interval 60, got %d", c.IMAPPollIntervalSec)
	}
	if c.IMAPSinceDays != 14 {
		t.Fatalf("want default IMAP since days 14, got %d", c.IMAPSinceDays)
	}
	if c.InboxEnabled() {
		t.Fatal("expected inbox disabled when IMAP/admin env missing")
	}
}

func TestLoadInboxEnabled(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	os.Setenv("IMAP_HOST", "imap.test")
	os.Setenv("IMAP_PORT", "1993")
	os.Setenv("IMAP_USER", "inbox@test")
	os.Setenv("IMAP_PASS", "secret")
	os.Setenv("IMAP_MAILBOX", "Leads")
	os.Setenv("IMAP_POLL_INTERVAL_SEC", "120")
	os.Setenv("IMAP_SINCE_DAYS", "30")
	os.Setenv("ADMIN_NOTIFY_EMAIL", "admin@test")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.InboxEnabled() {
		t.Fatal("expected inbox enabled")
	}
	if c.IMAPHost != "imap.test" || c.IMAPPort != "1993" || c.IMAPUser != "inbox@test" || c.IMAPPass != "secret" || c.IMAPMailbox != "Leads" || c.AdminNotifyEmail != "admin@test" {
		t.Fatalf("unexpected inbox config: %+v", c)
	}
	if c.IMAPPollIntervalSec != 120 || c.IMAPSinceDays != 30 {
		t.Fatalf("unexpected inbox intervals: poll=%d since=%d", c.IMAPPollIntervalSec, c.IMAPSinceDays)
	}
}

func TestLoadInvalidIMAPNumbers(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	os.Setenv("IMAP_POLL_INTERVAL_SEC", "bad")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "IMAP_POLL_INTERVAL_SEC") {
		t.Fatalf("expected invalid IMAP_POLL_INTERVAL_SEC error, got %v", err)
	}

	os.Setenv("IMAP_POLL_INTERVAL_SEC", "60")
	os.Setenv("IMAP_SINCE_DAYS", "bad")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "IMAP_SINCE_DAYS") {
		t.Fatalf("expected invalid IMAP_SINCE_DAYS error, got %v", err)
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
	os.Setenv("IMAP_HOST", "imap.test")
	os.Setenv("IMAP_USER", "inbox@test")
	os.Setenv("IMAP_PASS", "imap-pass")
	os.Setenv("ADMIN_NOTIFY_EMAIL", "admin@test")
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

func TestLoadWhatsAppDefaultsDisabled(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.WhatsAppEnabled() {
		t.Fatal("expected WhatsApp disabled when WA_BASE_URL/WA_DEVICE_ID missing")
	}
	if c.WABlockUnregistered {
		t.Fatal("WA_BLOCK_UNREGISTERED_SEND should default false")
	}
	if c.WASendMax != 0 || c.WASendDailyCap != 0 || c.WAWarmupPerDay != 0 {
		t.Fatalf("smart-send caps should default 0: %+v", c)
	}
}

func TestLoadWhatsAppEnabled(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	os.Setenv("WA_BASE_URL", "https://wa.test")
	os.Setenv("WA_BASIC_AUTH", "dXNlcjpwYXNz")
	os.Setenv("WA_DEVICE_ID", "cds")
	os.Setenv("WA_WEBHOOK_SECRET", "whsec")
	os.Setenv("WA_SEND_MAX", "10")
	os.Setenv("WA_SEND_WINDOW_SEC", "60")
	os.Setenv("WA_SEND_DAILY_CAP", "5")
	os.Setenv("WA_SEND_JITTER_MIN_MS", "1000")
	os.Setenv("WA_SEND_JITTER_MAX_MS", "3000")
	os.Setenv("WA_WARMUP_PER_DAY", "50")
	os.Setenv("WA_BLOCK_UNREGISTERED_SEND", "true")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.WhatsAppEnabled() {
		t.Fatal("expected WhatsApp enabled")
	}
	if c.WABaseURL != "https://wa.test" || c.WADeviceID != "cds" || c.WABasicAuth != "dXNlcjpwYXNz" || c.WAWebhookSecret != "whsec" {
		t.Fatalf("unexpected WA config: %+v", c)
	}
	if c.WASendMax != 10 || c.WASendWindowSec != 60 || c.WASendDailyCap != 5 {
		t.Fatalf("unexpected WA rate config: %+v", c)
	}
	if c.WAJitterMinMS != 1000 || c.WAJitterMaxMS != 3000 || c.WAWarmupPerDay != 50 {
		t.Fatalf("unexpected WA jitter/warmup: %+v", c)
	}
	if !c.WABlockUnregistered {
		t.Fatal("WA_BLOCK_UNREGISTERED_SEND=true not parsed")
	}
}

func TestLoadWhatsAppEnabledWithoutDeviceDisabled(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	os.Setenv("WA_BASE_URL", "https://wa.test")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.WhatsAppEnabled() {
		t.Fatal("WhatsApp must stay disabled without WA_DEVICE_ID")
	}
}

func TestLoadInvalidWhatsAppNumbers(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	os.Setenv("WA_SEND_MAX", "notanumber")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid WA_SEND_MAX")
	}
}
