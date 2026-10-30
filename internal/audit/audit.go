// Package audit provides best-effort JSON-line audit logging. It never logs
// process output or raw errors. Callers must build explicit safe metadata.
package audit

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"sync"
	"time"
	"unicode/utf8"
)

type Arguments struct {
	Executable       string   `json:"executable,omitempty"`
	Args             []string `json:"args,omitempty"`
	WorkingDirectory string   `json:"working_directory,omitempty"`
	Service          string   `json:"service,omitempty"`
	Lines            *int     `json:"lines,omitempty"`
}
type Event struct {
	Timestamp  string     `json:"timestamp"`
	Event      string     `json:"event"`
	RequestID  string     `json:"request_id"`
	UserID     string     `json:"user_id,omitempty"`
	Transport  string     `json:"transport"`
	Tool       string     `json:"tool,omitempty"`
	Arguments  *Arguments `json:"arguments,omitempty"`
	Status     string     `json:"status,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	DurationMS *int64     `json:"duration_ms,omitempty"`
	Reason     string     `json:"reason,omitempty"`
}

// Logger serializes writes. Use one shared instance for tools and authentication.
// A nil logger is a no-op, useful for callers that don't configure auditing.
type Logger struct {
	mu     sync.Mutex
	writer io.Writer
}

func New(writer io.Writer) *Logger { return &Logger{writer: writer} }
func RequestID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(data[:])
}
func bounded(s string) string {
	if len(s) <= 128 {
		return s
	}
	s = s[:128]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// Log ignores encoding/write failures; they never affect the operation result.
func (l *Logger) Log(e Event) {
	if l == nil || l.writer == nil {
		return
	}
	e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	e.Event = bounded(e.Event)
	e.RequestID = bounded(e.RequestID)
	e.UserID = bounded(e.UserID)
	e.Transport = bounded(e.Transport)
	e.Tool = bounded(e.Tool)
	e.Status = bounded(e.Status)
	e.Reason = bounded(e.Reason)
	if e.Arguments != nil {
		a := *e.Arguments
		a.Executable = bounded(a.Executable)
		a.Service = bounded(a.Service)
		a.WorkingDirectory = bounded(a.WorkingDirectory)
		a.Args = nil
		for i, arg := range e.Arguments.Args {
			if i == 16 {
				break
			}
			a.Args = append(a.Args, bounded(arg))
		}
		e.Arguments = &a
	}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	data = append(data, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.writer.Write(data)
}
