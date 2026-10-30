# Repository guide: shell-mcp

Read this before exploring source. This is a repository map, not an automatically
installed assistant skill. Read only files relevant to the requested change.

## Implemented purpose

Small Go MCP learning server for Linux/systemd. Official Go MCP SDK v1.8.0;
Go 1.25+ required. No LLM API key. Runs as its Linux process user without sudo.
Tools: system_info, service_status, service_logs, run_command.
Same tools are exposed through stdio and Streamable HTTP at /mcp.

## Directory map

- cmd/shell-mcp/main.go: configuration, dependency wiring, workspace creation, signals.
- internal/config/config.go: flag/env parsing, listen and HTTP token validation.
- internal/server/server.go: MCP server construction and registration.
- internal/transport/stdio.go: SDK StdioTransport.
- internal/auth/auth.go: bearer middleware, constant-time digests, Identity(ctx) accessor.
- internal/transport/http.go: /mcp handler, HTTP lifecycle and timeouts.
- internal/tools/register.go: shared dependencies, typed registration, audit middleware.
- internal/audit: best-effort mutex-serialized JSON lines; injected stderr writer.
- internal/tools/auditing.go: lifecycle wrapper and per-tool metadata allowlists.
- internal/tools/system.go: hostname, /etc/os-release, kernel and uptime from /proc.
- internal/tools/services.go: validated systemctl/journalctl calls, journal diagnostics.
- internal/tools/commands.go: run_command input, policy check, executor call.
- internal/policy/policy.go: allowlist, workspace/path/symlink rules, service validation.
- internal/executor/runner.go: process execution, deadline, bounded output capture.
- internal/{config,policy,executor}/*_test.go: focused behavior tests.
- internal/transport/integration_test.go: SDK client tests of both transports.
- demo/cmd/client/main.go: deterministic CLI flags, signals, JSON, and exit status.
- demo/internal/mcpclient/client.go: SDK connection, paginated ListTools, CallTool, Close.
- demo/README.md: no-LLM CLI examples, opt-in write, and future LLM adapter flow.
- README.md: client JSON examples, tool requests, permissions, limitations.
- LICENSE / NOTICE: Apache-2.0 terms and Murali Annamneni copyright attribution.

## Request flow

HTTP transport → bearer middleware → typed tool handler → policy → executor → result.
Stdio uses the same tool flow without HTTP authentication.
System info reads Linux files directly; service tools use service-name policy.
The executor does not enforce authorization: tools must call policy first.
The SDK infers schemas and serializes handler results; ordinary process nonzero
exit codes are data, while validation/start failures become tool errors.
All handlers and execution receive context.Context. HTTP details stay in transport.

## Configuration

- --transport=stdio (default); alternative: http.
- --listen=127.0.0.1:8080 (default); literal IP and port 1–65535, including in stdio.
- SHELL_MCP_API_TOKEN: nonempty shared token required for HTTP server and demo client.
- Token contains no whitespace; stdio ignores it. Old remote opt-in flag removed.
- SHELL_MCP_ALLOW_WRITE: unset/false disables writes; true enables; other values fail.
- SHELL_MCP_WORKSPACE: absolute workspace directory; default below.
- Default workspace: ./demo-workspace under the server process working directory.
- Config resolves the default to an absolute path; env overrides must be absolute.
- Startup creates the workspace, even with write commands disabled.
- Logs go to stderr; stdout is reserved for stdio MCP.
- SIGINT/SIGTERM stop the server; HTTP drains for at most five seconds.
- HTTP header timeout 5s, idle timeout 60s, no read/write stream deadline.
- Idle MCP sessions expire after five minutes.

## Build, run, test

From the repository root; HTTP examples require the same token in both processes:
load SHELL_MCP_API_TOKEN securely into each process environment (never log it).

```sh
gofmt -w cmd internal demo
go build ./...
go test ./...
go build -trimpath -o bin/shell-mcp ./cmd/shell-mcp
go build -trimpath -o bin/demo-client ./demo/cmd/client
./bin/demo-client --transport=stdio --server=./bin/shell-mcp --action=list
./bin/demo-client --transport=stdio --server=./bin/shell-mcp --action=call --tool=system_info
./bin/demo-client --transport=http --url=http://127.0.0.1:8080/mcp --action=list
./bin/shell-mcp --transport=stdio
./bin/shell-mcp --transport=http --listen=127.0.0.1:8080
```

Remote client endpoint: https://mcp.example.com/mcp (replace proxy hostname).
Commands execute on the server host; HTTP does not use SSH. Each remote host
needs its own running server. See README for verified VS Code .vscode/mcp.json
examples (servers object, stdio command/args/env, HTTP type/url).

Demo client: stdio starts a child; HTTP connects to an existing endpoint.
--transport defaults stdio; --server required for stdio; --url required for HTTP.
--action defaults list (all pages); call requires --tool; --arguments defaults {}.
--timeout defaults 15s for initialization and each operation; must be positive.
Prints full JSON schemas/results; errors and nonzero process results exit nonzero.

## Restrictions and limitations

- run_command allows only /usr/bin/{uptime,df,free,touch,mkdir}.
- touch/mkdir require writes enabled; args are relative paths only, no flags.
- No absolute write paths, dot/parent/empty components, or leading dash.
- Server inserts -- for write commands and checks existing symlink components.
- Working directories stay in the workspace; relative directories resolve within it.
- Read-command arguments are not confined to workspace files.
- Service names match a restricted character/length pattern; arguments are built internally.
- journalctl lines defaults to 50; accepted range 1–200.
- Journal permission/partial-access diagnostics become tool errors; no sudo fallback.
- Direct exec.CommandContext; no shell, pipes, redirection, expansion, or globbing.
- 10s execution deadline; process-group kill; 1s child-pipe cleanup bound.
- stdout/stderr capped at 64 KiB each; excess output is still drained.
- Result: stdout, stderr, exit_code, timed_out, output_truncated.
- Linux permissions apply; no production isolation, race-proof paths, or hard-link isolation.
- HTTP requires Authorization: Bearer <token> on every /mcp request; failure is 401.
- Middleware sets demo-user in request context, separate from execution policy.
- Remote use requires HTTPS via proxy; plain HTTP is for localhost testing only.
- Client uses SDK HTTPClient with a bearer RoundTripper and rejects redirects.
- Never expose directly to the internet; restrict firewall access to intended clients.
- Single shared identity only; no user accounts, roles, or concurrency limits.
- Tests use temp directories; do not restart services or change system configuration.

## Extension instructions

- Tool: add typed inputs/output and handler in internal/tools; register in register.go.
  Add relevant policy checks, context propagation, and focused tests.
- Transport: add implementation under internal/transport, config selection, and main wiring.
  Reuse internal/server.New; keep tool handlers transport-independent.
- Policy rule: modify internal/policy, test acceptance/rejection cases; executor stays generic.
- Update README and this guide when configuration, commands, or capabilities change.

## Auditing (implemented)

Events: authentication_failed, tool_denied, tool_started, tool_completed.
Fields: UTC RFC3339Nano timestamp, random request_id (not session ID), user_id,
transport, tool, sanitized arguments, status, exit_code, duration_ms, safe reason.
Start/completion share an ID; monotonic duration. No completion invented on crash.
HTTP demo-user is a shared-token identity, not a person; stdio uses the OS account.
Auth failures have no verified identity. No client-supplied identity is trusted.
No outputs, raw errors/bodies, headers, tokens, environment, or file contents.
Service/path values redacted; safe flags only; strings <=128 bytes, args <=16.
Restrict log access: paths/service names can be sensitive. Logs are not durable,
tamper-proof, or guaranteed; writer failures never alter results.
Inspect: redirect stderr to audit.log; jq -R 'fromjson? | select(.event)' audit.log.
Systemd: journalctl -u YOUR_UNIT -o cat; stderr must go to journald. README has example.
New tool: add metadata allowlist, validate/policy-check, then startOperation(ctx);
commandOutcome records process status. Shared middleware logs completion/denial.

## Future ideas (not implemented)

- Built-in TLS remains future work in internal/transport/http.go.
- auth.Identity(ctx) could support future per-user policy checks.
- Durable audit storage remains future work; current stderr logging is best-effort.
- No OAuth, RBAC, database, or plugin framework currently exists.

Future LLM adapters can reuse demo/internal/mcpclient schemas and results; keep
provider conversion and agent loops outside it. Neither is implemented today.
