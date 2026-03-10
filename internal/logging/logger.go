package logging

import (
	"fmt"
	"log/slog"
	"os"
)

type Opts struct {
	Format string
	Level  string
}

func New(opts *Opts) (*slog.Logger, error) {
	var level slog.Level
	switch opts.Level {
	case "error":
		level = slog.LevelError
	case "warn":
		level = slog.LevelWarn
	case "info":
		level = slog.LevelInfo
	case "debug":
		level = slog.LevelDebug
	default:
		return nil, fmt.Errorf("invalid log level %s", opts.Level)
	}

	var handler slog.Handler
	switch opts.Format {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	case "text":
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	default:
		return nil, fmt.Errorf("invalid logging formatter %s", opts.Format)
	}

	logger := slog.New(handler)
	return logger, nil
}
