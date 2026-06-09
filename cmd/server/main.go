package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/incredible-zetta/crm/internal/adapter/email"
	imapadapter "github.com/incredible-zetta/crm/internal/adapter/imap"
	"github.com/incredible-zetta/crm/internal/adapter/mysql"
	"github.com/incredible-zetta/crm/internal/adapter/system"
	"github.com/incredible-zetta/crm/internal/adapter/verify"
	whatsappadapter "github.com/incredible-zetta/crm/internal/adapter/whatsapp"
	"github.com/incredible-zetta/crm/internal/config"
	"github.com/incredible-zetta/crm/internal/inboxpoller"
	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/scheduler"
	"github.com/incredible-zetta/crm/internal/service"
	httptransport "github.com/incredible-zetta/crm/internal/transport/http"
	mcptransport "github.com/incredible-zetta/crm/internal/transport/mcp"
)

var version = "dev"

func main() {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	debug := cfg.DebugEnabled()
	debugLog(debug, "debug logging enabled: version=%s base_url=%s port=%s scheduler_interval_sec=%d inbox_enabled=%t db_dsn=%s", version, cfg.BaseURL, cfg.Port, cfg.SchedulerIntervalSec, cfg.InboxEnabled(), redactDSN(cfg.DBDSN))

	// 2. Database + migrations
	debugLog(debug, "opening database: %s", redactDSN(cfg.DBDSN))
	database, err := mysql.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()
	debugLog(debug, "running database migrations")
	if err := mysql.Migrate(database); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}
	debugLog(debug, "database migrations complete")
	store := mysql.New(database)

	// 3. Email sender (driven adapter). Falls back to a disabled sender that
	//    fails explicitly when no provider is configured.
	var sender port.EmailSender = disabledSender{}
	provider := "smtp"
	if cfg.MailgunAPIKey != "" && cfg.MailgunDomain != "" {
		provider = "mailgun"
	}
	debugLog(debug, "email provider selected: %s", provider)
	if provider == "mailgun" || cfg.SMTPHost != "" {
		s, senderErr := email.New(email.Config{
			Provider:      provider,
			SMTPHost:      cfg.SMTPHost,
			SMTPPort:      cfg.SMTPPort,
			SMTPUser:      cfg.SMTPUser,
			SMTPPass:      cfg.SMTPPass,
			SMTPFrom:      cfg.SMTPFrom,
			MailgunDomain: cfg.MailgunDomain,
			MailgunAPIKey: cfg.MailgunAPIKey,
			DefaultFrom:   cfg.SMTPFrom,
		})
		if senderErr != nil {
			log.Fatalf("failed to configure email sender: %v", senderErr)
		}
		sender = s
	} else {
		log.Println("WARNING: neither SMTP nor Mailgun is configured; email sending is disabled")
	}

	// Pace outbound email to stay under provider rate limits (e.g. Larksuite
	// allows 200 messages / 100s). Applies to single sends and campaign loops.
	if cfg.EmailRateEnabled() {
		sender = email.NewThrottledSender(sender, cfg.EmailRateMax, time.Duration(cfg.EmailRateWindowSec)*time.Second)
		debugLog(debug, "email rate limit enabled: %d per %ds", cfg.EmailRateMax, cfg.EmailRateWindowSec)
	}

	// 4. Export directory
	exportDir := os.Getenv("EXPORT_DIR")
	if exportDir == "" {
		exportDir = "./exports"
	}
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		log.Fatalf("failed to create export directory %q: %v", exportDir, err)
	}

	// 5. Wire the service (use-case) layer from ports.
	svc := service.New(
		service.Repos{
			Contacts:  store.Contacts(),
			Campaigns: store.Campaigns(),
			Templates: store.Templates(),
			Tasks:     store.Tasks(),
			Events:    store.Events(),
			Tracking:  store.Tracking(),
			Exports:   store.Exports(),
			Inbox:     store.Inbox(),
		},
		sender,
		system.RealClock{},
		system.CryptoIDGen{},
		service.Config{BaseURL: cfg.BaseURL, ExportDir: exportDir},
	)
	if cfg.InboxEnabled() {
		fetcher := imapadapter.NewFetcher(imapadapter.Config{Host: cfg.IMAPHost, Port: cfg.IMAPPort, User: cfg.IMAPUser, Pass: cfg.IMAPPass, Mailbox: cfg.IMAPMailbox, SinceDays: cfg.IMAPSinceDays})
		notifier := email.NewAdminNotifier(sender, cfg.AdminNotifyEmail)
		svc.Inbox = service.NewInboxService(store.Inbox(), store.Contacts(), fetcher, notifier, sender, system.RealClock{}, cfg.IMAPMailbox)
	} else {
		svc.Inbox = nil
		debugLog(debug, "inbox disabled: set IMAP_HOST, IMAP_USER, IMAP_PASS, IMAP_MAILBOX, ADMIN_NOTIFY_EMAIL to enable")
	}

	// WhatsApp channel (outbound send + inbound webhook ingest + capability audit)
	if cfg.WhatsAppEnabled() {
		waClient, err := whatsappadapter.New(whatsappadapter.Config{
			BaseURL:   cfg.WABaseURL,
			BasicAuth: cfg.WABasicAuth,
			DeviceID:  cfg.WADeviceID,
		})
		if err != nil {
			log.Fatalf("failed to create whatsapp adapter: %v", err)
		}
		// Wrap with smart-send policy (rate-limit + jitter + daily cap + warmup)
		policy := whatsappadapter.SmartSendPolicy{
			RateMax:              cfg.WASendMax,
			RateWindow:           time.Duration(cfg.WASendWindowSec) * time.Second,
			JitterMin:            time.Duration(cfg.WAJitterMinMS) * time.Millisecond,
			JitterMax:            time.Duration(cfg.WAJitterMaxMS) * time.Millisecond,
			DailyCapPerRecipient: cfg.WASendDailyCap,
			WarmupPerDay:         cfg.WAWarmupPerDay,
		}
		var gw port.WhatsAppGateway = waClient
		if policy.RateMax > 0 || policy.DailyCapPerRecipient > 0 || policy.WarmupPerDay > 0 || policy.JitterMax > 0 {
			gw = whatsappadapter.NewSmartSender(waClient, policy, store.WhatsApp())
			debugLog(debug, "whatsapp smart-send enabled: rate=%d/%ds cap=%d/day warmup=%d/day jitter=%d-%dms",
				policy.RateMax, cfg.WASendWindowSec, policy.DailyCapPerRecipient, policy.WarmupPerDay,
				cfg.WAJitterMinMS, cfg.WAJitterMaxMS)
		}
		svc.WhatsApp = service.NewWhatsAppService(gw, store.WhatsApp(), store.Contacts(), system.RealClock{}, nil, port.SmartSendPolicy{
			BlockNotRegistered: cfg.WABlockUnregistered,
			MaxPerSecond:       0, // smart-sender handles rate-limiting
		})
		debugLog(debug, "whatsapp channel enabled: base=%s device=%s block_unregistered=%v",
			cfg.WABaseURL, cfg.WADeviceID, cfg.WABlockUnregistered)
	} else {
		svc.WhatsApp = nil
		debugLog(debug, "whatsapp disabled: set WA_BASE_URL, WA_DEVICE_ID to enable")
	}

	// Email verification (self-hosted: syntax + DNS/MX + heuristics).
	if cfg.VerifyEmails {
		verifier := verify.New()
		svc.Contact.SetVerifier(verifier)
		debugLog(debug, "email verification enabled on create/update/audit")
	}
	if cfg.BlockInvalidSend {
		svc.Email.SetBlockInvalid(true)
		debugLog(debug, "send guard enabled: refusing contacts verified invalid")
	}

	// 6. MCP transport (auth-gated /mcp)
	mcpSrv := mcpserver.NewMCPServer("zettacrm", version)
	mcptransport.Register(mcpSrv, &mcptransport.Deps{
		Svc:     svc,
		Version: version,
		PingDB:  database.PingContext,
	})
	mcpHandler := mcpserver.Handler(cfg.MCPAPIKey, mcpSrv)

	// 7. Public HTTP transport (tracking, open pixel, export, unsubscribe, health, wa webhook)
	var waWebhook http.Handler
	if cfg.WhatsAppEnabled() && svc.WhatsApp != nil {
		waWebhook = httptransport.NewWhatsAppWebhookHandler(svc.WhatsApp, cfg.WAWebhookSecret)
	}
	pub := &httptransport.Handlers{
		Tracking:        svc.Tracking,
		Opens:           svc.Tracking,
		Exports:         svc.Contact,
		Unsub:           unsubscriberAdapter{svc.Contact},
		WhatsAppWebhook: waWebhook,
		Version:         version,
		BaseURL:         cfg.BaseURL,
	}

	mux := http.NewServeMux()
	pub.Register(mux)
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)
	debugLog(debug, "registered routes: GET /{$}, GET /healthz, GET /t/{code}, GET /o/{code}, GET /export/{id}, GET /u/{code}, POST /u/{code}, POST /wa/webhook, /mcp, /mcp/")

	// 8. Scheduler worker -> TaskService.Execute
	worker := &scheduler.Worker{
		Claimer: taskClaimer{store.Tasks()},
		Exec:    taskExecutor{svc.Task},
	}

	// 9. Run with graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go worker.Start(ctx, time.Duration(cfg.SchedulerIntervalSec)*time.Second)
	if cfg.InboxEnabled() && svc.Inbox != nil {
		inboxpoller.New(svc.Inbox, time.Duration(cfg.IMAPPollIntervalSec)*time.Second, 100).Start(ctx)
		debugLog(debug, "inbox poller started: mailbox=%s interval_sec=%d", cfg.IMAPMailbox, cfg.IMAPPollIntervalSec)
	}

	handler := http.Handler(mux)
	if debug {
		handler = debugRequestLogger(handler)
	}
	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: handler}
	go func() {
		<-ctx.Done()
		log.Println("Shutting down HTTP server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	log.Printf("Listening on :%s", cfg.Port)
	if listenErr := httpSrv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
		log.Fatalf("HTTP server listen and serve error: %v", listenErr)
	}
}

// unsubscriberAdapter narrows ContactService to the httptransport.Unsubscriber
// interface (the HTTP route only needs the by-code unsubscribe, discarding the
// returned contact).
type unsubscriberAdapter struct{ svc *service.ContactService }

func (u unsubscriberAdapter) UnsubscribeByCode(ctx context.Context, code string) error {
	_, err := u.svc.UnsubscribeByCode(ctx, code)
	return err
}

func debugLog(enabled bool, format string, args ...any) {
	if enabled {
		log.Printf("DEBUG: "+format, args...)
	}
}

func debugRequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("DEBUG: http %s %s status=%d duration_ms=%d", r.Method, r.URL.Path, rw.status, time.Since(start).Milliseconds())
	})
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

var dsnCredentialPattern = regexp.MustCompile(`^([^:@/]+):([^@]*)@`)

func redactDSN(dsn string) string {
	redacted := dsnCredentialPattern.ReplaceAllString(dsn, `$1:***@`)
	if idx := strings.Index(redacted, "?"); idx >= 0 {
		redacted = redacted[:idx] + "?..."
	}
	return redacted
}
