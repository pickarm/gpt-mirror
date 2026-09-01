package webmirror

import (
	"net/http"
	"net/url"
	"strings"
)

// RewriteResponseHeaders is the intentionally small rewrite surface for the
// transparent-mirror prototype. It does not weaken CSP/CORS, mutate HTML/JS,
// or forge authentication/challenge state.
func RewriteResponseHeaders(input http.Header, upstream, mirror *url.URL) http.Header {
	output := input.Clone()
	if upstream == nil || mirror == nil {
		return output
	}

	if location := output.Get("Location"); location != "" {
		output.Set("Location", RewriteLocation(location, upstream, mirror))
	}

	cookies := output.Values("Set-Cookie")
	if len(cookies) > 0 {
		output.Del("Set-Cookie")
		for _, cookie := range cookies {
			output.Add("Set-Cookie", rewriteCookieDomain(cookie, upstream.Hostname()))
		}
	}
	return output
}

func RewriteLocation(location string, upstream, mirror *url.URL) string {
	if upstream == nil || mirror == nil {
		return location
	}
	parsed, err := url.Parse(location)
	if err != nil || !parsed.IsAbs() || !sameOrigin(parsed, upstream) {
		return location
	}
	parsed.Scheme = mirror.Scheme
	parsed.Host = mirror.Host
	return parsed.String()
}

// rewriteCookieDomain makes an upstream Domain cookie host-only when it was
// explicitly scoped to the upstream host. Host-only cookies already work when
// emitted by the mirror and are left unchanged. Secure, HttpOnly, SameSite,
// Path, Max-Age and expiry attributes are deliberately preserved verbatim.
func rewriteCookieDomain(raw, upstreamHost string) string {
	parts := strings.Split(raw, ";")
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "domain=") {
			domain := strings.TrimSpace(strings.TrimPrefix(lower, "domain="))
			domain = strings.TrimPrefix(domain, ".")
			if strings.EqualFold(domain, upstreamHost) {
				continue
			}
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, "; ")
}
