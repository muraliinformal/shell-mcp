package tools

import (
	"context"

	"example.com/shell-mcp/internal/executor"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CommandInput struct {
	Executable       string   `json:"executable" jsonschema:"Absolute allowlisted executable"`
	Args             []string `json:"args" jsonschema:"Argument array; write commands accept relative paths only"`
	WorkingDirectory string   `json:"working_directory,omitempty" jsonschema:"Directory inside configured workspace"`
}

func (t *Tools) RunCommand(ctx context.Context, _ *mcp.CallToolRequest, in CommandInput) (*mcp.CallToolResult, executor.Result, error) {
	args, dir, err := t.Policy.Command(in.Executable, in.Args, in.WorkingDirectory)
	if err != nil {
		return nil, executor.Result{}, err
	}
	result, err := t.Runner.Run(ctx, in.Executable, args, dir)
	return nil, result, err
}
