package executor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const CaptureLimit = 64 * 1024
const Timeout = 10 * time.Second

type Result struct {
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	ExitCode        int    `json:"exit_code"`
	TimedOut        bool   `json:"timed_out"`
	OutputTruncated bool   `json:"output_truncated"`
}
type capture struct {
	data      []byte
	truncated bool
}

func (b *capture) Write(p []byte) (int, error) {
	n := len(p)
	remaining := CaptureLimit - len(b.data)
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	b.data = append(b.data, p...)
	return n, nil
}

type Runner struct{}

func (Runner) Run(ctx context.Context, executable string, args []string, dir string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	// Kill the process group as well, so children cannot keep output pipes open.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	var stdout, stderr capture
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	r := Result{string(stdout.data), string(stderr.data), 0, errors.Is(ctx.Err(), context.DeadlineExceeded), stdout.truncated || stderr.truncated}
	if err != nil {
		r.ExitCode = -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			r.ExitCode = exit.ExitCode()
		} else if ctx.Err() == nil {
			return r, err
		}
	}
	return r, nil
}
