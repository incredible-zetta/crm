package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
)

type inboxRepo struct {
	db *sql.DB
}

var _ port.InboxRepo = (*inboxRepo)(nil)

func (r *inboxRepo) GetCursor(ctx context.Context, mailbox string) (domain.InboxCursor, error) {
	query := `SELECT id, mailbox, last_uid, last_message_date, updated_at FROM inbox_cursors WHERE mailbox = ? AND tenant_id = ?`
	var c domain.InboxCursor
	var lastDate sql.NullTime
	err := r.db.QueryRowContext(ctx, query, mailbox, tenant.From(ctx)).Scan(&c.ID, &c.Mailbox, &c.LastUID, &lastDate, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.InboxCursor{Mailbox: mailbox}, nil
	}
	if err != nil {
		return domain.InboxCursor{}, fmt.Errorf("get inbox cursor: %w", err)
	}
	c.LastMessageDate = fromNullTime(lastDate)
	return c, nil
}

func (r *inboxRepo) UpsertCursor(ctx context.Context, cursor domain.InboxCursor) error {
	query := `INSERT INTO inbox_cursors (tenant_id, mailbox, last_uid, last_message_date)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		last_uid = GREATEST(last_uid, VALUES(last_uid)),
		last_message_date = VALUES(last_message_date),
		updated_at = CURRENT_TIMESTAMP`
	_, err := r.db.ExecContext(ctx, query, tenant.From(ctx), cursor.Mailbox, cursor.LastUID, toNullTime(cursor.LastMessageDate))
	if err != nil {
		return fmt.Errorf("upsert inbox cursor: %w", err)
	}
	return nil
}

func (r *inboxRepo) InsertMessage(ctx context.Context, msg domain.InboundMessage) (domain.InboundMessage, bool, error) {
	msg.FromEmail = strings.ToLower(strings.TrimSpace(msg.FromEmail))
	query := `INSERT IGNORE INTO inbound_messages
		(tenant_id, mailbox, uid, message_id, in_reply_to, references_header, from_email, from_name, to_email, subject, received_at,
		 text_body, html_body, snippet, contact_id, campaign_id, read_at, replied_at, deleted_at, notified_at, raw_headers_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query,
		tenant.From(ctx),
		msg.Mailbox,
		msg.UID,
		toNullString(msg.MessageID),
		toNullString(msg.InReplyTo),
		toNullString(msg.ReferencesHeader),
		msg.FromEmail,
		toNullString(msg.FromName),
		toNullString(msg.ToEmail),
		toNullString(msg.Subject),
		msg.ReceivedAt,
		msg.TextBody,
		msg.HTMLBody,
		msg.Snippet,
		toNullInt64Ptr(msg.ContactID),
		toNullInt64Ptr(msg.CampaignID),
		toNullTime(msg.ReadAt),
		toNullTime(msg.RepliedAt),
		toNullTime(msg.DeletedAt),
		toNullTime(msg.NotifiedAt),
		toNullString(msg.RawHeadersJSON),
	)
	if err != nil {
		return domain.InboundMessage{}, false, fmt.Errorf("insert inbound message: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return domain.InboundMessage{}, false, fmt.Errorf("insert inbound message rows affected: %w", err)
	}
	if rows == 0 {
		return r.getMessageByMailboxUID(ctx, msg.Mailbox, msg.UID, false)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.InboundMessage{}, false, fmt.Errorf("insert inbound message last id: %w", err)
	}
	msg.ID = id
	return msg, true, nil
}

func (r *inboxRepo) GetMessage(ctx context.Context, id int64) (domain.InboundMessage, error) {
	return r.scanMessage(r.db.QueryRowContext(ctx, inboxSelectSQL()+` WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`, id, tenant.From(ctx)))
}

func (r *inboxRepo) ListMessages(ctx context.Context, f domain.InboxFilter, p port.Paging) (port.InboxPage, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	where := []string{"deleted_at IS NULL", "tenant_id = ?"}
	args := []any{tenant.From(ctx)}
	if p.Cursor > 0 {
		where = append(where, "id < ?")
		args = append(args, p.Cursor)
	}
	if f.UnreadOnly {
		where = append(where, "read_at IS NULL")
	}
	if f.KnownOnly {
		where = append(where, "contact_id IS NOT NULL")
	}
	if f.ContactID != nil {
		where = append(where, "contact_id = ?")
		args = append(args, *f.ContactID)
	}
	whereSQL := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM inbound_messages`+whereSQL, args...).Scan(&total); err != nil {
		return port.InboxPage{}, fmt.Errorf("count inbound messages: %w", err)
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit+1)
	rows, err := r.db.QueryContext(ctx, inboxSelectSQL()+whereSQL+` ORDER BY id DESC LIMIT ?`, queryArgs...)
	if err != nil {
		return port.InboxPage{}, fmt.Errorf("list inbound messages: %w", err)
	}
	defer rows.Close()

	var items []domain.InboundMessage
	for rows.Next() {
		msg, err := scanInboundMessage(rows)
		if err != nil {
			return port.InboxPage{}, err
		}
		items = append(items, msg)
	}
	if err := rows.Err(); err != nil {
		return port.InboxPage{}, fmt.Errorf("list inbound messages rows: %w", err)
	}

	var next int64
	if len(items) > limit {
		next = items[limit-1].ID
		items = items[:limit]
	}
	return port.InboxPage{Items: items, Total: total, NextCursor: next}, nil
}

