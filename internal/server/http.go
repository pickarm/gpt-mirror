package server

import (
	"PandoraHelper"
	"PandoraHelper/internal/handler"
	"PandoraHelper/internal/middleware"
	"PandoraHelper/pkg/jwt"
	"PandoraHelper/pkg/log"
	"PandoraHelper/pkg/server/http"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/ulule/limiter/v3"
	"strings"

	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	smem "github.com/ulule/limiter/v3/drivers/store/memory"
	"io/fs"
	nethttp "net/http"
)

func NewHTTPServer(
	logger *log.Logger,
	conf *viper.Viper,
	jwt *jwt.JWT,
	userHandler *handler.UserHandler,
	shareHandler *handler.ShareHandler,
	accountHandler *handler.AccountHandler,
	conversationHandler *handler.ConversationHandler,
	hearthCheckHandler *handler.HealthCheckHandler,
) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	s := http.NewServer(
		gin.Default(),
		logger,
		http.WithServerHost(conf.GetString("http.host")),
		http.WithServerPort(conf.GetInt("http.port")),
	)

	var rateStr string
	if conf.InConfig("http.rate") {
		rateStr = fmt.Sprintf("%d-M", conf.GetInt("http.rate"))
	} else {
		rateStr = "100-M"
	}

	rate, err := limiter.NewRateFromFormatted(rateStr)
	if err != nil {
		panic(err)
	}
	store := smem.NewStore()
	limitMiddleware := mgin.NewMiddleware(limiter.New(store, rate))
	s.ForwardedByClientIP = true
	s.Use(limitMiddleware)

	s.Use(
		middleware.CORSMiddleware(),
		middleware.ResponseLogMiddleware(logger),
		middleware.RequestLogMiddleware(logger),
	)

	frontendFS, err := fs.Sub(PandoraHelper.EmbedFrontendFS, "frontend/dist")
	if err != nil {
		panic(err)
	}

	fileServer := nethttp.FileServer(nethttp.FS(frontendFS))

	s.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.String(nethttp.StatusNotFound, "404 not found")
			return
		}

		path := c.Request.URL.Path
		if _, err := frontendFS.Open(strings.TrimPrefix(path, "/")); err == nil {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		file, err := PandoraHelper.EmbedFrontendFS.ReadFile("frontend/dist/index.html")
		if err != nil {
			c.String(nethttp.StatusInternalServerError, err.Error())
			return
		}
		c.Data(nethttp.StatusOK, "text/html; charset=utf-8", file)
	})

	s.GET("/health", hearthCheckHandler.GetHealthCheck)
	s.GET("/readiness", hearthCheckHandler.ReadinessHandler)

	v1 := s.Group("/api")
	{
		v1.POST("/login_share", shareHandler.LoginShare)
		v1.POST("/reset_password", shareHandler.ShareResetPassword)
		v1.POST("/login", userHandler.Login)
		v1.POST("/share_accounts", accountHandler.GetShareAccountList)
		v1.POST("/login_free_account", accountHandler.LoginShareAccount)

		shareAuthRouter := v1.Group("/share").Use(middleware.StrictAuth(jwt, logger))
		{
			shareAuthRouter.POST("/add", shareHandler.CreateShare)
			shareAuthRouter.POST("/update", shareHandler.UpdateShare)
			shareAuthRouter.POST("/delete", shareHandler.DeleteShare)
			shareAuthRouter.POST("/search", shareHandler.SearchShare)
			shareAuthRouter.POST("/statistic", shareHandler.ShareStatistic)
		}

		accountAuthRouter := v1.Group("/account").Use(middleware.StrictAuth(jwt, logger))
		{
			accountAuthRouter.POST("/add", accountHandler.CreateAccount)
			accountAuthRouter.POST("/refresh", accountHandler.RefreshAccount)
			accountAuthRouter.POST("/search", accountHandler.SearchAccount)
			accountAuthRouter.POST("/delete", accountHandler.DeleteAccount)
			accountAuthRouter.POST("/update", accountHandler.UpdateAccount)
			accountAuthRouter.POST("/oneapi/channels", accountHandler.GetOneApiChannelList)
		}

		chatgptRouter := v1.Group("/chatgpt").Use(middleware.StrictAuth(jwt, logger))
		{
			chatgptRouter.POST("/health", conversationHandler.Health)
			chatgptRouter.POST("/models", conversationHandler.Models)
			chatgptRouter.POST("/conversations/list", conversationHandler.List)
			chatgptRouter.POST("/conversations/get", conversationHandler.Get)
			chatgptRouter.POST("/conversations/create", conversationHandler.Create)
			chatgptRouter.POST("/conversations/continue", conversationHandler.Continue)
			chatgptRouter.POST("/conversations/rename", conversationHandler.Rename)
			chatgptRouter.POST("/conversations/archive", conversationHandler.Archive)
			chatgptRouter.POST("/conversations/delete", conversationHandler.Delete)
		}

		userAuthRouter := v1.Group("/user").Use(middleware.StrictAuth(jwt, logger))
		{
			userAuthRouter.GET("/2fa_secret", userHandler.Get2FASecret)
			userAuthRouter.POST("/2fa_verify", userHandler.Verify2FA)
		}

		settingRouter := v1.Group("/setting")
		{
			settingRouter.GET("/login", userHandler.GetLoginSettings)
		}
	}

	return s
}
