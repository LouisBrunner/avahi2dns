package main

import (
	"context"
	"fmt"

	"github.com/holoplot/go-avahi"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

func (me *forwarder) onDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	ctx := me.ctx
	if me.timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, me.timeout)
		defer cancel()
	}

	me.logger.WithField("request", r).Debug("received request")
	m := me.forwardToAvahi(ctx, r)
	m.SetReply(r)
	me.logger.WithField("reply", m).Debug("sending reply")
	err := w.WriteMsg(m)
	if err != nil {
		me.logger.WithError(err).Error("failed to write response")
	}
}

func (me *forwarder) forwardToAvahi(ctx context.Context, r *dns.Msg) *dns.Msg {
	m := new(dns.Msg)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		for _, q := range r.Question {
			switch q.Qtype {
			case dns.TypeA:
				rr, err := me.queryAvahi(ctx, q.Name, avahi.ProtoInet, "A")
				if err != nil {
					me.logger.WithError(err).Error("avahi A lookup failed, skipping query...")
					continue
				}
				m.Answer = append(m.Answer, rr)

			case dns.TypeAAAA:
				rr, err := me.queryAvahi(ctx, q.Name, avahi.ProtoInet6, "AAAA")
				if err != nil {
					me.logger.WithError(err).Error("avahi AAAA lookup failed, skipping query...")
					continue
				}
				m.Answer = append(m.Answer, rr)

			default:
				me.logger.WithField("type", q.Qtype).Warning("unsupported question")
			}
		}

	default:
		me.logger.WithField("opcode", r.Opcode).Warning("unsupported opcode")
	}

	return m
}

func (me *forwarder) queryAvahi(ctx context.Context, name string, proto int32, recordType string) (dns.RR, error) {
	me.logger.WithFields(logrus.Fields{
		"name":     name,
		"type":     recordType,
		"protocol": proto,
	}).Info("forwarding query to avahi")
	address, err := me.doAvahiRequest(ctx, name, proto)
	if err != nil {
		return nil, err
	}
	rr, err := dns.NewRR(fmt.Sprintf("%s %s %s", name, recordType, address))
	if err != nil {
		return nil, fmt.Errorf("failed to create result record: %w", err)
	}
	return rr, err
}

func (me *forwarder) doAvahiRequest(ctx context.Context, name string, proto int32) (string, error) {
	type avahiResult struct {
		address string
		err     error
	}

	resultChan := make(chan avahiResult)

	go func() {
		defer close(resultChan)

		hn, err := me.avahi.ResolveHostName(avahi.InterfaceUnspec, proto, name, proto, 0)
		select {
		case resultChan <- avahiResult{address: hn.Address, err: err}:
		default:
			if err != nil {
				me.logger.WithError(err).Error("avahi resolve failed (post-timeout)")
			}
		}
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("avahi request timed out")
	case result := <-resultChan:
		if result.err != nil {
			return "", fmt.Errorf("avahi resolve failure: %w", result.err)
		}
		return result.address, nil
	}
}
