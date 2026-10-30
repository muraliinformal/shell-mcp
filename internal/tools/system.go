package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SystemInfo struct {
	Hostname        string  `json:"hostname"`
	OperatingSystem string  `json:"operating_system"`
	KernelVersion   string  `json:"kernel_version"`
	UptimeSeconds   float64 `json:"uptime_seconds"`
}

func (t *Tools) SystemInfo(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, SystemInfo, error) {
	startOperation(ctx)
	var out SystemInfo
	if err := ctx.Err(); err != nil {
		return nil, out, err
	}
	host, err := os.Hostname()
	if err != nil {
		return nil, out, err
	}
	out.Hostname = host
	release, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, out, err
	}
	out.OperatingSystem = "Linux"
	for _, line := range strings.Split(string(release), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			out.OperatingSystem = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	kernel, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return nil, out, err
	}
	out.KernelVersion = strings.TrimSpace(string(kernel))
	uptime, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return nil, out, err
	}
	_, err = fmt.Sscanf(string(uptime), "%f", &out.UptimeSeconds)
	return nil, out, err
}
