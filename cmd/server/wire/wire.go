//go:build wireinject
// +build wireinject

package wire

import (
	"PandoraHelper/internal/handler"
	chatgptprovider "PandoraHelper/internal/provider/chatgpt"
	credentialprovider "PandoraHelper/internal/provider/credential"
	"PandoraHelper/internal/repository"
	"PandoraHelper/internal/server"
	"PandoraHelper/internal/service"
	"PandoraHelper/pkg/app"
	"PandoraHelper/pkg/jwt"
	"PandoraHelper/pkg/log"
	"PandoraHelper/pkg/server/http"
	"PandoraHelper/pkg/sid"
	"github.com/google/wire"
	"github.com/spf13/viper"
)

var repositorySet = wire.NewSet(
	repository.NewDB,
	repository.NewRepository,
	repository.NewTransaction,
	repository.NewAccountRepository,
	repository.NewCredentialRepository,
	repository.NewShareRepository,
)

var providerSet = wire.NewSet(
	chatgptprovider.NewUnavailableProvider,
	credentialprovider.NewCipher,
	credentialprovider.NewUnavailableValidator,
	credentialprovider.NewProvider,
)

var serviceCoordinatorSet = wire.NewSet(
	service.NewServiceCoordinator,
)

var serviceSet = wire.NewSet(
	providerSet,
	service.NewService,
	service.NewUserService,
	serviceCoordinatorSet,
	service.NewAccountService,
	service.NewShareService,
	server.NewTask,
)

var migrateSet = wire.NewSet(
	server.NewMigrate,
)

var handlerSet = wire.NewSet(
	handler.NewHandler,
	handler.NewUserHandler,
	handler.NewShareHandler,
	handler.NewAccountHandler,
	handler.NewHealthCheckHandler,
)

var serverSet = wire.NewSet(
	server.NewHTTPServer,
	server.NewJob,
)

func newApp(httpServer *http.Server, job *server.Job, task *server.Task, migrate *server.Migrate) *app.App {
	return app.NewApp(
		app.WithServer(httpServer, job, task, migrate),
		app.WithName("gpt-mirror"),
	)
}

func NewWire(*viper.Viper, *log.Logger) (*app.App, func(), error) {
	panic(wire.Build(
		repositorySet,
		serviceSet,
		handlerSet,
		serverSet,
		migrateSet,
		sid.NewSid,
		jwt.NewJwt,
		newApp,
	))
}
