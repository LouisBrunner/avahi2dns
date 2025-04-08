package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
)

func work(ctx context.Context, logger *logrus.Logger) error {
	cfg, err := parseArgs(logger)
	if err != nil {
		return fmt.Errorf("failed to parse args: %w", err)
	}

	// TODO: add warning if current process doesn't have CAP_NET_BIND_SERVICE?

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fwd, err := NewForwarder(logger, cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	defer fwd.Close()

	return fwd.Serve(ctx)
}

func main() {
	logger := logrus.New()
	err := work(context.Background(), logger)
	if err != nil && err != context.Canceled {
		logger.WithError(err).Fatal("failed")
		os.Exit(1)
	}
}
