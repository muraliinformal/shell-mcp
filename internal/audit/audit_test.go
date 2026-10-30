package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentJSONAndBounds(t *testing.T) {
	var out bytes.Buffer
	logger := New(&out)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Log(Event{Event: "tool_completed", RequestID: RequestID(), UserID: strings.Repeat("u", 1000), Arguments: &Arguments{Args: make([]string, 100), Service: strings.Repeat("x", 1000)}})
		}()
	}
	wg.Wait()
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 100 {
		t.Fatalf("%d lines", len(lines))
	}
	ids := map[string]bool{}
	for _, line := range lines {
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		if _, err := time.Parse(time.RFC3339Nano, e.Timestamp); err != nil || !strings.HasSuffix(e.Timestamp, "Z") {
			t.Fatal("invalid UTC timestamp")
		}
		if len(e.UserID) > 128 || len(e.Arguments.Service) > 128 || len(e.Arguments.Args) > 16 {
			t.Fatal("metadata not bounded")
		}
		if e.RequestID == "" || ids[e.RequestID] {
			t.Fatal("missing or duplicate ID")
		}
		ids[e.RequestID] = true
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("writer unavailable") }
func TestWriterFailure(t *testing.T)           { New(brokenWriter{}).Log(Event{Event: "tool_started"}) }
