package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultWorkspace is resolved relative to the server process working directory.
const DefaultWorkspace = "demo-workspace"

type Config struct {
	Transport, Listen, Workspace string
	APIToken                     string
	AllowWrite                   bool
}

func Load(args []string, getenv func(string) string) (Config, error) {
	c := Config{Workspace: DefaultWorkspace}
	f := flag.NewFlagSet("shell-mcp", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&c.Transport, "transport", "stdio", "stdio or http")
	f.StringVar(&c.Listen, "listen", "127.0.0.1:8080", "HTTP IP:port")
	if err := f.Parse(args); err != nil {
		return c, err
	}
	if f.NArg() != 0 {
		return c, fmt.Errorf("unexpected positional arguments")
	}
	if c.Transport != "stdio" && c.Transport != "http" {
		return c, fmt.Errorf("transport must be stdio or http")
	}
	host, port, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return c, fmt.Errorf("listen must be IP:port: %w", err)
	}
	ip := net.ParseIP(host)
	n, e := strconv.Atoi(port)
	if ip == nil || e != nil || n < 1 || n > 65535 {
		return c, fmt.Errorf("listen requires a literal IP and port 1–65535")
	}
	if c.Transport == "http" {
		c.APIToken = getenv("SHELL_MCP_API_TOKEN")
		if strings.TrimSpace(c.APIToken) == "" || strings.ContainsAny(c.APIToken, " \t\r\n") {
			return c, fmt.Errorf("HTTP requires a nonempty SHELL_MCP_API_TOKEN without whitespace")
		}
	}
	if v := getenv("SHELL_MCP_WORKSPACE"); v != "" {
		if !filepath.IsAbs(v) {
			return c, fmt.Errorf("SHELL_MCP_WORKSPACE must be an absolute path")
		}
		c.Workspace = filepath.Clean(v)
	} else {
		c.Workspace, err = filepath.Abs(DefaultWorkspace)
		if err != nil {
			return c, fmt.Errorf("resolve default workspace: %w", err)
		}
	}
	if v := getenv("SHELL_MCP_ALLOW_WRITE"); v != "" && v != "true" && v != "false" {
		return c, fmt.Errorf("SHELL_MCP_ALLOW_WRITE must be true or false")
	} else {
		c.AllowWrite = v == "true"
	}
	return c, nil
}
