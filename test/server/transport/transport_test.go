package transport_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apptransport "PandoraHelper/internal/transport"
)

func TestParseProxyRedactsCredentials(t *testing.T) {
	spec, err := apptransport.ParseProxy("socks5h://alice:super-secret@proxy.example:1080")
	if err != nil {
		t.Fatalf("ParseProxy: %v", err)
	}
	if spec.Scheme() != apptransport.ProxySchemeSOCKS5H {
		t.Fatalf("scheme = %s", spec.Scheme())
	}
	if !spec.RemoteDNS() {
		t.Fatal("socks5h must use remote DNS")
	}
	if strings.Contains(spec.Redacted(), "alice") || strings.Contains(spec.Redacted(), "super-secret") {
		t.Fatalf("credentials leaked in redacted URL: %s", spec.Redacted())
	}
	if !strings.Contains(spec.Redacted(), "proxy.example:1080") {
		t.Fatalf("redacted route lost host: %s", spec.Redacted())
	}
}

func TestParseProxyRejectsUnsupportedSchemeWithoutCredentialLeak(t *testing.T) {
	_, err := apptransport.ParseProxy("ftp://alice:secret@proxy.example:21")
	if err == nil {
		t.Fatal("expected unsupported scheme error")
	}
	if strings.Contains(err.Error(), "alice") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("credentials leaked in error: %v", err)
	}
}

func TestResolveRoutePriority(t *testing.T) {
	route, err := apptransport.ResolveRoute(
		"socks5h://account-user:account-pass@account.proxy:1080",
		"http://global-user:global-pass@global.proxy:8080",
	)
	if err != nil {
		t.Fatalf("ResolveRoute: %v", err)
	}
	if route.Source != apptransport.RouteSourceAccount || route.Scheme != apptransport.ProxySchemeSOCKS5H {
		t.Fatalf("unexpected route: %#v", route)
	}
	if strings.Contains(route.ProxyURL, "account-pass") {
		t.Fatalf("account proxy credentials leaked: %s", route.ProxyURL)
	}

	route, err = apptransport.ResolveRoute("", "http://global.proxy:8080")
	if err != nil {
		t.Fatalf("ResolveRoute global: %v", err)
	}
	if route.Source != apptransport.RouteSourceGlobal || route.Scheme != apptransport.ProxySchemeHTTP {
		t.Fatalf("unexpected global route: %#v", route)
	}

	route, err = apptransport.ResolveRoute("", "")
	if err != nil {
		t.Fatalf("ResolveRoute direct: %v", err)
	}
	if route.Source != apptransport.RouteSourceDirect {
		t.Fatalf("unexpected direct route: %#v", route)
	}
}

func TestFactoryBuildsHTTPProxyClientWithoutOverallTimeout(t *testing.T) {
	factory, err := apptransport.NewFactory(apptransport.Settings{
		GlobalProxyURL: "http://user:pass@proxy.example:8080",
	})
	if err != nil {
		t.Fatalf("NewFactory: %v", err)
	}

	client, route, err := factory.Client("")
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	if client.Timeout != 0 {
		t.Fatalf("streaming client must not have an overall timeout: %s", client.Timeout)
	}
	if route.Source != apptransport.RouteSourceGlobal {
		t.Fatalf("route source = %s", route.Source)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", client.Transport)
	}
	if transport.ResponseHeaderTimeout <= 0 || transport.TLSHandshakeTimeout <= 0 {
		t.Fatal("phase timeouts must be configured")
	}
	proxyURL, err := transport.Proxy(&http.Request{URL: mustURL(t, "https://chat.example/path")})
	if err != nil {
		t.Fatalf("proxy callback: %v", err)
	}
	if proxyURL == nil || proxyURL.Scheme != "http" || proxyURL.Host != "proxy.example:8080" {
		t.Fatalf("unexpected proxy URL: %#v", proxyURL)
	}
}

