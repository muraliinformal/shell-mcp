package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommands(t *testing.T) {
	dir := t.TempDir()
	p, err := New(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []string{"/usr/bin/uptime", "/usr/bin/df", "/usr/bin/free"} {
		if _, _, err := p.Command(e, nil, ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range []string{"uptime", "/bin/sh", "/usr/bin/touch", "/usr/bin/mkdir"} {
		if _, _, err := p.Command(e, []string{"file"}, ""); err == nil {
			t.Fatalf("allowed %s", e)
		}
	}
	p.AllowWrite = true
	for _, a := range []string{"/tmp/x", "../x", "a/../b", "-p", "a//b", "./x"} {
		if _, _, err := p.Command("/usr/bin/touch", []string{a}, ""); err == nil {
			t.Fatalf("allowed %s", a)
		}
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"link/file", "link"} {
		if _, _, err := p.Command("/usr/bin/touch", []string{a}, ""); err == nil {
			t.Fatal("allowed symlink")
		}
	}
	if _, _, err := p.Command("/usr/bin/df", nil, filepath.Dir(dir)); err == nil {
		t.Fatal("escaped workspace")
	}
	if _, _, err := p.Command("/usr/bin/df", nil, "link"); err == nil {
		t.Fatal("symlink cwd")
	}
	args, wd, err := p.Command("/usr/bin/touch", []string{"hello.txt"}, "")
	if err != nil || wd != dir || len(args) != 2 || args[0] != "--" {
		t.Fatalf("%v %s %v", args, wd, err)
	}
}
func TestService(t *testing.T) {
	for _, s := range []string{"-x", "a/b", "a;id", "", "a\n"} {
		if Service(s) == nil {
			t.Fatalf("allowed %q", s)
		}
	}
	if Service("ssh.service") != nil {
		t.Fatal("valid service rejected")
	}
}
