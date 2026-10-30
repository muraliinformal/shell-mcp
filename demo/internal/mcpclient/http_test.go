package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHTTPAuthenticationFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "Unauthorized", 401) }))
	defer server.Close()
	for _, token := range []string{"", "wrong-token"} {
		t.Setenv("SHELL_MCP_API_TOKEN", token)
		_, err := Connect(context.Background(), Options{Transport: "http", URL: server.URL, Timeout: time.Second})
		if err == nil || !strings.Contains(err.Error(), "SHELL_MCP_API_TOKEN") {
			t.Fatalf("unclear authentication failure: %v", err)
		}
		if token != "" && strings.Contains(err.Error(), token) {
			t.Fatal("token exposed in error")
		}
	}
}

func TestBearerTransportAndRedirect(t *testing.T) {
	received := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received++; w.WriteHeader(200) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing bearer header")
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
			return
		}
		w.WriteHeader(204)
	}))
	defer source.Close()
	origin, _ := url.Parse(source.URL)
	rt := &bearerTransport{base: http.DefaultTransport, origin: origin, token: "test-token"}
	client := &http.Client{Transport: rt, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, method := range []string{"GET", "POST", "DELETE"} {
		r, _ := http.NewRequest(method, source.URL+"/mcp", nil)
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 204 || r.Header.Get("Authorization") != "" {
			t.Fatal("header attachment failed or request mutated")
		}
	}
	t.Setenv("SHELL_MCP_API_TOKEN", "test-token")
	_, connectErr := Connect(context.Background(), Options{Transport: "http", URL: source.URL + "/redirect", Timeout: time.Second})
	if connectErr == nil || !strings.Contains(connectErr.Error(), "redirects are disabled") {
		t.Fatalf("redirect not rejected by SDK client: %v", connectErr)
	}
	response, err := client.Get(source.URL + "/redirect")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	r, _ := http.NewRequest("GET", target.URL, nil)
	if _, err = rt.RoundTrip(r); err == nil {
		t.Fatal("cross-origin request accepted")
	}
	if received != 0 {
		t.Fatal("redirect target received request")
	}
}
