package webmirror

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxProbeBody = 2 << 20

type Verdict string

const (
	VerdictTransparent        Verdict = "transparent"
	VerdictRewriteRequired    Verdict = "rewrite_required"
	VerdictBrowserRequired    Verdict = "browser_session_required"
	VerdictExternalDependency Verdict = "external_dependency"
	VerdictInconclusive       Verdict = "inconclusive"
)

type Area string

const (
	AreaRedirects     Area = "redirects"
	AreaCookies       Area = "cookies"
	AreaCSP           Area = "csp"
	AreaCORS          Area = "cors_origin"
	AreaStaticAssets  Area = "static_assets"
	AreaRuntimeHost   Area = "runtime_host_checks"
	AreaServiceWorker Area = "service_worker"
	AreaWebSocket     Area = "websocket"
	AreaAuthChallenge Area = "auth_challenge"
)

type Finding struct {
	Area     Area     `json:"area"`
	Verdict  Verdict  `json:"verdict"`
	Summary  string   `json:"summary"`
	Evidence []string `json:"evidence,omitempty"`
}

type Report struct {
	CheckedAt      time.Time `json:"checkedAt"`
	Upstream       string    `json:"upstream"`
	Mirror         string    `json:"mirror"`
	StatusCode     int       `json:"statusCode"`
	Findings       []Finding `json:"findings"`
	Recommendation string    `json:"recommendation"`
}

type Snapshot struct {
	URL        string
	StatusCode int
	Header     http.Header
	Body       string
}

func Probe(ctx context.Context, client *http.Client, upstreamURL, mirrorURL string) (Report, error) {
	upstream, err := url.Parse(strings.TrimSpace(upstreamURL))
	if err != nil || upstream.Scheme == "" || upstream.Host == "" {
		return Report{}, fmt.Errorf("invalid upstream URL %q", upstreamURL)
	}
	mirror, err := url.Parse(strings.TrimSpace(mirrorURL))
	if err != nil || mirror.Scheme == "" || mirror.Host == "" {
		return Report{}, fmt.Errorf("invalid mirror URL %q", mirrorURL)
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	probeClient := *client
	probeClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream.String(), nil)
	if err != nil {
		return Report{}, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; gpt-mirror-webmirror-probe/1.0)")

	resp, err := probeClient.Do(req)
	if err != nil {
		return Report{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBody))
	if err != nil {
		return Report{}, err
	}

	return Analyze(Snapshot{
		URL:        upstream.String(),
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       string(body),
	}, mirror.String())
}

func Analyze(snapshot Snapshot, mirrorURL string) (Report, error) {
	upstream, err := url.Parse(snapshot.URL)
	if err != nil || upstream.Scheme == "" || upstream.Host == "" {
		return Report{}, fmt.Errorf("invalid snapshot URL %q", snapshot.URL)
	}
	mirror, err := url.Parse(mirrorURL)
	if err != nil || mirror.Scheme == "" || mirror.Host == "" {
		return Report{}, fmt.Errorf("invalid mirror URL %q", mirrorURL)
	}

	findings := []Finding{
		analyzeRedirect(snapshot.Header, upstream),
		analyzeCookies(snapshot.Header, upstream, mirror),
		analyzeCSP(snapshot.Header, upstream),
		analyzeCORS(snapshot.Header, upstream),
		analyzeStaticAssets(snapshot.Body, upstream, mirror),
		analyzeRuntimeHostChecks(snapshot.Body, upstream),
		analyzeServiceWorker(snapshot.Body),
		analyzeWebSocket(snapshot.Body, upstream),
		analyzeAuthChallenge(snapshot.Body),
	}

	report := Report{
		CheckedAt:  time.Now().UTC(),
		Upstream:   upstream.String(),
		Mirror:     mirror.String(),
		StatusCode: snapshot.StatusCode,
		Findings:   findings,
	}
	report.Recommendation = recommendation(findings)
	return report, nil
}

func analyzeRedirect(header http.Header, upstream *url.URL) Finding {
	location := strings.TrimSpace(header.Get("Location"))
	if location == "" {
		return Finding{Area: AreaRedirects, Verdict: VerdictTransparent, Summary: "no redirect observed in this response"}
	}
	parsed, err := url.Parse(location)
	if err != nil {
		return Finding{Area: AreaRedirects, Verdict: VerdictRewriteRequired, Summary: "redirect is not safely parseable", Evidence: []string{location}}
	}
	if parsed.IsAbs() && sameOrigin(parsed, upstream) {
		return Finding{Area: AreaRedirects, Verdict: VerdictRewriteRequired, Summary: "absolute upstream redirect must be mapped back to the mirror origin", Evidence: []string{location}}
	}
	if parsed.IsAbs() {
		return Finding{Area: AreaRedirects, Verdict: VerdictExternalDependency, Summary: "redirect leaves the upstream origin", Evidence: []string{location}}
	}
	return Finding{Area: AreaRedirects, Verdict: VerdictTransparent, Summary: "relative redirect is origin-portable", Evidence: []string{location}}
}

