package server

import (
	"PandoraHelper/internal/model"
	credentialprovider "PandoraHelper/internal/provider/credential"
	"PandoraHelper/internal/repository"
	"PandoraHelper/pkg/log"
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Migrate struct {
	db                 *gorm.DB
	log                *log.Logger
	tm                 repository.Transaction
	accountRepository  repository.AccountRepository
	credentialProvider credentialprovider.Provider
}

func NewMigrate(
	db *gorm.DB,
	log *log.Logger,
	tm repository.Transaction,
	accountRepository repository.AccountRepository,
	credentialProvider credentialprovider.Provider,
) *Migrate {
	return &Migrate{
		db:                 db,
		log:                log,
		tm:                 tm,
		accountRepository:  accountRepository,
		credentialProvider: credentialProvider,
	}
}

func (m *Migrate) Start(ctx context.Context) error {
	if err := m.db.AutoMigrate(
		model.Share{},
		model.Account{},
		model.AccountCredential{},
		model.Conversation{},
	); err != nil {
		m.log.Error("user migrate error", zap.Error(err))
		return err
	}
	m.db.Exec("UPDATE account set account_type = 'chatgpt' where account_type = '' or account_type is null")
	m.db.Exec("UPDATE share set share_type = 'chatgpt' where share_type = '' or share_type is null")
	if err := m.migrateLegacyCredentials(ctx); err != nil {
		return err
	}
	m.log.Info("AutoMigrate success")
	return nil
}

func (m *Migrate) migrateLegacyCredentials(ctx context.Context) error {
	var accounts []model.Account
	if err := m.db.WithContext(ctx).
		Where("COALESCE(password, '') <> '' OR COALESCE(session_token, '') <> '' OR COALESCE(access_token, '') <> '' OR COALESCE(refresh_token, '') <> '' OR COALESCE(session_key, '') <> ''").
		Find(&accounts).Error; err != nil {
		return fmt.Errorf("find legacy account credentials: %w", err)
	}
	if len(accounts) == 0 {
		return nil
	}
	if !m.credentialProvider.CanPersist() {
		m.log.Warn(
			"legacy account credentials remain in plaintext until security.credential_key is configured",
			zap.Int("accounts", len(accounts)),
		)
		return nil
	}

	migrated := 0
	for i := range accounts {
		account := accounts[i]
		secret := credentialprovider.Secret{
			Representation: credentialprovider.RepresentationLegacyFields,
			Password:       account.Password,
			SessionToken:   account.SessionToken,
			AccessToken:    account.AccessToken,
			RefreshToken:   account.RefreshToken,
			SessionKey:     account.SessionKey,
		}
		if secret.Empty() {
			continue
		}

		if err := m.tm.Transaction(ctx, func(txCtx context.Context) error {
			status, err := m.credentialProvider.Status(txCtx, account.ID)
			if err != nil {
				return err
			}
			if status.HasCredential {
				if _, err := m.credentialProvider.Resolve(txCtx, account.ID); err != nil {
					return fmt.Errorf("verify existing encrypted credential for account %d: %w", account.ID, err)
				}
			} else if err := m.credentialProvider.Put(txCtx, account.ID, secret); err != nil {
				return fmt.Errorf("encrypt legacy credential for account %d: %w", account.ID, err)
			}

			account.Password = ""
			account.SessionToken = ""
			account.AccessToken = ""
			account.RefreshToken = ""
			account.SessionKey = ""
			if err := m.accountRepository.Update(txCtx, &account); err != nil {
				return fmt.Errorf("clear legacy credential columns for account %d: %w", account.ID, err)
			}
			return nil
		}); err != nil {
			return err
		}
		migrated++
	}

	m.log.Info("legacy credential migration complete", zap.Int("accounts", migrated))
	return nil
}

func (m *Migrate) Stop(ctx context.Context) error {
	m.log.Info("AutoMigrate stop")
	return nil
}
