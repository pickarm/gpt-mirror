package webmirror_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"PandoraHelper/internal/webmirror"
)

func TestAnalyzeDetectsFullSiteMirrorBlockers(t *testing.T) {
	header := make(http.Header)
	header.Set("Location", "https://chatgpt.com/auth/login?next=%2F")
	header.Add("Set-Cookie", "session=abc; Domain=.chatgpt.com; Path=/; Secure; HttpOnly; SameSite=Lax")
	header.Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' https://chatgpt.com wss://ws.chatgpt.com; script-src 'self' https://cdn.oaistatic.com")
	header.Set("Access-Control-Allow-Origin", "https://chatgpt.com")
	body := `
		<script src="https://cdn.oaistatic.com/app.js"></script>
		<script>
			if (location.hostname === "chatgpt.com") {
				navigator.serviceWorker.register('/sw.js');
				const socket = new WebSocket('wss://ws.chatgpt.com/');
			}
		</script>
		<a href="https://auth.openai.com/login">login</a>
		<iframe src="https://challenges.cloudflare.com/widget"></iframe>
	`

	report, err := webmirror.Analyze(webmirror.Snapshot{
		URL:        "https://chatgpt.com/",
		StatusCode: http.StatusFound,
		Header:     header,
		Body:       body,
	}, "https://mirror.example.com/")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	assertVerdict(t, report, webmirror.AreaRedirects, webmirror.VerdictRewriteRequired)
	assertVerdict(t, report, webmirror.AreaCookies, webmirror.VerdictRewriteRequired)
	assertVerdict(t, report, webmirror.AreaCSP, webmirror.VerdictRewriteRequired)
	assertVerdict(t, report, webmirror.AreaCORS, webmirror.VerdictRewriteRequired)
	assertVerdict(t, report, webmirror.AreaStaticAssets, webmirror.VerdictExternalDependency)
	assertVerdict(t, report, webmirror.AreaRuntimeHost, webmirror.VerdictRewriteRequired)
	assertVerdict(t, report, webmirror.AreaServiceWorker, webmirror.VerdictRewriteRequired)
	assertVerdict(t, report, webmirror.AreaWebSocket, webmirror.VerdictExternalDependency)
	assertVerdict(t, report, webmirror.AreaAuthChallenge, webmirror.VerdictBrowserRequired)
	if !strings.Contains(report.Recommendation, "transparent_full_site_mirror_no_go") {
		t.Fatalf("recommendation = %q", report.Recommendation)
	}
}

func TestRewriteResponseHeadersOnlyChangesSafeMechanicalFields(t *testing.T) {
	upstream, _ := url.Parse("https://chatgpt.com")
	mirror, _ := url.Parse("https://mirror.example.com")
	header := make(http.Header)
	header.Set("Location", "https://chatgpt.com/c/123?foo=bar")
	header.Add("Set-Cookie", "session=abc; Domain=.chatgpt.com; Path=/; Secure; HttpOnly; SameSite=Lax")
	header.Add("Set-Cookie", "__Host-device=xyz; Path=/; Secure; SameSite=None")
	header.Set("Content-Security-Policy", "default-src 'self'; connect-src https://chatgpt.com")
	header.Set("Access-Control-Allow-Origin", "https://chatgpt.com")

	rewritten := webmirror.RewriteResponseHeaders(header, upstream, mirror)
	if got := rewritten.Get("Location"); got != "https://mirror.example.com/c/123?foo=bar" {
		t.Fatalf("Location = %q", got)
	}
	cookies := rewritten.Values("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("cookies = %#v", cookies)
	}
	if strings.Contains(strings.ToLower(cookies[0]), "domain=") {
		t.Fatalf("upstream Domain was not removed: %q", cookies[0])
	}
	for _, required := range []string{"Path=/", "Secure", "HttpOnly", "SameSite=Lax"} {
		if !strings.Contains(cookies[0], required) {
			t.Fatalf("cookie attribute %q lost: %q", required, cookies[0])
		}
	}
	if cookies[1] != "__Host-device=xyz; Path=/; Secure; SameSite=None" {
		t.Fatalf("host-only cookie changed: %q", cookies[1])
	}
	if rewritten.Get("Content-Security-Policy") != header.Get("Content-Security-Policy") {
		t.Fatal("prototype must not weaken or rewrite CSP")
	}
	if rewritten.Get("Access-Control-Allow-Origin") != header.Get("Access-Control-Allow-Origin") {
		t.Fatal("prototype must not rewrite CORS")
	}
}

func TestProbeDoesNotFollowRedirects(t *testing.T) {
	var destinationHits int
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationHits++
		_, _ = io.WriteString(w, "unexpected redirect follow")
	}))
	defer destination.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL+"/login", http.StatusFound)
	}))
	defer upstream.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	report, err := webmirror.Probe(ctx, nil, upstream.URL, "https://mirror.example.com")
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if report.StatusCode != http.StatusFound {
		t.Fatalf("status = %d", report.StatusCode)
	}
	if destinationHits != 0 {
		t.Fatalf("probe followed redirect %d times", destinationHits)
	}
	assertVerdict(t, report, webmirror.AreaRedirects, webmirror.VerdictExternalDependency)
}

func TestAnalyzeAnonymousShellIsNotPromotedToAuthenticatedMirror(t *testing.T) {
	report, err := webmirror.Analyze(webmirror.Snapshot{
		URL:        "https://chatgpt.com/",
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       "<html><body>anonymous shell</body></html>",
	}, "https://mirror.example.com/")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if report.Recommendation != "anonymous_shell_candidate_only; authenticated browser behavior still requires separate validation" {
		t.Fatalf("recommendation = %q", report.Recommendation)
	}
}

func assertVerdict(t *testing.T, report webmirror.Report, area webmirror.Area, want webmirror.Verdict) {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.Area == area {
			if finding.Verdict != want {
				t.Fatalf("%s verdict = %s, want %s; finding=%#v", area, finding.Verdict, want, finding)
			}
			return
		}
	}
	t.Fatalf("finding %s not present", area)
}
