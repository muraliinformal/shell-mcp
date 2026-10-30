# shell-mcp

A small Go project for learning how an MCP server works. It lets a client check a
Linux machine, read service logs, and run a few allowed commands.

You can try everything from your terminal using the included demo client.
**No LLM or LLM API key is needed.**

## What can it do?

| Tool | What it does |
| --- | --- |
| `system_info` | Shows the hostname, operating system, kernel version, and uptime. |
| `service_status` | Checks whether a systemd service is running. |
| `service_logs` | Reads recent log entries for a service. |
| `run_command` | Runs an allowed command, such as checking disk space. |

MCP is a way for applications to discover and call tools. A client connects to
this server, asks which tools are available, and sends a tool name with its
arguments. The server does the work and sends back a result.

**The work always happens on the machine running the server**, using that
server's Linux user permissions. The server never automatically uses `sudo`.

## 1. Build the project

You need Linux with systemd, Go 1.25 or newer, and `systemctl` and `journalctl`
installed under `/usr/bin`. The project uses the official
[Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk), version 1.8.0.

Open a terminal in the repository folder and run:

```sh
go build -trimpath -o bin/shell-mcp ./cmd/shell-mcp
go build -trimpath -o bin/demo-client ./demo/cmd/client
```

This creates two programs: the server and a simple client for testing it.
The first build may download Go dependencies. Running the demo needs no LLM
service or internet connection when the server is local.

Run the remaining commands from the repository folder too.

## 2. Try your first query

```sh
./bin/demo-client \
  --transport=stdio \
  --server=./bin/shell-mcp \
  --action=call \
  --tool=system_info
```

You should see JSON containing your machine's hostname, operating system, kernel
version, and uptime in seconds.

`stdio` means the client talks to the server through the child process's input
and output. The client starts the server for you and closes it when finished.
You do not need to start another server or set an authentication token.

To list all tools and their available arguments:

```sh
./bin/demo-client --transport=stdio --server=./bin/shell-mcp --action=list
```

In each tool's `inputSchema`, `properties` lists the arguments and `required`
identifies the ones you must provide. Listing tools never runs their commands.

## 3. Try the other tools

### Check a service

```sh
./bin/demo-client --transport=stdio --server=./bin/shell-mcp \
  --action=call --tool=service_status \
  --arguments='{"service":"ssh.service"}'
```

Replace `ssh.service` with a service installed on your machine. Some systems use
`sshd.service`. This tool runs `systemctl status` with paging turned off.
It does not start, stop, or restart the service.

### Read service logs

```sh
./bin/demo-client --transport=stdio --server=./bin/shell-mcp \
  --action=call --tool=service_logs \
  --arguments='{"service":"ssh.service","lines":20}'
```

This reads the latest 20 entries using `journalctl`. You can request 1–200 lines.
Leave out `lines` to get 50. If your Linux user cannot read the journal, the tool
reports an access error. No entries can also mean the service has no logs.

### Check disk space

```sh
./bin/demo-client --transport=stdio --server=./bin/shell-mcp \
  --action=call --tool=run_command \
  --arguments='{"executable":"/usr/bin/df","args":["-h"]}'
```

Use the full executable path. These are the only commands allowed today:

| Executable | Purpose | Enabled by default? |
| --- | --- | --- |
| `/usr/bin/uptime` | Show uptime and system load | Yes |
| `/usr/bin/df` | Show disk space | Yes |
| `/usr/bin/free` | Show memory usage | Yes |
| `/usr/bin/touch` | Create a file or update its timestamps | No |
| `/usr/bin/mkdir` | Create a directory | No |

`systemctl` and `journalctl` are available only through the dedicated service
tools, with arguments built by the server.

## 4. Enable file creation when you want to try writes

Writes are off by default. This example explicitly enables them for one child
server and creates `demo-workspace/hello.txt`:

```sh
SHELL_MCP_ALLOW_WRITE=true \
SHELL_MCP_WORKSPACE="$PWD/demo-workspace" \
./bin/demo-client --transport=stdio --server=./bin/shell-mcp \
  --action=call --tool=run_command \
  --arguments='{"executable":"/usr/bin/touch","args":["hello.txt"]}'
```

