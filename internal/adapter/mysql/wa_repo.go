package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

type waMessageRepo struct {
	db *sql.DB
}

var _ port.WAMessageRepo = (*waMessageRepo)(nil)

func waSelectSQL() string {
	return `SELECT id, message_id, chat_id, direction, phone, sender_name, contact_id, body, media_type, media_url, media_caption,
		status, error, replied_to, sent_at, delivered_at, read_at, received_at, notified_at, replied_at, deleted_at, created_at
		FROM wa_messages`
}

func (r *waMessageRepo) Insert(ctx context.Context, msg domain.WAMessage) (domain.WAMessage, bool, error) {
	// When a gateway message_id is present, INSERT IGNORE makes ingest
	// idempotent (webhook re-delivery). Outbound rows always have an id from
	// the send response; inbound rows carry the wamid.
	query := `INSERT IGNORE INTO wa_messages
		(message_id, chat_id, direction, phone, sender_name, contact_id, body, media_type, media_url, media_caption,
		 status, error, replied_to, sent_at, delivered_at, read_at, received_at, notified_at, replied_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query,
		toNullString(msg.MessageID),
		toNullString(msg.ChatID),
		string(msg.Direction),
		msg.Phone,
		toNullString(msg.SenderName),
		toNullInt64Ptr(msg.ContactID),
		msg.Body,
		string(msg.MediaType),
		toNullString(msg.MediaURL),
		toNullString(msg.MediaCaption),
		string(msg.Status),
		toNullString(msg.Error),
		toNullString(msg.RepliedTo),
		toNullTime(msg.SentAt),
		toNullTime(msg.DeliveredAt),
		toNullTime(msg.ReadAt),
		toNullTime(msg.ReceivedAt),
		toNullTime(msg.NotifiedAt),
		toNullTime(msg.RepliedAt),
		toNullTime(msg.DeletedAt),
	)
	if err != nil {
		return domain.WAMessage{}, false, fmt.Errorf("insert wa message: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return domain.WAMessage{}, false, fmt.Errorf("insert wa message rows affected: %w", err)
	}
	if rows == 0 {
		// Duplicate message_id: fetch the existing row.
		existing, gerr := r.GetByMessageID(ctx, msg.MessageID)
		return existing, false, gerr
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.WAMessage{}, false, fmt.Errorf("insert wa message last id: %w", err)
	}
	msg.ID = id
	return msg, true, nil
}

func (r *waMessageRepo) Get(ctx context.Context, id int64) (domain.WAMessage, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, waSelectSQL()+` WHERE id = ? AND deleted_at IS NULL`, id))
}

func (r *waMessageRepo) GetByMessageID(ctx context.Context, messageID string) (domain.WAMessage, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, waSelectSQL()+` WHERE message_id = ?`, messageID))
}

func (r *waMessageRepo) List(ctx context.Context, f domain.WAInboundFilter, p port.Paging) (port.WAMessagePage, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	where := []string{"deleted_at IS NULL"}
	args := []any{}
	if p.Cursor > 0 {
		where = append(where, "id < ?")
		args = append(args, p.Cursor)
	}
	if f.Direction == "in" || f.Direction == "out" {
		where = append(where, "direction = ?")
		args = append(args, f.Direction)
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
	if f.Phone != "" {
		where = append(where, "phone = ?")
		args = append(args, f.Phone)
	}
	if f.ChatID != "" {
		where = append(where, "chat_id = ?")
		args = append(args, f.ChatID)
	}
	if f.Since != nil {
		where = append(where, "created_at >= ?")
		args = append(args, f.Since.UTC())
	}
	if f.Until != nil {
		where = append(where, "created_at <= ?")
		args = append(args, f.Until.UTC())
	}
	whereSQL := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wa_messages`+whereSQL, args...).Scan(&total); err != nil {
		return port.WAMessagePage{}, fmt.Errorf("count wa messages: %w", err)
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit+1)
	rows, err := r.db.QueryContext(ctx, waSelectSQL()+whereSQL+` ORDER BY id DESC LIMIT ?`, queryArgs...)
	if err != nil {
		return port.WAMessagePage{}, fmt.Errorf("list wa messages: %w", err)
	}
	defer rows.Close()

	var items []domain.WAMessage
	for rows.Next() {
		msg, err := scanWAMessage(rows)
		if err != nil {
			return port.WAMessagePage{}, err
		}
		items = append(items, msg)
	}
	if err := rows.Err(); err != nil {
		return port.WAMessagePage{}, fmt.Errorf("list wa messages rows: %w", err)
	}

	var next int64
	if len(items) > limit {
		next = items[limit-1].ID
		items = items[:limit]
	}
	return port.WAMessagePage{Items: items, Total: total, NextCursor: next}, nil
}

