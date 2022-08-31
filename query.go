package main

import (
	"fmt"

	"github.com/holoplot/go-avahi"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

func createDNSReply(logger *logrus.Entry, aserver *avahi.Server, r *dns.Msg) *dns.Msg {
	m := new(dns.Msg)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		for _, q := range r.Question {
			switch q.Qtype {
			case dns.TypeA:
				rr, err := avahiToRecord(logger, aserver, q.Name, avahi.ProtoInet, "A")
				if err != nil {
					logger.WithError(err).Error("avahi A lookup failed, skipping query...")
					continue
				}
				m.Answer = append(m.Answer, rr)

			case dns.TypeAAAA:
				rr, err := avahiToRecord(logger, aserver, q.Name, avahi.ProtoInet6, "AAAA")
				if err != nil {
					logger.WithError(err).Error("avahi AAAA lookup failed, skipping query...")
					continue
				}
				m.Answer = append(m.Answer, rr)

			default:
				logger.WithField("type", q.Qtype).Warning("unsupported question")
			}
		}

	default:
		logger.WithField("opcode", r.Opcode).Warning("unsupported opcode")
	}

	return m
}

func avahiToRecord(logger *logrus.Entry, aserver *avahi.Server, name string, proto int32, recordType string) (dns.RR, error) {
	logger.WithFields(logrus.Fields{
		"name":     name,
		"type":     recordType,
		"protocol": proto,
	}).Info("forwarding query to avahi")
	hn, err := aserver.ResolveHostName(avahi.InterfaceUnspec, proto, name, proto, 0)
	if err != nil {
		return nil, fmt.Errorf("avahi resolve failure: %w", err)
	}
	rr, err := dns.NewRR(fmt.Sprintf("%s %s %s", name, recordType, hn.Address))
	if err != nil {
		return nil, fmt.Errorf("failured to create record: %w", err)
	}
	return rr, err
}
