package security_test

import (
	appsecurity "PandoraHelper/internal/security"
	"net/http"
	"strings"
	"testing"
)

func TestRedactURLRemovesCredentialsSensitiveQueryAndFragment(t *testing.T) {
	raw := "socks5://alice:proxy-secret@127.0.0.1:1080?access_token=query-secret&mode=test#fragment-secret"
	got := appsecurity.RedactURL(raw)
	for _, forbidden := range []string{"alice", "proxy-secret", "query-secret", "fragment-secret"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("RedactURL leaked %q: %s", forbidden, got)
		}
	}
	if !strings.Contains(got, "127.0.0.1:1080") || !strings.Contains(got, "mode=test") {
		t.Fatalf("RedactURL removed safe routing metadata: %s", got)
	}
}

func TestRedactHeadersRemovesSensitiveValues(t *testing.T) {
	header := make(http.Header)
	header.Set("Authorization", "Bearer auth-secret")
	header.Set("Proxy-Authorization", "Basic proxy-secret")
	header.Set("Cookie", "session=browser-secret")
	header.Set("Set-Cookie", "session=response-secret")
	header.Set("Openai-Sentinel-Chat-Requirements-Token", "sentinel-secret")
	header.Set("Content-Type", "application/json")

	got := appsecurity.RedactHeaders(header)
	for _, key := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "Openai-Sentinel-Chat-Requirements-Token"} {
		if value := got.Get(key); value != appsecurity.Redacted {
			t.Fatalf("%s = %q, want redacted", key, value)
		}
	}
	if got.Get("Content-Type") != "application/json" {
		t.Fatalf("safe header changed: %q", got.Get("Content-Type"))
	}
	if header.Get("Authorization") != "Bearer auth-secret" {
		t.Fatal("RedactHeaders mutated the input header")
	}
}

func TestRedactTextScrubsCommonCredentialShapes(t *testing.T) {
	input := strings.Join([]string{
		"Authorization=Bearer bearer-secret-1234567890",
		"Basic dXNlcjpwYXNzd29yZA==",
		"eyJabcdefghijklmno.abcdefghijklmnop.qrstuvwxyz123456",
		"sk-abcdefghijklmnopqrstuvwx",
		"proxy=socks5h://alice:proxy-pass@proxy.example:1080",
		"password=hunter2",
		"access_token=query-secret",
	}, " ")
	got := appsecurity.RedactText(input)
	for _, forbidden := range []string{
		"bearer-secret-1234567890",
		"dXNlcjpwYXNzd29yZA==",
		"eyJabcdefghijklmno.abcdefghijklmnop.qrstuvwxyz123456",
		"abcdefghijklmnopqrstuvwx",
		"alice",
		"proxy-pass",
		"hunter2",
		"query-secret",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("RedactText leaked %q: %s", forbidden, got)
		}
	}
}

func TestSensitiveKeyNormalization(t *testing.T) {
	for _, key := range []string{
		"Authorization",
		"proxy-authorization",
		"access_token",
		"RefreshToken",
		"Set-Cookie",
		"Openai-Sentinel-Chat-Requirements-Token",
		"client_secret",
	} {
		if !appsecurity.IsSensitiveKey(key) {
			t.Fatalf("expected sensitive key: %s", key)
		}
	}
	if appsecurity.IsSensitiveKey("Content-Type") {
		t.Fatal("Content-Type must not be treated as sensitive")
	}
}