// UpdateStatus advances the delivery lifecycle of an outbound message. Status
// only moves forward (sent -> delivered -> read); a stale or out-of-order
// receipt never downgrades. The relevant timestamp column is set when first
// reached.
func (r *waMessageRepo) UpdateStatus(ctx context.Context, messageID string, status domain.WAMessageStatus, at time.Time) error {
	if messageID == "" {
		return fmt.Errorf("update wa status: empty message id")
	}
	var query string
	var args []any
	switch status {
	case domain.WAStatusDelivered:
		// Only sent -> delivered; never downgrade read back to delivered.
		query = `UPDATE wa_messages SET status = 'delivered', delivered_at = COALESCE(delivered_at, ?)
			WHERE message_id = ? AND status = 'sent'`
		args = []any{at, messageID}
	case domain.WAStatusRead:
		query = `UPDATE wa_messages SET status = 'read', read_at = COALESCE(read_at, ?),
			delivered_at = COALESCE(delivered_at, ?)
			WHERE message_id = ? AND status IN ('sent','delivered')`
		args = []any{at, at, messageID}
	case domain.WAStatusFailed:
		query = `UPDATE wa_messages SET status = 'failed' WHERE message_id = ? AND status = 'sent'`
		args = []any{messageID}
	default:
		query = `UPDATE wa_messages SET status = ? WHERE message_id = ?`
		args = []any{string(status), messageID}
	}
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update wa status: %w", err)
	}
	return nil
}

func (r *waMessageRepo) MarkRead(ctx context.Context, id int64, at *time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wa_messages SET read_at = ? WHERE id = ? AND deleted_at IS NULL`, toNullTime(at), id)
	if err != nil {
		return fmt.Errorf("mark wa read: %w", err)
	}
	return nil
}

func (r *waMessageRepo) MarkNotified(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wa_messages SET notified_at = ? WHERE id = ? AND deleted_at IS NULL`, at, id)
	if err != nil {
		return fmt.Errorf("mark wa notified: %w", err)
	}
	return nil
}

// MarkReplied sets the replied_at timestamp for an inbound message.
func (r *waMessageRepo) MarkReplied(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wa_messages SET replied_at = ? WHERE id = ? AND deleted_at IS NULL`, at, id)
	if err != nil {
		return fmt.Errorf("mark wa replied: %w", err)
	}
	return nil
}

// SetRepliedTo links an outbound message to the inbound message it replies to.
func (r *waMessageRepo) SetRepliedTo(ctx context.Context, outboundID int64, inboundMessageID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wa_messages SET replied_to = ? WHERE id = ? AND deleted_at IS NULL`, inboundMessageID, outboundID)
	if err != nil {
		return fmt.Errorf("set wa replied_to: %w", err)
	}
	return nil
}

func (r *waMessageRepo) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE wa_messages SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete wa message: %w", err)
	}
	return nil
}

func (r *waMessageRepo) CountSentSince(ctx context.Context, phone string, since time.Time) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wa_messages WHERE direction = 'out' AND phone = ? AND created_at >= ?`,
		phone, since).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count wa sent since: %w", err)
	}
	return n, nil
}

func (r *waMessageRepo) CountSentSinceAll(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wa_messages WHERE direction = 'out' AND created_at >= ?`,
		since).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count wa sent since all: %w", err)
	}
	return n, nil
}

func (r *waMessageRepo) scanOne(row rowScanner) (domain.WAMessage, error) {
	msg, err := scanWAMessage(row)
	if err == sql.ErrNoRows {
		return domain.WAMessage{}, fmt.Errorf("wa message not found: %w", domain.ErrNotFound)
	}
	return msg, err
}

func scanWAMessage(scanner rowScanner) (domain.WAMessage, error) {
	var msg domain.WAMessage
	var messageID, chatID, mediaURL, mediaCaption, errStr, repliedTo sql.NullString
	var senderName sql.NullString
	var direction, status, mediaType string
	var contactID sql.NullInt64
	var sentAt, deliveredAt, readAt, receivedAt, notifiedAt, repliedAt, deletedAt sql.NullTime
	err := scanner.Scan(
		&msg.ID,
		&messageID,
		&chatID,
		&direction,
		&msg.Phone,
		&senderName,
		&contactID,
		&msg.Body,
		&mediaType,
		&mediaURL,
		&mediaCaption,
		&status,
		&errStr,
		&repliedTo,
		&sentAt,
		&deliveredAt,
		&readAt,
		&receivedAt,
		&notifiedAt,
		&repliedAt,
		&deletedAt,
		&msg.CreatedAt,
	)
	if err != nil {
		return domain.WAMessage{}, err
	}
	msg.MessageID = fromNullString(messageID)
	msg.ChatID = fromNullString(chatID)
	msg.SenderName = fromNullString(senderName)
	msg.Direction = domain.WADirection(direction)
	msg.ContactID = fromNullInt64(contactID)
	msg.MediaType = domain.WAMediaType(mediaType)
	msg.MediaURL = fromNullString(mediaURL)
	msg.MediaCaption = fromNullString(mediaCaption)
	msg.Status = domain.WAMessageStatus(status)
	msg.Error = fromNullString(errStr)
	msg.RepliedTo = fromNullString(repliedTo)
	msg.SentAt = fromNullTime(sentAt)
	msg.DeliveredAt = fromNullTime(deliveredAt)
	msg.ReadAt = fromNullTime(readAt)
	msg.ReceivedAt = fromNullTime(receivedAt)
	msg.NotifiedAt = fromNullTime(notifiedAt)
	msg.RepliedAt = fromNullTime(repliedAt)
	msg.DeletedAt = fromNullTime(deletedAt)
	return msg, nil
}
