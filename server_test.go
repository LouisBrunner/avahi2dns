package main

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/holoplot/go-avahi"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDomainLocal = "local"
	testServerAddr  = "127.0.0.1:15353"
	testServerPort  = 15353
)

func testConfig(mutate func(*config)) *config {
	cfg := &config{
		Domains:  []string{testDomainLocal},
		BindAddr: "127.0.0.1",
		Port:     testServerPort,
	}
	if mutate != nil {
		mutate(cfg)
	}
	return cfg
}

func startTestForwarder(t *testing.T, cfg *config, client avahiClient) string {
	t.Helper()

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	fwd, err := NewForwarder(logger, cfg, client)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := fwd.Serve(ctx)
		if err != nil {
			t.Log(err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	return testServerAddr
}

func exchange(t *testing.T, addr string, m *dns.Msg) (*dns.Msg, error) {
	t.Helper()

	client := &dns.Client{Timeout: 100 * time.Millisecond}

	var reply *dns.Msg
	var err error
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		reply, _, err = client.Exchange(m, addr)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return reply, err
}

func mustRR(t *testing.T, s string) dns.RR {
	t.Helper()

	rr, err := dns.NewRR(s)
	require.NoError(t, err)

	return rr
}

func repack(t *testing.T, msg *dns.Msg) *dns.Msg {
	t.Helper()

	buf, err := msg.Pack()
	require.NoError(t, err)

	result := new(dns.Msg)
	require.NoError(t, result.Unpack(buf))

	return result
}

func assertExchange(t *testing.T, addr string, query, want dns.Msg, wantErr string) {
	t.Helper()

	reply, err := exchange(t, addr, &query)

	if wantErr != "" {
		require.Error(t, err)
		assert.Contains(t, err.Error(), wantErr)
		return
	}

	require.NoError(t, err)
	require.NotNil(t, reply)

	want.Id = reply.Id
	want.Response = true
	if want.Rcode == dns.RcodeSuccess || want.Rcode == dns.RcodeRefused {
		want.Question = query.Question
	}
	assert.Equal(t, repack(t, &want), reply)
}

func TestForwarder(t *testing.T) {
	testCases := []struct {
		name      string
		cfg       func(c *config)
		query     dns.Msg
		setupMock func(client *mockavahiClient)
		want      dns.Msg
		wantErr   string
	}{
		{
			name:  "SOA apex of the only configured domain",
			query: dns.Msg{Question: []dns.Question{{Name: "local.", Qtype: dns.TypeSOA, Qclass: dns.ClassINET}}},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
				Answer: []dns.RR{mustRR(t, "local. 10 SOA local. avahi2dns.local. 1 60 60 3600 10")},
			},
		},
		{
			name:  "SOA apex of a custom domain list",
			cfg:   func(c *config) { c.Domains = []string{"foo", "bar"} },
			query: dns.Msg{Question: []dns.Question{{Name: "bar.", Qtype: dns.TypeSOA, Qclass: dns.ClassINET}}},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
				Answer: []dns.RR{mustRR(t, "bar. 10 SOA bar. avahi2dns.bar. 1 60 60 3600 10")},
			},
		},
		{
			name:  "SOA non-apex name under a configured domain",
			query: dns.Msg{Question: []dns.Question{{Name: "something.local.", Qtype: dns.TypeSOA, Qclass: dns.ClassINET}}},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
			},
		},
		{
			name:  "SOA for a non-supported apex",
			query: dns.Msg{Question: []dns.Question{{Name: "example.com.", Qtype: dns.TypeSOA, Qclass: dns.ClassINET}}},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
					Rcode:  dns.RcodeRefused,
				},
			},
		},
		{
			name:  "A record resolves",
			query: dns.Msg{Question: []dns.Question{{Name: "host.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
			setupMock: func(client *mockavahiClient) {
				client.EXPECT().
					ResolveHostName(int32(avahi.InterfaceUnspec), int32(avahi.ProtoInet), "host.local.", int32(avahi.ProtoInet), uint32(0)).
					Return(avahi.HostName{Address: "192.168.1.42"}, nil)
			},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
				Answer: []dns.RR{mustRR(t, "host.local. A 192.168.1.42")},
			},
		},
		{
			name:  "AAAA record resolves",
			query: dns.Msg{Question: []dns.Question{{Name: "host.local.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}}},
			setupMock: func(client *mockavahiClient) {
				client.EXPECT().
					ResolveHostName(int32(avahi.InterfaceUnspec), int32(avahi.ProtoInet6), "host.local.", int32(avahi.ProtoInet6), uint32(0)).
					Return(avahi.HostName{Address: "fe80::1"}, nil)
			},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
				Answer: []dns.RR{mustRR(t, "host.local. AAAA fe80::1")},
			},
		},
		{
			name:  "A query skipped when v6only",
			cfg:   func(c *config) { c.V6only = true },
			query: dns.Msg{Question: []dns.Question{{Name: "host.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
			},
		},
		{
			name:  "AAAA query skipped when v4only",
			cfg:   func(c *config) { c.V4only = true },
			query: dns.Msg{Question: []dns.Question{{Name: "host.local.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}}},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
			},
		},
		{
			name:  "avahi resolution failure yields no answer",
			query: dns.Msg{Question: []dns.Question{{Name: "host.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
			setupMock: func(client *mockavahiClient) {
				client.EXPECT().
					ResolveHostName(int32(avahi.InterfaceUnspec), int32(avahi.ProtoInet), "host.local.", int32(avahi.ProtoInet), uint32(0)).
					Return(avahi.HostName{}, errors.New("no such host"))
			},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
			},
		},
		{
			name:  "avahi resolution times out",
			cfg:   func(c *config) { c.Timeout = 10 * time.Millisecond },
			query: dns.Msg{Question: []dns.Question{{Name: "host.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
			setupMock: func(client *mockavahiClient) {
				client.EXPECT().
					ResolveHostName(int32(avahi.InterfaceUnspec), int32(avahi.ProtoInet), "host.local.", int32(avahi.ProtoInet), uint32(0)).
					Run(func(_ int32, _ int32, _ string, _ int32, _ uint32) {
						time.Sleep(100 * time.Millisecond)
					}).
					Return(avahi.HostName{Address: "192.168.1.1"}, nil)
			},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
			},
		},
		{
			name:  "unsupported question type",
			query: dns.Msg{Question: []dns.Question{{Name: "host.local.", Qtype: dns.TypeTXT, Qclass: dns.ClassINET}}},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode:        dns.OpcodeQuery,
					Authoritative: true,
					Rcode:         dns.RcodeSuccess,
				},
			},
		},
		{
			name:  "unconfigured domain",
			query: dns.Msg{Question: []dns.Question{{Name: "example.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{
					Opcode: dns.OpcodeQuery,
					Rcode:  dns.RcodeRefused,
				},
			},
		},
		{
			name: "multiple questions is rejected",
			query: dns.Msg{Question: []dns.Question{
				{Name: "host.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET},
				{Name: "host.local.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET},
			}},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{Rcode: dns.RcodeFormatError},
			},
		},
		{
			name: "zero questions is rejected",
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{Rcode: dns.RcodeFormatError},
			},
		},
		{
			name:  "unsupported opcode is rejected",
			query: dns.Msg{MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeStatus}, Question: []dns.Question{{Name: "host.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
			want: dns.Msg{
				MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeStatus, Rcode: dns.RcodeNotImplemented},
			},
		},
		{
			name:    "a response message sent as a request is ignored",
			query:   dns.Msg{MsgHdr: dns.MsgHdr{Response: true}, Question: []dns.Question{{Name: "host.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
			wantErr: "i/o timeout",
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			client := newMockavahiClient(t)
			if testcase.setupMock != nil {
				testcase.setupMock(client)
			}

			addr := startTestForwarder(t, testConfig(testcase.cfg), client)

			assertExchange(t, addr, testcase.query, testcase.want, testcase.wantErr)
		})
	}
}

func TestForwarderConcurrentRequests(t *testing.T) {
	type concurrentQuery struct {
		name      string
		query     dns.Msg
		setupMock func(client *mockavahiClient)
		want      dns.Msg
	}

	scenarios := []struct {
		name    string
		cfg     func(c *config)
		queries []concurrentQuery
	}{
		{
			name: "mixed A/AAAA successes and failures alongside SOA",
			queries: []concurrentQuery{
				{
					name:  "A succeeds",
					query: dns.Msg{Question: []dns.Question{{Name: "host1.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
					setupMock: func(client *mockavahiClient) {
						client.EXPECT().
							ResolveHostName(int32(avahi.InterfaceUnspec), int32(avahi.ProtoInet), "host1.local.", int32(avahi.ProtoInet), uint32(0)).
							Return(avahi.HostName{Address: "192.168.1.1"}, nil)
					},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
						Answer: []dns.RR{mustRR(t, "host1.local. A 192.168.1.1")},
					},
				},
				{
					name:  "A fails",
					query: dns.Msg{Question: []dns.Question{{Name: "host2.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
					setupMock: func(client *mockavahiClient) {
						client.EXPECT().
							ResolveHostName(int32(avahi.InterfaceUnspec), int32(avahi.ProtoInet), "host2.local.", int32(avahi.ProtoInet), uint32(0)).
							Return(avahi.HostName{}, errors.New("no such host"))
					},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
					},
				},
				{
					name:  "AAAA succeeds",
					query: dns.Msg{Question: []dns.Question{{Name: "host3.local.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}}},
					setupMock: func(client *mockavahiClient) {
						client.EXPECT().
							ResolveHostName(int32(avahi.InterfaceUnspec), int32(avahi.ProtoInet6), "host3.local.", int32(avahi.ProtoInet6), uint32(0)).
							Return(avahi.HostName{Address: "fe80::1"}, nil)
					},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
						Answer: []dns.RR{mustRR(t, "host3.local. AAAA fe80::1")},
					},
				},
				{
					name:  "AAAA fails",
					query: dns.Msg{Question: []dns.Question{{Name: "host4.local.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}}},
					setupMock: func(client *mockavahiClient) {
						client.EXPECT().
							ResolveHostName(int32(avahi.InterfaceUnspec), int32(avahi.ProtoInet6), "host4.local.", int32(avahi.ProtoInet6), uint32(0)).
							Return(avahi.HostName{}, errors.New("no such host"))
					},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
					},
				},
				{
					name:  "SOA succeeds",
					query: dns.Msg{Question: []dns.Question{{Name: "local.", Qtype: dns.TypeSOA, Qclass: dns.ClassINET}}},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
						Answer: []dns.RR{mustRR(t, "local. 10 SOA local. avahi2dns.local. 1 60 60 3600 10")},
					},
				},
			},
		},
		{
			name: "v6only applies to every concurrent request",
			cfg:  func(c *config) { c.V6only = true },
			queries: []concurrentQuery{
				{
					name:  "first A is skipped",
					query: dns.Msg{Question: []dns.Question{{Name: "host1.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
					},
				},
				{
					name:  "second A is skipped",
					query: dns.Msg{Question: []dns.Question{{Name: "host2.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
					},
				},
				{
					name:  "AAAA resolves",
					query: dns.Msg{Question: []dns.Question{{Name: "host3.local.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}}},
					setupMock: func(client *mockavahiClient) {
						client.EXPECT().
							ResolveHostName(int32(avahi.InterfaceUnspec), int32(avahi.ProtoInet6), "host3.local.", int32(avahi.ProtoInet6), uint32(0)).
							Return(avahi.HostName{Address: "fe80::1"}, nil)
					},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
						Answer: []dns.RR{mustRR(t, "host3.local. AAAA fe80::1")},
					},
				},
			},
		},
		{
			name: "v4only applies to every concurrent request",
			cfg:  func(c *config) { c.V4only = true },
			queries: []concurrentQuery{
				{
					name:  "first AAAA is skipped",
					query: dns.Msg{Question: []dns.Question{{Name: "host1.local.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}}},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
					},
				},
				{
					name:  "second AAAA is skipped",
					query: dns.Msg{Question: []dns.Question{{Name: "host2.local.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}}},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
					},
				},
				{
					name:  "A resolves",
					query: dns.Msg{Question: []dns.Question{{Name: "host3.local.", Qtype: dns.TypeA, Qclass: dns.ClassINET}}},
					setupMock: func(client *mockavahiClient) {
						client.EXPECT().
							ResolveHostName(int32(avahi.InterfaceUnspec), int32(avahi.ProtoInet), "host3.local.", int32(avahi.ProtoInet), uint32(0)).
							Return(avahi.HostName{Address: "192.168.1.1"}, nil)
					},
					want: dns.Msg{
						MsgHdr: dns.MsgHdr{Opcode: dns.OpcodeQuery, Authoritative: true, Rcode: dns.RcodeSuccess},
						Answer: []dns.RR{mustRR(t, "host3.local. A 192.168.1.1")},
					},
				},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			client := newMockavahiClient(t)
			for _, query := range scenario.queries {
				if query.setupMock != nil {
					query.setupMock(client)
				}
			}

			addr := startTestForwarder(t, testConfig(scenario.cfg), client)

			for _, query := range scenario.queries {
				t.Run(query.name, func(t *testing.T) {
					t.Parallel()

					assertExchange(t, addr, query.query, query.want, "")
				})
			}
		})
	}
}
