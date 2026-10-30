# Deterministic MCP demo client

This Go client tests shell-mcp directly through MCP. No LLM, LLM API key, or extra
MCP application is involved. It shares the root go.mod and official Go MCP SDK
v1.8.0 dependency. Runtime tests need no internet or LLM service; the first Go
build may need to download module dependencies.

Run all commands below from the repository root.

## Build

```sh
go build -trimpath -o bin/shell-mcp ./cmd/shell-mcp
go build -trimpath -o bin/demo-client ./demo/cmd/client
```

## Local stdio

The client launches the specified binary, completes MCP setup through the SDK,
performs one operation, closes the session, and waits for the child to exit.
The child inherits your environment and working directory; its logs go to stderr.
The default workspace is `./demo-workspace` relative to that directory. Set
`SHELL_MCP_WORKSPACE` to an absolute path to override it.

```sh
./bin/demo-client --transport=stdio --server=./bin/shell-mcp --action=list
./bin/demo-client --transport=stdio --server=./bin/shell-mcp \
  --action=call --tool=system_info --arguments='{}'
```

## HTTP authentication

HTTP requires the same `SHELL_MCP_API_TOKEN` in the server and client environments.
Load it in each terminal without displaying it (Bash):

```sh
read -rsp 'MCP API token: ' SHELL_MCP_API_TOKEN; echo
export SHELL_MCP_API_TOKEN
```

Use a strong randomly generated secret with no whitespace. Stdio needs no token.
The client attaches `Authorization: Bearer <token>` to every SDK HTTP request,
including stream and session-close requests. It never prints credentials and
rejects redirects; supply the final MCP endpoint URL. Missing client tokens fail
before connecting; a wrong token produces a clear HTTP 401 authentication error.

## Localhost HTTP

Start a server in one terminal:

```sh
./bin/shell-mcp --transport=http --listen=127.0.0.1:8080
```

In another terminal:

```sh
./bin/demo-client --transport=http --url=http://127.0.0.1:8080/mcp --action=list
./bin/demo-client --transport=http --url=http://127.0.0.1:8080/mcp \
  --action=call --tool=system_info --arguments='{}'
```

HTTP mode connects to an existing server; it never starts a server process.

## Remote HTTP

On the remote Linux host, run behind an HTTPS reverse proxy on the same host
(with the token loaded):

```sh
./bin/shell-mcp --transport=http --listen=127.0.0.1:8080
```

On the client machine, load the same token and use your HTTPS proxy endpoint:

```sh
./bin/demo-client --transport=http --url=https://mcp.example.com/mcp --action=list
./bin/demo-client --transport=http --url=https://mcp.example.com/mcp \
  --action=call --tool=system_info --arguments='{}'
```

Commands run on the **server's host**, with its Linux user's permissions, without
sudo or SSH. The remote host must have its own running instance. HTTP uses shared-token authentication, mapping all authenticated requests to
`demo-user`. Remote use requires HTTPS with a trusted certificate, for example
from a reverse proxy that forwards Authorization and supports SSE streaming.
The server has no built-in TLS. Plain HTTP is suitable only for localhost testing
or the loopback proxy hop. Keep the backend private and restrict firewall access.
There is no OAuth, user registration, or roles.

## Call each tool

These examples use localhost HTTP. Substitute the remote endpoint, or replace
`--transport=http --url=...` with `--transport=stdio --server=./bin/shell-mcp`.

```sh
./bin/demo-client --transport=http --url=http://127.0.0.1:8080/mcp \
  --action=call --tool=system_info --arguments='{}'

./bin/demo-client --transport=http --url=http://127.0.0.1:8080/mcp \
  --action=call --tool=service_status --arguments='{"service":"nginx.service"}'

./bin/demo-client --transport=http --url=http://127.0.0.1:8080/mcp \
  --action=call --tool=service_logs --arguments='{"service":"nginx.service","lines":20}'

./bin/demo-client --transport=http --url=http://127.0.0.1:8080/mcp \
  --action=call --tool=run_command \
  --arguments='{"executable":"/usr/bin/uptime","args":[]}'
```

