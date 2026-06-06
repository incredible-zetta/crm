package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

type taskRepo struct {
	db *sql.DB
}

var _ port.TaskRepo = (*taskRepo)(nil)

func (r *taskRepo) Insert(ctx context.Context, t domain.ScheduledTask) (int64, error) {
	if t.Status == "" {
		t.Status = domain.TaskPending
	}

	if !t.Kind.Valid() {
		return 0, fmt.Errorf("%w: invalid task kind: %q", domain.ErrValidation, t.Kind)
	}
	if !t.Status.Valid() {
		return 0, fmt.Errorf("%w: invalid task status: %q", domain.ErrValidation, t.Status)
	}

	payloadJSON, err := marshalJSON(t.Payload, "{}")
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}

	query := `INSERT INTO scheduled_tasks (kind, payload, run_at, status, attempts, last_error) VALUES (?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, string(t.Kind), payloadJSON, t.RunAt, string(t.Status), t.Attempts, toNullString(t.LastError))
	if err != nil {
		return 0, fmt.Errorf("insert task exec: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}

	return id, nil
}

func (r *taskRepo) List(ctx context.Context, status string, limit int) ([]domain.ScheduledTask, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	var query string
	var args []any

	if status == "" {
		query = `SELECT id, kind, payload, run_at, status, attempts, last_error, created_at 
FROM scheduled_tasks 
ORDER BY CASE WHEN status IN ('pending', 'running') THEN 0 ELSE 1 END ASC, run_at ASC 
LIMIT ?`
		args = append(args, limit)
	} else {
		query = `SELECT id, kind, payload, run_at, status, attempts, last_error, created_at 
FROM scheduled_tasks 
WHERE status = ? 
ORDER BY run_at ASC 
LIMIT ?`
		args = append(args, status, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []domain.ScheduledTask
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tasks rows: %w", err)
	}

	return tasks, nil
}

func (r *taskRepo) ClaimDue(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledTask, error) {
	if limit <= 0 {
		limit = 10
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `SELECT id, kind, payload, run_at, status, attempts, last_error, created_at 
FROM scheduled_tasks 
WHERE status = 'pending' AND run_at <= ? 
ORDER BY run_at ASC, id ASC 
LIMIT ? 
FOR UPDATE SKIP LOCKED`

	rows, err := tx.QueryContext(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("select tasks for update: %w", err)
	}
	defer rows.Close()

	var tasks []domain.ScheduledTask
	var ids []int64

	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		t.Status = domain.TaskRunning
		tasks = append(tasks, t)
		ids = append(ids, t.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim due rows: %w", err)
	}

	if len(ids) == 0 {
		return nil, nil
	}

	updateQuery := `UPDATE scheduled_tasks SET status = 'running' WHERE id IN (`
	args := []any{}
	for i, id := range ids {
		if i > 0 {
			updateQuery += ","
		}
		updateQuery += "?"
		args = append(args, id)
	}
	updateQuery += ")"

	_, err = tx.ExecContext(ctx, updateQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("update tasks to running: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return tasks, nil
}

func (r *taskRepo) MarkDone(ctx context.Context, id int64) error {
	query := `UPDATE scheduled_tasks SET status = 'done' WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark done: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		var exists int
		err = r.db.QueryRowContext(ctx, "SELECT 1 FROM scheduled_tasks WHERE id = ?", id).Scan(&exists)
		if err == sql.ErrNoRows {
			return fmt.Errorf("task not found: %w", domain.ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("check task existence: %w", err)
		}
	}

	return nil
}

func (r *taskRepo) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	query := `UPDATE scheduled_tasks SET status = 'failed', attempts = attempts + 1, last_error = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, errMsg, id)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		var exists int
		err = r.db.QueryRowContext(ctx, "SELECT 1 FROM scheduled_tasks WHERE id = ?", id).Scan(&exists)
		if err == sql.ErrNoRows {
			return fmt.Errorf("task not found: %w", domain.ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("check task existence: %w", err)
		}
	}

	return nil
}

func (r *taskRepo) HasActiveCampaignTask(ctx context.Context, campaignID int64) (bool, error) {
	query := `SELECT 1 FROM scheduled_tasks
WHERE kind = 'campaign'
  AND status IN ('pending', 'running')
  AND JSON_EXTRACT(payload, '$.campaign_id') = ?
LIMIT 1`
	var exists int
	err := r.db.QueryRowContext(ctx, query, campaignID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("has active campaign task: %w", err)
	}
	return true, nil
}

func (r *taskRepo) Cancel(ctx context.Context, id int64) error {
	var status string
	err := r.db.QueryRowContext(ctx, "SELECT status FROM scheduled_tasks WHERE id = ?", id).Scan(&status)
	if err == sql.ErrNoRows {
		return fmt.Errorf("task not found: %w", domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("get task status: %w", err)
	}

	if status != string(domain.TaskPending) {
		return fmt.Errorf("task not pending (status: %s): %w", status, domain.ErrConflict)
	}

	query := `UPDATE scheduled_tasks SET status = 'cancelled' WHERE id = ? AND status = 'pending'`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("cancel task: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("task status changed during cancel execution: %w", domain.ErrConflict)
	}

	return nil
}

func scanTask(row interface{ Scan(dest ...any) error }) (domain.ScheduledTask, error) {
	var t domain.ScheduledTask
	var (
		payloadBuf []byte
		lastError  sql.NullString
	)

	err := row.Scan(
		&t.ID,
		(*string)(&t.Kind),
		&payloadBuf,
		&t.RunAt,
		(*string)(&t.Status),
		&t.Attempts,
		&lastError,
		&t.CreatedAt,
	)
	if err != nil {
		return domain.ScheduledTask{}, err
	}

	t.LastError = lastError.String

	if payloadBuf != nil {
		if err := unmarshalJSON(payloadBuf, &t.Payload); err != nil {
			return domain.ScheduledTask{}, fmt.Errorf("unmarshal payload: %w", err)
		}
	}
	if t.Payload == nil {
		t.Payload = make(map[string]any)
	}

	return t, nil
}
