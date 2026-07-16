package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

type forwarder struct {
	logger  *logrus.Logger
	address string
	avahi   avahiClient
	mux     *dns.ServeMux
	ctx     context.Context
	timeout time.Duration
	v4only  bool
	v6only  bool
	domains map[string]bool
}

func NewForwarder(logger *logrus.Logger, cfg *config, avahi avahiClient) (*forwarder, error) {
	srv := &forwarder{
		logger:  logger,
		address: fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port),
		avahi:   avahi,
		mux:     dns.NewServeMux(),
		timeout: cfg.Timeout,
		v4only:  cfg.V4only,
		v6only:  cfg.V6only,
		domains: make(map[string]bool, len(cfg.Domains)),
	}

	for _, domain := range cfg.Domains {
		logger.WithField("domain", domain).Debug("adding dns handler")
		fqdn := dns.Fqdn(domain)
		srv.mux.Handle(fqdn, dns.HandlerFunc(srv.onDNSRequest))
		srv.domains[strings.ToLower(fqdn)] = true
	}

	return srv, nil
}

func (me *forwarder) Serve(ctx context.Context) error {
	dserver := &dns.Server{
		Addr:    me.address,
		Net:     "udp",
		Handler: me.mux,
	}

	me.ctx = ctx

	go func() {
		<-ctx.Done()
		err := dserver.Shutdown()
		if err != nil {
			me.logger.WithError(err).Error("failed to shut down DNS server")
		}
	}()

	me.logger.WithField("addr", dserver.Addr).Info("starting DNS server")
	err := dserver.ListenAndServe()
	if err != nil {
		return fmt.Errorf("failed to start DNS server: %w", err)
	}
	return nil
}
