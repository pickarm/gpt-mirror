package service

import (
	chatgptprovider "PandoraHelper/internal/provider/chatgpt"
	"PandoraHelper/internal/repository"
	"PandoraHelper/pkg/jwt"
	"PandoraHelper/pkg/log"
	"PandoraHelper/pkg/sid"
)

type Service struct {
	logger          *log.Logger
	sid             *sid.Sid
	jwt             *jwt.JWT
	tm              repository.Transaction
	chatgptProvider chatgptprovider.Provider
}

func NewService(
	tm repository.Transaction,
	logger *log.Logger,
	sid *sid.Sid,
	jwt *jwt.JWT,
	chatgptProvider chatgptprovider.Provider,
) *Service {
	return &Service{
		logger:          logger,
		sid:             sid,
		jwt:             jwt,
		tm:              tm,
		chatgptProvider: chatgptProvider,
	}
}
