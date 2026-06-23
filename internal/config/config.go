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
	// Multi-tenancy (optional). When MultiTenancy is false the server runs as a
	// single implicit tenant (tenant.DefaultID) and behaves exactly as before.
	// When true, an MCP middleware resolves/auto-provisions a tenant from the
	// (Authorization, X-Session-Id) pair and scopes all data by tenant_id.
	MultiTenancy        bool
	IMAPHost            string
	IMAPPort            string
	IMAPUser            string
	IMAPPass            string
	IMAPMailbox         string
	IMAPPollIntervalSec int
	IMAPSinceDays       int
	AdminNotifyEmail    string
	EmailRateMax        int
	EmailRateWindowSec  int
	VerifyEmails        bool
	BlockInvalidSend    bool
	// WhatsApp channel
	WABaseURL           string // gateway base URL, e.g. https://notification.dev.lazyindra.online
	WABasicAuth         string // raw base64 of "user:pass" (never logged)
	WADeviceID          string // x-device-id header, e.g. "cds"
	WAWebhookSecret     string // HMAC-SHA256 secret for webhook validation (empty = no validation)
	WASendMax           int    // token bucket: max sends per window
	WASendWindowSec     int    // token bucket: window in seconds
	WASendDailyCap      int    // per-recipient daily cap
	WAJitterMinMS       int    // min jitter before send (ms)
	WAJitterMaxMS       int    // max jitter before send (ms)
	WAWarmupPerDay      int    // global warmup ceiling per 24h
	WABlockUnregistered bool   // refuse sends to numbers verified not on WhatsApp
	// Threads channel
	ThreadsAccessToken   string // Graph API access token (never logged)
	ThreadsAppSecret     string // app secret for token exchange (never logged)
	ThreadsUserID        string // Threads user id, defaults to "me"
	ThreadsAPIVersion    string // Graph API version, defaults to v1.0
	ThreadsDiscoveryBin  string // path to x-threads-utils cookie-only discovery binary (empty = disabled)
	ThreadsCookiesFile   string // path to Netscape cookie file for cookie-only discovery (empty = disabled)
	GHToken              string // GitHub token to auto-download the discovery binary when missing (never logged)
	ThreadsDiscoveryRepo string // owner/name of the discovery binary release repo (optional override)
	ThreadsDiscoveryTag  string // release tag to download (optional, default latest)
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

// WhatsAppEnabled reports whether the WhatsApp channel is configured.
func (c *Config) WhatsAppEnabled() bool {
	return c.WABaseURL != "" && c.WADeviceID != ""
}

// ThreadsEnabled reports whether the Threads channel is configured.
func (c *Config) ThreadsEnabled() bool {
	return c.ThreadsAccessToken != ""
}

// ThreadsDiscoveryEnabled reports whether the cookie-only discovery binary is
// configured. Independent of the Graph API channel. Needs both the binary and
// a cookie file.
func (c *Config) ThreadsDiscoveryEnabled() bool {
	return c.ThreadsDiscoveryBin != "" && c.ThreadsCookiesFile != ""
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
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

	multiTenancy := boolEnv("MULTI_TENANCY", false)

	verifyEmails := boolEnv("VERIFY_EMAILS", false)
	blockInvalidSend := boolEnv("BLOCK_INVALID_SEND", false)
	waBlockUnregistered := boolEnv("WA_BLOCK_UNREGISTERED_SEND", false)

	waSendMax := 0
	if v := os.Getenv("WA_SEND_MAX"); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WA_SEND_MAX: %w", err)
		}
		waSendMax = val
	}
	waSendWindowSec := 0
	if v := os.Getenv("WA_SEND_WINDOW_SEC"); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WA_SEND_WINDOW_SEC: %w", err)
		}
		waSendWindowSec = val
	}
	waSendDailyCap := 0
	if v := os.Getenv("WA_SEND_DAILY_CAP"); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WA_SEND_DAILY_CAP: %w", err)
		}
		waSendDailyCap = val
	}
	waJitterMinMS := 0
	if v := os.Getenv("WA_SEND_JITTER_MIN_MS"); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WA_SEND_JITTER_MIN_MS: %w", err)
		}
		waJitterMinMS = val
	}
	waJitterMaxMS := 0
	if v := os.Getenv("WA_SEND_JITTER_MAX_MS"); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WA_SEND_JITTER_MAX_MS: %w", err)
		}
		waJitterMaxMS = val
	}
	waWarmupPerDay := 0
	if v := os.Getenv("WA_WARMUP_PER_DAY"); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WA_WARMUP_PER_DAY: %w", err)
		}
		waWarmupPerDay = val
	}

	threadsUserID := os.Getenv("THREADS_USER_ID")
	if threadsUserID == "" {
		threadsUserID = "me"
	}
	threadsAPIVersion := os.Getenv("THREADS_API_VERSION")
	if threadsAPIVersion == "" {
		threadsAPIVersion = "v1.0"
	}

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
		MultiTenancy:         multiTenancy,
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
		WABaseURL:            os.Getenv("WA_BASE_URL"),
		WABasicAuth:          os.Getenv("WA_BASIC_AUTH"),
		WADeviceID:           os.Getenv("WA_DEVICE_ID"),
		WAWebhookSecret:      os.Getenv("WA_WEBHOOK_SECRET"),
		WASendMax:            waSendMax,
		WASendWindowSec:      waSendWindowSec,
		WASendDailyCap:       waSendDailyCap,
		WAJitterMinMS:        waJitterMinMS,
		WAJitterMaxMS:        waJitterMaxMS,
		WAWarmupPerDay:       waWarmupPerDay,
		WABlockUnregistered:  waBlockUnregistered,
		ThreadsAccessToken:   os.Getenv("THREADS_ACCESS_TOKEN"),
		ThreadsAppSecret:     os.Getenv("THREADS_APP_SECRET"),
		ThreadsUserID:        threadsUserID,
		ThreadsAPIVersion:    threadsAPIVersion,
		ThreadsDiscoveryBin:  os.Getenv("THREADS_DISCOVERY_BIN"),
		ThreadsCookiesFile:   os.Getenv("THREADS_COOKIES_FILE"),
		GHToken:              firstEnv("GH_TOKEN", "GITHUB_TOKEN"),
		ThreadsDiscoveryRepo: os.Getenv("THREADS_DISCOVERY_REPO"),
		ThreadsDiscoveryTag:  os.Getenv("THREADS_DISCOVERY_TAG"),
	}, nil
}
