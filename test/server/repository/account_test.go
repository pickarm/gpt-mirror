package repository_test

import (
	"PandoraHelper/internal/model"
	"PandoraHelper/internal/repository"
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAccountRepository(t *testing.T) (repository.AccountRepository, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Account{}, &model.Share{}))

	base := repository.NewRepository(nil, db)
	return repository.NewAccountRepository(base), db
}

func TestAccountRepositoryCRUDAndSearch(t *testing.T) {
	repo, _ := newAccountRepository(t)
	ctx := context.Background()

	account := &model.Account{
		Email:        "alice@example.com",
		AccountType:  "chatgpt",
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
	}

	require.NoError(t, repo.Create(ctx, account))
	require.NotZero(t, account.ID)

	stored, err := repo.GetAccount(ctx, int64(account.ID))
	require.NoError(t, err)
	require.Equal(t, "alice@example.com", stored.Email)
	require.Equal(t, "chatgpt", stored.AccountType)

	matches, err := repo.SearchAccount(ctx, "chatgpt", "alice")
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, account.ID, matches[0].ID)

	account.Email = "alice+updated@example.com"
	require.NoError(t, repo.Update(ctx, account))

	updated, err := repo.GetAccount(ctx, int64(account.ID))
	require.NoError(t, err)
	require.Equal(t, "alice+updated@example.com", updated.Email)

	require.NoError(t, repo.DeleteAccount(ctx, int64(account.ID)))
	_, err = repo.GetAccount(ctx, int64(account.ID))
	require.Error(t, err)
}

func TestAccountRepositoryPreloadsShares(t *testing.T) {
	repo, db := newAccountRepository(t)
	ctx := context.Background()

	account := &model.Account{
		Email:       "shared@example.com",
		AccountType: "chatgpt",
		Shared:      1,
	}
	require.NoError(t, repo.Create(ctx, account))

	share := &model.Share{
		AccountID:  account.ID,
		UniqueName: "demo-share",
		ShareType:  "chatgpt",
	}
	require.NoError(t, db.Create(share).Error)

	stored, err := repo.GetAccount(ctx, int64(account.ID))
	require.NoError(t, err)
	require.Len(t, stored.Shares, 1)
	require.Equal(t, "demo-share", stored.Shares[0].UniqueName)
}
