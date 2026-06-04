package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/incredible-zetta/crm/internal/adapter/email"
	"github.com/incredible-zetta/crm/internal/adapter/mysql"
	"github.com/incredible-zetta/crm/internal/adapter/system"
	"github.com/incredible-zetta/crm/internal/config"
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

	// 2. Database + migrations
	database, err := mysql.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()
	if err := mysql.Migrate(database); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}
	store := mysql.New(database)

	// 3. Email sender (driven adapter). Falls back to a disabled sender that
	//    fails explicitly when no provider is configured.
	var sender port.EmailSender = disabledSender{}
	provider := "smtp"
	if cfg.MailgunAPIKey != "" && cfg.MailgunDomain != "" {
		provider = "mailgun"
	}
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
		},
		sender,
		system.RealClock{},
		system.CryptoIDGen{},
		service.Config{BaseURL: cfg.BaseURL, ExportDir: exportDir},
	)

	// 6. MCP transport (auth-gated /mcp)
	mcpSrv := mcpserver.NewMCPServer("zettacrm", version)
	mcptransport.Register(mcpSrv, &mcptransport.Deps{
		Svc:     svc,
		Version: version,
		PingDB:  database.PingContext,
	})
	mcpHandler := mcpserver.Handler(cfg.MCPAPIKey, mcpSrv)

	// 7. Public HTTP transport (tracking, open pixel, export, unsubscribe, health)
	pub := &httptransport.Handlers{
		Tracking: svc.Tracking,
		Opens:    svc.Tracking,
		Exports:  svc.Contact,
		Unsub:    unsubscriberAdapter{svc.Contact},
		Version:  version,
		BaseURL:  cfg.BaseURL,
	}

	mux := http.NewServeMux()
	pub.Register(mux)
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)

	// 8. Scheduler worker -> TaskService.Execute
	worker := &scheduler.Worker{
		Claimer: taskClaimer{store.Tasks()},
		Exec:    taskExecutor{svc.Task},
	}

	// 9. Run with graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go worker.Start(ctx, time.Duration(cfg.SchedulerIntervalSec)*time.Second)

	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: mux}
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
