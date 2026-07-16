package main

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeMustParse(t *testing.T, args []string) *int {
	t.Helper()

	exitCode := -1

	orig := mustParse
	t.Cleanup(func() { mustParse = orig })

	mustParse = func(dest ...any) *arg.Parser {
		out := &bytes.Buffer{}
		p, err := arg.NewParser(arg.Config{Exit: func(code int) { exitCode = code }, Out: out}, dest...)
		if err != nil {
			exitCode = 2
			return nil
		}
		p.MustParse(args)
		return p
	}

	return &exitCode
}

func defaultConfig(mutate func(*config)) *config {
	cfg := &config{
		Domains:  defaultDomains,
		BindAddr: "localhost",
		Port:     53,
	}
	if mutate != nil {
		mutate(cfg)
	}
	return cfg
}

func TestParseArgs(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		envs     map[string]string
		wantExit int // -1 means no exit expected
		wantErr  string
		wantCfg  *config
	}{
		{
			name:     "defaults",
			args:     []string{},
			wantExit: -1,
			wantCfg:  defaultConfig(nil),
		},
		{
			name:     "custom domain list",
			args:     []string{"-d", "foo", "-d", "bar"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.Domains = []string{"foo", "bar"} }),
		},
		{
			name:     "bind address",
			args:     []string{"--addr", "0.0.0.0"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.BindAddr = "0.0.0.0" }),
		},
		{
			name:     "bind address short flag",
			args:     []string{"-a", "192.168.1.1"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.BindAddr = "192.168.1.1" }),
		},
		{
			name:     "port",
			args:     []string{"-p", "5353"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.Port = 5353 }),
		},
		{
			name:     "debug",
			args:     []string{"-v"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.Debug = true }),
		},
		{
			name:     "v4only",
			args:     []string{"-4"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.V4only = true }),
		},
		{
			name:     "v6only",
			args:     []string{"-6"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.V6only = true }),
		},
		{
			name:     "v4only and v6only conflict",
			args:     []string{"-4", "-6"},
			wantExit: -1,
			wantErr:  "cannot set both --v4only and --v6only",
		},
		{
			name:     "timeout",
			args:     []string{"-t", "2.5s"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.Timeout = 2500 * time.Millisecond }),
		},
		{
			name:     "bind address from env",
			args:     []string{},
			envs:     map[string]string{"BIND": "10.0.0.1"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.BindAddr = "10.0.0.1" }),
		},
		{
			name:     "port from env",
			args:     []string{},
			envs:     map[string]string{"PORT": "1053"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.Port = 1053 }),
		},
		{
			name:     "flag overrides env",
			args:     []string{"-p", "2053"},
			envs:     map[string]string{"PORT": "1053"},
			wantExit: -1,
			wantCfg:  defaultConfig(func(c *config) { c.Port = 2053 }),
		},
		{
			name:     "invalid port value exits",
			args:     []string{"-p", "not-a-number"},
			wantExit: 2,
		},
		{
			name:     "help exits cleanly",
			args:     []string{"--help"},
			wantExit: 0,
		},
	}

	for _, testcase := range testCases {
		t.Run(testcase.name, func(t *testing.T) {
			for k, v := range testcase.envs {
				t.Setenv(k, v)
			}

			exitCode := fakeMustParse(t, testcase.args)

			logger := logrus.New()
			logger.SetOutput(io.Discard)

			cfg, err := parseArgs(logger)

			if testcase.wantExit != -1 {
				assert.Equal(t, testcase.wantExit, *exitCode)
				return
			}
			require.Equal(t, -1, *exitCode)

			if testcase.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), testcase.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testcase.wantCfg, cfg)
		})
	}
}
