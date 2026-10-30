package tools

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"example.com/shell-mcp/internal/audit"
	"example.com/shell-mcp/internal/auth"
	"example.com/shell-mcp/internal/executor"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type invocationKey struct{}
type invocation struct {
	logger       *audit.Logger
	event        audit.Event
	started      bool
	start        time.Time
	exitCode     *int
	timedOut     bool
	handlerError bool
	reason       string
}

// startOperation must be called only after all input/policy checks succeed.
func startOperation(ctx context.Context) {
	if op, ok := ctx.Value(invocationKey{}).(*invocation); ok {
		op.started = true
		op.start = time.Now()
		e := op.event
		e.Event = "tool_started"
		op.logger.Log(e)
	}
}
func commandOutcome(ctx context.Context, r executor.Result, err error) {
	if op, ok := ctx.Value(invocationKey{}).(*invocation); ok {
		op.exitCode = &r.ExitCode
		op.timedOut = r.TimedOut
		op.handlerError = err != nil
	}
}
func operationError(ctx context.Context, reason string) {
	if op, ok := ctx.Value(invocationKey{}).(*invocation); ok {
		op.handlerError = true
		op.reason = reason
	}
}

// auditArguments deliberately recognizes fields per tool instead of serializing
// raw input. Service names and paths are always redacted, even when valid.
func auditArguments(tool string, raw json.RawMessage) *audit.Arguments {
	a := &audit.Arguments{}
	switch tool {
	case "system_info":
	case "service_status", "service_logs":
		var in struct {
			Service json.RawMessage
			Lines   *int
		}
		_ = json.Unmarshal(raw, &in)
		if len(in.Service) > 0 {
			a.Service = "[redacted]"
		}
		if tool == "service_logs" {
			a.Lines = in.Lines
		}
	case "run_command":
		var in CommandInput
		if json.Unmarshal(raw, &in) != nil {
			return a
		}
		switch in.Executable {
		case "/usr/bin/uptime", "/usr/bin/df", "/usr/bin/free", "/usr/bin/touch", "/usr/bin/mkdir":
			a.Executable = in.Executable
		default:
			a.Executable = "[redacted]"
		}
		if in.WorkingDirectory != "" {
			a.WorkingDirectory = "[redacted]"
		}
		for i, arg := range in.Args {
			if i == 16 {
				break
			}
			// These flags are safe only for the read commands. For writes even a
			// flag-like string is a user path and is redacted.
			safe := false
			if in.Executable == "/usr/bin/df" || in.Executable == "/usr/bin/free" || in.Executable == "/usr/bin/uptime" {
				switch arg {
				case "-h", "-m", "-g", "--human-readable":
					safe = true
				}
			}
			if !safe {
				arg = "[redacted]"
			}
			a.Args = append(a.Args, arg)
		}
	}
	return a
}
func auditing(logger *audit.Logger, transport, localUser string) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			call, ok := req.(*mcp.CallToolRequest)
			if !ok {
				return next(ctx, method, req)
			}
			tool := call.Params.Name
			switch tool {
			case "system_info", "service_status", "service_logs", "run_command":
			default:
				tool = "unknown"
			}
			user := ""
			if transport == "stdio" {
				user = localUser
			}
			if identity, ok := auth.Identity(ctx); ok {
				user = identity
			}
			op := &invocation{logger: logger, event: audit.Event{RequestID: audit.RequestID(), UserID: user, Transport: transport, Tool: tool, Arguments: auditArguments(tool, call.Params.Arguments)}}
			result, err := next(context.WithValue(ctx, invocationKey{}, op), method, req)
			e := op.event
			if !op.started {
				e.Event = "tool_denied"
				e.Status = "denied"
				e.Reason = "invalid_input_or_policy_denied"
			} else {
				duration := time.Since(op.start).Milliseconds()
				e.DurationMS = &duration
				e.ExitCode = op.exitCode
				e.Event = "tool_completed"
				e.Status = "success"
				toolResult, _ := result.(*mcp.CallToolResult)
				switch {
				case op.timedOut || errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
					e.Status = "timeout"
					e.Reason = "execution_timeout"
				case errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled):
					e.Status = "cancelled"
					e.Reason = "request_cancelled"
				case err != nil:
					e.Status = "error"
					e.Reason = "mcp_handler_error"
				case op.handlerError:
					e.Status = "error"
					e.Reason = "operation_error"
					if op.reason != "" {
						e.Reason = op.reason
					}
				case toolResult != nil && toolResult.IsError:
					e.Status = "error"
					e.Reason = "tool_error"
				case op.exitCode != nil && *op.exitCode != 0:
					e.Status = "error"
					e.Reason = "command_nonzero_exit"
				}
			}
			logger.Log(e)
			return result, err
		}
	}
}
