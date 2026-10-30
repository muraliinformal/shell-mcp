package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"example.com/shell-mcp/internal/audit"
	"example.com/shell-mcp/internal/executor"
	"example.com/shell-mcp/internal/policy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func events(t *testing.T, out *bytes.Buffer) []audit.Event {
	t.Helper()
	var result []audit.Event
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var e audit.Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		result = append(result, e)
	}
	return result
}
func TestLifecycle(t *testing.T) {
	for _, test := range []struct {
		name, status, reason                 string
		code                                 int
		timeout, cancel, handlerErr, toolErr bool
	}{
		{name: "success", status: "success"},
		{name: "nonzero", status: "error", reason: "command_nonzero_exit", code: 3},
		{name: "timeout", status: "timeout", reason: "execution_timeout", code: -1, timeout: true},
		{name: "cancel", status: "cancelled", reason: "request_cancelled", cancel: true},
		{name: "handler", status: "error", reason: "mcp_handler_error", handlerErr: true},
		{name: "tool", status: "error", reason: "tool_error", toolErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "run_command", Arguments: json.RawMessage(`{"executable":"/usr/bin/df","args":["-h","secret-password"],"working_directory":"secret-path","user_id":"forged"}`)}}
			handler := auditing(audit.New(&out), "stdio", "local-account")(func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
				startOperation(ctx)
				if test.cancel {
					cancel()
				}
				if test.handlerErr {
					return nil, fmt.Errorf("sensitive error text")
				}
				if !test.toolErr {
					commandOutcome(ctx, executor.Result{ExitCode: test.code, TimedOut: test.timeout, Stdout: "secret-output", Stderr: "secret-stderr"}, nil)
				}
				return &mcp.CallToolResult{IsError: test.toolErr}, nil
			})
			_, _ = handler(ctx, "tools/call", req)
			logs := events(t, &out)
			if len(logs) != 2 {
				t.Fatalf("%d events", len(logs))
			}
			if logs[0].Event != "tool_started" || logs[1].Event != "tool_completed" || logs[0].RequestID == "" || logs[0].RequestID != logs[1].RequestID {
				t.Fatal("invalid lifecycle")
			}
			if logs[1].Status != test.status || logs[1].Reason != test.reason || logs[1].DurationMS == nil {
				t.Fatalf("unexpected result %+v", logs[1])
			}
			if logs[1].UserID != "local-account" {
				t.Fatal("forged identity")
			}
			for _, secret := range []string{"secret-password", "secret-path", "secret-output", "secret-stderr", "forged", "sensitive error text"} {
				if strings.Contains(out.String(), secret) {
					t.Fatalf("leaked %s", secret)
				}
			}
		})
	}
}
func TestDeniedDoesNotExecute(t *testing.T) {
	var out bytes.Buffer
	directory := t.TempDir()
	p, err := policy.New(directory, false)
	if err != nil {
		t.Fatal(err)
	}
	tools := &Tools{Policy: p}
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "run_command"}}
	handler := auditing(audit.New(&out), "stdio", "local-account")(func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		_, _, err := tools.RunCommand(ctx, req, CommandInput{Executable: "/usr/bin/touch", Args: []string{"denied-file"}})
		return nil, err
	})
	_, err = handler(context.Background(), "tools/call", req)
	if err == nil {
		t.Fatal("policy did not deny")
	}
	if _, err := os.Stat(filepath.Join(directory, "denied-file")); !os.IsNotExist(err) {
		t.Fatal("denied command executed")
	}
	logs := events(t, &out)
	if len(logs) != 1 || logs[0].Event != "tool_denied" || logs[0].Status != "denied" {
		t.Fatalf("invalid denial %+v", logs)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("failed") }
func TestLoggingFailurePreservesResult(t *testing.T) {
	expected := &mcp.CallToolResult{}
	handler := auditing(audit.New(failWriter{}), "stdio", "account")(func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		startOperation(ctx)
		return expected, nil
	})
	result, err := handler(context.Background(), "tools/call", &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "system_info"}})
	if err != nil || result != expected {
		t.Fatal("logger changed result")
	}
}
func TestSanitizedMetadata(t *testing.T) {
	raw, _ := json.Marshal(CommandInput{Executable: "/usr/bin/touch", Args: strings.Fields(strings.Repeat("secret ", 100)), WorkingDirectory: "secret"})
	a := auditArguments("run_command", raw)
	if len(a.Args) > 16 || a.WorkingDirectory != "[redacted]" {
		t.Fatal("metadata bounds")
	}
	data, _ := json.Marshal(a)
	if strings.Contains(string(data), "secret") {
		t.Fatal("secret leaked")
	}
	a = auditArguments("service_logs", json.RawMessage(`{"service":"private-service","lines":20,"token":"secret"}`))
	data, _ = json.Marshal(a)
	if strings.Contains(string(data), "private-service") || strings.Contains(string(data), "secret") {
		t.Fatal("service leaked")
	}
}
