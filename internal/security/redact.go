package security

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const Redacted = "***"

var (
	bearerPattern     = regexp.MustCompile(`(?i)\bBearer[ \t]+[A-Za-z0-9._~+/=-]+`)
	basicPattern      = regexp.MustCompile(`(?i)\bBasic[ \t]+[A-Za-z0-9+/=]+`)
	jwtPattern        = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{8,}\b`)
	openAIKeyPattern  = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`)
	urlUserInfoPattern = regexp.MustCompile(`(?i)\b(https?|socks5h?)://[^/@\s]+@`)
	assignmentPattern = regexp.MustCompile(`(?i)\b(password|passwd|session[_-]?token|access[_-]?token|refresh[_-]?token|session[_-]?key|api[_-]?key|authorization|proxy[_-]?authorization|cookie|client[_-]?secret)=([^&\s;,]+)`)
)

var sensitiveKeys = map[string]struct{}{
	"password":                             {},
	"passwd":                               {},
	"sessiontoken":                         {},
	"accesstoken":                          {},
	"refreshtoken":                         {},
	"sessionkey":                           {},
	"apikey":                               {},
	"authorization":                        {},
	"proxyauthorization":                   {},
	"cookie":                               {},
	"setcookie":                            {},
	"clientsecret":                         {},
	"secret":                               {},
	"ciphertext":                           {},
	"openaisentinelchatrequirementstoken": {},
}

// IsSensitiveKey reports whether a field/header/query key should never be
// emitted with its value in logs or read APIs.
func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.NewReplacer("-", "", "_", "", ".", "", " ", "").Replace(normalized)
	_, ok := sensitiveKeys[normalized]
	return ok
}

// RedactURL removes userinfo credentials, sensitive query values and fragments
// from a URL intended for diagnostics. Invalid URLs return a fixed placeholder
// instead of echoing potentially sensitive input.
func RedactURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "<redacted-url>"
	}
	copyURL := *u
	if copyURL.User != nil {
		copyURL.User = url.UserPassword(Redacted, Redacted)
	}
	query := copyURL.Query()
	for key := range query {
		if IsSensitiveKey(key) {
			query.Set(key, Redacted)
		}
	}
	copyURL.RawQuery = query.Encode()
	copyURL.Fragment = ""
	return RedactText(copyURL.String())
}

// RedactHeaders returns a copy safe for diagnostics. Sensitive header values
// are replaced wholesale rather than partially preserved.
func RedactHeaders(input http.Header) http.Header {
	if input == nil {
		return nil
	}
	output := input.Clone()
	for key := range output {
		if IsSensitiveKey(key) {
			output[key] = []string{Redacted}
		}
	}
	return output
}

// RedactText performs best-effort scrubbing for common credential forms that
// may appear inside errors from HTTP clients or external validators. Structured
// code should still avoid logging secret-bearing objects in the first place.
func RedactText(value string) string {
	if value == "" {
		return ""
	}
	value = bearerPattern.ReplaceAllString(value, "Bearer "+Redacted)
	value = basicPattern.ReplaceAllString(value, "Basic "+Redacted)
	value = jwtPattern.ReplaceAllString(value, "<redacted-jwt>")
	value = openAIKeyPattern.ReplaceAllString(value, "sk-"+Redacted)
	value = urlUserInfoPattern.ReplaceAllString(value, `${1}://***:***@`)
	value = assignmentPattern.ReplaceAllString(value, `${1}=***`)
	return value
}
