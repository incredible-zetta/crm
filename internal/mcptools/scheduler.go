package mcptools

import (
	"context"
	"time"

	"github.com/cipta/crm-for-aiagents/internal/db"
	"github.com/cipta/crm-for-aiagents/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ScheduleTaskIn struct {
	Kind    string         `json:"kind" jsonschema:"required,description=The kind of task (email or campaign)"`
	Payload map[string]any `json:"payload" jsonschema:"description=JSON payload for the task"`
	RunAt   string         `json:"run_at" jsonschema:"required,description=Execution time in RFC3339 format"`
}

type ScheduleTaskOut struct {
	TaskID int64  `json:"task_id"`
	Status string `json:"status"` // "pending"
}

func isValidTaskKind(kind string) bool {
	for _, k := range db.ValidTaskKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func (d *Deps) ScheduleTask(ctx context.Context, req *mcp.CallToolRequest, in ScheduleTaskIn) (*mcp.CallToolResult, ScheduleTaskOut, error) {
	if !isValidTaskKind(in.Kind) {
		return mcpserver.Err("bad_kind", "invalid task kind"), ScheduleTaskOut{}, nil
	}

	parsedTime, err := time.Parse(time.RFC3339, in.RunAt)
	if err != nil {
		return mcpserver.Err("invalid_run_at", err.Error()), ScheduleTaskOut{}, nil
	}

	taskID, err := d.Repo.InsertTask(ctx, db.ScheduledTask{
		Kind:    in.Kind,
		Payload: in.Payload,
		RunAt:   parsedTime,
		Status:  "pending",
	})
	if err != nil {
		return mcpserver.Err("insert_task_failed", err.Error()), ScheduleTaskOut{}, nil
	}

	return nil, ScheduleTaskOut{
		TaskID: taskID,
		Status: "pending",
	}, nil
}
