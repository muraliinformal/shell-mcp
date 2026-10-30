package main

import (
	"context"
	"errors"
	"example.com/shell-mcp/internal/audit"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"syscall"

	"example.com/shell-mcp/internal/config"
	"example.com/shell-mcp/internal/executor"
	"example.com/shell-mcp/internal/policy"
	"example.com/shell-mcp/internal/server"
	"example.com/shell-mcp/internal/transport"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	c, err := config.Load(os.Args[1:], os.Getenv)
	if err != nil {
		return err
	}
	p, err := policy.New(c.Workspace, c.AllowWrite)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(p.Workspace, 0755); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := audit.New(os.Stderr)
	localUser := fmt.Sprintf("uid:%d", os.Getuid())
	if account, err := user.Current(); err == nil {
		localUser = account.Username
	}
	s := server.New(p, executor.Runner{}, logger, c.Transport, localUser)
	if c.Transport == "http" {
		log.Printf("serving MCP at http://%s/mcp", c.Listen)
		return transport.HTTP(ctx, s, c.Listen, c.APIToken, logger)
	}
	log.Print("serving MCP over stdio")
	err = transport.Stdio(ctx, s)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