To create a directory, use `/usr/bin/mkdir` and a path such as `notes` in `args`.
To work inside an existing subdirectory, add `"working_directory":"notes"` to
the arguments object.

The **workspace** is the folder used as the command's working directory. By
default it is `demo-workspace` under the directory where you start the server.
Startup creates that folder even when write commands are disabled. Set
`SHELL_MCP_WORKSPACE` to an absolute path to use a different folder.

For `touch` and `mkdir`, supply relative paths only. Absolute paths, `..`, `.`,
empty path components, paths beginning with `-`, and paths involving existing
symlinks are rejected. Flags such as `mkdir -p` are not accepted, so parent
folders must already exist. Supplied working directories must stay within the
workspace and cannot involve symlinks.

Read commands can inspect things outside the workspace; for example, `df /`
is allowed. The workspace restriction is not a general filesystem sandbox.

## 5. Connect over HTTP on your own machine

HTTP lets a client connect to a server that is already running. Unlike stdio,
the client does not start or stop that server.

HTTP requires a shared secret called a **bearer token**. Anyone who has the
token and can reach the server can use its enabled tools. Use a strong randomly
generated secret with no whitespace. Do not commit it to Git or put it in logs.

### Terminal 1: start the server

In Bash, enter your token without displaying it or saving it in command history:

```sh
read -rsp 'MCP API token: ' SHELL_MCP_API_TOKEN; echo
export SHELL_MCP_API_TOKEN

./bin/shell-mcp --transport=http --listen=127.0.0.1:8080
```

### Terminal 2: connect with the same token

Open another terminal in the repository folder and enter the same token:

```sh
read -rsp 'MCP API token: ' SHELL_MCP_API_TOKEN; echo
export SHELL_MCP_API_TOKEN

./bin/demo-client --transport=http --url=http://127.0.0.1:8080/mcp \
  --action=call --tool=system_info
```

Use `--action=list` to discover tools over HTTP. The client adds the token to
every HTTP request for you. If it is missing or wrong, the request fails.
The client rejects redirects; use the final endpoint URL.

To confirm the server rejects requests without a token:

```sh
curl -i http://127.0.0.1:8080/mcp
```

Expect `401 Unauthorized`. Stop the server with Ctrl+C. It also handles SIGTERM
and allows up to five seconds for HTTP shutdown.

## 6. Connect to another machine

The other machine needs its own running copy of this server. Connecting over
MCP does not use SSH and does not install or start the remote server for you.
Commands run on that remote machine, not on your client machine.

**Use HTTPS for remote connections.** Plain HTTP exposes the token, requests,
and results to anyone able to observe the traffic. Plain HTTP is suitable for
localhost testing or the local connection between a proxy and this server.

The server does not provide HTTPS itself yet. A reverse proxy can provide it:

```text
Client → HTTPS → Reverse proxy → HTTP on localhost → shell-mcp
```

On the remote host, load `SHELL_MCP_API_TOKEN` and run the server behind a proxy
on the same machine:

```sh
./bin/shell-mcp --transport=http --listen=127.0.0.1:8080
```

Configure the proxy with a trusted TLS certificate. Forward `/mcp` and the
Authorization header, allow streaming responses without buffering, and use
stream timeouts that support long-lived connections. Keep credentials out of
proxy logs too.

On your client machine, load the same token and connect to your proxy's address:

```sh
./bin/demo-client --transport=http --url=https://mcp.example.com/mcp --action=list
```

Replace `mcp.example.com` with your actual proxy hostname. Binding the server to
`0.0.0.0:8080` is supported when a token is set, but do not expose that plain-HTTP
port directly to the internet. Restrict backend access with firewall rules.

## Use it from VS Code

