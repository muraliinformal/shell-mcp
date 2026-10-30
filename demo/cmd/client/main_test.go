package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestInvalidArguments(t *testing.T) {
	for _, raw := range []string{"not json", "[]", "null", "1", `{"x":`, "{} {}"} {
		var out, stderr bytes.Buffer
		err := run(context.Background(), []string{"--action=call", "--tool=system_info", "--arguments=" + raw}, &out, &stderr)
		if err == nil || !strings.Contains(err.Error(), "JSON object") {
			t.Fatalf("%q: %v", raw, err)
		}
		if out.Len() != 0 {
			t.Fatal("printed output for invalid arguments")
		}
	}
	for _, args := range [][]string{{"--action=nope"}, {"--action=call"}, {"--timeout=0", "--server=missing"}, {"unexpected"}} {
		var out, stderr bytes.Buffer
		if err := run(context.Background(), args, &out, &stderr); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
