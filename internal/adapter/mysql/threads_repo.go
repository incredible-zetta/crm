package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

type threadsRepo struct{ db *sql.DB }

var _ port.ThreadsRepo = (*threadsRepo)(nil)

func threadsPostSelectSQL() string {
	return `SELECT id, threads_id, media_product_type, media_type, text, permalink, timestamp, username, topic_tag, is_quote_post, raw_json, deleted_at, created_at, updated_at FROM threads_posts`
}

func (r *threadsRepo) UpsertPost(ctx context.Context, post domain.ThreadsPost) (domain.ThreadsPost, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO threads_posts
		(threads_id, media_product_type, media_type, text, permalink, timestamp, username, topic_tag, is_quote_post, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE media_product_type=VALUES(media_product_type), media_type=VALUES(media_type), text=VALUES(text), permalink=VALUES(permalink), timestamp=VALUES(timestamp), username=VALUES(username), topic_tag=VALUES(topic_tag), is_quote_post=VALUES(is_quote_post), raw_json=VALUES(raw_json), deleted_at=NULL`,
		post.ThreadsID, post.MediaProductType, post.MediaType, post.Text, post.Permalink, toNullTime(post.Timestamp), post.Username, toNullString(post.TopicTag), post.IsQuotePost, toNullJSON(post.RawJSON))
	if err != nil {
		return domain.ThreadsPost{}, fmt.Errorf("upsert threads post: %w", err)
	}
	return r.GetPostByThreadsID(ctx, post.ThreadsID)
}

func (r *threadsRepo) GetPost(ctx context.Context, id int64) (domain.ThreadsPost, error) {
	return r.scanPost(r.db.QueryRowContext(ctx, threadsPostSelectSQL()+` WHERE id = ? AND deleted_at IS NULL`, id))
}

func (r *threadsRepo) GetPostByThreadsID(ctx context.Context, threadsID string) (domain.ThreadsPost, error) {
	return r.scanPost(r.db.QueryRowContext(ctx, threadsPostSelectSQL()+` WHERE threads_id = ? AND deleted_at IS NULL`, threadsID))
}

func (r *threadsRepo) ListPosts(ctx context.Context, f domain.ThreadsListFilter, p port.Paging) (port.ThreadsPostPage, error) {
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
	if f.Username != "" {
		where = append(where, "username = ?")
		args = append(args, f.Username)
	}
	if f.Q != "" {
		where = append(where, "text LIKE ?")
		args = append(args, "%"+f.Q+"%")
	}
	whereSQL := " WHERE " + strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM threads_posts`+whereSQL, args...).Scan(&total); err != nil {
		return port.ThreadsPostPage{}, fmt.Errorf("count threads posts: %w", err)
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit+1)
	rows, err := r.db.QueryContext(ctx, threadsPostSelectSQL()+whereSQL+` ORDER BY id DESC LIMIT ?`, queryArgs...)
	if err != nil {
		return port.ThreadsPostPage{}, fmt.Errorf("list threads posts: %w", err)
	}
	defer rows.Close()
	var items []domain.ThreadsPost
	for rows.Next() {
		post, err := scanThreadsPost(rows)
		if err != nil {
			return port.ThreadsPostPage{}, err
		}
		items = append(items, post)
	}
	if err := rows.Err(); err != nil {
		return port.ThreadsPostPage{}, err
	}
	var next int64
	if len(items) > limit {
		next = items[limit-1].ID
		items = items[:limit]
	}
	return port.ThreadsPostPage{Items: items, Total: total, NextCursor: next}, nil
}

func (r *threadsRepo) SoftDeletePost(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE threads_posts SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete threads post: %w", err)
	}
	return nil
}

