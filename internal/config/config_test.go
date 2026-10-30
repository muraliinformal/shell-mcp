package config

import (
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	empty := func(string) string { return "" }
	tokenEnv := func(key string) string {
		if key == "SHELL_MCP_API_TOKEN" {
			return "test-token"
		}
		return ""
	}
	c, err := Load(nil, empty)
	if err != nil || c.Transport != "stdio" || c.Listen != "127.0.0.1:8080" || c.AllowWrite {
		t.Fatalf("%+v %v", c, err)
	}
	for _, args := range [][]string{{"--transport=bad"}, {"--listen=localhost:8080"}, {"--listen=127.0.0.1:0"}, {"--listen=bad"}, {"--transport=http", "--listen=0.0.0.0:8080"}, {"--transport=http", "--listen=[::]:8080"}, {"extra"}} {
		if _, err := Load(args, empty); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, addr := range []string{"127.0.0.1:8080", "[::1]:8080"} {
		if _, err := Load([]string{"--transport=http", "--listen=" + addr}, tokenEnv); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Load([]string{"--transport=http", "--listen=0.0.0.0:8080"}, tokenEnv); err != nil {
		t.Fatal(err)
	}
	if _, err := Load([]string{"--transport=http"}, empty); err == nil {
		t.Fatal("missing HTTP token accepted")
	}
	if _, err := Load([]string{"--allow-unauthenticated-remote"}, empty); err == nil {
		t.Fatal("removed flag accepted")
	}
	if _, err := Load(nil, func(string) string { return "invalid" }); err == nil {
		t.Fatal("invalid boolean accepted")
	}
}

func TestWorkspacePaths(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	c, err := Load(nil, func(string) string { return "" })
	if err != nil || c.Workspace != filepath.Join(dir, DefaultWorkspace) {
		t.Fatalf("default workspace: %q, %v", c.Workspace, err)
	}
	custom := filepath.Join(t.TempDir(), "custom-workspace")
	c, err = Load(nil, func(key string) string {
		if key == "SHELL_MCP_WORKSPACE" {
			return custom
		}
		return ""
	})
	if err != nil || c.Workspace != custom {
		t.Fatalf("custom workspace: %q, %v", c.Workspace, err)
	}
	if _, err = Load(nil, func(key string) string {
		if key == "SHELL_MCP_WORKSPACE" {
			return "relative-workspace"
		}
		return ""
	}); err == nil {
		t.Fatal("relative override accepted")
	}
}
