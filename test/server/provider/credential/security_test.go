package credential_test

import (
	credential "PandoraHelper/internal/provider/credential"
	"context"
	"strings"
	"testing"
)

type leakingValidator struct{}

func (leakingValidator) Validate(context.Context, uint, credential.Secret) (credential.Health, error) {
	return credential.Health{
		State:   credential.StateUnknown,
		Message: "Authorization=Bearer validator-secret password=hunter2 proxy=socks5://alice:proxy-pass@127.0.0.1:1080",
	}, nil
}

func TestProviderRedactsHealthMessageBeforePersistenceAndRead(t *testing.T) {
	store := newMemoryStore()
	provider := credential.NewProvider(store, testCipher(t), leakingValidator{})
	if err := provider.Put(context.Background(), 51, credential.Secret{AccessToken: "stored-secret"}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	health, err := provider.Validate(context.Background(), 51)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	record, err := store.Get(context.Background(), 51)
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	status, err := provider.Status(context.Background(), 51)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	for _, value := range []string{health.Message, record.Message, status.Message} {
		for _, forbidden := range []string{"validator-secret", "hunter2", "alice", "proxy-pass"} {
			if strings.Contains(value, forbidden) {
				t.Fatalf("health message leaked %q: %s", forbidden, value)
			}
		}
	}
}

func TestProviderEncryptsAuthenticatedProxyURL(t *testing.T) {
	store := newMemoryStore()
	provider := credential.NewProvider(store, testCipher(t), credential.NewUnavailableValidator())
	want := "socks5h://alice:proxy-pass@proxy.example:1080"
	if err := provider.Put(context.Background(), 52, credential.Secret{ProxyURL: want}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	record, err := store.Get(context.Background(), 52)
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if strings.Contains(record.Ciphertext, "alice") || strings.Contains(record.Ciphertext, "proxy-pass") {
		t.Fatalf("proxy credentials leaked into ciphertext representation: %s", record.Ciphertext)
	}
	resolved, err := provider.Resolve(context.Background(), 52)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ProxyURL != want {
		t.Fatalf("resolved proxy = %q, want %q", resolved.ProxyURL, want)
	}
}