Open this repository as your VS Code workspace. The examples below use VS Code's
`.vscode/mcp.json` format, described in its official
[MCP configuration reference](https://code.visualstudio.com/docs/agents/reference/mcp-configuration).
Other clients, including portable Copilot configurations, may use a different format.

For local stdio, create `.vscode/mcp.json` with:

```json
{
  "servers": {
    "shell-local": {
      "type": "stdio",
      "command": "${workspaceFolder}/bin/shell-mcp",
      "cwd": "${workspaceFolder}",
      "args": ["--transport=stdio"],
      "env": {
        "SHELL_MCP_ALLOW_WRITE": "false",
        "SHELL_MCP_WORKSPACE": "${workspaceFolder}/demo-workspace"
      }
    }
  }
}
```

Use **MCP: List Servers** in the Command Palette to start it. Then ask your
MCP-enabled chat to call `system_info`.

For a running localhost HTTP server, use this configuration instead. VS Code
must inherit `SHELL_MCP_API_TOKEN` from its environment:

```json
{
  "servers": {
    "shell-http": {
      "type": "http",
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "Authorization": "Bearer ${env:SHELL_MCP_API_TOKEN}"
      }
    }
  }
}
```

For a remote server, use the same HTTP configuration with
`"url": "https://mcp.example.com/mcp"` and your actual proxy hostname.
The shared token must match the remote server's token. Your chat application's
LLM may have its own account or API requirements; shell-mcp itself does not.

## Settings at a glance

| Setting | Default | Meaning |
| --- | --- | --- |
| `--transport` | `stdio` | Choose `stdio` or `http`. |
| `--listen` | `127.0.0.1:8080` | HTTP listening IP address and port. |
| `SHELL_MCP_API_TOKEN` | Unset | Required for HTTP server and demo client; unused by stdio. |
| `SHELL_MCP_ALLOW_WRITE` | `false` | Set exactly `true` to enable write commands; `false` is also accepted. |
| `SHELL_MCP_WORKSPACE` | `./demo-workspace` | Default is relative to the server's starting directory; an override must be absolute. |

The listening address must contain a literal IP and a port from 1 to 65535.
For IPv6, use brackets, such as `[::1]:8080`. Hostnames and empty addresses are
not accepted. Address syntax is checked even in stdio mode. The old
`--allow-unauthenticated-remote` flag has been removed; all HTTP connections now
require a token.

The demo client defaults to `--action=list` and a `--timeout=15s` request timeout.
Calls require `--tool`; `--arguments` defaults to `{}` and must be a JSON object.
See [demo/README.md](demo/README.md) for the full client reference.

## Understand results and common errors

Command results include these fields:

| Field | Meaning |
| --- | --- |
| `stdout` | Normal command output. |
| `stderr` | Error messages or other diagnostic output. |
| `exit_code` | The process exit status; usually `0` means success. `-1` means no normal exit status. |
| `timed_out` | Whether the command exceeded its time limit. |
| `output_truncated` | Whether some output was too large to keep. |

An inactive service can return a nonzero exit code. The server returns that as
result data. The demo client prints the result and exits nonzero for failed
commands, timeouts, connection errors, or tool errors, making it useful in scripts.

| Problem | What to check |
| --- | --- |
| HTTP startup fails | Set a nonempty `SHELL_MCP_API_TOKEN` before starting the server. |
| `401 Unauthorized` | Make sure client and server have the same token. |
| Connection refused | Start the HTTP server and check the address and port. |
| Executable is not allowed | Use an exact path from the allowed-command table. |
| Writes disabled | Set `SHELL_MCP_ALLOW_WRITE=true` on the server process. |
| Journal access denied or incomplete | Check the server user's journal permissions. |
| Service not found | Use a service name installed on the server host. |

Journal access may require membership in a group such as `systemd-journal` or
`adm`, or suitable file permissions. Ask your administrator for the access you
need; this project does not change groups or system settings. Service names are
validated before commands run.

When using HTTP, setting write or workspace variables on the client does not
change the running server's settings. Restart the server with the desired values.

## Limits to know about

This is a learning project, not production-grade isolation.

- Commands run directly with `exec.CommandContext`, with a 10-second time limit.
  There is no shell: pipes, redirection, `$HOME` expansion, and wildcards do not
  work. Passing `|` or `>` sends an ordinary argument to the command.
- Standard output and error are each limited to 64 KiB. Extra output is still
  read and discarded so a full output pipe cannot block the process.
- Linux permissions still apply. Workspace checks reject symlinks but are not
  race-proof if another process changes files between checking and execution.
  Hard links and other filesystem mechanisms are not isolated. Use a dedicated
  workspace and Linux account.
- HTTP authentication uses one shared token and identifies every authenticated
  request as `demo-user`. There are no user accounts, roles, or per-user rules.
  Token checks use constant-time comparison. Tokens and headers are never logged.
- There is no built-in TLS, audit trail, or limit on simultaneous tool calls.

<details>
<summary>More details about execution and HTTP connections</summary>

Timeout or cancellation kills the command's process group. Child output-pipe
cleanup has a one-second limit. Captured output is intended for text. Journal
diagnostics use the C locale so permission errors can be recognized consistently.

HTTP uses MCP Streamable HTTP at `/mcp`. Headers must arrive within five seconds;
idle HTTP connections expire after 60 seconds and idle MCP sessions after five
minutes. Overall HTTP read/write deadlines are left unset because MCP can keep
streaming connections open.

System information comes from the Linux hostname, `/etc/os-release`, and `/proc`.
These and the `/usr/bin` executable paths are Linux platform requirements, not
paths specific to the author's machine.

</details>

## Find your way around the code

| Location | Responsibility |
| --- | --- |
| `cmd/shell-mcp` | Start the server, load settings, and handle shutdown. |
| `internal/config` | Read and validate flags and environment variables. |
| `internal/server` | Create the MCP server and register tools. |
| `internal/transport` | Handle stdio and HTTP connections. |
| `internal/auth` | Check HTTP tokens and attach the authenticated identity. |
| `internal/tools` | Validate tool inputs and coordinate the work. |
| `internal/policy` | Decide which commands and paths are allowed. |
| `internal/executor` | Run processes and capture their output. |
| `demo/cmd/client` | Handle demo-client flags and display JSON. |
| `demo/internal/mcpclient` | Connect, list tools, and call them through MCP. |

A typical request follows this path:

```text
Client → transport → authentication (HTTP only) → tool → policy → executor → result
```

`system_info` reads Linux information directly instead of starting a command.
The SDK handles connection setup, the standard `tools/list` and `tools/call`
operations, and argument schemas generated from Go structs.

To add a tool, create its input/output structs and handler in `internal/tools`,
then register it in `register.go`. Add any required policy checks and tests.
To allow another executable through `run_command`, update
`internal/policy/policy.go`, review which arguments should be accepted, and update
its tests. Rebuild the binary and restart any running HTTP server afterward.

Future additions have clear places: TLS in the HTTP transport, per-user rules in
policy using `auth.Identity(ctx)`, and audit logging around tool calls. Additional
transports can reuse `internal/server.New`. The demo client preserves tool
schemas and results for a future LLM adapter; no LLM provider or agent loop is
implemented. There is no OAuth, user registration, database, or plugin framework.

## Run the checks

```sh
gofmt -w cmd internal demo
go build ./...
go test ./...
```

Tests cover command restrictions, workspace validation, timeouts, output limits,
HTTP tokens, redirect protection, and discovery/calls through both transports.
Write tests use temporary folders. Tests do not restart services or change system
configuration; journal access still depends on the target machine's permissions.

For SDK details, see its [quick start](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/docs/quick_start.md)
and [transport guide](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/docs/protocol.md).

[SKILL.md](SKILL.md) is a short repository guide for coding assistants. Every AI
tool does not automatically load it; [AGENTS.md](AGENTS.md) points supporting
assistants to it. Keep the guide up to date when behavior or configuration changes.

## License

Copyright 2026 Murali Annamneni.

Licensed under the [Apache License, Version 2.0](LICENSE).
See [NOTICE](NOTICE) for author attribution.
