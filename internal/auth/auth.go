// Package auth authenticates HTTP requests independently of command policy.
package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

type identityKey struct{}

// Identity returns the authenticated HTTP identity, if middleware set one.
func Identity(ctx context.Context) (string, bool) {
	identity, ok := ctx.Value(identityKey{}).(string)
	return identity, ok
}

// Middleware requires bearer credentials for every request passed to next.
// Hashing first makes the constant-time comparison independent of token length.
func Middleware(token string, next http.Handler) (http.Handler, error) {
	if strings.TrimSpace(token) == "" || strings.ContainsAny(token, " \t\r\n") {
		return nil, fmt.Errorf("SHELL_MCP_API_TOKEN must be nonempty and contain no whitespace")
	}
	expected := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := r.Header.Values("Authorization")
		fields := strings.Fields(r.Header.Get("Authorization"))
		supplied := ""
		valid := len(headers) == 1 && len(fields) == 2 && strings.EqualFold(fields[0], "Bearer")
		if valid {
			supplied = fields[1]
		}
		actual := sha256.Sum256([]byte(supplied))
		if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 || !valid {
			w.Header().Set("WWW-Authenticate", `Bearer realm="shell-mcp"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, "demo-user")))
	}), nil
}
