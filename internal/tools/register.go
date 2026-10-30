package tools

import (
	"example.com/shell-mcp/internal/audit"
	"example.com/shell-mcp/internal/executor"
	"example.com/shell-mcp/internal/policy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tools struct {
	Policy *policy.Policy
	Runner executor.Runner
}

func Register(s *mcp.Server, p *policy.Policy, r executor.Runner, logger *audit.Logger, transport, localUser string) {
	s.AddReceivingMiddleware(auditing(logger, transport, localUser))
	t := &Tools{p, r}
	mcp.AddTool(s, &mcp.Tool{Name: "system_info", Description: "Inspect the server host's Linux system information."}, t.SystemInfo)
	mcp.AddTool(s, &mcp.Tool{Name: "service_status", Description: "Read systemd service status on the server host."}, t.ServiceStatus)
	mcp.AddTool(s, &mcp.Tool{Name: "service_logs", Description: "Read recent systemd journal entries on the server host."}, t.ServiceLogs)
	mcp.AddTool(s, &mcp.Tool{Name: "run_command", Description: "Execute an allowed command on the server host, subject to workspace and write policy."}, t.RunCommand)
}
