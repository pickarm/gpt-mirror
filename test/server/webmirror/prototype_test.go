package webmirror_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"PandoraHelper/internal/webmirror"
)

func TestPrototypeProxiesAnonymousGETWithMechanicalHeaderRewrite(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if r.Host != strings.TrimPrefix(upstreamURLForHost(r), "http://") {
			// The reverse proxy must present the upstream Host, not the mirror host.
			t.Fatalf("unexpected Host: %s", r.Host)
		}
		w.Header().Set("Location", "https://upstream.invalid/auth/login")
		w.Header().Add("Set-Cookie", "session=abc; Domain=.upstream.invalid; Path=/; Secure; HttpOnly")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src https://upstream.invalid")
		w.WriteHeader(http.StatusFound)
		_, _ = io.WriteString(w, "redirect")
	}))
	defer upstream.Close()

	// Use a custom transport that maps the logical upstream.invalid host to the
	// httptest server while preserving the reverse-proxy URL/Host behavior.
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(upstream.URL, "http://")
		return http.DefaultTransport.RoundTrip(clone)
	})
	prototype, err := webmirror.NewPrototype("https://upstream.invalid", "https://mirror.example.com", transport)
	if err != nil {
		t.Fatalf("NewPrototype: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "https://mirror.example.com/", nil)
	res := httptest.NewRecorder()
	prototype.ServeHTTP(res, req)
	if res.Code != http.StatusFound {
		t.Fatalf("status = %d", res.Code)
	}
	if got := res.Header().Get("Location"); got != "https://mirror.example.com/auth/login" {
		t.Fatalf("Location = %q", got)
	}
	cookie := res.Header().Get("Set-Cookie")
	if strings.Contains(strings.ToLower(cookie), "domain=") {
		t.Fatalf("cookie Domain not made host-only: %q", cookie)
	}
	if got := res.Header().Get("Content-Security-Policy"); got != "default-src 'self'; connect-src https://upstream.invalid" {
		t.Fatalf("CSP was unexpectedly modified: %q", got)
	}
}

func TestPrototypeRejectsWritesAndWebSocketUpgrade(t *testing.T) {
	prototype, err := webmirror.NewPrototype("https://chatgpt.com", "https://mirror.example.com", roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("transport should not be reached")
		return nil, nil
	}))
	if err != nil {
		t.Fatalf("NewPrototype: %v", err)
	}

	postReq := httptest.NewRequest(http.MethodPost, "https://mirror.example.com/backend-api/f/conversation", strings.NewReader("{}"))
	postRes := httptest.NewRecorder()
	prototype.ServeHTTP(postRes, postReq)
	if postRes.Code != http.StatusNotImplemented {
		t.Fatalf("POST status = %d", postRes.Code)
	}

	wsReq := httptest.NewRequest(http.MethodGet, "https://mirror.example.com/ws", nil)
	wsReq.Header.Set("Upgrade", "websocket")
	wsRes := httptest.NewRecorder()
	prototype.ServeHTTP(wsRes, wsReq)
	if wsRes.Code != http.StatusNotImplemented {
		t.Fatalf("WebSocket status = %d", wsRes.Code)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// upstreamURLForHost returns the URL prefix used by the test transport. It is
// intentionally derived from the request URL so the assertion stays local to
// the request that reached the test server.
func upstreamURLForHost(r *http.Request) string {
	return "http://" + r.Host
}
