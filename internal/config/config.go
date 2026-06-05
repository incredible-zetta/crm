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
}

func (c *Config) DebugEnabled() bool {
	return strings.EqualFold(c.LogLevel, "debug")
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
	}, nil
}
