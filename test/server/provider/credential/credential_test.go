package credential_test

import (
	"PandoraHelper/internal/model"
	credential "PandoraHelper/internal/provider/credential"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
)

type memoryStore struct {
	mu      sync.Mutex
	records map[uint]*credential.Record
}

func newMemoryStore() *memoryStore {
	return &memoryStore{records: make(map[uint]*credential.Record)}
}

func (s *memoryStore) Get(_ context.Context, accountID uint) (*credential.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[accountID]
	if !ok {
		return nil, credential.ErrCredentialNotFound
	}
	copyRecord := *record
	return &copyRecord, nil
}

func (s *memoryStore) Upsert(_ context.Context, record *credential.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copyRecord := *record
	s.records[record.AccountID] = &copyRecord
	return nil
}

func (s *memoryStore) Delete(_ context.Context, accountID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, accountID)
	return nil
}

func (s *memoryStore) UpdateHealth(_ context.Context, accountID uint, health credential.Health) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[accountID]
	if !ok {
		return credential.ErrCredentialNotFound
	}
	record.State = health.State
	record.Message = health.Message
	checkedAt := health.CheckedAt
	record.CheckedAt = &checkedAt
	return nil
}

type expiredValidator struct{}

func (expiredValidator) Validate(context.Context, uint, credential.Secret) (credential.Health, error) {
	return credential.Health{}, credential.ErrExpired
}

func testCipher(t *testing.T) credential.Cipher {
	t.Helper()
	conf := viper.New()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	conf.Set("security.credential_key", base64.StdEncoding.EncodeToString(key))
	cipher, err := credential.NewCipher(conf)
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	return cipher
}

func TestCipherRoundTripDoesNotContainPlaintext(t *testing.T) {
	cipher := testCipher(t)
	plaintext := []byte(`{"access_token":"top-secret-access-token"}`)
	ciphertext, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if strings.Contains(ciphertext, "top-secret-access-token") {
		t.Fatal("ciphertext contains plaintext secret")
	}
	got, err := cipher.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", got, plaintext)
	}
}

func TestDisabledCipherRefusesSecretPersistence(t *testing.T) {
	conf := viper.New()
	cipher, err := credential.NewCipher(conf)
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	provider := credential.NewProvider(newMemoryStore(), cipher, credential.NewUnavailableValidator())
	err = provider.Put(context.Background(), 1, credential.Secret{AccessToken: "secret"})
	if !errors.Is(err, credential.ErrEncryptionKeyMissing) {
		t.Fatalf("Put() error = %v, want ErrEncryptionKeyMissing", err)
	}
}

func TestProviderStoresEncryptedSecretAndMetadata(t *testing.T) {
	store := newMemoryStore()
	provider := credential.NewProvider(store, testCipher(t), credential.NewUnavailableValidator())
	want := credential.Secret{
		Representation: credential.RepresentationToken,
		AccessToken:    "access-secret",
		RefreshToken:   "refresh-secret",
	}
	if err := provider.Put(context.Background(), 42, want); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	record, err := store.Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if strings.Contains(record.Ciphertext, "access-secret") || strings.Contains(record.Ciphertext, "refresh-secret") {
		t.Fatal("stored ciphertext contains plaintext credential")
	}
	status, err := provider.Status(context.Background(), 42)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.HasCredential || status.State != credential.StateUnknown {
		t.Fatalf("Status() = %+v", status)
	}
	got, err := provider.Resolve(context.Background(), 42)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Fatalf("Resolve() = %+v", got)
	}
}

func TestExpiredValidatorProducesTypedHealth(t *testing.T) {
	store := newMemoryStore()
	provider := credential.NewProvider(store, testCipher(t), expiredValidator{})
	if err := provider.Put(context.Background(), 7, credential.Secret{SessionKey: "session-secret"}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	health, err := provider.Validate(context.Background(), 7)
	if !errors.Is(err, credential.ErrExpired) {
		t.Fatalf("Validate() error = %v, want ErrExpired", err)
	}
	if health.State != credential.StateExpired || health.Message == "" || health.CheckedAt.IsZero() {
		t.Fatalf("Validate() health = %+v", health)
	}
	status, err := provider.Status(context.Background(), 7)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.State != credential.StateExpired || status.CheckedAt == nil {
		t.Fatalf("Status() after validation = %+v", status)
	}
}

func TestAccountJSONNeverSerializesSecrets(t *testing.T) {
	account := model.Account{
		ID:           1,
		Email:        "safe@example.com",
		Password:     "password-secret",
		SessionToken: "session-token-secret",
		AccessToken:  "access-token-secret",
		RefreshToken: "refresh-token-secret",
		SessionKey:   "session-key-secret",
		ProxyURL:     "http://user:proxy-secret@127.0.0.1:8080",
	}
	raw, err := json.Marshal(account)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	text := string(raw)
	for _, forbidden := range []string{
		"password-secret",
		"session-token-secret",
		"access-token-secret",
		"refresh-token-secret",
		"session-key-secret",
		"proxy-secret",
		"\"password\"",
		"\"sessionToken\"",
		"\"accessToken\"",
		"\"refreshToken\"",
		"\"sessionKey\"",
		"\"proxyUrl\"",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("serialized Account contains sensitive value/key %q: %s", forbidden, text)
		}
	}
}

func TestHealthFromBlockedError(t *testing.T) {
	health := credential.HealthFromError(credential.ErrBlocked)
	if health.State != credential.StateBlocked || health.Message == "" {
		t.Fatalf("HealthFromError() = %+v", health)
	}
}

func TestInvalidCredentialKeyIsRejected(t *testing.T) {
	conf := viper.New()
	conf.Set("security.credential_key", base64.StdEncoding.EncodeToString([]byte("too-short")))
	if _, err := credential.NewCipher(conf); err == nil {
		t.Fatal("NewCipher() accepted a key that is not 32 bytes")
	}
}

var _ = time.Time{}
