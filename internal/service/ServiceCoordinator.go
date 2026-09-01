package service

import (
	credentialprovider "PandoraHelper/internal/provider/credential"
	"PandoraHelper/internal/repository"
	"github.com/spf13/viper"
)

type Coordinator struct {
	AccountSvc AccountService
	ShareSvc   ShareService
}

func NewServiceCoordinator(
	service *Service,
	accountRepo repository.AccountRepository,
	shareRepo repository.ShareRepository,
	credentialProvider credentialprovider.Provider,
	viper *viper.Viper,
) *Coordinator {
	coordinator := &Coordinator{}

	accountSvc := NewAccountService(service, accountRepo, credentialProvider, viper, coordinator)
	shareSvc := NewShareService(service, shareRepo, viper, coordinator)

	coordinator.AccountSvc = accountSvc
	coordinator.ShareSvc = shareSvc

	return coordinator
}
