package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestNew_level(t *testing.T) {

	t.Run("warn level ignores Info logs", func(t *testing.T) {
		var buf bytes.Buffer
		logger, err := New(&buf, "dev", "warn")
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		logger.Info("Should be filtered")

		if out := buf.String(); out != "" {
			t.Errorf("output = %q, want empty", out)
		}

		logger.Warn("Warning")

		out := buf.String()

		if !strings.Contains(out, "msg=Warning") {
			t.Errorf("output = %q, want it to contain msg=hello", out)
		}

	})

	t.Run("Invalid level", func(t *testing.T) {
		logger, err := New(io.Discard, "dev", "loud")
		if logger != nil {
			t.Errorf("Logger should be nil, %v", logger)
		}

		if err == nil {
			t.Errorf("We expected an error")
		}
	})
}
func TestNew_format(t *testing.T) {
	t.Run("dev writes text", func(t *testing.T) {
		var buf bytes.Buffer
		logger, err := New(&buf, "dev", "info")
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		logger.Info("hello")

		out := buf.String()

		if !strings.Contains(out, "msg=hello") {
			t.Errorf("output = %q, want it to contain msg=hello", out)
		}
	})

	t.Run("prod writes to JSON", func(t *testing.T) {
		var m map[string]any
		var buf bytes.Buffer

		logger, err := New(&buf, "prod", "info")

		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		logger.Info("hello")

		err = json.Unmarshal(buf.Bytes(), &m)

		if err != nil {
			t.Fatalf("Error json.Unmarshal value %q, %v", buf.String(), err)
		}

		if m["msg"] != "hello" {
			t.Errorf("output msg=%v, want msg='hello'", m["msg"])
		}
	})

}
