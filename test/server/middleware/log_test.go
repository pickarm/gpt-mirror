package middleware_test

import (
	"PandoraHelper/internal/middleware"
	"strings"
	"testing"
)

func TestRedactRequestBodyForLogRemovesNestedCredentials(t *testing.T) {
	body := []byte(`{
		"email":"safe@example.com",
		"password":"password-secret",
		"accessToken":"access-secret",
		"nested":{
			"refresh_token":"refresh-secret",
			"proxyUrl":"socks5h://user:proxy-secret@127.0.0.1:1080",
			"safe":"visible"
		},
		"items":[{"sessionKey":"session-secret"}]
	}`)

	got := middleware.RedactRequestBodyForLog(body)
	for _, forbidden := range []string{
		"password-secret",
		"access-secret",
		"refresh-secret",
		"proxy-secret",
		"session-secret",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("redacted log contains secret %q: %s", forbidden, got)
		}
	}
	for _, expected := range []string{"safe@example.com", "visible", "[REDACTED]"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("redacted log missing expected value %q: %s", expected, got)
		}
	}
}

func TestRedactRequestBodyForLogDoesNotEmitNonJSONBody(t *testing.T) {
	got := middleware.RedactRequestBodyForLog([]byte("password=plaintext-secret&token=token-secret"))
	if strings.Contains(got, "plaintext-secret") || strings.Contains(got, "token-secret") {
		t.Fatalf("non-JSON log body leaked secret: %s", got)
	}
	if !strings.Contains(got, "non-json body") {
		t.Fatalf("non-JSON log body did not return safe metadata: %s", got)
	}
}
