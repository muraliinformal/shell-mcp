package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHelper(t *testing.T) {
	if os.Getenv("SHELL_MCP_TEST_HELPER") != "1" {
		return
	}
	switch os.Args[len(os.Args)-1] {
	case "output":
		os.Stdout.WriteString(strings.Repeat("a", CaptureLimit*3))
		os.Stderr.WriteString(strings.Repeat("b", CaptureLimit*2))
	case "sleep":
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}
func TestRunner(t *testing.T) {
	t.Setenv("SHELL_MCP_TEST_HELPER", "1")
	r, err := (Runner{}).Run(context.Background(), os.Args[0], []string{"-test.run=TestHelper", "--", "output"}, "")
	if err != nil || len(r.Stdout) != CaptureLimit || len(r.Stderr) != CaptureLimit || !r.OutputTruncated || r.ExitCode != 0 {
		t.Fatalf("capture failed %v", err)
	}
	start := time.Now()
	r, err = (Runner{}).Run(context.Background(), os.Args[0], []string{"-test.run=TestHelper", "--", "sleep"}, "")
	if err != nil || !r.TimedOut || time.Since(start) > 13*time.Second {
		t.Fatalf("timeout failed %+v %v", r, err)
	}
	dir := t.TempDir()
	r, err = (Runner{}).Run(context.Background(), "/usr/bin/touch", []string{"--", "test.txt"}, dir)
	if err != nil || r.ExitCode != 0 {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, "test.txt")); err != nil {
		t.Fatal(err)
	}
	r, err = (Runner{}).Run(context.Background(), "/usr/bin/false", nil, "")
	if err != nil || r.ExitCode != 1 {
		t.Fatalf("exit status: %+v %v", r, err)
	}
}
