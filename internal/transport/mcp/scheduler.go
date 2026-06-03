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
