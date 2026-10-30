package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware(t *testing.T) {
	calls := 0
	handler, err := Middleware("test-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		identity, ok := Identity(r.Context())
		if !ok || identity != "demo-user" {
			t.Fatal("missing identity")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST", "DELETE", "OPTIONS"} {
		for _, credential := range []string{"", "Bearer wrong", "Bearer", "Basic test-token", "Bearer test-token"} {
			r := httptest.NewRequest(method, "/mcp", nil)
			if credential != "" {
				r.Header.Set("Authorization", credential)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			expected := http.StatusUnauthorized
			if credential == "Bearer test-token" {
				expected = http.StatusNoContent
			}
			if w.Code != expected {
				t.Fatalf("%s: status %d", method, w.Code)
			}
			if w.Code == 401 && w.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing challenge")
			}
		}
	}
	if calls != 4 {
		t.Fatalf("unauthorized handler calls: %d", calls)
	}
	r := httptest.NewRequest("POST", "/mcp", nil)
	r.Header.Add("Authorization", "Bearer test-token")
	r.Header.Add("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("duplicate headers accepted")
	}
	for _, token := range []string{"", " ", "bad\ntoken"} {
		if _, err := Middleware(token, http.NotFoundHandler()); err == nil {
			t.Fatal("invalid configured token accepted")
		}
	}
}
