package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/config"
	"github.com/cipta/crm-for-aiagents/internal/db"
	"github.com/cipta/crm-for-aiagents/internal/email"
	"github.com/cipta/crm-for-aiagents/internal/httpx"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
	"github.com/cipta/crm-for-aiagents/internal/mcptools"
	"github.com/cipta/crm-for-aiagents/internal/scheduler"
)

func main() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// 2. Open DB + migrate
	database, err := db.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

	// 3. New Repo
	repo := db.NewRepo(database)

	// 4. Build email sender
	var sender email.Sender
	provider := "smtp"
	if cfg.MailgunAPIKey != "" && cfg.MailgunDomain != "" {
		provider = "mailgun"
	}

	if provider == "smtp" && cfg.SMTPHost == "" {
		log.Println("WARNING: neither SMTP nor Mailgun is configured; email sending is disabled")
	} else {
		var senderErr error
		sender, senderErr = email.New(email.Config{
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
	}

	// 5. Build email pipeline
	pipe := &email.Pipeline{
		Sender:  sender,
		Tmpl:    mcptools.RepoTemplateStore{Repo: repo},
		Links:   mcptools.RepoLinkMaker{Repo: repo},
		Events:  mcptools.RepoEventLogger{Repo: repo},
		BaseURL: cfg.BaseURL,
	}

	// 6. Build mcptools.Deps
	version := "1.0.0"
	exportDir := os.Getenv("EXPORT_DIR")
	if exportDir == "" {
		exportDir = "./exports"
	}
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		log.Fatalf("failed to create export directory %q: %v", exportDir, err)
	}

	deps := &mcptools.Deps{
		Repo:      repo,
		Pipeline:  pipe,
		BaseURL:   cfg.BaseURL,
		ExportDir: exportDir,
		Version:   version,
		PingDB: func(ctx context.Context) error {
			return database.PingContext(ctx)
		},
	}

	// 7. MCP Server
	srv := mcpserver.NewMCPServer("crm-for-aiagents", version)
	mcptools.Register(srv, deps)
	mcpHandler := mcpserver.Handler(cfg.MCPAPIKey, srv)

	// 8. HTTP handlers (tracking/export/health) + Adapters
	h := &httpx.Handlers{
		Links:   linkResolver{repo},
		Events:  eventRecorder{repo},
		Exports: exportResolver{repo},
	}

	mux := http.NewServeMux()
	h.Register(mux)                 // /t /o /export /healthz
	mux.Handle("/mcp", mcpHandler)  // POST /mcp (auth-gated)
	mux.Handle("/mcp/", mcpHandler) // in case streamable uses subpaths

	// 9. Scheduler worker
	worker := &scheduler.Worker{
		Claimer: taskClaimer{repo},
		Exec:    taskExecutor{repo, pipe, deps},
	}

	// 10. Start everything with graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start scheduler background loop
	interval := time.Duration(cfg.SchedulerIntervalSec) * time.Second
	go worker.Start(ctx, interval)

	// Build and start HTTP server
	httpSrv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

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
