package logging

import (
	"context"
	"log/slog"
)

type ctxLogger struct{}

func FromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(ctxLogger{}).(*slog.Logger)
	if !ok {
		logger = slog.New(slog.DiscardHandler)
	}
	return logger
}

func WithLogger(parent context.Context, logger *slog.Logger) context.Context {
	ctx := context.WithValue(parent, ctxLogger{}, logger)
	return ctx
}
