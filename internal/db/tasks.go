package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type ScheduledTask struct {
	ID        int64
	Kind      string
	Payload   map[string]any // stored as JSON
	RunAt     time.Time
	Status    string
	Attempts  int
	LastError string
	CreatedAt time.Time
}

var ValidTaskKinds = []string{"email", "campaign"}
var ValidTaskStatuses = []string{"pending", "running", "done", "failed"}

func validTaskKind(k string) bool {
	for _, v := range ValidTaskKinds {
		if k == v {
			return true
		}
	}
	return false
}

func validTaskStatus(s string) bool {
	for _, v := range ValidTaskStatuses {
		if s == v {
			return true
		}
	}
	return false
}

func (r *Repo) InsertTask(ctx context.Context, t ScheduledTask) (int64, error) {
	if t.Status == "" {
		t.Status = "pending"
	}

	if !validTaskKind(t.Kind) {
		return 0, fmt.Errorf("invalid task kind: %q", t.Kind)
	}
	if !validTaskStatus(t.Status) {
		return 0, fmt.Errorf("invalid task status: %q", t.Status)
	}

	var payloadJSON []byte
	if t.Payload != nil {
		var err error
		payloadJSON, err = json.Marshal(t.Payload)
		if err != nil {
			return 0, fmt.Errorf("marshal payload: %w", err)
		}
	} else {
		payloadJSON = []byte("{}")
	}

	query := `INSERT INTO scheduled_tasks (kind, payload, run_at, status, attempts, last_error) VALUES (?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, t.Kind, payloadJSON, t.RunAt, t.Status, t.Attempts, t.LastError)
	if err != nil {
		return 0, fmt.Errorf("insert task: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}

	return id, nil
}

func (r *Repo) ClaimDue(ctx context.Context, now time.Time, limit int) ([]ScheduledTask, error) {
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

	var tasks []ScheduledTask
	var ids []int64

	for rows.Next() {
		var t ScheduledTask
		var (
			payloadBuf []byte
			lastError  sql.NullString
		)
		err := rows.Scan(
			&t.ID,
			&t.Kind,
			&payloadBuf,
			&t.RunAt,
			&t.Status,
			&t.Attempts,
			&lastError,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

		t.LastError = lastError.String
		if payloadBuf != nil {
			if err := json.Unmarshal(payloadBuf, &t.Payload); err != nil {
				return nil, fmt.Errorf("unmarshal payload: %w", err)
			}
		}
		if t.Payload == nil {
			t.Payload = make(map[string]any)
		}

		t.Status = "running"
		tasks = append(tasks, t)
		ids = append(ids, t.ID)
	}
	rows.Close()

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

func (r *Repo) MarkDone(ctx context.Context, id int64) error {
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
			return fmt.Errorf("task not found: %w", ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("check task existence: %w", err)
		}
	}

	return nil
}

func (r *Repo) MarkFailed(ctx context.Context, id int64, errMsg string) error {
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
			return fmt.Errorf("task not found: %w", ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("check task existence: %w", err)
		}
	}

	return nil
}