Choose an installed service. `service_logs` defaults to 50 lines if omitted;
accepted range is 1–200. Journal access depends on server-user permissions.
`run_command` accepts `executable`, `args`, and optional `working_directory`.
Shell pipes, expansion, and redirection are not interpreted by the server.
See the [root README](../README.md) for the execution policy and limitations.

### Explicitly opt in to a write

The client performs only the requested tool call. It never enables writes or
executes a write automatically. This example deliberately enables writes on a
child server and creates `hello-demo.txt` in the configured demo workspace:

```sh
SHELL_MCP_ALLOW_WRITE=true \
SHELL_MCP_WORKSPACE="$PWD/demo-workspace" \
./bin/demo-client --transport=stdio --server=./bin/shell-mcp \
  --action=call --tool=run_command \
  --arguments='{"executable":"/usr/bin/touch","args":["hello-demo.txt"]}'
```

For HTTP, enable those environment variables on the **server process** before
starting it; setting them on an HTTP client does not alter server policy.

## Flags, output, and failures

`SHELL_MCP_API_TOKEN` is required for HTTP; there is no token command-line flag.
The client never forwards the token to another origin and does not follow redirects.


| Flag | Default | Purpose |
| --- | --- | --- |
| `--transport` | `stdio` | `stdio` or `http` |
| `--server` | empty | Required stdio server binary path |
| `--url` | empty | Required HTTP(S) MCP endpoint |
| `--action` | `list` | `list` or `call` |
| `--tool` | empty | Required for `call` |
| `--arguments` | `{}` | JSON object for `call` |
| `--timeout` | `15s` | Positive Go duration for initialization and each operation |

Do not combine `--server` with HTTP or `--url` with stdio. `list` does not accept
tool/argument options. `--help` prints available flags. A list operation's timeout
covers retrieval of all pages, not each page separately.

Discovery prints an indented JSON array of complete SDK tool definitions,
including names, descriptions, and input schemas. Invocation prints the complete
SDK result, including content, structuredContent, and any isError field.
Diagnostics and child-server logs go to stderr so stdout remains readable JSON.

Exit status is nonzero for invalid input, connection/protocol errors, timeout,
tool-level errors, session-close failures, and structured process results with a
nonzero `exit_code` or `timed_out=true`. Thus an inactive service's status is still
printed as normal MCP result data, but the CLI returns nonzero for shell scripts.
Ctrl+C cancels outstanding requests and closes the session; stdio cleanup is
handled by the SDK (stdin close, bounded waits, then termination if needed).

Try error cases without an LLM:

```sh
# Invalid JSON arguments; rejected before connecting.
./bin/demo-client --transport=stdio --server=./bin/shell-mcp \
  --action=call --tool=system_info --arguments='not-json'

# If nothing is listening here, connection fails and exits nonzero.
./bin/demo-client --transport=http --url=http://127.0.0.1:65534/mcp --timeout=2s
```

## Code and later LLM integration

- `cmd/client/main.go`: flags, signal handling, JSON formatting, and exit behavior.
- `internal/mcpclient/client.go`: connection, paginated `ListTools`, `CallTool`,
  and `Close`; uses the SDK without importing server handlers.

The reusable package returns SDK tool schemas and structured results intact.
A future adapter could implement:

`user request → LLM → MCP tool call → tool result → LLM response`

It would discover tools with `ListTools`, translate schemas for its chosen
provider, pass selected calls to `CallTool`, and feed results back to the LLM.
Keep provider conversion and any agent loop outside mcpclient. No LLM provider,
agent loop, or provider-specific tool conversion is implemented here.

## Checks

```sh
gofmt -w cmd internal demo
go build ./...
go test ./...
```

Client tests check JSON/config rejection, paginated discovery, preservation of
structured results and tool-level errors, and request cancellation using an authenticated local
SDK fixture. Authentication checks cover missing/invalid tokens and redirect safety. The root transport tests cover stdio and HTTP initialization and
system_info. Actual demo binaries were also checked over both transports for
discovery/system_info, invalid JSON, a tool-level error, and an unavailable HTTP
endpoint. Verification does not restart services or modify system configuration.
