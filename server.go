package main

import (
	"context"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

type forwarder struct {
	logger  *logrus.Logger
	address string
	dbus    *dbus.Conn
	avahi   *avahi.Server
	mux     *dns.ServeMux
	ctx     context.Context
	timeout time.Duration
}

func NewForwarder(logger *logrus.Logger, cfg *config) (*forwarder, error) {
	srv := &forwarder{
		logger:  logger,
		address: fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port),
		mux:     dns.NewServeMux(),
		timeout: cfg.Timeout,
	}

	var err error

	srv.logger.Debug("connection to dbus...")
	srv.dbus, err = dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("connection to dbus failed: %w", err)
	}

	logger.Debug("connection to avahi through dbus...")
	srv.avahi, err = avahi.ServerNew(srv.dbus)
	if err != nil {
		defer srv.dbus.Close()
		return nil, fmt.Errorf("connection to avahi failed: %w", err)
	}

	for _, domain := range cfg.Domains {
		logger.WithField("domain", domain).Debug("adding dns handler")
		srv.mux.Handle(fmt.Sprintf("%s.", domain), dns.HandlerFunc(srv.onDNSRequest))
	}

	return srv, nil
}

func (me *forwarder) Close() error {
	if me.avahi != nil {
		me.avahi.Close()
	}
	if me.dbus != nil {
		me.dbus.Close()
	}
	return nil
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
		dserver.Shutdown()
	}()

	me.logger.WithField("addr", dserver.Addr).Info("starting DNS server")
	err := dserver.ListenAndServe()
	if err != nil {
		return fmt.Errorf("failed to start DNS server: %w", err)
	}
	return nil
}
