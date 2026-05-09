package server

import (
	"os"
	"strings"

	"tracehub/internal/ent"
	adminhandler "tracehub/internal/handler/admin"
	gatewayhandler "tracehub/internal/handler/gateway"
	"tracehub/internal/handler/health"
	userhandler "tracehub/internal/handler/user"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/cryptokey"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	redis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Options struct {
	Config *config.Config
	Client *ent.Client
	Redis  *redis.Client
	Log    *zap.Logger
}

func NewRouter(opt Options) (*gin.Engine, error) {
	aesKey, err := cryptokey.DeriveAESKey(opt.Config.Security.JWTSecret, opt.Config.Security.AESKeyBase64)
	if err != nil {
		return nil, err
	}
	jwtSecret := []byte(opt.Config.Security.JWTSecret)
	if len(jwtSecret) < 16 {
		opt.Log.Warn("jwt_secret 过短，生产环境请使用足够长度随机串")
	}

	switch opt.Config.Server.Mode {
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.ZapLogger(opt.Log))
	r.Use(middleware.CORS(opt.Config.Site.CORSOrigins))

	r.GET("/health", health.Liveness())
	r.GET("/ready", health.Readiness(opt.Client, opt.Redis))
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	billingEngine := &billing.Engine{Client: opt.Client}

	gate := r.Group("/v1")
	gate.Use(middleware.GatewayAPIKeyAuth(opt.Client))
	gate.POST("/chat/completions", gatewayhandler.ChatCompletions(gatewayhandler.ChatDeps{
		Client:  opt.Client,
		RDB:     opt.Redis,
		AESKey:  aesKey,
		Billing: billingEngine,
	}))
	gate.GET("/models", gatewayhandler.ModelsList(opt.Client))

	u := r.Group("/api/user")
	u.POST("/register", userhandler.Register(opt.Client, opt.Config))
	u.POST("/login", userhandler.Login(opt.Client, opt.Config, jwtSecret))
	u.POST("/refresh", userhandler.Refresh(opt.Client, jwtSecret))

	authUser := u.Group("")
	authUser.Use(middleware.JWTAuth(jwtSecret, opt.Client))
	authUser.GET("/balance", userhandler.Balance(opt.Client, opt.Config))
	authUser.GET("/usage", userhandler.Usage(opt.Client, opt.Config))
	authUser.GET("/usage/export", userhandler.UsageExport(opt.Client))
	authUser.GET("/apikeys", userhandler.ListAPIKeys(opt.Client))
	authUser.POST("/apikeys", userhandler.CreateAPIKey(opt.Client, opt.Config))
	authUser.PUT("/apikeys/:id", userhandler.UpdateAPIKey(opt.Client))
	authUser.DELETE("/apikeys/:id", userhandler.DeleteAPIKey(opt.Client))
	authUser.POST("/redeem", userhandler.Redeem(opt.Client))
	authUser.PUT("/password", userhandler.ChangePassword(opt.Client))

	chAdmin := adminhandler.ChannelDeps{Client: opt.Client, AESKey: aesKey}

	a := r.Group("/api/admin")
	a.Use(middleware.JWTAuth(jwtSecret, opt.Client))
	a.Use(middleware.RequireRole("admin", "superadmin"))
	a.GET("/dashboard/summary", adminhandler.DashboardSummary(opt.Client))
	a.GET("/users", adminhandler.ListUsers(opt.Client))
	a.PUT("/users/:id", adminhandler.UpdateUser(opt.Client))
	a.POST("/users/:id/balance", adminhandler.AdjustBalance(opt.Client))
	a.GET("/channels", adminhandler.ListChannels(opt.Client))
	a.POST("/channels", adminhandler.CreateChannel(chAdmin))
	a.PUT("/channels/:id", adminhandler.UpdateChannel(chAdmin))
	a.DELETE("/channels/:id", adminhandler.DeleteChannel(opt.Client))
	a.GET("/groups", adminhandler.ListGroups(opt.Client))
	a.POST("/groups", adminhandler.CreateGroup(opt.Client))
	a.PUT("/groups/:id/model-rates", adminhandler.PutGroupModelRates(opt.Client))
	a.GET("/logs", adminhandler.ListLogs(opt.Client))
	a.POST("/redeem-codes", adminhandler.BatchRedeemCodes(opt.Client))

	registerSPA(r)
	return r, nil
}

func registerSPA(r *gin.Engine) {
	const root = "web/frontend/dist"
	if st, err := os.Stat(root + "/index.html"); err != nil || st.IsDir() {
		return
	}
	r.Static("/assets", root+"/assets")
	r.StaticFile("/vite.svg", root+"/vite.svg")
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/v1/") ||
			p == "/health" || p == "/ready" || p == "/metrics" {
			c.JSON(404, gin.H{"code": 40400, "message": "not_found"})
			return
		}
		c.File(root + "/index.html")
	})
}
