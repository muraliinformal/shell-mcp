package mcpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/shell-mcp/internal/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDiscoveryAndCalls(t *testing.T) {
	t.Setenv("SHELL_MCP_API_TOKEN", "test-token")
	server := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "1"}, &mcp.ServerOptions{PageSize: 1})
	type input struct {
		Value string `json:"value"`
	}
	type output struct {
		Value string `json:"value"`
	}
	for _, name := range []string{"one", "two", "three"} {
		mcp.AddTool(server, &mcp.Tool{Name: name, Description: "Echo a value"}, func(ctx context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, output, error) {
			if in.Value == "error" {
				return nil, output{}, fmt.Errorf("requested failure")
			}
			if in.Value == "wait" {
				select {
				case <-ctx.Done():
					return nil, output{}, ctx.Err()
				case <-time.After(time.Second):
					return nil, output{}, fmt.Errorf("fixture wait elapsed")
				}
			}
			return nil, output{Value: in.Value}, nil
		})
	}
	handler, err := auth.Middleware("test-token", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{PropagateRequestCancellation: true}), nil)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	client, err := Connect(context.Background(), Options{Transport: "http", URL: httpServer.URL, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	tools, err := client.ListTools(context.Background())
	if err != nil || len(tools) != 3 {
		t.Fatalf("pagination: %d tools, %v", len(tools), err)
	}
	for _, tool := range tools {
		if tool.InputSchema == nil || tool.Description == "" {
			t.Fatal("lost tool metadata")
		}
	}
	result, err := client.CallTool(context.Background(), "one", map[string]any{"value": "hello"})
	if err != nil || result.IsError || result.StructuredContent == nil || len(result.Content) == 0 {
		t.Fatalf("result: %+v %v", result, err)
	}
	result, err = client.CallTool(context.Background(), "one", map[string]any{"value": "error"})
	if err != nil || !result.IsError || len(result.Content) == 0 {
		t.Fatalf("tool error lost: %+v %v", result, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := client.CallTool(ctx, "one", map[string]any{"value": "wait"}); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestOptions(t *testing.T) {
	for _, opts := range []Options{
		{Transport: "invalid", Timeout: time.Second},
		{Transport: "stdio", Timeout: time.Second},
		{Transport: "stdio", Server: "server", URL: "http://localhost/mcp", Timeout: time.Second},
		{Transport: "http", URL: "file:///tmp/server", Timeout: time.Second},
		{Transport: "http", URL: "http://", Timeout: time.Second},
		{Transport: "http", URL: "http://localhost/mcp", Server: "server", Timeout: time.Second},
		{Transport: "http", URL: "http://localhost/mcp"},
	} {
		if _, err := Connect(context.Background(), opts); err == nil {
			t.Fatalf("accepted %+v", opts)
		}
	}
}
