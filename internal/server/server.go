package server

import (
	"example.com/shell-mcp/internal/executor"
	"example.com/shell-mcp/internal/policy"
	"example.com/shell-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func New(p *policy.Policy, r executor.Runner) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "shell-mcp", Version: "0.1.0"}, nil)
	tools.Register(s, p, r)
	return s
}
