package mcptools

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HealthCheckIn struct{}

type HealthCheckOut struct {
	Status    string `json:"status"`
	DBOk      bool   `json:"db_ok"`
	SMTPOk    *bool  `json:"smtp_ok,omitempty"`
	MailgunOk *bool  `json:"mailgun_ok,omitempty"`
	Version   string `json:"version"`
	Time      string `json:"time"`
}

func (d *Deps) HealthCheck(ctx context.Context, req *mcp.CallToolRequest, in HealthCheckIn) (*mcp.CallToolResult, HealthCheckOut, error) {
	dbOk := true
	if d.PingDB != nil {
		if err := d.PingDB(ctx); err != nil {
			dbOk = false
		}
	}

	var smtpOk *bool
	if d.PingSMTP != nil {
		ok := true
		if err := d.PingSMTP(ctx); err != nil {
			ok = false
		}
		smtpOk = &ok
	}

	var mailgunOk *bool
	if d.PingMailgun != nil {
		ok := true
		if err := d.PingMailgun(ctx); err != nil {
			ok = false
		}
		mailgunOk = &ok
	}

	status := "ok"
	if !dbOk || (smtpOk != nil && !*smtpOk) || (mailgunOk != nil && !*mailgunOk) {
		status = "degraded"
	}

	out := HealthCheckOut{
		Status:    status,
		DBOk:      dbOk,
		SMTPOk:    smtpOk,
		MailgunOk: mailgunOk,
		Version:   d.Version,
		Time:      time.Now().Format(time.RFC3339),
	}

	return nil, out, nil
}
