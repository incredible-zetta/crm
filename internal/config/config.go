package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	MCPAPIKey            string
	DBDSN                string
	BaseURL              string
	Port                 string
	SchedulerIntervalSec int
	SMTPHost             string
	SMTPPort             string
	SMTPUser             string
	SMTPPass             string
	SMTPFrom             string
	MailgunDomain        string
	MailgunAPIKey        string
	LogLevel             string
	IMAPHost             string
	IMAPPort             string
	IMAPUser             string
	IMAPPass             string
	IMAPMailbox          string
	IMAPPollIntervalSec  int
	IMAPSinceDays        int
	AdminNotifyEmail     string
	EmailRateMax         int
	EmailRateWindowSec   int
	VerifyEmails         bool
	BlockInvalidSend     bool
}

func (c *Config) DebugEnabled() bool {
	return strings.EqualFold(c.LogLevel, "debug")
}

func (c *Config) InboxEnabled() bool {
	return c.IMAPHost != "" && c.IMAPUser != "" && c.IMAPPass != "" && c.IMAPMailbox != "" && c.AdminNotifyEmail != ""
}

// EmailRateEnabled reports whether outbound email throttling is configured.
// Both the message cap and the window must be positive to take effect.
func (c *Config) EmailRateEnabled() bool {
	return c.EmailRateMax > 0 && c.EmailRateWindowSec > 0
}

func boolEnv(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func Load() (*Config, error) {
	mcpAPIKey := os.Getenv("MCP_API_KEY")
	if mcpAPIKey == "" {
		return nil, fmt.Errorf("MCP_API_KEY is required")
	}

	dbDSN := os.Getenv("DB_DSN")
	if dbDSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("BASE_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	schedulerIntervalSec := 15
	if intervalStr := os.Getenv("SCHEDULER_INTERVAL_SEC"); intervalStr != "" {
		val, err := strconv.Atoi(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SCHEDULER_INTERVAL_SEC: %w", err)
		}
		schedulerIntervalSec = val
	}

	imapPort := os.Getenv("IMAP_PORT")
	if imapPort == "" {
		imapPort = "993"
	}
	imapMailbox := os.Getenv("IMAP_MAILBOX")
	if imapMailbox == "" {
		imapMailbox = "INBOX"
	}
	imapPollIntervalSec := 60
	if intervalStr := os.Getenv("IMAP_POLL_INTERVAL_SEC"); intervalStr != "" {
		val, err := strconv.Atoi(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid IMAP_POLL_INTERVAL_SEC: %w", err)
		}
		imapPollIntervalSec = val
	}
	imapSinceDays := 14
	if daysStr := os.Getenv("IMAP_SINCE_DAYS"); daysStr != "" {
		val, err := strconv.Atoi(daysStr)
		if err != nil {
			return nil, fmt.Errorf("invalid IMAP_SINCE_DAYS: %w", err)
		}
		imapSinceDays = val
	}

	emailRateMax := 0
	if v := os.Getenv("EMAIL_RATE_MAX"); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid EMAIL_RATE_MAX: %w", err)
		}
		emailRateMax = val
	}
	emailRateWindowSec := 0
	if v := os.Getenv("EMAIL_RATE_WINDOW_SEC"); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid EMAIL_RATE_WINDOW_SEC: %w", err)
		}
		emailRateWindowSec = val
	}

	verifyEmails := boolEnv("VERIFY_EMAILS", false)
	blockInvalidSend := boolEnv("BLOCK_INVALID_SEND", false)

	return &Config{
		MCPAPIKey:            mcpAPIKey,
		DBDSN:                dbDSN,
		BaseURL:              baseURL,
		Port:                 port,
		SchedulerIntervalSec: schedulerIntervalSec,
		SMTPHost:             os.Getenv("SMTP_HOST"),
		SMTPPort:             os.Getenv("SMTP_PORT"),
		SMTPUser:             os.Getenv("SMTP_USER"),
		SMTPPass:             os.Getenv("SMTP_PASS"),
		SMTPFrom:             os.Getenv("SMTP_FROM"),
		MailgunDomain:        os.Getenv("MAILGUN_DOMAIN"),
		MailgunAPIKey:        os.Getenv("MAILGUN_API_KEY"),
		LogLevel:             os.Getenv("LOG_LEVEL"),
		IMAPHost:             os.Getenv("IMAP_HOST"),
		IMAPPort:             imapPort,
		IMAPUser:             os.Getenv("IMAP_USER"),
		IMAPPass:             os.Getenv("IMAP_PASS"),
		IMAPMailbox:          imapMailbox,
		IMAPPollIntervalSec:  imapPollIntervalSec,
		IMAPSinceDays:        imapSinceDays,
		AdminNotifyEmail:     os.Getenv("ADMIN_NOTIFY_EMAIL"),
		EmailRateMax:         emailRateMax,
		EmailRateWindowSec:   emailRateWindowSec,
		VerifyEmails:         verifyEmails,
		BlockInvalidSend:     blockInvalidSend,
	}, nil
}
