package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/viper"
	xproxy "golang.org/x/net/proxy"
)

type Settings struct {
	GlobalProxyURL       string
	ConnectTimeout       time.Duration
	TLSHandshakeTimeout  time.Duration
	ResponseHeaderTimeout time.Duration
	IdleConnTimeout      time.Duration
	MaxIdleConns         int
	MaxIdleConnsPerHost  int
}

func DefaultSettings() Settings {
	return Settings{
		ConnectTimeout:        10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
	}
}

func SettingsFromViper(conf *viper.Viper) Settings {
	settings := DefaultSettings()
	settings.GlobalProxyURL = conf.GetString("transport.proxy_url")
	if value := conf.GetDuration("transport.connect_timeout"); value > 0 {
		settings.ConnectTimeout = value
	}
	if value := conf.GetDuration("transport.tls_handshake_timeout"); value > 0 {
		settings.TLSHandshakeTimeout = value
	}
	if value := conf.GetDuration("transport.response_header_timeout"); value > 0 {
		settings.ResponseHeaderTimeout = value
	}
	if value := conf.GetDuration("transport.idle_conn_timeout"); value > 0 {
		settings.IdleConnTimeout = value
	}
	if value := conf.GetInt("transport.max_idle_conns"); value > 0 {
		settings.MaxIdleConns = value
	}
	if value := conf.GetInt("transport.max_idle_conns_per_host"); value > 0 {
		settings.MaxIdleConnsPerHost = value
	}
	return settings
}

type Factory struct {
	settings Settings
}

func NewFactory(settings Settings) (*Factory, error) {
	if settings.ConnectTimeout <= 0 {
		settings.ConnectTimeout = DefaultSettings().ConnectTimeout
	}
	if settings.TLSHandshakeTimeout <= 0 {
		settings.TLSHandshakeTimeout = DefaultSettings().TLSHandshakeTimeout
	}
	if settings.ResponseHeaderTimeout <= 0 {
		settings.ResponseHeaderTimeout = DefaultSettings().ResponseHeaderTimeout
	}
	if settings.IdleConnTimeout <= 0 {
		settings.IdleConnTimeout = DefaultSettings().IdleConnTimeout
	}
	if settings.MaxIdleConns <= 0 {
		settings.MaxIdleConns = DefaultSettings().MaxIdleConns
	}
	if settings.MaxIdleConnsPerHost <= 0 {
		settings.MaxIdleConnsPerHost = DefaultSettings().MaxIdleConnsPerHost
	}
	if settings.GlobalProxyURL != "" {
		if _, err := ParseProxy(settings.GlobalProxyURL); err != nil {
			return nil, fmt.Errorf("invalid global transport proxy: %w", err)
		}
	}
	return &Factory{settings: settings}, nil
}

func NewFactoryFromViper(conf *viper.Viper) (*Factory, error) {
	return NewFactory(SettingsFromViper(conf))
}

// Client creates an isolated client for one account route. No package-global or
// shared DefaultTransport state is modified, so an account override cannot
// affect concurrent requests for another account.
//
// The returned client intentionally has no overall Timeout. Streaming bodies
// may remain open indefinitely; connect, TLS, and response-header phases are
// bounded by the transport settings instead.
func (f *Factory) Client(accountProxyURL string) (*http.Client, RouteInfo, error) {
	route, err := resolveRoute(accountProxyURL, f.settings.GlobalProxyURL)
	if err != nil {
		return nil, RouteInfo{}, err
	}

	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: f.settings.ConnectTimeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          f.settings.MaxIdleConns,
		MaxIdleConnsPerHost:   f.settings.MaxIdleConnsPerHost,
		IdleConnTimeout:       f.settings.IdleConnTimeout,
		TLSHandshakeTimeout:   f.settings.TLSHandshakeTimeout,
		ResponseHeaderTimeout: f.settings.ResponseHeaderTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}

	if route.proxy != nil {
		switch route.proxy.Scheme() {
		case ProxySchemeHTTP, ProxySchemeHTTPS:
			transport.Proxy = http.ProxyURL(cloneURL(route.proxy.proxyURL))
		case ProxySchemeSOCKS5, ProxySchemeSOCKS5H:
			dialContext, err := f.socksDialContext(*route.proxy)
			if err != nil {
				return nil, RouteInfo{}, err
			}
			transport.DialContext = dialContext
		default:
			return nil, RouteInfo{}, fmt.Errorf("unsupported route scheme %q", route.proxy.Scheme())
		}
	}

	return &http.Client{Transport: transport}, route.info, nil
}

func cloneURL(input *url.URL) *url.URL {
	if input == nil {
		return nil
	}
	copyURL := *input
	return &copyURL
}

func (f *Factory) socksDialContext(spec ProxySpec) (func(context.Context, string, string) (net.Conn, error), error) {
	var auth *xproxy.Auth
	if spec.proxyURL.User != nil {
		username := spec.proxyURL.User.Username()
		password, _ := spec.proxyURL.User.Password()
		auth = &xproxy.Auth{User: username, Password: password}
	}

	forward := &net.Dialer{Timeout: f.settings.ConnectTimeout, KeepAlive: 30 * time.Second}
	dialer, err := xproxy.SOCKS5("tcp", spec.proxyURL.Host, auth, forward)
	if err != nil {
		return nil, fmt.Errorf("create %s dialer for %s: %w", spec.Scheme(), spec.Redacted(), err)
	}

	return func(ctx context.Context, network, address string) (net.Conn, error) {
		target := address
		if !spec.RemoteDNS() {
			resolved, err := resolveAddress(ctx, address)
			if err != nil {
				return nil, fmt.Errorf("resolve SOCKS5 target %q: %w", address, err)
			}
			target = resolved
		}
		return dialProxyContext(ctx, dialer, network, target)
	}, nil
}

func resolveAddress(ctx context.Context, address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	if ip := net.ParseIP(host); ip != nil {
		return net.JoinHostPort(ip.String(), port), nil
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses for %s", host)
	}
	return net.JoinHostPort(ips[0].String(), port), nil
}

func dialProxyContext(ctx context.Context, dialer xproxy.Dialer, network, address string) (net.Conn, error) {
	if contextDialer, ok := dialer.(xproxy.ContextDialer); ok {
		return contextDialer.DialContext(ctx, network, address)
	}

	type result struct {
		conn net.Conn
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		conn, err := dialer.Dial(network, address)
		resultCh <- result{conn: conn, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		return result.conn, result.err
	}
}
