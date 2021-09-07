package main

import (
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

func runServer(logger *logrus.Logger, cfg *config) error {
	// connect to dbus
	logger.Debug("connection to dbus...")
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("connection to dbus failed: %w", err)
	}

	// connect to avahi server
	logger.Debug("connection to avahi through dbus...")
	aserver, err := avahi.ServerNew(conn)
	if err != nil {
		return fmt.Errorf("connection to avahi failed: %w", err)
	}

	// add dns handlers
	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		rlogger := logger.WithField("component", "main")
		rlogger.WithField("request", r).Debug("received request")
		m := createDNSReply(rlogger, aserver, r)
		rlogger.WithField("reply", m).Debug("sending reply")
		m.SetReply(r)
		w.WriteMsg(m)
	}
	for _, domain := range cfg.Domains {
		logger.WithField("domain", domain).Debug("adding dns handler")
		dns.HandleFunc(fmt.Sprintf("%s.", domain), handler)
	}

	// start DNS server
	dserver := &dns.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port),
		Net:  "udp",
	}
	logger.WithField("addr", dserver.Addr).Info("starting DNS server")
	err = dserver.ListenAndServe()
	defer dserver.Shutdown()
	if err != nil {
		return fmt.Errorf("failed to start DNS server: %w", err)
	}
	return nil
}