func analyzeCookies(header http.Header, upstream, mirror *url.URL) Finding {
	cookies := header.Values("Set-Cookie")
	if len(cookies) == 0 {
		return Finding{Area: AreaCookies, Verdict: VerdictInconclusive, Summary: "no Set-Cookie header observed in the anonymous response"}
	}

	var evidence []string
	needsRewrite := false
	browserBound := false
	for _, raw := range cookies {
		lower := strings.ToLower(raw)
		for _, part := range strings.Split(raw, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "domain=") {
				domain := strings.TrimPrefix(strings.ToLower(part), "domain=")
				domain = strings.TrimPrefix(domain, ".")
				if strings.EqualFold(domain, upstream.Hostname()) {
					needsRewrite = true
					evidence = append(evidence, "Domain="+domain)
				}
			}
		}
		if strings.Contains(lower, "samesite=none") && !strings.Contains(lower, "; secure") {
			browserBound = true
			evidence = append(evidence, "SameSite=None without Secure")
		}
	}
	if browserBound {
		return Finding{Area: AreaCookies, Verdict: VerdictBrowserRequired, Summary: "observed cookie attributes depend on browser security rules", Evidence: uniqueSorted(evidence)}
	}
	if needsRewrite {
		return Finding{Area: AreaCookies, Verdict: VerdictRewriteRequired, Summary: "upstream Domain cookies must be rewritten or made host-only on the mirror", Evidence: uniqueSorted(evidence)}
	}
	if mirror.Scheme != "https" {
		return Finding{Area: AreaCookies, Verdict: VerdictBrowserRequired, Summary: "secure session cookies require an HTTPS mirror origin"}
	}
	return Finding{Area: AreaCookies, Verdict: VerdictTransparent, Summary: "host-only cookies are portable through an HTTPS reverse proxy"}
}

func analyzeCSP(header http.Header, upstream *url.URL) Finding {
	csp := strings.TrimSpace(header.Get("Content-Security-Policy"))
	if csp == "" {
		csp = strings.TrimSpace(header.Get("Content-Security-Policy-Report-Only"))
	}
	if csp == "" {
		return Finding{Area: AreaCSP, Verdict: VerdictInconclusive, Summary: "no CSP header observed in this response"}
	}
	if strings.Contains(strings.ToLower(csp), strings.ToLower(upstream.Hostname())) {
		return Finding{Area: AreaCSP, Verdict: VerdictRewriteRequired, Summary: "CSP contains the upstream host; absolute connect/script destinations can bypass or reject the mirror", Evidence: []string{truncate(csp, 320)}}
	}
	return Finding{Area: AreaCSP, Verdict: VerdictTransparent, Summary: "observed CSP is not explicitly pinned to the upstream hostname", Evidence: []string{truncate(csp, 320)}}
}

func analyzeCORS(header http.Header, upstream *url.URL) Finding {
	allowOrigin := strings.TrimSpace(header.Get("Access-Control-Allow-Origin"))
	if allowOrigin == "" {
		return Finding{Area: AreaCORS, Verdict: VerdictInconclusive, Summary: "no Access-Control-Allow-Origin header observed"}
	}
	if allowOrigin == "*" {
		return Finding{Area: AreaCORS, Verdict: VerdictTransparent, Summary: "wildcard CORS is origin-portable", Evidence: []string{allowOrigin}}
	}
	parsed, err := url.Parse(allowOrigin)
	if err == nil && sameOrigin(parsed, upstream) {
		return Finding{Area: AreaCORS, Verdict: VerdictRewriteRequired, Summary: "CORS explicitly trusts the upstream origin", Evidence: []string{allowOrigin}}
	}
	return Finding{Area: AreaCORS, Verdict: VerdictExternalDependency, Summary: "CORS trusts a different fixed origin", Evidence: []string{allowOrigin}}
}

var absoluteURLPattern = regexp.MustCompile(`(?i)(?:https?|wss?)://[a-z0-9._:-]+`)

func analyzeStaticAssets(body string, upstream, mirror *url.URL) Finding {
	hosts := absoluteHosts(body)
	delete(hosts, strings.ToLower(upstream.Hostname()))
	delete(hosts, strings.ToLower(mirror.Hostname()))
	if len(hosts) == 0 {
		return Finding{Area: AreaStaticAssets, Verdict: VerdictTransparent, Summary: "no cross-origin absolute asset/API hosts were observed in the sampled HTML"}
	}
	return Finding{Area: AreaStaticAssets, Verdict: VerdictExternalDependency, Summary: "the page depends on additional origins that a single-host mirror cannot encapsulate", Evidence: mapKeys(hosts)}
}

