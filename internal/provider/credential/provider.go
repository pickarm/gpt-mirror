package credential

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type encryptedProvider struct {
	store     Store
	cipher    Cipher
	validator Validator
	now       func() time.Time
}

func NewProvider(store Store, cipher Cipher, validator Validator) Provider {
	return &encryptedProvider{
		store:     store,
		cipher:    cipher,
		validator: validator,
		now:       time.Now,
	}
}

func (p *encryptedProvider) CanPersist() bool {
	return p.cipher != nil && p.cipher.Enabled()
}

func (p *encryptedProvider) Resolve(ctx context.Context, accountID uint) (Secret, error) {
	record, err := p.store.Get(ctx, accountID)
	if err != nil {
		return Secret{}, err
	}
	plaintext, err := p.cipher.Decrypt(record.Ciphertext)
	if err != nil {
		return Secret{}, fmt.Errorf("decrypt credential for account %d: %w", accountID, err)
	}
	var secret Secret
	if err := json.Unmarshal(plaintext, &secret); err != nil {
		return Secret{}, fmt.Errorf("decode credential for account %d: %w", accountID, err)
	}
	return secret, nil
}

func (p *encryptedProvider) Status(ctx context.Context, accountID uint) (Status, error) {
	record, err := p.store.Get(ctx, accountID)
	if err != nil {
		if errors.Is(err, ErrCredentialNotFound) {
			return Status{
				HasCredential: false,
				State:         StateUnknown,
				Message:       "credential is not configured",
			}, nil
		}
		return Status{}, err
	}
	return Status{
		HasCredential: true,
		State:         record.State,
		Message:       record.Message,
		CheckedAt:     record.CheckedAt,
	}, nil
}

func (p *encryptedProvider) Put(ctx context.Context, accountID uint, secret Secret) error {
	if secret.Empty() {
		return nil
	}
	if !p.CanPersist() {
		return ErrEncryptionKeyMissing
	}
	plaintext, err := json.Marshal(secret)
	if err != nil {
		return fmt.Errorf("encode credential: %w", err)
	}
	ciphertext, err := p.cipher.Encrypt(plaintext)
	if err != nil {
		return err
	}
	return p.store.Upsert(ctx, &Record{
		AccountID:  accountID,
		Kind:       secret.Kind(),
		Ciphertext: ciphertext,
		State:      StateUnknown,
		Message:    "credential stored; remote validation pending",
	})
}

func (p *encryptedProvider) Delete(ctx context.Context, accountID uint) error {
	return p.store.Delete(ctx, accountID)
}

func (p *encryptedProvider) Validate(ctx context.Context, accountID uint) (Health, error) {
	secret, err := p.Resolve(ctx, accountID)
	if err != nil {
		health := HealthFromError(err)
		if errors.Is(err, ErrCredentialNotFound) {
			health.Message = "credential is not configured"
		}
		health.CheckedAt = p.now()
		return health, err
	}

	health, err := p.validator.Validate(ctx, accountID, secret)
	if err != nil {
		health = HealthFromError(err)
	}
	if health.State == "" {
		health.State = StateUnknown
	}
	if health.CheckedAt.IsZero() {
		health.CheckedAt = p.now()
	}
	if updateErr := p.store.UpdateHealth(ctx, accountID, health); updateErr != nil && err == nil {
		return health, updateErr
	}
	return health, err
}

type unavailableValidator struct{}

func NewUnavailableValidator() Validator {
	return unavailableValidator{}
}

func (unavailableValidator) Validate(context.Context, uint, Secret) (Health, error) {
	return Health{
		State:   StateUnknown,
		Message: "remote session validation is not configured",
	}, nil
}
