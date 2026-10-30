package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"example.com/shell-mcp/internal/executor"
	"example.com/shell-mcp/internal/policy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ServiceInput struct {
	Service string `json:"service"`
}
type LogsInput struct {
	Service string `json:"service"`
	Lines   *int   `json:"lines,omitempty" jsonschema:"Number of lines, 1 to 200; defaults to 50"`
}

func (t *Tools) ServiceStatus(ctx context.Context, _ *mcp.CallToolRequest, in ServiceInput) (*mcp.CallToolResult, executor.Result, error) {
	if err := policy.Service(in.Service); err != nil {
		return nil, executor.Result{}, err
	}
	startOperation(ctx)
	r, err := t.Runner.Run(ctx, "/usr/bin/systemctl", []string{"status", "--no-pager", "--", in.Service}, "")
	commandOutcome(ctx, r, err)
	return nil, r, err
}
func (t *Tools) ServiceLogs(ctx context.Context, _ *mcp.CallToolRequest, in LogsInput) (*mcp.CallToolResult, executor.Result, error) {
	if err := policy.Service(in.Service); err != nil {
		return nil, executor.Result{}, err
	}
	lines := 50
	if in.Lines != nil {
		lines = *in.Lines
	}
	if lines < 1 || lines > 200 {
		return nil, executor.Result{}, fmt.Errorf("lines must be between 1 and 200")
	}
	startOperation(ctx)
	r, err := t.Runner.Run(ctx, "/usr/bin/journalctl", []string{"--no-pager", "--unit=" + in.Service, "--lines=" + strconv.Itoa(lines)}, "")
	commandOutcome(ctx, r, err)
	message := strings.ToLower(r.Stderr)
	if strings.Contains(message, "permission") || strings.Contains(message, "not seeing messages") || strings.Contains(message, "access denied") {
		operationError(ctx, "journal_access_denied")
		return nil, r, fmt.Errorf("journal access denied or incomplete for server user; check journal permissions: %s", r.Stderr)
	}
	return nil, r, err
}
