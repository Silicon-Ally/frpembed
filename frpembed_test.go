package frpembed

import (
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		desc      string
		in        *config
		wantError bool
	}{
		{
			desc: "valid config, blank",
			in:   &config{},
		},
		{
			desc: "valid config, standard adapter",
			in: &config{
				AdapterName:   "file",
				AdapterConfig: []string{`{"filename":"test.log"}`},
			},
		},
		{
			desc: "valid config, custom adapter",
			in: &config{
				CustomLogger: discardLogger{},
			},
		},
		{
			desc: "valid config, valid proxy",
			in: &config{
				proxies: []ProxyConfig{
					{
						Name:           "test",
						TargetDomain:   "testsite",
						UseEncryption:  true,
						UseCompression: true,
						LocalPort:      3000,
					},
				},
			},
		},
		{
			desc: "invalid config, standard and custom loggers",
			in: &config{
				AdapterName:   "file",
				AdapterConfig: []string{`{"filename":"test.log"}`},
				CustomLogger:  discardLogger{},
			},
			wantError: true,
		},
		{
			desc: "invalid config, proxy config missing name",
			in: &config{
				proxies: []ProxyConfig{
					{
						TargetDomain:   "testsite",
						UseEncryption:  true,
						UseCompression: true,
						LocalPort:      3000,
					},
				},
			},
			wantError: true,
		},
		{
			desc: "invalid config, proxy config missing target domain",
			in: &config{
				proxies: []ProxyConfig{
					{
						Name:           "test",
						UseEncryption:  true,
						UseCompression: true,
						LocalPort:      3000,
					},
				},
			},
			wantError: true,
		},
		{
			desc: "invalid config, proxy config missing port",
			in: &config{
				proxies: []ProxyConfig{
					{
						Name:           "test",
						TargetDomain:   "testsite",
						UseEncryption:  true,
						UseCompression: true,
					},
				},
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.in.validate(); err != nil {
				if test.wantError {
					// Expected, just return
					return
				}
				t.Fatalf("cfg.validate: %v", err)
			}

			if test.wantError {
				t.Fatal("no error was returned, one was expected")
			}
		})
	}
}

type discardLogger struct{}

func (discardLogger) Init(_ string) error { return nil }

func (discardLogger) WriteMsg(_ time.Time, _ string, _ int) error { return nil }

func (discardLogger) Destroy() {}

func (discardLogger) Flush() {}
