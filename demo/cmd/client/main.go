package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/shell-mcp/demo/internal/mcpclient"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "demo-client:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) (err error) {
	var opts mcpclient.Options
	var action, tool, raw string
	flags := flag.NewFlagSet("demo-client", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.Transport, "transport", "stdio", "stdio or http")
	flags.StringVar(&opts.Server, "server", "", "server binary path (required for stdio)")
	flags.StringVar(&opts.URL, "url", "", "full MCP endpoint (required for http)")
	flags.DurationVar(&opts.Timeout, "timeout", 15*time.Second, "initialization and per-operation timeout")
	flags.StringVar(&action, "action", "list", "list or call")
	flags.StringVar(&tool, "tool", "", "tool name (required for call)")
	flags.StringVar(&raw, "arguments", "{}", "tool arguments as a JSON object")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if action != "list" && action != "call" {
		return fmt.Errorf("action must be list or call")
	}
	if action == "call" && tool == "" {
		return fmt.Errorf("--action=call requires --tool")
	}
	var arguments map[string]any
	if err := json.Unmarshal([]byte(raw), &arguments); err != nil || arguments == nil {
		return fmt.Errorf("--arguments must be a valid JSON object")
	}
	if action == "list" && (tool != "" || raw != "{}") {
		return fmt.Errorf("--tool and --arguments require --action=call")
	}
	client, err := mcpclient.Connect(ctx, opts)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close MCP session: %w", closeErr))
		}
	}()
	output := json.NewEncoder(stdout)
	output.SetIndent("", "  ")
	if action == "list" {
		tools, err := client.ListTools(ctx)
		if err != nil {
			return err
		}
		return output.Encode(tools)
	}
	result, err := client.CallTool(ctx, tool, arguments)
	if err != nil {
		return err
	}
	if err := output.Encode(result); err != nil {
		return fmt.Errorf("print tool result: %w", err)
	}
	if result.IsError {
		return fmt.Errorf("tool %q reported an execution error; see JSON result", tool)
	}
	// This server deliberately returns process exit statuses as successful MCP
	// results. Preserve that result but signal a failed process to shell scripts.
	if data, ok := result.StructuredContent.(map[string]any); ok {
		if code, ok := data["exit_code"].(float64); ok && code != 0 {
			return fmt.Errorf("tool %q process exited with code %g; see JSON result", tool, code)
		}
		if timedOut, _ := data["timed_out"].(bool); timedOut {
			return fmt.Errorf("tool %q process timed out; see JSON result", tool)
		}
	}
	return nil
}
