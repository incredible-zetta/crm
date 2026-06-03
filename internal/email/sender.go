package email

import (
	"context"
	"errors"
	"net/http"
	"net/smtp"
)

type Message struct {
	To      string
	From    string
	Subject string
	HTML    string // optional
	Text    string // optional (at least one of HTML/Text should be set)
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

type Config struct {
	Provider      string // "smtp" or "mailgun"
	SMTPHost      string
	SMTPPort      string
	SMTPUser      string
	SMTPPass      string
	SMTPFrom      string
	MailgunDomain string
	MailgunAPIKey string
	DefaultFrom   string
}

func New(cfg Config) (Sender, error) {
	switch cfg.Provider {
	case "smtp":
		if cfg.SMTPHost == "" || cfg.SMTPPort == "" {
			return nil, errors.New("smtp host and port are required")
		}
		from := cfg.DefaultFrom
		if from == "" {
			from = cfg.SMTPFrom
		}
		var auth smtp.Auth
		if cfg.SMTPUser != "" {
			auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
		}
		return &SMTPSender{
			addr:     cfg.SMTPHost + ":" + cfg.SMTPPort,
			auth:     auth,
			from:     from,
			sendFunc: smtp.SendMail,
		}, nil
	case "mailgun":
		if cfg.MailgunDomain == "" || cfg.MailgunAPIKey == "" {
			return nil, errors.New("mailgun domain and apiKey are required")
		}
		from := cfg.DefaultFrom
		return &MailgunSender{
			domain:  cfg.MailgunDomain,
			apiKey:  cfg.MailgunAPIKey,
			from:    from,
			baseURL: "https://api.mailgun.net/v3",
			client:  http.DefaultClient,
		}, nil
	default:
		return nil, errors.New("unknown provider: " + cfg.Provider)
	}
}
