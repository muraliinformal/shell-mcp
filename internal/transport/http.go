package transport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"example.com/shell-mcp/internal/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Handler(s *mcp.Server, token string) (http.Handler, error) {
	mux := http.NewServeMux()
	handler, err := auth.Middleware(token, mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{SessionTimeout: 5 * time.Minute}))
	if err != nil {
		return nil, err
	}
	mux.Handle("/mcp", handler)
	return mux, nil
}
func HTTP(ctx context.Context, s *mcp.Server, address, token string) error {
	handler, err := Handler(s, token)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	// No global read/write deadline: MCP may keep SSE responses open.
	h := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20, BaseContext: func(net.Listener) context.Context { return ctx }}
	results := make(chan error, 1)
	go func() { results <- h.Serve(listener) }()
	select {
	case err = <-results:
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if h.Shutdown(shutdown) != nil {
			_ = h.Close()
		}
		err = <-results
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
