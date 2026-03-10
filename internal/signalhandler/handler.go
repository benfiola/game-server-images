package signalhandler

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/benfiola/game-server-images/internal/logging"
)

func Setup(ctx context.Context, cb func(ctx context.Context, signal os.Signal)) {
	logger := logging.FromContext(ctx)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		logger.Info("received signal", "signal", sig)
		cb(ctx, sig)
	}()
}
