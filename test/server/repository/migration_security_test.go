package repository_test

import (
	"PandoraHelper/internal/model"
	credential "PandoraHelper/internal/provider/credential"
	"PandoraHelper/internal/repository"
	"PandoraHelper/internal/server"
	applog "PandoraHelper/pkg/log"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestMigrationEncryptsLegacyProxyAndMergesExistingCredential(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Account{}, &model.AccountCredential{}, &model.Share{}, &model.Conversation{}))

	account := &model.Account{
		Email:       "legacy@example.com",
		AccountType: "chatgpt",
		Password:    "legacy-password",
		ProxyURL:    "socks5h://proxy-user:proxy-password@proxy.example:1080",
	}
	require.NoError(t, db.Create(account).Error)

	base := repository.NewRepository(nil, db)
	accountRepo := repository.NewAccountRepository(base)
	credentialStore := repository.NewCredentialRepository(base)
	cipherConf := viper.New()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cipherConf.Set("security.credential_key", base64.StdEncoding.EncodeToString(key))
	cipher, err := credential.NewCipher(cipherConf)
	require.NoError(t, err)
	credentialProvider := credential.NewProvider(credentialStore, cipher, credential.NewUnavailableValidator())

	// Simulate M4 already having an encrypted token while old account columns
	// still contain legacy password/proxy credentials.
	require.NoError(t, credentialProvider.Put(context.Background(), account.ID, credential.Secret{
		Representation: credential.RepresentationToken,
		AccessToken:    "existing-encrypted-token",
	}))

	logger := &applog.Logger{Logger: zap.NewNop()}
	migrate := server.NewMigrate(db, logger, repository.NewTransaction(base), accountRepo, credentialProvider)
	require.NoError(t, migrate.Start(context.Background()))

	stored, err := accountRepo.GetAccount(context.Background(), int64(account.ID))
	require.NoError(t, err)
	require.Empty(t, stored.Password)
	require.Equal(t, "socks5h://proxy.example:1080", stored.ProxyURL)
	require.NotContains(t, stored.ProxyURL, "proxy-user")
	require.NotContains(t, stored.ProxyURL, "proxy-password")

	secret, err := credentialProvider.Resolve(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, "existing-encrypted-token", secret.AccessToken)
	require.Equal(t, "legacy-password", secret.Password)
	require.Equal(t, "socks5h://proxy-user:proxy-password@proxy.example:1080", secret.ProxyURL)

	var row model.AccountCredential
	require.NoError(t, db.First(&row, "account_id = ?", account.ID).Error)
	for _, forbidden := range []string{"legacy-password", "proxy-user", "proxy-password", "existing-encrypted-token"} {
		if strings.Contains(row.Ciphertext, forbidden) {
			t.Fatalf("encrypted credential row leaked %q: %s", forbidden, row.Ciphertext)
		}
	}
}
