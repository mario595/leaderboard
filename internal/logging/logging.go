package logging

import (
	"fmt"
	"io"
	"log/slog"
)

// New builds a logger that writes to w: JSON when env is "prod", text otherwise
func New(w io.Writer, env, level string) (*slog.Logger, error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("parse log level %q: %w", level, err)
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var h slog.Handler

	if env == "prod" {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}

	return slog.New(h), nil
}
