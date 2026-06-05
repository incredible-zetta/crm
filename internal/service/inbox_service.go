package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// InboxService coordinates inbound mail sync, notifications, and agent inbox actions.
type InboxService struct {
	repo     port.InboxRepo
	contacts port.ContactRepo
	fetcher  port.InboxFetcher
	notifier port.AdminNotifier
	sender   port.EmailSender
	clock    port.Clock
	mailbox  string
}

func NewInboxService(repo port.InboxRepo, contacts port.ContactRepo, fetcher port.InboxFetcher, notifier port.AdminNotifier, sender port.EmailSender, clock port.Clock, mailbox string) *InboxService {
	return &InboxService{repo: repo, contacts: contacts, fetcher: fetcher, notifier: notifier, sender: sender, clock: clock, mailbox: mailbox}
}

func (s *InboxService) Sync(ctx context.Context, limit int) (domain.InboxSyncResult, error) {
	if s == nil || s.repo == nil || s.fetcher == nil {
		return domain.InboxSyncResult{}, fmt.Errorf("inbox disabled")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	cursor, err := s.repo.GetCursor(ctx, s.mailbox)
	if err != nil {
		return domain.InboxSyncResult{}, err
	}
	messages, maxUID, err := s.fetcher.FetchNew(ctx, cursor, limit)
	if err != nil {
		return domain.InboxSyncResult{}, err
	}
	result := domain.InboxSyncResult{Fetched: len(messages), LastUID: maxUID}
	for _, msg := range messages {
		msg.Mailbox = s.mailbox
		msg.FromEmail = strings.ToLower(strings.TrimSpace(msg.FromEmail))
		if msg.ContactID == nil && s.contacts != nil && msg.FromEmail != "" {
			contact, err := s.contacts.GetByEmail(ctx, msg.FromEmail)
			if err == nil {
				msg.ContactID = &contact.ID
				result.KnownContacts++
			} else if !errors.Is(err, domain.ErrNotFound) {
				return result, err
			}
		}
		stored, isNew, err := s.repo.InsertMessage(ctx, msg)
		if err != nil {
			return result, err
		}
		if !isNew {
			continue
		}
		result.New++
		if stored.ContactID != nil && s.notifier != nil {
			contact, err := s.contacts.Get(ctx, *stored.ContactID)
			if err == nil {
				if err := s.notifier.NotifyInboundMessage(ctx, stored, contact); err == nil {
					_ = s.repo.MarkNotified(ctx, stored.ID, s.clock.Now())
					result.Notified++
				}
			}
		}
	}
	if maxUID > cursor.LastUID {
		if err := s.repo.UpsertCursor(ctx, domain.InboxCursor{Mailbox: s.mailbox, LastUID: maxUID}); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *InboxService) List(ctx context.Context, f domain.InboxFilter, p port.Paging) (port.InboxPage, error) {
	return s.repo.ListMessages(ctx, f, p)
}

func (s *InboxService) Get(ctx context.Context, id int64) (domain.InboundMessage, error) {
	return s.repo.GetMessage(ctx, id)
}

func (s *InboxService) MarkRead(ctx context.Context, id int64, read bool) error {
	if read {
		now := s.clock.Now()
		return s.repo.MarkRead(ctx, id, &now)
	}
	return s.repo.MarkRead(ctx, id, nil)
}

func (s *InboxService) Delete(ctx context.Context, id int64) error {
	return s.repo.SoftDeleteMessage(ctx, id)
}

func (s *InboxService) Reply(ctx context.Context, reply domain.InboxReply) error {
	msg, err := s.repo.GetMessage(ctx, reply.InboundID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(reply.BodyText) == "" && strings.TrimSpace(reply.BodyHTML) == "" {
		return fmt.Errorf("reply body required")
	}
	subject := msg.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}
	if err := s.sender.Send(ctx, port.OutboundMessage{To: msg.FromEmail, Subject: subject, Text: reply.BodyText, HTML: reply.BodyHTML}); err != nil {
		return err
	}
	return s.repo.MarkReplied(ctx, msg.ID, s.clock.Now())
}

func (s *InboxService) RetryNotifications(ctx context.Context, limit int) (int, error) {
	messages, err := s.repo.ListUnnotifiedKnown(ctx, limit)
	if err != nil {
		return 0, err
	}
	notified := 0
	for _, msg := range messages {
		if msg.ContactID == nil || s.notifier == nil {
			continue
		}
		contact, err := s.contacts.Get(ctx, *msg.ContactID)
		if err != nil {
			continue
		}
		if err := s.notifier.NotifyInboundMessage(ctx, msg, contact); err != nil {
			continue
		}
		if err := s.repo.MarkNotified(ctx, msg.ID, s.clock.Now()); err != nil {
			return notified, err
		}
		notified++
	}
	return notified, nil
}
