package service

import (
	v1 "PandoraHelper/api/v1"
	"PandoraHelper/internal/model"
	credentialprovider "PandoraHelper/internal/provider/credential"
	"PandoraHelper/internal/repository"
	apptransport "PandoraHelper/internal/transport"
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type AccountService interface {
	RefreshAccount(ctx context.Context, id int64) error
	GetAccount(ctx context.Context, id int64) (*model.Account, error)
	Update(ctx context.Context, account *model.Account) error
	Create(ctx context.Context, account *model.Account) error
	SearchAccount(ctx context.Context, accountType string, keyword string) ([]*v1.AccountSummary, error)
	DeleteAccount(ctx context.Context, id int64) error
	GetShareAccountList(ctx *gin.Context) ([]*model.Account, bool, bool, error)
	LoginShareAccount(ctx *gin.Context, req *v1.LoginShareAccountRequest) (string, error)
	GetOneApiChannelList(ctx context.Context) ([]*model.OneApiChannel, error)
	UpdateOneApiChannelToken(ctx context.Context, id int64, token string) error
}

func NewAccountService(
	service *Service,
	accountRepository repository.AccountRepository,
	credentialProvider credentialprovider.Provider,
	viper *viper.Viper,
	coordinator *Coordinator,
) AccountService {
	return &accountService{
		Service:            service,
		accountRepository:  accountRepository,
		credentialProvider: credentialProvider,
		viper:              viper,
		coordinator:        coordinator,
	}
}

type accountService struct {
	*Service
	accountRepository  repository.AccountRepository
	credentialProvider credentialprovider.Provider
	viper              *viper.Viper
	coordinator        *Coordinator
}

func (s *accountService) UpdateOneApiChannelToken(ctx context.Context, id int64, token string) error {
	if s.viper.GetString("oneapi.token") == "" || s.viper.GetString("oneapi.domain") == "" {
		s.logger.Warn("oneapi token is empty, disable oneapi channel")
		return nil
	}
	oneToken := s.viper.GetString("oneapi.token")
	oneURL := fmt.Sprintf("%s/api/channel", s.viper.GetString("oneapi.domain"))
	client := resty.New()

	getURL := fmt.Sprintf("%s/%d", oneURL, id)
	resp := struct {
		Data model.OneApiChannel `json:"data"`
	}{Data: model.OneApiChannel{}}

	res, err := client.R().SetHeader("Authorization", "Bearer "+oneToken).SetResult(&resp).Get(getURL)
	if err != nil {
		return err
	}
	s.logger.Info("GetOneApiChannel", zap.Any("result", res))

	param := resp.Data
	param.Key = token
	res, err = client.R().SetHeader("Authorization", "Bearer "+oneToken).SetBody(param).Put(oneURL)
	s.logger.Info("UpdateOneApiChannelToken", zap.Any("result", res))
	return err
}

func (s *accountService) GetOneApiChannelList(ctx context.Context) ([]*model.OneApiChannel, error) {
	if s.viper.GetString("oneapi.token") == "" || s.viper.GetString("oneapi.domain") == "" {
		s.logger.Warn("oneapi token is empty, disable oneapi channel")
		return []*model.OneApiChannel{}, nil
	}
	oneToken := s.viper.GetString("oneapi.token")
	oneURL := fmt.Sprintf("%s/api/channel/?p=0&page_size=1000&id_sort=true", s.viper.GetString("oneapi.domain"))

	result := struct {
		Data []*model.OneApiChannel `json:"data"`
	}{Data: make([]*model.OneApiChannel, 0)}
	client := resty.New()
	resp, err := client.R().SetHeader("Authorization", oneToken).SetResult(&result).Get(oneURL)
	if err != nil {
		return nil, err
	}
	s.logger.Info("GetOneApiChannelList", zap.Any("result", resp))
	return result.Data, nil
}

func (s *accountService) LoginShareAccount(ctx *gin.Context, req *v1.LoginShareAccountRequest) (string, error) {
	account, err := s.accountRepository.GetAccount(ctx, req.Id)
	if err != nil {
		return "", err
	}
	if account.Shared == 0 {
		return "", errors.New("账户未开启共享")
	}
	if account.AccountType == "" || account.AccountType == "chatgpt" || account.AccountType == "claude" {
		return "", ErrProviderNotConfigured
	}
	return "", errors.New("不支持的账户类型")
}

func (s *accountService) GetShareAccountList(ctx *gin.Context) ([]*model.Account, bool, bool, error) {
	accounts, err := s.accountRepository.GetShareAccountList(ctx)
	if err != nil {
		return nil, false, false, err
	}
	custom := s.viper.GetBool("share.custom")
	random := s.viper.GetBool("share.random")
	if len(accounts) == 0 {
		return accounts, false, false, nil
	}
	if !custom && !random {
		return []*model.Account{}, false, false, nil
	}
	return accounts, custom, random, nil
}

func (s *accountService) RefreshAccount(ctx context.Context, id int64) error {
	account, err := s.accountRepository.GetAccount(ctx, id)
	if err != nil {
		return err
	}
	health, err := s.credentialProvider.Validate(ctx, account.ID)
	if err != nil {
		if health.Message != "" {
			return fmt.Errorf("%s: %w", health.Message, err)
		}
		return err
	}
	return nil
}

func (s *accountService) Update(ctx context.Context, account *model.Account) error {
	if account.ID == 0 {
		return errors.New("account id is required")
	}

	existing, err := s.accountRepository.GetAccount(ctx, int64(account.ID))
	if err != nil {
		return err
	}

	incomingSecret := credentialFromAccount(account)
	legacySecret := credentialFromAccount(existing)
	if !incomingSecret.Empty() && !s.credentialProvider.CanPersist() {
		return fmt.Errorf("cannot update account credentials: %w", credentialprovider.ErrEncryptionKeyMissing)
	}

	if account.ProxyURL != "" {
		if err := validateAccountProxy(account.ProxyURL); err != nil {
			return err
		}
		existing.ProxyURL = account.ProxyURL
	}
	existing.Email = account.Email
	if account.AccountType != "" {
		existing.AccountType = account.AccountType
	}
	existing.Shared = account.Shared
	existing.OneApiChannelId = account.OneApiChannelId

	var secretToPersist credentialprovider.Secret
	if !incomingSecret.Empty() {
		baseSecret, resolveErr := s.credentialProvider.Resolve(ctx, existing.ID)
		if resolveErr != nil {
			if !errors.Is(resolveErr, credentialprovider.ErrCredentialNotFound) {
				return resolveErr
			}
			baseSecret = legacySecret
		}
		secretToPersist = mergeCredential(baseSecret, incomingSecret)
	} else if !legacySecret.Empty() && s.credentialProvider.CanPersist() {
		secretToPersist = legacySecret
	}

	if s.credentialProvider.CanPersist() {
		clearAccountCredentialFields(existing)
	}

	return s.tm.Transaction(ctx, func(txCtx context.Context) error {
		if err := s.accountRepository.Update(txCtx, existing); err != nil {
			return err
		}
		if !secretToPersist.Empty() {
			if err := s.credentialProvider.Put(txCtx, existing.ID, secretToPersist); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *accountService) Create(ctx context.Context, account *model.Account) error {
	if err := validateAccountProxy(account.ProxyURL); err != nil {
		return err
	}

	secret := credentialFromAccount(account)
	if !secret.Empty() && !s.credentialProvider.CanPersist() {
		return fmt.Errorf("cannot store account credentials: %w", credentialprovider.ErrEncryptionKeyMissing)
	}

	persisted := *account
	clearAccountCredentialFields(&persisted)

	if err := s.tm.Transaction(ctx, func(txCtx context.Context) error {
		if err := s.accountRepository.Create(txCtx, &persisted); err != nil {
			return err
		}
		if !secret.Empty() {
			if err := s.credentialProvider.Put(txCtx, persisted.ID, secret); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	account.ID = persisted.ID
	return nil
}

func validateAccountProxy(proxyURL string) error {
	if proxyURL == "" {
		return nil
	}
	if _, err := apptransport.ParseProxy(proxyURL); err != nil {
		return fmt.Errorf("invalid account proxy: %w", err)
	}
	return nil
}

func credentialFromAccount(account *model.Account) credentialprovider.Secret {
	if account == nil {
		return credentialprovider.Secret{}
	}
	return credentialprovider.Secret{
		Representation: credentialprovider.RepresentationLegacyFields,
		Password:       account.Password,
		SessionToken:   account.SessionToken,
		AccessToken:    account.AccessToken,
		RefreshToken:   account.RefreshToken,
		SessionKey:     account.SessionKey,
	}
}

func mergeCredential(base credentialprovider.Secret, patch credentialprovider.Secret) credentialprovider.Secret {
	merged := base
	if merged.Representation == "" {
		merged.Representation = patch.Representation
	}
	if patch.Password != "" {
		merged.Password = patch.Password
	}
	if patch.SessionToken != "" {
		merged.SessionToken = patch.SessionToken
	}
	if patch.AccessToken != "" {
		merged.AccessToken = patch.AccessToken
	}
	if patch.RefreshToken != "" {
		merged.RefreshToken = patch.RefreshToken
	}
	if patch.SessionKey != "" {
		merged.SessionKey = patch.SessionKey
	}
	if patch.Cookie != "" {
		merged.Cookie = patch.Cookie
	}
	if patch.Reference != "" {
		merged.Reference = patch.Reference
	}
	return merged
}

func clearAccountCredentialFields(account *model.Account) {
	account.Password = ""
	account.SessionToken = ""
	account.AccessToken = ""
	account.RefreshToken = ""
	account.SessionKey = ""
}

func (s *accountService) SearchAccount(ctx context.Context, accountType string, keyword string) ([]*v1.AccountSummary, error) {
	accounts, err := s.accountRepository.SearchAccount(ctx, accountType, keyword)
	if err != nil {
		return nil, err
	}

	summaries := make([]*v1.AccountSummary, 0, len(accounts))
	for _, account := range accounts {
		status, err := s.credentialProvider.Status(ctx, account.ID)
		if err != nil {
			return nil, err
		}
		if !status.HasCredential && !credentialFromAccount(account).Empty() {
			status.HasCredential = true
			status.State = credentialprovider.StateUnknown
			status.Message = "legacy credential is pending encrypted migration"
		}

		summary := &v1.AccountSummary{
			ID:                account.ID,
			Email:             account.Email,
			AccountType:       account.AccountType,
			CreateTime:        account.CreateTime,
			UpdateTime:        account.UpdateTime,
			Shared:            account.Shared,
			OneApiChannelId:   account.OneApiChannelId,
			HasCredential:     status.HasCredential,
			CredentialState:   string(status.State),
			CredentialMessage: status.Message,
			CredentialChecked: status.CheckedAt,
			ProxyConfigured:   account.ProxyURL != "",
		}
		if account.ProxyURL != "" {
			summary.ProxyDisplay = apptransport.RedactProxyURL(account.ProxyURL)
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (s *accountService) DeleteAccount(ctx context.Context, id int64) error {
	return s.tm.Transaction(ctx, func(txCtx context.Context) error {
		if err := s.credentialProvider.Delete(txCtx, uint(id)); err != nil {
			return err
		}
		return s.accountRepository.DeleteAccount(txCtx, id)
	})
}

func (s *accountService) GetAccount(ctx context.Context, id int64) (*model.Account, error) {
	return s.accountRepository.GetAccount(ctx, id)
}
