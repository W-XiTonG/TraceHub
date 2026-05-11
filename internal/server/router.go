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
	gate.POST("/images/generations", gatewayhandler.ImageGenerations(gatewayhandler.ChatDeps{
		Client:  opt.Client,
		RDB:     opt.Redis,
		AESKey:  aesKey,
		Billing: billingEngine,
	}))
	gate.POST("/embeddings", gatewayhandler.Embeddings(gatewayhandler.ChatDeps{
		Client:  opt.Client,
		RDB:     opt.Redis,
		AESKey:  aesKey,
		Billing: billingEngine,
	}))
	gate.POST("/completions", gatewayhandler.Completions(gatewayhandler.ChatDeps{
		Client:  opt.Client,
		RDB:     opt.Redis,
		AESKey:  aesKey,
		Billing: billingEngine,
	}))
	gate.POST("/moderations", gatewayhandler.Moderations(gatewayhandler.ChatDeps{
		Client:  opt.Client,
		RDB:     opt.Redis,
		AESKey:  aesKey,
		Billing: billingEngine,
	}))
	gate.POST("/audio/transcriptions", gatewayhandler.AudioTranscriptions(gatewayhandler.ChatDeps{
		Client:  opt.Client,
		RDB:     opt.Redis,
		AESKey:  aesKey,
		Billing: billingEngine,
	}))
	gate.POST("/audio/translations", gatewayhandler.AudioTranslations(gatewayhandler.ChatDeps{
		Client:  opt.Client,
		RDB:     opt.Redis,
		AESKey:  aesKey,
		Billing: billingEngine,
	}))
	gate.POST("/audio/speech", gatewayhandler.AudioSpeech(gatewayhandler.ChatDeps{
		Client:  opt.Client,
		RDB:     opt.Redis,
		AESKey:  aesKey,
		Billing: billingEngine,
	}))
	rtDeps := gatewayhandler.ChatDeps{
		Client:  opt.Client,
		RDB:     opt.Redis,
		AESKey:  aesKey,
		Billing: billingEngine,
	}
	gate.GET("/realtime", gatewayhandler.RealtimeWebSocket(rtDeps))
	gate.GET("/realtime/*realtimePath", gatewayhandler.RealtimeWebSocket(rtDeps))
	gate.GET("/models", gatewayhandler.ModelsList(opt.Client))

	u := r.Group("/api/user")
	u.POST("/register", userhandler.Register(opt.Client, opt.Config))
	u.POST("/login", userhandler.Login(opt.Client, opt.Config, jwtSecret))
	u.POST("/refresh", userhandler.Refresh(opt.Client, jwtSecret))

	authUser := u.Group("")
	authUser.Use(middleware.JWTAuth(jwtSecret, opt.Client))
	authUser.GET("/me", userhandler.Me(opt.Client))
	authUser.GET("/balance", userhandler.Balance(opt.Client, opt.Config))
	authUser.GET("/usage-logs", userhandler.UsageLogs(opt.Client))
	authUser.GET("/group", userhandler.MyGroup(opt.Client))
	authUser.GET("/available-models", userhandler.AvailableModels(opt.Client))
	authUser.GET("/models", userhandler.AvailableModels(opt.Client))
	authUser.GET("/usage", userhandler.Usage(opt.Client, opt.Config))
	authUser.GET("/usage/export", userhandler.UsageExport(opt.Client))
	authUser.GET("/apikeys", userhandler.ListAPIKeys(opt.Client))
	authUser.POST("/apikeys", userhandler.CreateAPIKey(opt.Client, opt.Config))
	authUser.PUT("/apikeys/:id", userhandler.UpdateAPIKey(opt.Client))
	authUser.DELETE("/apikeys/:id", userhandler.DeleteAPIKey(opt.Client))
	authUser.POST("/redeem", userhandler.Redeem(opt.Client))
	authUser.PUT("/password", userhandler.ChangePassword(opt.Client))
	authUser.PUT("/bind", userhandler.BindProfile(opt.Client))
	authUser.PUT("/ip_whitelist", userhandler.SetIPWhitelist(opt.Client))

	chAdmin := adminhandler.ChannelDeps{Client: opt.Client, AESKey: aesKey}
	mcAdmin := adminhandler.ModelCatalogDeps{Client: opt.Client, AESKey: aesKey}

	a := r.Group("/api/admin")
	a.Use(middleware.JWTAuth(jwtSecret, opt.Client))
	a.Use(middleware.RequireRole("admin", "superadmin"))
	a.GET("/dashboard/summary", adminhandler.DashboardSummary(opt.Client))
	a.GET("/dashboard/overview", adminhandler.DashboardOverview(opt.Client))
	a.GET("/dashboard/trends", adminhandler.DashboardTrends(opt.Client))
	a.GET("/dashboard/channels", adminhandler.DashboardChannels(opt.Client))
	a.GET("/users", adminhandler.ListUsers(opt.Client))
	a.GET("/users/:id", adminhandler.GetUser(opt.Client))
	a.POST("/users/batch-balance", adminhandler.BatchAdjustBalance(opt.Client))
	a.PUT("/users/:id", adminhandler.UpdateUser(opt.Client))
	a.POST("/users/:id/balance", adminhandler.AdjustBalance(opt.Client))
	a.POST("/users/:id/ban", adminhandler.BanUser(opt.Client))
	a.POST("/users/:id/unban", adminhandler.UnbanUser(opt.Client))
	a.GET("/channels", adminhandler.ListChannels(opt.Client))
	a.POST("/channels", adminhandler.CreateChannel(chAdmin))
	a.PUT("/channels/:id", adminhandler.UpdateChannel(chAdmin))
	a.DELETE("/channels/:id", adminhandler.DeleteChannel(opt.Client))
	a.GET("/channel-model-presets", adminhandler.ChannelModelPresets(opt.Client))
	a.POST("/channel-models/fetch-upstream", adminhandler.FetchUpstreamModelList())
	a.GET("/groups", adminhandler.ListGroups(opt.Client))
	a.POST("/groups", adminhandler.CreateGroup(opt.Client))
	a.PUT("/groups/:id", adminhandler.UpdateGroup(opt.Client))
	a.DELETE("/groups/:id", adminhandler.DeleteGroup(opt.Client))
	a.GET("/groups/:id/model-rates", adminhandler.ListGroupModelRates(opt.Client))
	a.PUT("/groups/:id/model-rates", adminhandler.PutGroupModelRates(opt.Client))
	a.GET("/logs/export", adminhandler.ExportLogs(opt.Client))
	a.GET("/logs/stats", adminhandler.LogStats(opt.Client))
	a.GET("/logs", adminhandler.ListLogs(opt.Client))
	a.GET("/redeem-codes", adminhandler.ListRedeemCodes(opt.Client))
	a.POST("/redeem-codes", adminhandler.BatchRedeemCodes(opt.Client))
	a.DELETE("/redeem-codes/:id", adminhandler.RevokeRedeemCode(opt.Client))
	a.GET("/config", adminhandler.GetSystemConfig(opt.Client))
	a.PUT("/config", adminhandler.PutSystemConfig(opt.Client))
	a.POST("/config/reload", adminhandler.ReloadSystemConfig())
	a.GET("/model-catalog/sources", adminhandler.ListModelCatalogSources(opt.Client))
	a.POST("/model-catalog/sources", adminhandler.CreateModelCatalogSource(mcAdmin))
	a.PUT("/model-catalog/sources/:id", adminhandler.UpdateModelCatalogSource(mcAdmin))
	a.DELETE("/model-catalog/sources/:id", adminhandler.DeleteModelCatalogSource(opt.Client))
	a.POST("/model-catalog/sources/:id/sync", adminhandler.SyncModelCatalogSource(mcAdmin))
	a.GET("/model-catalog/entries", adminhandler.ListModelCatalogEntries(opt.Client))
	a.PUT("/model-catalog/entries/:id", adminhandler.UpdateModelCatalogEntry(opt.Client))
	a.DELETE("/model-catalog/entries/:id", adminhandler.DeleteModelCatalogEntry(opt.Client))

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