func (r *inboxRepo) MarkRead(ctx context.Context, id int64, at *time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE inbound_messages SET read_at = ? WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`, toNullTime(at), id, tenant.From(ctx))
	if err != nil {
		return fmt.Errorf("mark inbound read: %w", err)
	}
	return nil
}

func (r *inboxRepo) MarkReplied(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE inbound_messages SET replied_at = ? WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`, at, id, tenant.From(ctx))
	if err != nil {
		return fmt.Errorf("mark inbound replied: %w", err)
	}
	return nil
}

func (r *inboxRepo) SoftDeleteMessage(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE inbound_messages SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`, id, tenant.From(ctx))
	if err != nil {
		return fmt.Errorf("soft delete inbound message: %w", err)
	}
	return nil
}

func (r *inboxRepo) MarkNotified(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE inbound_messages SET notified_at = ? WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`, at, id, tenant.From(ctx))
	if err != nil {
		return fmt.Errorf("mark inbound notified: %w", err)
	}
	return nil
}

func (r *inboxRepo) ListUnnotifiedKnown(ctx context.Context, limit int) ([]domain.InboundMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, inboxSelectSQL()+` WHERE deleted_at IS NULL AND tenant_id = ? AND contact_id IS NOT NULL AND notified_at IS NULL ORDER BY received_at ASC, id ASC LIMIT ?`, tenant.From(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("list unnotified known inbound messages: %w", err)
	}
	defer rows.Close()
	var items []domain.InboundMessage
	for rows.Next() {
		msg, err := scanInboundMessage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list unnotified known inbound messages rows: %w", err)
	}
	return items, nil
}

func (r *inboxRepo) getMessageByMailboxUID(ctx context.Context, mailbox string, uid uint32, isNew bool) (domain.InboundMessage, bool, error) {
	msg, err := r.scanMessage(r.db.QueryRowContext(ctx, inboxSelectSQL()+` WHERE mailbox = ? AND uid = ? AND tenant_id = ?`, mailbox, uid, tenant.From(ctx)))
	return msg, isNew, err
}

func (r *inboxRepo) scanMessage(row rowScanner) (domain.InboundMessage, error) {
	msg, err := scanInboundMessage(row)
	if err == sql.ErrNoRows {
		return domain.InboundMessage{}, fmt.Errorf("inbound message not found: %w", domain.ErrNotFound)
	}
	return msg, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func inboxSelectSQL() string {
	return `SELECT id, mailbox, uid, message_id, in_reply_to, references_header, from_email, from_name, to_email, subject,
		received_at, text_body, html_body, snippet, contact_id, campaign_id, read_at, replied_at, deleted_at, notified_at,
		raw_headers_json, created_at FROM inbound_messages`
}

func scanInboundMessage(scanner rowScanner) (domain.InboundMessage, error) {
	var msg domain.InboundMessage
	var messageID, inReplyTo, referencesHeader, fromName, toEmail, subject, rawHeaders sql.NullString
	var contactID, campaignID sql.NullInt64
	var readAt, repliedAt, deletedAt, notifiedAt sql.NullTime
	err := scanner.Scan(
		&msg.ID,
		&msg.Mailbox,
		&msg.UID,
		&messageID,
		&inReplyTo,
		&referencesHeader,
		&msg.FromEmail,
		&fromName,
		&toEmail,
		&subject,
		&msg.ReceivedAt,
		&msg.TextBody,
		&msg.HTMLBody,
		&msg.Snippet,
		&contactID,
		&campaignID,
		&readAt,
		&repliedAt,
		&deletedAt,
		&notifiedAt,
		&rawHeaders,
		&msg.CreatedAt,
	)
	if err != nil {
		return domain.InboundMessage{}, err
	}
	msg.MessageID = fromNullString(messageID)
	msg.InReplyTo = fromNullString(inReplyTo)
	msg.ReferencesHeader = fromNullString(referencesHeader)
	msg.FromName = fromNullString(fromName)
	msg.ToEmail = fromNullString(toEmail)
	msg.Subject = fromNullString(subject)
	msg.RawHeadersJSON = fromNullString(rawHeaders)
	msg.ContactID = fromNullInt64(contactID)
	msg.CampaignID = fromNullInt64(campaignID)
	msg.ReadAt = fromNullTime(readAt)
	msg.RepliedAt = fromNullTime(repliedAt)
	msg.DeletedAt = fromNullTime(deletedAt)
	msg.NotifiedAt = fromNullTime(notifiedAt)
	return msg, nil
}

func toNullInt64Ptr(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

func fromNullInt64(i sql.NullInt64) *int64 {
	if !i.Valid {
		return nil
	}
	v := i.Int64
	return &v
}

func fromNullString(s sql.NullString) string {
	if !s.Valid {
		return ""
	}
	return s.String
}
