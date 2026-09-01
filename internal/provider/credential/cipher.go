package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/viper"
)

const ciphertextVersion = "v1:"

type Cipher interface {
	Enabled() bool
	Encrypt(plaintext []byte) (string, error)
	Decrypt(ciphertext string) ([]byte, error)
}

type disabledCipher struct{}

func (disabledCipher) Enabled() bool { return false }
func (disabledCipher) Encrypt([]byte) (string, error) {
	return "", ErrEncryptionKeyMissing
}
func (disabledCipher) Decrypt(string) ([]byte, error) {
	return nil, ErrEncryptionKeyMissing
}

type aesGCMCipher struct {
	aead cipher.AEAD
}

func (c *aesGCMCipher) Enabled() bool { return true }

func (c *aesGCMCipher) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate credential nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, sealed...)
	return ciphertextVersion + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (c *aesGCMCipher) Decrypt(value string) ([]byte, error) {
	if !strings.HasPrefix(value, ciphertextVersion) {
		return nil, fmt.Errorf("unsupported credential ciphertext version")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, ciphertextVersion))
	if err != nil {
		return nil, fmt.Errorf("decode credential ciphertext: %w", err)
	}
	nonceSize := c.aead.NonceSize()
	if len(payload) <= nonceSize {
		return nil, fmt.Errorf("credential ciphertext is truncated")
	}
	plaintext, err := c.aead.Open(nil, payload[:nonceSize], payload[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt credential ciphertext: %w", err)
	}
	return plaintext, nil
}

// NewCipher accepts a 32-byte AES key encoded as base64 or hexadecimal. An
// empty setting intentionally disables encrypted persistence instead of falling
// back to plaintext storage.
func NewCipher(conf *viper.Viper) (Cipher, error) {
	raw := strings.TrimSpace(conf.GetString("security.credential_key"))
	if raw == "" {
		return disabledCipher{}, nil
	}

	key, err := decodeKey(raw)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create credential cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create credential GCM: %w", err)
	}
	return &aesGCMCipher{aead: aead}, nil
}

func decodeKey(raw string) ([]byte, error) {
	var (
		key []byte
		err error
	)

	switch {
	case strings.HasPrefix(raw, "base64:"):
		key, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(raw, "base64:"))
	case strings.HasPrefix(raw, "hex:"):
		key, err = hex.DecodeString(strings.TrimPrefix(raw, "hex:"))
	default:
		key, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			key, err = hex.DecodeString(raw)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("security.credential_key must be base64 or hex encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("security.credential_key must decode to exactly 32 bytes, got %d", len(key))
	}
	return key, nil
}
