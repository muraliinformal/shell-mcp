package transport

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Stdio(ctx context.Context, s *mcp.Server) error { return s.Run(ctx, &mcp.StdioTransport{}) }
