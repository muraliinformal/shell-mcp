package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
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
	s := server.New(p, executor.Runner{})
	if c.Transport == "http" {
		log.Printf("serving MCP at http://%s/mcp", c.Listen)
		return transport.HTTP(ctx, s, c.Listen, c.APIToken)
	}
	log.Print("serving MCP over stdio")
	err = transport.Stdio(ctx, s)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
