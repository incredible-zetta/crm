package mcptransport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ScheduleTaskIn struct {
	Kind    string         `json:"kind" jsonschema:"The kind of task (email or campaign)"`
	Payload map[string]any `json:"payload" jsonschema:"JSON payload for the task"`
	RunAt   string         `json:"run_at" jsonschema:"Execution time in RFC3339 format"`
}

type ScheduleTaskOut struct {
	TaskID int64  `json:"task_id"`
	Status string `json:"status"` // "pending"
}

type TaskListIn struct {
	Status string `json:"status,omitempty" jsonschema:"Filter task by status (pending, running, done, failed, cancelled)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Max tasks to return (default 50, capped at 200)"`
}

type TaskListOut struct {
	Count int              `json:"count"`
	Items []map[string]any `json:"items"`
}

type TaskCancelIn struct {
	ID int64 `json:"id" jsonschema:"ID of the task to cancel"`
}

type TaskCancelOut struct {
	ID        int64 `json:"id"`
	Cancelled bool  `json:"cancelled"`
}

func (d *Deps) ScheduleTask(ctx context.Context, req *mcp.CallToolRequest, in ScheduleTaskIn) (*mcp.CallToolResult, ScheduleTaskOut, error) {
	parsedTime, err := time.Parse(time.RFC3339, in.RunAt)
	if err != nil {
		return mcpserver.Err("invalid_input", "invalid RFC3339 format"), ScheduleTaskOut{}, nil
	}

	id, err := d.Svc.Task.Schedule(ctx, in.Kind, in.Payload, parsedTime)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			msg := err.Error()
			msg = strings.TrimPrefix(msg, "validation error: ")
			return mcpserver.Err("invalid_input", msg), ScheduleTaskOut{}, nil
		}
		return nil, ScheduleTaskOut{}, fmt.Errorf("schedule_task: %w", err)
	}

	return nil, ScheduleTaskOut{
		TaskID: id,
		Status: "pending",
	}, nil
}

func (d *Deps) TaskList(ctx context.Context, req *mcp.CallToolRequest, in TaskListIn) (*mcp.CallToolResult, TaskListOut, error) {
	list, err := d.Svc.Task.List(ctx, in.Status, in.Limit)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return mcpserver.Err("invalid_input", err.Error()), TaskListOut{}, nil
		}
		return nil, TaskListOut{}, fmt.Errorf("task_list: %w", err)
	}

	var items []map[string]any
	for _, t := range list {
		item := map[string]any{
			"id":       t.ID,
			"kind":     string(t.Kind),
			"status":   string(t.Status),
			"run_at":   t.RunAt.Format(time.RFC3339),
			"attempts": t.Attempts,
		}
		if t.LastError != "" {
			item["last_error"] = t.LastError
		}
		items = append(items, item)
	}

	return nil, TaskListOut{
		Count: len(items),
		Items: items,
	}, nil
}

func (d *Deps) TaskCancel(ctx context.Context, req *mcp.CallToolRequest, in TaskCancelIn) (*mcp.CallToolResult, TaskCancelOut, error) {
	err := d.Svc.Task.Cancel(ctx, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return mcpserver.Err("not_found", "task not found"), TaskCancelOut{}, nil
		}
		if errors.Is(err, domain.ErrConflict) {
			return mcpserver.Err("conflict", "task not pending"), TaskCancelOut{}, nil
		}
		return nil, TaskCancelOut{}, fmt.Errorf("task_cancel: %w", err)
	}

	return nil, TaskCancelOut{
		ID:        in.ID,
		Cancelled: true,
	}, nil
}
