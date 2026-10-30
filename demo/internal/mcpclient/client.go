// Package mcpclient connects to MCP servers without depending on an LLM provider.
package mcpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Options selects one transport. Timeout applies to initialization and each
// operation (including all pages of a discovery request).
type Options struct {
	Transport string
	Server    string
	URL       string
	Timeout   time.Duration
}

// Client preserves SDK tools and results for callers such as a future LLM adapter.
type Client struct {
	session *mcp.ClientSession
	timeout time.Duration
}

func Connect(ctx context.Context, opts Options) (*Client, error) {
	if opts.Timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}
	var transport mcp.Transport
	switch opts.Transport {
	case "stdio":
		if opts.Server == "" {
			return nil, fmt.Errorf("stdio requires --server")
		}
		if opts.URL != "" {
			return nil, fmt.Errorf("--url is only supported with HTTP")
		}
		// The SDK owns starting, waiting for, and terminating this child on Close.
		cmd := exec.Command(opts.Server, "--transport=stdio")
		cmd.Stderr = os.Stderr
		transport = &mcp.CommandTransport{Command: cmd, TerminateDuration: 2 * time.Second}
	case "http":
		if opts.Server != "" {
			return nil, fmt.Errorf("--server is only supported with stdio")
		}
		endpoint, err := url.Parse(opts.URL)
		if err != nil || endpoint == nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Hostname() == "" || endpoint.Fragment != "" || endpoint.User != nil {
			return nil, fmt.Errorf("--url must be a full HTTP(S) MCP endpoint without credentials or fragment")
		}
		token := os.Getenv("SHELL_MCP_API_TOKEN")
		if strings.TrimSpace(token) == "" || strings.ContainsAny(token, " \t\r\n") {
			return nil, fmt.Errorf("HTTP requires a nonempty SHELL_MCP_API_TOKEN without whitespace")
		}
		transport = &mcp.StreamableClientTransport{Endpoint: opts.URL, HTTPClient: &http.Client{
			Transport: &bearerTransport{base: http.DefaultTransport, origin: endpoint, token: token},
			// Reject all redirects, including same-origin redirects, to keep credentials
			// pinned to the configured MCP endpoint and avoid HTTPS downgrade.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return fmt.Errorf("MCP HTTP redirects are disabled; use the final endpoint URL")
			},
		}}
	default:
		return nil, fmt.Errorf("transport must be stdio or http")
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	sdk := mcp.NewClient(&mcp.Implementation{Name: "shell-mcp-demo", Version: "0.1.0"}, nil)
	session, err := sdk.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect/initialize MCP (%s): %w", opts.Transport, err)
	}
	return &Client{session: session, timeout: opts.Timeout}, nil
}

func (c *Client) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	tools := make([]*mcp.Tool, 0)
	// SDK iterator follows nextCursor until every page has been retrieved.
	for tool, err := range c.session.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list MCP tools: %w", err)
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

// CallTool returns tool-level errors intact in result.IsError. Only connection
// or protocol failures are Go errors; the caller decides how to present results.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (*mcp.CallToolResult, error) {
	if name == "" {
		return nil, fmt.Errorf("tool name is required")
	}
	if arguments == nil {
		arguments = map[string]any{}
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		return nil, fmt.Errorf("call MCP tool %q: %w", name, err)
	}
	return result, nil
}

// Close closes the SDK session and, in stdio mode, reaps its child process.
func (c *Client) Close() error { return c.session.Close() }

// bearerTransport attaches credentials to every SDK HTTP request, including SSE
// and session deletion. It never mutates requests owned by the SDK.
type bearerTransport struct {
	base   http.RoundTripper
	origin *url.URL
	token  string
}

func (t *bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if !strings.EqualFold(r.URL.Scheme, t.origin.Scheme) || !strings.EqualFold(r.URL.Host, t.origin.Host) {
		return nil, fmt.Errorf("refusing to send MCP credentials to a different origin")
	}
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	response, err := t.base.RoundTrip(clone)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusUnauthorized {
		response.Body.Close()
		return nil, fmt.Errorf("HTTP 401: MCP authentication failed; check SHELL_MCP_API_TOKEN matches the server")
	}
	return response, nil
}
