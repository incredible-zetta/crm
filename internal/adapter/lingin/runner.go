// Package lingin is the LinkedIn adapter. It shells out to the private `lingin`
// binary (https://github.com/incredible-zetta/lingin-utils) running as an MCP
// stdio server, and speaks MCP to it via the official go-sdk CommandTransport.
//
// The lingin binary is pre-authorized two ways:
//   - a build-time key baked in via -ldflags; the CRM must present the same
//     value in LINGIN_MCP_KEY or the binary refuses to start.
//   - LinkedIn sessions stored as rows in the SAME MySQL database the CRM uses,
//     under an isolated `lingin_` table prefix. Accounts are selected per call
//     by label (mapped from the CRM tenant), so one binary serves many tenants.
//
// This adapter holds no LinkedIn session state; it spawns a fresh child per
// call, lists/calls a tool, and tears the child down.
package lingin

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/tenant"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config configures the lingin runner.
type Config struct {
	// BinPath is the path to the lingin binary. Required.
	BinPath string
	// DSN is the MySQL DSN the binary uses to load stored LinkedIn accounts.
	// Should point at the same database as the CRM (isolated by lingin_ prefix).
	DSN string
	// MCPKey must match the build-time key baked into the binary; passed via
	// LINGIN_MCP_KEY. Empty only for unlocked dev builds.
	MCPKey string
	// Tenant scopes stored accounts. Defaults to "default".
	Tenant string
	// Timeout bounds a single call (spawn + list/call + teardown). Default 60s.
	Timeout time.Duration
}

// Runner executes the lingin binary as a short-lived MCP subprocess. It is
// stateless; all session data lives in the binary's MySQL store.
type Runner struct {
	cfg Config
}

// New returns a lingin runner. BinPath and DSN are required.
func New(cfg Config) (*Runner, error) {
	if strings.TrimSpace(cfg.BinPath) == "" {
		return nil, fmt.Errorf("lingin binary path required")
	}
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("lingin DSN required")
	}
	if cfg.Tenant == "" {
		cfg.Tenant = "default"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &Runner{cfg: cfg}, nil
}

// spawn builds the CommandTransport for a fresh lingin MCP child. Env carries
// the build-key gate, DB DSN, and tenant scope; account selection is per call.
func (r *Runner) spawn(ctx context.Context, account string) *mcp.CommandTransport {
	// Tenant flows per request from the CRM context (multi-tenant), falling back
	// to the runner default. It scopes which stored LinkedIn accounts are visible.
	tnt := tenant.From(ctx)
	if tnt == "" {
		tnt = r.cfg.Tenant
	}
	cmd := exec.CommandContext(ctx, r.cfg.BinPath, "mcp",
		"-dsn", r.cfg.DSN,
		"-tenant", tnt,
	)
	env := []string{
		"LINGIN_DSN=" + r.cfg.DSN,
		"LINGIN_TENANT=" + tnt,
	}
	if r.cfg.MCPKey != "" {
		env = append(env, "LINGIN_MCP_KEY="+r.cfg.MCPKey)
	}
	if account != "" {
		env = append(env, "LINGIN_ACCOUNT="+account)
	}
	// Do NOT inherit the parent env: the CRM process may carry unrelated
	// secrets, and lingin needs only the vars above.
	cmd.Env = env
	return &mcp.CommandTransport{Command: cmd}
}

// call connects to a fresh lingin child, invokes one tool, and returns the
// concatenated text content. The account (LinkedIn identity label) is passed
// both via env default and as an explicit "account" tool argument so it wins
// even if a default is configured.
func (r *Runner) call(ctx context.Context, account, tool string, args map[string]any) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "crm", Version: "1.0.0"}, nil)
	sess, err := client.Connect(ctx, r.spawn(ctx, account), nil)
	if err != nil {
		return "", fmt.Errorf("lingin connect: %w", err)
	}
	defer sess.Close()

	if args == nil {
		args = map[string]any{}
	}
	if account != "" {
		args["account"] = account
	}
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return "", fmt.Errorf("lingin call %s: %w", tool, err)
	}
	text := extractText(res)
	if res.IsError {
		return text, fmt.Errorf("lingin tool %s error: %s", tool, text)
	}
	return text, nil
}

// extractText joins all text content blocks of a tool result.
func extractText(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
