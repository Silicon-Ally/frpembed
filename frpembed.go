// Package frpembed embeds an FRP client in an application. It's really just
// a thin shim around the FRP client libraries themselves, but provides a
// convenient interface for quickly setting up a reverse proxy. The usecase
// for such a reverse proxy is the same as any of the reasons one might use
// ngrok (https://ngrok.com/).
//
// Example usage:
//
//  proxyOpt := frpembed.WithProxies(frp.ProxyConfig{
//    Name: "web-server",
//    TargetDomain: "mysite.frp.mydomain.com",
//    UseEncryption: true,
//    UseCompression: true,
//    LocalPort: 8080,
//  })
//  if err := frpembed.Run(ctx, "frp.mydomain.com", "secret-token-123", proxyOpt); err != nil {
//    log.Fatalf("error while running frp client: %w", err)
//  }
//
// Check out the examples/ directory for more, well, examples.
package frpembed

import (
	"context"
	"errors"
	"fmt"
	"time"

	beegologs "github.com/fatedier/beego/logs"
	frpclient "github.com/fatedier/frp/client"
	frpconfig "github.com/fatedier/frp/pkg/config"
	frpconsts "github.com/fatedier/frp/pkg/consts"
	frplog "github.com/fatedier/frp/pkg/util/log"
)

type Logger = beegologs.Logger

type config struct {
	// Currently, the beego logging library supports 'console', 'file', 'smtp',
	// and 'conn' by default,
	// see https://pkg.go.dev/github.com/fatedier/beego/logs#section-readme
	// Only one of these options or CustomLogger may be specified
	AdapterName   string
	AdapterConfig []string

	// Provide a custom logger. Only one of this or the Adapter* options may be specified
	CustomLogger Logger

	serverPort int

	proxies []ProxyConfig

	gracefulCloseDuration time.Duration
}

type ConfigOpt func(*config)

func WithLogAdapter(name string, configs ...string) ConfigOpt {
	return func(c *config) {
		c.AdapterName = name
		c.AdapterConfig = configs
	}
}

func WithCustomLogger(lg Logger) ConfigOpt {
	return func(c *config) {
		c.CustomLogger = lg
	}
}

// WithProxies adds the provided proxy configurations to FRP.
func WithProxies(proxies ...ProxyConfig) ConfigOpt {
	return func(c *config) {
		c.proxies = append(c.proxies, proxies...)
	}
}

func WithGracefulCloseDuration(d time.Duration) ConfigOpt {
	return func(c *config) {
		c.gracefulCloseDuration = d
	}
}

type ProxyConfig struct {
	// Name is a unique identifier for the proxy configuration.
	Name string
	// TargetDomain is the location this should be available at, like webhooktest.frp.mydomain.com
	TargetDomain string

	UseEncryption  bool
	UseCompression bool
	LocalPort      int
}

func (pc ProxyConfig) validate() error {
	if pc.Name == "" {
		return errors.New("no proxy name was specified")
	}

	if pc.TargetDomain == "" {
		return errors.New("no target domain was specified")
	}

	if pc.LocalPort == 0 {
		return errors.New("no local port was specified")
	}

	return nil
}

func (c *config) validate() error {
	adapterSet := c.AdapterName != "" || len(c.AdapterConfig) > 0
	customLoggerSet := c.CustomLogger != nil
	if adapterSet && customLoggerSet {
		return errors.New("can only set one of adapter logger or custom logger")
	}

	for _, pc := range c.proxies {
		if err := pc.validate(); err != nil {
			return fmt.Errorf("failed to validate proxy %q: %w", pc.Name, err)
		}
	}

	return nil
}

// Run configures an FRP client against the provided configuration.
//  - serverAddr is the domain of the FRP server, like frp.mydomain.com
//  - serverPort is the port of the FRP server,
//  - token is the authentication token to use with the FRP server. 'oidc' mode isn't currently supported.
func Run(ctx context.Context, serverAddr string, token string, opts ...ConfigOpt) error {
	cfg := &config{
		gracefulCloseDuration: 5 * time.Second,
	}
	for _, o := range opts {
		o(cfg)
	}
	if err := cfg.validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	conf := frpconfig.GetDefaultClientConf()
	conf.AuthenticationMethod = "token"
	conf.Token = token
	conf.ServerAddr = serverAddr
	if cfg.serverPort != 0 {
		conf.ServerPort = cfg.serverPort
	}

	// Configure logging.
	switch {
	case cfg.CustomLogger != nil:
		beegologs.Register("custom", func() beegologs.Logger {
			return cfg.CustomLogger
		})
		// We assume there's no config for a custom logger. If there is, the caller can
		// include it directly in their custom implementation.
		frplog.Log.SetLogger("custom", "")
	case cfg.AdapterName != "":
		frplog.Log.SetLogger(cfg.AdapterName, cfg.AdapterConfig...)
	}

	proxyConf := make(map[string]frpconfig.ProxyConf)
	for _, pc := range cfg.proxies {
		hpc, err := toHTTPProxyConf(pc)
		if err != nil {
			return fmt.Errorf("failed to convert config %q: %w", pc.Name, err)
		}
		proxyConf[pc.Name] = hpc
	}

	svr, err := frpclient.NewService(
		conf,
		proxyConf,
		make(map[string]frpconfig.VisitorConf),
		"", // config file
	)
	if err != nil {
		return fmt.Errorf("failed to init FRP client: %w", err)
	}
	defer svr.GracefulClose(cfg.gracefulCloseDuration)

	errC := make(chan error)
	go func() {
		errC <- svr.Run()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errC:
		return err
	}
}

func toHTTPProxyConf(pc ProxyConfig) (*frpconfig.HTTPProxyConf, error) {
	defaultProxyCfg := frpconfig.DefaultProxyConf(frpconsts.HTTPProxy)
	proxyCfg, ok := defaultProxyCfg.(*frpconfig.HTTPProxyConf)
	if !ok {
		return nil, fmt.Errorf("unexpected default proxy config type %T, expected *frpconfig.HTTPProxyConf", defaultProxyCfg)
	}

	proxyCfg.ProxyName = pc.Name
	proxyCfg.UseEncryption = pc.UseEncryption
	proxyCfg.UseCompression = pc.UseCompression
	proxyCfg.LocalPort = pc.LocalPort
	proxyCfg.DomainConf = frpconfig.DomainConf{SubDomain: pc.TargetDomain}
	proxyCfg.Locations = []string{"/"}

	return proxyCfg, nil
}
