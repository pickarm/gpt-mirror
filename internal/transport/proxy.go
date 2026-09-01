package transport

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

type ProxyScheme string

const (
	ProxySchemeHTTP    ProxyScheme = "http"
	ProxySchemeHTTPS   ProxyScheme = "https"
	ProxySchemeSOCKS5  ProxyScheme = "socks5"
	ProxySchemeSOCKS5H ProxyScheme = "socks5h"
)

// ProxySpec is a validated outbound proxy configuration. The parsed URL is
// kept private so callers cannot accidentally log credentials.
type ProxySpec struct {
	scheme    ProxyScheme
	proxyURL  *url.URL
	redacted  string
	remoteDNS bool
}

func (p ProxySpec) Scheme() ProxyScheme { return p.scheme }
func (p ProxySpec) Redacted() string    { return p.redacted }
func (p ProxySpec) RemoteDNS() bool     { return p.remoteDNS }

func ParseProxy(raw string) (ProxySpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ProxySpec{}, fmt.Errorf("proxy URL is empty")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ProxySpec{}, fmt.Errorf("invalid proxy URL %q: %w", RedactProxyURL(raw), err)
	}

	scheme := ProxyScheme(strings.ToLower(u.Scheme))
	switch scheme {
	case ProxySchemeHTTP, ProxySchemeHTTPS, ProxySchemeSOCKS5, ProxySchemeSOCKS5H:
	default:
		return ProxySpec{}, fmt.Errorf("unsupported proxy scheme %q in %s", u.Scheme, RedactProxyURL(raw))
	}

	if u.Hostname() == "" {
		return ProxySpec{}, fmt.Errorf("proxy host is required in %s", RedactProxyURL(raw))
	}
	if u.Path != "" && u.Path != "/" {
		return ProxySpec{}, fmt.Errorf("proxy URL must not contain a path: %s", RedactProxyURL(raw))
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return ProxySpec{}, fmt.Errorf("proxy URL must not contain query or fragment data: %s", RedactProxyURL(raw))
	}

	normalized := *u
	if normalized.Port() == "" {
		port := defaultProxyPort(scheme)
		normalized.Host = net.JoinHostPort(normalized.Hostname(), port)
	}
	normalized.Path = ""

	return ProxySpec{
		scheme:    scheme,
		proxyURL:  &normalized,
		redacted:  redactURL(&normalized),
		remoteDNS: scheme == ProxySchemeSOCKS5H,
	}, nil
}

func defaultProxyPort(scheme ProxyScheme) string {
	switch scheme {
	case ProxySchemeHTTP:
		return "80"
	case ProxySchemeHTTPS:
		return "443"
	default:
		return "1080"
	}
}

// RedactProxyURL removes userinfo credentials without returning parse errors or
// preserving secret text in diagnostics.
func RedactProxyURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "<invalid-proxy-url>"
	}
	return redactURL(u)
}

func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	copyURL := *u
	if copyURL.User != nil {
		copyURL.User = url.UserPassword("***", "***")
	}
	return copyURL.String()
}