func analyzeRuntimeHostChecks(body string, upstream *url.URL) Finding {
	lower := strings.ToLower(body)
	markers := []string{"location.hostname", "location.host", "window.location", "document.location"}
	var evidence []string
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			evidence = append(evidence, marker)
		}
	}
	if len(evidence) > 0 && strings.Contains(lower, strings.ToLower(upstream.Hostname())) {
		return Finding{Area: AreaRuntimeHost, Verdict: VerdictRewriteRequired, Summary: "sampled client code appears to combine runtime host inspection with the upstream hostname", Evidence: uniqueSorted(evidence)}
	}
	if len(evidence) > 0 {
		return Finding{Area: AreaRuntimeHost, Verdict: VerdictInconclusive, Summary: "runtime location checks are present and require browser validation", Evidence: uniqueSorted(evidence)}
	}
	return Finding{Area: AreaRuntimeHost, Verdict: VerdictTransparent, Summary: "no obvious runtime hostname check was found in the sampled HTML"}
}

func analyzeServiceWorker(body string) Finding {
	lower := strings.ToLower(body)
	markers := []string{"serviceworker", "service-worker", "navigator.serviceworker", "sw.js"}
	var evidence []string
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			evidence = append(evidence, marker)
		}
	}
	if len(evidence) == 0 {
		return Finding{Area: AreaServiceWorker, Verdict: VerdictInconclusive, Summary: "service-worker registration was not visible in the sampled HTML"}
	}
	return Finding{Area: AreaServiceWorker, Verdict: VerdictRewriteRequired, Summary: "service workers are origin/scope bound and must be validated or rewritten for a mirror origin", Evidence: uniqueSorted(evidence)}
}

func analyzeWebSocket(body string, upstream *url.URL) Finding {
	lower := strings.ToLower(body)
	var evidence []string
	for _, marker := range []string{"wss://", "websocket", "register-websocket", "ws.chatgpt.com"} {
		if strings.Contains(lower, marker) {
			evidence = append(evidence, marker)
		}
	}
	if len(evidence) == 0 {
		return Finding{Area: AreaWebSocket, Verdict: VerdictInconclusive, Summary: "WebSocket use was not visible in the sampled HTML"}
	}
	if strings.Contains(lower, "ws.chatgpt.com") || (strings.Contains(lower, "wss://") && strings.Contains(lower, strings.ToLower(upstream.Hostname()))) {
		return Finding{Area: AreaWebSocket, Verdict: VerdictExternalDependency, Summary: "WebSocket traffic targets an upstream/sibling origin and needs explicit proxy support", Evidence: uniqueSorted(evidence)}
	}
	return Finding{Area: AreaWebSocket, Verdict: VerdictRewriteRequired, Summary: "WebSocket URLs/upgrade behavior require dedicated mirror validation", Evidence: uniqueSorted(evidence)}
}

func analyzeAuthChallenge(body string) Finding {
	lower := strings.ToLower(body)
	var evidence []string
	for _, marker := range []string{"auth.openai.com", "challenges.cloudflare.com", "sentinel/chat-requirements", "turnstile", "arkose"} {
		if strings.Contains(lower, marker) {
			evidence = append(evidence, marker)
		}
	}
	if len(evidence) == 0 {
		return Finding{Area: AreaAuthChallenge, Verdict: VerdictInconclusive, Summary: "auth/challenge dependencies were not visible in the sampled HTML"}
	}
	return Finding{Area: AreaAuthChallenge, Verdict: VerdictBrowserRequired, Summary: "authentication or anti-abuse flow is browser/session bound and is not safely reproduced by header rewriting", Evidence: uniqueSorted(evidence)}
}

func recommendation(findings []Finding) string {
	hasBrowser := false
	hasRewrite := false
	hasExternal := false
	for _, finding := range findings {
		switch finding.Verdict {
		case VerdictBrowserRequired:
			hasBrowser = true
		case VerdictRewriteRequired:
			hasRewrite = true
		case VerdictExternalDependency:
			hasExternal = true
		}
	}
	if hasBrowser {
		return "transparent_full_site_mirror_no_go; use a provider-backed native UI plus an isolated real-browser session bridge where required"
	}
	if hasRewrite {
		return "transparent_mirror_not_portable; a rewrite layer and ongoing compatibility tests are required"
	}
	if hasExternal {
		return "single_host_mirror_incomplete; additional upstream origins must remain reachable or be proxied explicitly"
	}
	return "anonymous_shell_candidate_only; authenticated browser behavior still requires separate validation"
}

func absoluteHosts(body string) map[string]struct{} {
	hosts := make(map[string]struct{})
	for _, raw := range absoluteURLPattern.FindAllString(body, -1) {
		parsed, err := url.Parse(raw)
		if err == nil && parsed.Hostname() != "" {
			hosts[strings.ToLower(parsed.Hostname())] = struct{}{}
		}
	}
	return hosts
}

func sameOrigin(left, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		effectivePort(left) == effectivePort(right)
}

func effectivePort(value *url.URL) string {
	if value == nil {
		return ""
	}
	if port := value.Port(); port != "" {
		return port
	}
	switch strings.ToLower(value.Scheme) {
	case "http", "ws":
		return "80"
	case "https", "wss":
		return "443"
	default:
		return ""
	}
}

func mapKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func uniqueSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return mapKeys(set)
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
