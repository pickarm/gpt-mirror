package service

import (
	v1 "PandoraHelper/api/v1"
	"PandoraHelper/internal/model"
	"PandoraHelper/internal/repository"
	"context"
	"github.com/spf13/viper"
)

type ShareService interface {
	GetShareTokenByAccessToken(ctx context.Context, accessToken string, share *model.Share, resetLimit bool) (string, error)
	RefreshShareToken(ctx context.Context, share *model.Share, accessToken string, resetLimit bool) (string, error)
	ResetShareLimit(ctx context.Context, id int64) error
	GetShare(ctx context.Context, id int64) (*model.Share, error)
	Update(ctx context.Context, share *model.Share) error
	Create(ctx context.Context, share *model.Share) error
	SearchShare(ctx context.Context, accountType string, email string, uniqueName string) ([]*model.Share, error)
	DeleteShare(ctx context.Context, id int64) error
	LoginShareByPassword(ctx context.Context, username string, password string) (string, error)
	ShareStatistic(ctx context.Context, accountId int) (interface{}, interface{})
	ShareResetPassword(ctx context.Context, uniqueName string, password string, newPassword string) error
	GetSharesByAccountId(ctx context.Context, accountId int) ([]*model.Share, error)
	GetOauthLoginUrl(ctx context.Context, share *model.Share) (string, error)
}

func NewShareService(service *Service, shareRepository repository.ShareRepository, viper *viper.Viper, coordinator *Coordinator) ShareService {
	return &shareService{
		Service:         service,
		shareRepository: shareRepository,
		viper:           viper,
		coordinator:     coordinator,
	}
}

type shareService struct {
	*Service
	shareRepository repository.ShareRepository
	viper           *viper.Viper
	coordinator     *Coordinator
}

func (s *shareService) GetOauthLoginUrl(ctx context.Context, share *model.Share) (string, error) {
	return "", ErrProviderNotConfigured
}

func (s *shareService) GetSharesByAccountId(ctx context.Context, accountId int) ([]*model.Share, error) {
	return s.shareRepository.GetSharesByAccountId(ctx, accountId)
}

func (s *shareService) ShareResetPassword(ctx context.Context, uniqueName string, password string, newPassword string) error {
	share, err := s.shareRepository.GetShareByUniqueName(ctx, uniqueName)
	if err != nil {
		return err
	}
	if share == nil || share.Password != password {
		return v1.ErrUsernameOrPassword
	}
	share.Password = newPassword
	return s.shareRepository.Update(ctx, share)
}

func (s *shareService) ShareStatistic(ctx context.Context, accountId int) (interface{}, interface{}) {
	return nil, ErrProviderNotConfigured
}

func (s *shareService) LoginShareByPassword(ctx context.Context, username string, password string) (string, error) {
	share, err := s.shareRepository.GetShareByUniqueName(ctx, username)
	if err != nil {
		return "", err
	}
	if share == nil || share.Password != password {
		return "", v1.ErrUsernameOrPassword
	}
	return "", ErrProviderNotConfigured
}

func (s *shareService) GetChatGPTOauthLoginUrl(share *model.Share) (string, error) {
	return "", ErrProviderNotConfigured
}

func (s *shareService) GetClaudeOauthLoginUrl(ctx context.Context, share *model.Share) (string, error) {
	return "", ErrProviderNotConfigured
}

func (s *shareService) GetShareTokenByAccessToken(ctx context.Context, accessToken string, share *model.Share, resetLimit bool) (string, error) {
	return "", ErrProviderNotConfigured
}

func (s *shareService) RefreshShareToken(ctx context.Context, share *model.Share, accessToken string, resetLimit bool) (string, error) {
	return "", ErrProviderNotConfigured
}

// Update/Create/Delete intentionally remain local-only during the provider migration.
// Remote share/session semantics will be reintroduced behind the provider layer.
func (s *shareService) Update(ctx context.Context, share *model.Share) error {
	return s.shareRepository.Update(ctx, share)
}

func (s *shareService) Create(ctx context.Context, share *model.Share) error {
	return s.shareRepository.Create(ctx, share)
}

func (s *shareService) SearchShare(ctx context.Context, accountType string, email string, uniqueName string) ([]*model.Share, error) {
	return s.shareRepository.SearchShare(ctx, accountType, email, uniqueName)
}

func (s *shareService) DeleteShare(ctx context.Context, id int64) error {
	return s.shareRepository.DeleteShare(ctx, id)
}

func (s *shareService) GetShare(ctx context.Context, id int64) (*model.Share, error) {
	return s.shareRepository.GetShare(ctx, id)
}

func (s *shareService) ResetShareLimit(ctx context.Context, id int64) error {
	if _, err := s.shareRepository.GetShare(ctx, id); err != nil {
		return err
	}
	return ErrProviderNotConfigured
}

func (s *shareService) GetShareTokenInfo(shareToken string, accessToken string) (v1.StatisticResult, error) {
	return v1.StatisticResult{}, ErrProviderNotConfigured
}
