package main

import (
	"github.com/alexflint/go-arg"
	"github.com/sirupsen/logrus"
)

var defaultDomains = []string{
	"home",
	"internal",
	"intranet",
	"lan",
	"local",
	"private",
	"test",
}

type config struct {
	Domains  []string `arg:"-d" help:"comma-separated list of domains to resolve"`
	BindAddr string   `arg:"-a,--addr" default:"localhost" help:"address to bind on" env:"BIND"`
	Port     uint16   `arg:"-p" default:"53" help:"port to bind on" env:"PORT"`
	Debug    bool     `arg:"-v" default:"false" help:"also include debug information"`
}

func parseArgs(logger *logrus.Logger) (*config, error) {
	cfg := &config{}
	arg.MustParse(cfg)
	if len(cfg.Domains) == 0 {
		cfg.Domains = defaultDomains
	}
	configureLogger(logger, cfg)
	logger.WithField("config", cfg).Debug("config parsed")
	return cfg, nil
}

func configureLogger(logger *logrus.Logger, cfg *config) {
	if cfg.Debug {
		logger.SetReportCaller(true)
		logger.SetLevel(logrus.DebugLevel)
	}
}
