package inboxpoller

import (
	"context"
	"log"
	"time"

	"github.com/incredible-zetta/crm/internal/service"
)

type Worker struct {
	inbox    *service.InboxService
	interval time.Duration
	limit    int
}

func New(inbox *service.InboxService, interval time.Duration, limit int) *Worker {
	if interval <= 0 {
		interval = time.Minute
	}
	if limit <= 0 {
		limit = 100
	}
	return &Worker{inbox: inbox, interval: interval, limit: limit}
}

func (w *Worker) RunOnce(ctx context.Context) {
	if w == nil || w.inbox == nil {
		return
	}
	result, err := w.inbox.Sync(ctx, w.limit)
	if err != nil {
		log.Printf("inbox poller error: %v", err)
		return
	}
	log.Printf("inbox poller sync: fetched=%d new=%d known_contacts=%d notified=%d", result.Fetched, result.New, result.KnownContacts, result.Notified)
}

func (w *Worker) Start(ctx context.Context) {
	if w == nil || w.inbox == nil {
		return
	}
	go func() {
		w.RunOnce(ctx)
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.RunOnce(ctx)
			}
		}
	}()
}
