package transport_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"example.com/shell-mcp/internal/executor"
	"example.com/shell-mcp/internal/policy"
	"example.com/shell-mcp/internal/server"
	"example.com/shell-mcp/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSDKTransports(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "shell-mcp")
	cmd := exec.Command("go", "build", "-o", binary, "../../cmd/shell-mcp")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %s %v", out, err)
	}
	p, err := policy.New(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := transport.Handler(server.New(p, executor.Runner{}), "test-token")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	stdio := exec.Command(binary, "--transport=stdio")
	stdio.Env = append(os.Environ(), "SHELL_MCP_WORKSPACE="+t.TempDir(), "SHELL_MCP_ALLOW_WRITE=false")
	for name, tr := range map[string]mcp.Transport{"stdio": &mcp.CommandTransport{Command: stdio}, "http": &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp", HTTPClient: &http.Client{Transport: testBearer{}}}} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			c := mcp.NewClient(&mcp.Implementation{Name: "verification", Version: "1"}, nil)
			s, err := c.Connect(ctx, tr, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if s.InitializeResult() == nil {
				t.Fatal("no initialization result")
			}
			list, err := s.ListTools(ctx, nil)
			if err != nil || len(list.Tools) != 4 {
				t.Fatalf("tools: %v %v", list, err)
			}
			r, err := s.CallTool(ctx, &mcp.CallToolParams{Name: "system_info", Arguments: map[string]any{}})
			if err != nil || r.IsError {
				t.Fatalf("call: %v %v", r, err)
			}
			t.Log("SDK initialization, four tools, system_info call passed")
		})
	}
}

type testBearer struct{}

func (testBearer) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer test-token")
	return http.DefaultTransport.RoundTrip(clone)
}
