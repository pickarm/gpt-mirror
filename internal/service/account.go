package service

import (
	v1 "PandoraHelper/api/v1"
	"PandoraHelper/internal/model"
	"PandoraHelper/internal/repository"
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
	SearchAccount(ctx context.Context, accountType string, keyword string) ([]*model.Account, error)
	DeleteAccount(ctx context.Context, id int64) error
	GetShareAccountList(ctx *gin.Context) ([]*model.Account, bool, bool, error)
	LoginShareAccount(ctx *gin.Context, req *v1.LoginShareAccountRequest) (string, error)
	GetOneApiChannelList(ctx context.Context) ([]*model.OneApiChannel, error)
	UpdateOneApiChannelToken(ctx context.Context, id int64, token string) error
}

func NewAccountService(service *Service, accountRepository repository.AccountRepository, viper *viper.Viper, coordinator *Coordinator) AccountService {
	return &accountService{
		Service:           service,
		accountRepository: accountRepository,
		viper:             viper,
		coordinator:       coordinator,
	}
}

type accountService struct {
	*Service
	accountRepository repository.AccountRepository
	viper             *viper.Viper
	coordinator       *Coordinator
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
	if _, err := s.accountRepository.GetAccount(ctx, id); err != nil {
		return err
	}
	return ErrProviderNotConfigured
}

func (s *accountService) Update(ctx context.Context, account *model.Account) error {
	return s.accountRepository.Update(ctx, account)
}

func (s *accountService) Create(ctx context.Context, account *model.Account) error {
	return s.accountRepository.Create(ctx, account)
}

func (s *accountService) SearchAccount(ctx context.Context, accountType string, keyword string) ([]*model.Account, error) {
	return s.accountRepository.SearchAccount(ctx, accountType, keyword)
}

func (s *accountService) DeleteAccount(ctx context.Context, id int64) error {
	return s.accountRepository.DeleteAccount(ctx, id)
}

func (s *accountService) GetAccount(ctx context.Context, id int64) (*model.Account, error) {
	return s.accountRepository.GetAccount(ctx, id)
}