func TestFactoryBuildsSOCKSRoutes(t *testing.T) {
	factory, err := apptransport.NewFactory(apptransport.DefaultSettings())
	if err != nil {
		t.Fatalf("NewFactory: %v", err)
	}

	client, route, err := factory.Client("socks5://127.0.0.1:1080")
	if err != nil {
		t.Fatalf("SOCKS5 client: %v", err)
	}
	if route.Scheme != apptransport.ProxySchemeSOCKS5 || route.RemoteDNS {
		t.Fatalf("SOCKS5 must use local DNS: %#v", route)
	}
	transport := client.Transport.(*http.Transport)
	if transport.Proxy != nil || transport.DialContext == nil {
		t.Fatal("SOCKS routing must use a custom DialContext without HTTP proxy callback")
	}

	_, route, err = factory.Client("socks5h://127.0.0.1:1080")
	if err != nil {
		t.Fatalf("SOCKS5H client: %v", err)
	}
	if route.Scheme != apptransport.ProxySchemeSOCKS5H || !route.RemoteDNS {
		t.Fatalf("SOCKS5H must use remote DNS: %#v", route)
	}
}

func TestAccountRouteDoesNotMutateGlobalRoute(t *testing.T) {
	factory, err := apptransport.NewFactory(apptransport.Settings{GlobalProxyURL: "http://global.proxy:8080"})
	if err != nil {
		t.Fatalf("NewFactory: %v", err)
	}

	_, accountRoute, err := factory.Client("socks5h://account.proxy:1080")
	if err != nil {
		t.Fatalf("account Client: %v", err)
	}
	_, globalRoute, err := factory.Client("")
	if err != nil {
		t.Fatalf("global Client: %v", err)
	}
	if accountRoute.Source != apptransport.RouteSourceAccount || globalRoute.Source != apptransport.RouteSourceGlobal {
		t.Fatalf("route state leaked between clients: account=%#v global=%#v", accountRoute, globalRoute)
	}
}

func TestProbeSeparatesAuthAndUpstreamFailures(t *testing.T) {
	factory, err := apptransport.NewFactory(apptransport.DefaultSettings())
	if err != nil {
		t.Fatalf("NewFactory: %v", err)
	}

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer authServer.Close()

	result, err := factory.Probe(context.Background(), "", authServer.URL)
	if err == nil || result.Kind != apptransport.ProbeKindAuth || result.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected auth probe result=%#v err=%v", result, err)
	}

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstreamServer.Close()

	result, err = factory.Probe(context.Background(), "", upstreamServer.URL)
	if err == nil || result.Kind != apptransport.ProbeKindUpstream || result.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected upstream probe result=%#v err=%v", result, err)
	}
}

func TestProbeClassifiesCancelledRouteAsConnectivityFailure(t *testing.T) {
	factory, err := apptransport.NewFactory(apptransport.DefaultSettings())
	if err != nil {
		t.Fatalf("NewFactory: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := factory.Probe(ctx, "", "http://127.0.0.1:1")
	if err == nil || result.Kind != apptransport.ProbeKindRoute {
		t.Fatalf("unexpected route probe result=%#v err=%v", result, err)
	}
}

func TestDefaultPorts(t *testing.T) {
	for raw, scheme := range map[string]apptransport.ProxyScheme{
		"http://proxy.example":    apptransport.ProxySchemeHTTP,
		"https://proxy.example":   apptransport.ProxySchemeHTTPS,
		"socks5://proxy.example":  apptransport.ProxySchemeSOCKS5,
		"socks5h://proxy.example": apptransport.ProxySchemeSOCKS5H,
	} {
		spec, err := apptransport.ParseProxy(raw)
		if err != nil {
			t.Fatalf("ParseProxy(%q): %v", raw, err)
		}
		if spec.Scheme() != scheme {
			t.Fatalf("ParseProxy(%q) scheme=%s", raw, spec.Scheme())
		}
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse URL %q: %v", raw, err)
	}
	return u
}

var _ = time.Second
