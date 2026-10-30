package transport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/shell-mcp/internal/audit"
	"example.com/shell-mcp/internal/executor"
	"example.com/shell-mcp/internal/policy"
	"example.com/shell-mcp/internal/server"
	"example.com/shell-mcp/internal/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHTTPAuditing(t *testing.T) {
	var out bytes.Buffer
	logger := audit.New(&out)
	p, err := policy.New(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := transport.Handler(server.New(p, executor.Runner{}, logger, "http", ""), "test-token", logger)
	if err != nil {
		t.Fatal(err)
	}
	// Authentication failures have no verified identity, regardless of supplied headers.
	request := httptest.NewRequest("POST", "/mcp", strings.NewReader("secret-body"))
	request.Header.Set("Authorization", "Bearer submitted-secret")
	request.Header.Set("user_id", "forged-user")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != 401 {
		t.Fatal("missing authentication failure")
	}
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "audit-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp", HTTPClient: &http.Client{Transport: testBearer{}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "system_info", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("system info failed: %v", err)
	}
	for _, params := range []*mcp.CallToolParams{
		{Name: "service_logs", Arguments: map[string]any{"service": "private-service", "lines": "wrong-type"}},
		{Name: "service_logs", Arguments: map[string]any{"service": "private-service", "lines": 201}},
		{Name: "service_status", Arguments: map[string]any{"service": "invalid/secret"}},
		{Name: "run_command", Arguments: map[string]any{"executable": "/usr/bin/touch", "args": []string{"secret-file"}}},
	} {
		result, err := session.CallTool(ctx, params)
		if err != nil || !result.IsError {
			t.Fatalf("expected denial: %v", err)
		}
	}
	// No tool events can be written after CallTool returns: middleware logs before responding.
	var logs []audit.Event
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var e audit.Event
		if err = json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		logs = append(logs, e)
	}
	if len(logs) != 7 {
		t.Fatalf("unexpected event count %d: %s", len(logs), out.String())
	}
	if logs[0].Event != "authentication_failed" || logs[0].UserID != "" || logs[0].RequestID == "" {
		t.Fatal("incorrect failed-auth identity")
	}
	if logs[1].Event != "tool_started" || logs[2].Event != "tool_completed" || logs[2].Status != "success" || logs[1].RequestID != logs[2].RequestID {
		t.Fatal("incorrect lifecycle")
	}
	for _, e := range logs[1:] {
		if e.UserID != "demo-user" || e.Transport != "http" {
			t.Fatalf("lost authenticated context: %+v", e)
		}
	}
	for _, e := range logs[3:] {
		if e.Event != "tool_denied" {
			t.Fatalf("executed rejected operation: %+v", e)
		}
	}
	for _, secret := range []string{"submitted-secret", "secret-body", "forged-user", "private-service", "invalid/secret", "secret-file"} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("leaked %s", secret)
		}
	}
}