func (r *threadsRepo) UpsertReply(ctx context.Context, reply domain.ThreadsReply) (domain.ThreadsReply, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO threads_replies (reply_id, post_id, text, username, timestamp, hide_status, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE post_id=VALUES(post_id), text=VALUES(text), username=VALUES(username), timestamp=VALUES(timestamp), hide_status=VALUES(hide_status), raw_json=VALUES(raw_json)`,
		reply.ReplyID, reply.PostID, reply.Text, reply.Username, toNullTime(reply.Timestamp), reply.HideStatus, toNullJSON(reply.RawJSON))
	if err != nil {
		return domain.ThreadsReply{}, fmt.Errorf("upsert threads reply: %w", err)
	}
	return reply, nil
}

func (r *threadsRepo) UpsertMention(ctx context.Context, mention domain.ThreadsMention) (domain.ThreadsMention, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO threads_mentions (mention_id, text, username, permalink, timestamp, raw_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE text=VALUES(text), username=VALUES(username), permalink=VALUES(permalink), timestamp=VALUES(timestamp), raw_json=VALUES(raw_json)`,
		mention.MentionID, mention.Text, mention.Username, mention.Permalink, toNullTime(mention.Timestamp), toNullJSON(mention.RawJSON))
	if err != nil {
		return domain.ThreadsMention{}, fmt.Errorf("upsert threads mention: %w", err)
	}
	return mention, nil
}

func (r *threadsRepo) InsertAudit(ctx context.Context, event domain.ThreadsAuditEvent) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO threads_audit_events (action, object_id, ok, error, raw_json) VALUES (?, ?, ?, ?, ?)`, event.Action, toNullString(event.ObjectID), event.OK, toNullString(event.Error), toNullJSON(event.RawJSON))
	if err != nil {
		return fmt.Errorf("insert threads audit: %w", err)
	}
	return nil
}

func (r *threadsRepo) ListAudit(ctx context.Context, p port.Paging) (port.ThreadsAuditPage, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	where := ""
	args := []any{}
	if p.Cursor > 0 {
		where = " WHERE id < ?"
		args = append(args, p.Cursor)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM threads_audit_events`+where, args...).Scan(&total); err != nil {
		return port.ThreadsAuditPage{}, fmt.Errorf("count threads audit: %w", err)
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit+1)
	rows, err := r.db.QueryContext(ctx, `SELECT id, action, object_id, ok, error, raw_json, created_at FROM threads_audit_events`+where+` ORDER BY id DESC LIMIT ?`, queryArgs...)
	if err != nil {
		return port.ThreadsAuditPage{}, fmt.Errorf("list threads audit: %w", err)
	}
	defer rows.Close()
	var items []domain.ThreadsAuditEvent
	for rows.Next() {
		var item domain.ThreadsAuditEvent
		var objectID, errStr sql.NullString
		var raw []byte
		if err := rows.Scan(&item.ID, &item.Action, &objectID, &item.OK, &errStr, &raw, &item.CreatedAt); err != nil {
			return port.ThreadsAuditPage{}, err
		}
		item.ObjectID = objectID.String
		item.Error = errStr.String
		item.RawJSON = raw
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return port.ThreadsAuditPage{}, err
	}
	var next int64
	if len(items) > limit {
		next = items[limit-1].ID
		items = items[:limit]
	}
	return port.ThreadsAuditPage{Items: items, Total: total, NextCursor: next}, nil
}

func (r *threadsRepo) scanPost(row rowScanner) (domain.ThreadsPost, error) {
	post, err := scanThreadsPost(row)
	if err == sql.ErrNoRows {
		return domain.ThreadsPost{}, fmt.Errorf("threads post not found: %w", domain.ErrNotFound)
	}
	return post, err
}

func scanThreadsPost(scanner rowScanner) (domain.ThreadsPost, error) {
	var post domain.ThreadsPost
	var mediaProductType, mediaType, text, permalink, username, topicTag sql.NullString
	var ts, deletedAt sql.NullTime
	var raw []byte
	if err := scanner.Scan(&post.ID, &post.ThreadsID, &mediaProductType, &mediaType, &text, &permalink, &ts, &username, &topicTag, &post.IsQuotePost, &raw, &deletedAt, &post.CreatedAt, &post.UpdatedAt); err != nil {
		return domain.ThreadsPost{}, err
	}
	post.MediaProductType = mediaProductType.String
	post.MediaType = mediaType.String
	post.Text = text.String
	post.Permalink = permalink.String
	post.Timestamp = fromNullTime(ts)
	post.Username = username.String
	post.TopicTag = topicTag.String
	post.RawJSON = raw
	post.DeletedAt = fromNullTime(deletedAt)
	return post, nil
}

func toNullJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
