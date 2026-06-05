package imap

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"time"

	imaplib "github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// Config defines one IMAP mailbox connection.
type Config struct {
	Host      string
	Port      string
	User      string
	Pass      string
	Mailbox   string
	SinceDays int
}

// Fetcher fetches new messages from one IMAP mailbox.
type Fetcher struct {
	cfg Config
}

var _ port.InboxFetcher = (*Fetcher)(nil)

func NewFetcher(cfg Config) *Fetcher {
	return &Fetcher{cfg: cfg}
}

func (f *Fetcher) FetchNew(ctx context.Context, cursor domain.InboxCursor, limit int) ([]domain.InboundMessage, uint32, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	addr := f.cfg.Host + ":" + f.cfg.Port
	c, err := client.DialTLS(addr, nil)
	if err != nil {
		return nil, cursor.LastUID, fmt.Errorf("imap dial: %w", err)
	}
	defer c.Logout()
	if err := c.Login(f.cfg.User, f.cfg.Pass); err != nil {
		return nil, cursor.LastUID, fmt.Errorf("imap login: %w", err)
	}
	mailbox := f.cfg.Mailbox
	if mailbox == "" {
		mailbox = "INBOX"
	}
	if _, err := c.Select(mailbox, true); err != nil {
		return nil, cursor.LastUID, fmt.Errorf("imap select mailbox: %w", err)
	}

	uids, err := f.searchUIDs(c, cursor)
	if err != nil {
		return nil, cursor.LastUID, err
	}
	if len(uids) == 0 {
		return nil, cursor.LastUID, nil
	}
	sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })
	if len(uids) > limit {
		uids = uids[:limit]
	}

	seqset := new(imaplib.SeqSet)
	for _, uid := range uids {
		seqset.AddNum(uid)
	}
	section := &imaplib.BodySectionName{}
	items := []imaplib.FetchItem{imaplib.FetchUid, section.FetchItem()}
	messages := make(chan *imaplib.Message, len(uids))
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(seqset, items, messages) }()

	var out []domain.InboundMessage
	maxUID := cursor.LastUID
	for msg := range messages {
		select {
		case <-ctx.Done():
			return nil, maxUID, ctx.Err()
		default:
		}
		body := msg.GetBody(section)
		if body == nil {
			continue
		}
		parsed, err := ParseMessage(body)
		if err != nil {
			continue
		}
		parsed.Mailbox = mailbox
		parsed.UID = uint32(msg.Uid)
		if parsed.UID > maxUID {
			maxUID = parsed.UID
		}
		out = append(out, parsed)
	}
	if err := <-done; err != nil {
		return nil, maxUID, fmt.Errorf("imap fetch: %w", err)
	}
	return out, maxUID, nil
}

func (f *Fetcher) searchUIDs(c *client.Client, cursor domain.InboxCursor) ([]uint32, error) {
	criteria := imaplib.NewSearchCriteria()
	if cursor.LastUID > 0 {
		criteria.Uid = new(imaplib.SeqSet)
		criteria.Uid.AddRange(cursor.LastUID+1, 0)
	} else {
		sinceDays := f.cfg.SinceDays
		if sinceDays <= 0 {
			sinceDays = 14
		}
		criteria.Since = time.Now().AddDate(0, 0, -sinceDays)
	}
	uids, err := c.UidSearch(criteria)
	if err != nil && cursor.LastUID > 0 && isBadUIDRange(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("imap search: %w", err)
	}
	return uids, nil
}

func isBadUIDRange(err error) bool {
	_, convErr := strconv.Atoi(err.Error())
	return convErr == nil
}

var _ io.Reader
