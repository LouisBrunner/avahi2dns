package main

import (
	"fmt"
	"io"

	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
)

//go:generate go tool mockery

type avahiClient interface {
	io.Closer
	ResolveHostName(iface, protocol int32, name string, aprotocol int32, flags uint32) (avahi.HostName, error)
}

type avahiConn struct {
	dbus   *dbus.Conn
	server *avahi.Server
}

func newAvahiClient() (avahiClient, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("connection to dbus failed: %w", err)
	}

	server, err := avahi.ServerNew(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("connection to avahi failed: %w", err)
	}

	return &avahiConn{dbus: conn, server: server}, nil
}

func (me *avahiConn) ResolveHostName(iface, protocol int32, name string, aprotocol int32, flags uint32) (avahi.HostName, error) {
	return me.server.ResolveHostName(iface, protocol, name, aprotocol, flags)
}

func (me *avahiConn) Close() error {
	me.server.Close()
	return me.dbus.Close()
}
