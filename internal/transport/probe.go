package transport

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type ProbeKind string

const (
	ProbeKindHealthy   ProbeKind = "healthy"
	ProbeKindRoute     ProbeKind = "route"
	ProbeKindProxyAuth ProbeKind = "proxy_auth"
	ProbeKindAuth      ProbeKind = "auth"
	ProbeKindUpstream  ProbeKind = "upstream"
)

type ProbeResult struct {
	Kind       ProbeKind
	Route      RouteInfo
	StatusCode int
	Latency    time.Duration
}

// Probe verifies an outbound route without interpreting application response
// bodies. Network/TLS failures are classified separately from proxy auth and
// upstream session/auth responses.
func (f *Factory) Probe(ctx context.Context, accountProxyURL, targetURL string) (ProbeResult, error) {
	client, route, err := f.Client(accountProxyURL)
	if err != nil {
		return ProbeResult{Kind: ProbeKindRoute}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return ProbeResult{Kind: ProbeKindUpstream, Route: route}, fmt.Errorf("build transport probe request: %w", err)
	}

	started := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(started)
	if err != nil {
		return ProbeResult{Kind: ProbeKindRoute, Route: route, Latency: latency}, fmt.Errorf("transport route via %s failed: %w", routeLabel(route), err)
	}
	defer resp.Body.Close()

	result := ProbeResult{
		Kind:       ProbeKindHealthy,
		Route:      route,
		StatusCode: resp.StatusCode,
		Latency:    latency,
	}

	switch resp.StatusCode {
	case http.StatusProxyAuthRequired:
		result.Kind = ProbeKindProxyAuth
		return result, fmt.Errorf("proxy authentication failed via %s", routeLabel(route))
	case http.StatusUnauthorized, http.StatusForbidden:
		result.Kind = ProbeKindAuth
		return result, fmt.Errorf("upstream authentication/session rejected request with HTTP %d", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		result.Kind = ProbeKindUpstream
		return result, fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}

	return result, nil
}

func routeLabel(route RouteInfo) string {
	if route.Source == RouteSourceDirect {
		return "direct"
	}
	return string(route.Source) + ":" + route.ProxyURL
}
