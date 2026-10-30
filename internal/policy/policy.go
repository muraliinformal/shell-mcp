package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Policy struct {
	Workspace  string
	AllowWrite bool
}

func New(workspace string, write bool) (*Policy, error) {
	if !filepath.IsAbs(workspace) {
		return nil, fmt.Errorf("workspace must be absolute")
	}
	p := &Policy{filepath.Clean(workspace), write}
	if err := noSymlinks(p.Workspace); err != nil {
		return nil, err
	}
	return p, nil
}

// Check every existing component, including workspace ancestors. This is not race-proof isolation.
func noSymlinks(path string) error {
	current := "/"
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed: %s", current)
		}
	}
	return nil
}
func simple(path string) bool {
	if path == "" || filepath.IsAbs(path) || strings.HasPrefix(path, "-") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".." || part == "." || part == "" {
			return false
		}
	}
	return true
}
func (p *Policy) Directory(dir string) (string, error) {
	if dir == "" {
		dir = p.Workspace
	} else if !filepath.IsAbs(dir) {
		dir = filepath.Join(p.Workspace, dir)
	}
	dir = filepath.Clean(dir)
	rel, err := filepath.Rel(p.Workspace, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("working directory must be within workspace")
	}
	if err = noSymlinks(dir); err != nil {
		return "", err
	}
	return dir, nil
}
func (p *Policy) Command(executable string, args []string, dir string) ([]string, string, error) {
	switch executable {
	case "/usr/bin/uptime", "/usr/bin/df", "/usr/bin/free":
	case "/usr/bin/touch", "/usr/bin/mkdir":
		if !p.AllowWrite {
			return nil, "", fmt.Errorf("writes disabled; set SHELL_MCP_ALLOW_WRITE=true")
		}
	default:
		return nil, "", fmt.Errorf("executable is not allowed")
	}
	wd, err := p.Directory(dir)
	if err != nil {
		return nil, "", err
	}
	if executable == "/usr/bin/touch" || executable == "/usr/bin/mkdir" {
		if len(args) == 0 {
			return nil, "", fmt.Errorf("provide at least one relative path")
		}
		safe := []string{"--"}
		for _, arg := range args {
			if !simple(arg) {
				return nil, "", fmt.Errorf("write arguments must be simple relative paths without flags or ..")
			}
			if err := noSymlinks(filepath.Join(wd, arg)); err != nil {
				return nil, "", err
			}
			safe = append(safe, arg)
		}
		return safe, wd, nil
	}
	return append([]string(nil), args...), wd, nil
}

var servicePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@:-]{0,199}$`)

func Service(name string) error {
	if !servicePattern.MatchString(name) {
		return fmt.Errorf("invalid service name")
	}
	return nil
}
