package main

import (
	"os"

	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	cfg, err := parseArgs(logger)
	if err != nil {
		os.Exit(1)
	}

	// TODO: add warning if current process doesn't have CAP_NET_BIND_SERVICE?

	err = runServer(logger, cfg)
	if err != nil {
		logger.WithError(err).Fatal("server failed to start")
	}
}
