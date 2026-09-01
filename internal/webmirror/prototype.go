package webmirror

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// Prototype is deliberately read-only. Its purpose is to measure how far a
// transparent reverse proxy can get before browser-origin/auth/challenge
// assumptions require a different architecture. It is not wired into the main
// application server.
type Prototype struct {
	proxy *httputil.ReverseProxy
}

func NewPrototype(upstreamURL, mirrorURL string, transport http.RoundTripper) (*Prototype, error) {
	upstream, err := url.Parse(strings.TrimSpace(upstreamURL))
	if err != nil || upstream.Scheme == "" || upstream.Host == "" {
		return nil, fmt.Errorf("invalid upstream URL %q", upstreamURL)
	}
	mirror, err := url.Parse(strings.TrimSpace(mirrorURL))
	if err != nil || mirror.Scheme == "" || mirror.Host == "" {
		return nil, fmt.Errorf("invalid mirror URL %q", mirrorURL)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstream)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Match what a transparent reverse proxy would present upstream. We do
		// not forge browser security headers, Origin, Referer, Sentinel tokens,
		// or challenge state.
		req.Host = upstream.Host
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header = RewriteResponseHeaders(resp.Header, upstream, mirror)
		return nil
	}
	if transport != nil {
		proxy.Transport = transport
	}

	return &Prototype{proxy: proxy}, nil
}

func (p *Prototype) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p == nil || p.proxy == nil {
		http.Error(w, "web mirror prototype is not initialized", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "web mirror prototype is read-only", http.StatusNotImplemented)
		return
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		http.Error(w, "websocket mirroring is intentionally outside the read-only prototype", http.StatusNotImplemented)
		return
	}
	p.proxy.ServeHTTP(w, r)
}
