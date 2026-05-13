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
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/service/balancealert"
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

	r.GET("/health", func(c *gin.Context) {
		op := sitecfg.EffectiveOperatorDisplayName(c.Request.Context(), opt.Client, opt.Config)
		health.Liveness(op)(c)
	})
	r.GET("/ready", func(c *gin.Context) {
		op := sitecfg.EffectiveOperatorDisplayName(c.Request.Context(), opt.Client, opt.Config)
		health.Readiness(opt.Client, opt.Redis, op)(c)
	})
	r.GET("/api/pub/site-meta", userhandler.PubSiteMeta(opt.Client))
	uploadRoot := adminhandler.BrandUploadDir(opt.Config)
	if mkErr := os.MkdirAll(uploadRoot, 0o755); mkErr != nil {
		opt.Log.Warn("brand_upload_dir mkdir failed", zap.String("path", uploadRoot), zap.Error(mkErr))
	} else {
		r.Static(adminhandler.BrandIconPublicURLPrefix, uploadRoot)
	}
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	balAlerts := balancealert.New(opt.Config, opt.Log)
	billingEngine := &billing.Engine{Client: opt.Client, Alerts: balAlerts}

	gate := r.Group("/v1")
	gate.Use(middleware.GatewayAPIKeyAuth(opt.Client))
	gate.Use(middleware.DenyOpenAIAccountAdminUnderV1(opt.Config.Site.BlockOpenAIAccountAdminPaths, opt.Config.Site.OpenAIAccountAdminDenyPrefixes))

	gwBase := gatewayhandler.ChatDeps{
		Client:             opt.Client,
		RDB:                opt.Redis,
		AESKey:             aesKey,
		Billing:            billingEngine,
		OpenAICompatExtras: opt.Config.Site.OpenAICompatChannelTypes,
		Passthrough: gatewayhandler.OpenAIPassthroughSettings{
			Enabled:          opt.Config.Site.OpenAIPassthrough.Enabled,
			AllowPrefixes:    opt.Config.Site.OpenAIPassthrough.AllowPrefixes,
			DenyPrefixes:     opt.Config.Site.OpenAIPassthrough.DenyPrefixes,
			PatchModelInJSON: opt.Config.Site.OpenAIPassthrough.PatchModelInJSON,
		},
	}

	gate.POST("/chat/completions", gatewayhandler.ChatCompletions(gwBase))
	gate.POST("/images/generations", gatewayhandler.ImageGenerations(gwBase))
	gate.POST("/images/edits", gatewayhandler.ImageEdits(gwBase))
	gate.POST("/images/variations", gatewayhandler.ImageVariations(gwBase))
	gate.POST("/embeddings", gatewayhandler.Embeddings(gwBase))
	gate.POST("/completions", gatewayhandler.Completions(gwBase))
	gate.POST("/moderations", gatewayhandler.Moderations(gwBase))
	gate.POST("/audio/transcriptions", gatewayhandler.AudioTranscriptions(gwBase))
	gate.POST("/audio/translations", gatewayhandler.AudioTranslations(gwBase))
	gate.POST("/audio/speech", gatewayhandler.AudioSpeech(gwBase))
	respDeps := gwBase
	gate.POST("/responses", gatewayhandler.ResponsesCreate(respDeps))
	gate.GET("/responses/:response_id", gatewayhandler.ResponsesGet(respDeps))
	gate.DELETE("/responses/:response_id", gatewayhandler.ResponsesDelete(respDeps))
	gate.POST("/responses/:response_id/cancel", gatewayhandler.ResponsesCancel(respDeps))
	rtDeps := gwBase
	rtDeps.RTCall = gatewayhandler.RealtimeCallGatewayConfig{
		MappingEnabled:   opt.Config.Site.RealtimeCallMappingEnabled,
		MappingTTLSec:    opt.Config.Site.RealtimeCallMappingTTLSec,
		BillRejectHangup: opt.Config.Site.RealtimeCallControlBillRejectHangup,
	}
	gate.POST("/realtime/sessions", gatewayhandler.RealtimeSessions(rtDeps))
	gate.POST("/realtime/transcription_sessions", gatewayhandler.RealtimeTranscriptionSessions(rtDeps))
	gate.POST("/realtime/calls", gatewayhandler.RealtimeCallsWebRTC(rtDeps))
	gate.POST("/realtime/calls/:call_id/accept", gatewayhandler.RealtimeCallAccept(rtDeps))
	gate.POST("/realtime/calls/:call_id/reject", gatewayhandler.RealtimeCallReject(rtDeps))
	gate.POST("/realtime/calls/:call_id/hangup", gatewayhandler.RealtimeCallHangup(rtDeps))
	gate.POST("/realtime/calls/:call_id/refer", gatewayhandler.RealtimeCallRefer(rtDeps))
	gate.POST("/realtime/client_secrets", gatewayhandler.RealtimeClientSecrets(rtDeps))
	gate.GET("/realtime", gatewayhandler.RealtimeWebSocket(rtDeps))
	gate.GET("/realtime/*realtimePath", gatewayhandler.RealtimeWebSocket(rtDeps))
	gate.GET("/models", gatewayhandler.ModelsList(opt.Client))
	gate.GET("/models/:model_id", gatewayhandler.ModelsGet(opt.Client))

	gate.POST("/files", gatewayhandler.OpenAIFilesCreate(respDeps))
	gate.GET("/files", gatewayhandler.OpenAIFilesList(respDeps))
	gate.GET("/files/:file_id/content", gatewayhandler.OpenAIFilesContent(respDeps))
	gate.GET("/files/:file_id", gatewayhandler.OpenAIFilesRetrieve(respDeps))
	gate.DELETE("/files/:file_id", gatewayhandler.OpenAIFilesDelete(respDeps))

	gate.POST("/batches", gatewayhandler.OpenAIBatchesCreate(respDeps))
	gate.GET("/batches", gatewayhandler.OpenAIBatchesList(respDeps))
	gate.POST("/batches/:batch_id/cancel", gatewayhandler.OpenAIBatchesCancel(respDeps))
	gate.GET("/batches/:batch_id", gatewayhandler.OpenAIBatchesRetrieve(respDeps))

	gate.POST("/fine_tuning/jobs", gatewayhandler.OpenAIFineTuningJobsCreate(respDeps))
	gate.GET("/fine_tuning/jobs/:job_id/events", gatewayhandler.OpenAIFineTuningJobEvents(respDeps))
	gate.GET("/fine_tuning/jobs/:job_id/checkpoints/:checkpoint_id", gatewayhandler.OpenAIFineTuningJobCheckpointRetrieve(respDeps))
	gate.GET("/fine_tuning/jobs/:job_id/checkpoints", gatewayhandler.OpenAIFineTuningJobCheckpoints(respDeps))
	gate.POST("/fine_tuning/jobs/:job_id/cancel", gatewayhandler.OpenAIFineTuningJobCancel(respDeps))
	gate.GET("/fine_tuning/jobs/:job_id", gatewayhandler.OpenAIFineTuningJobRetrieve(respDeps))
	gate.GET("/fine_tuning/jobs", gatewayhandler.OpenAIFineTuningJobsList(respDeps))

	// OpenAI Assistants / Threads / Runs / Vector stores（透传上游；选路：JSON model 或 ?model= / X-TraceHub-Model）
	gate.POST("/assistants", gatewayhandler.OpenAIAssistantsCreate(respDeps))
	gate.GET("/assistants", gatewayhandler.OpenAIAssistantsList(respDeps))
	gate.GET("/assistants/:assistant_id", gatewayhandler.OpenAIAssistantsRetrieve(respDeps))
	gate.POST("/assistants/:assistant_id", gatewayhandler.OpenAIAssistantsModify(respDeps))
	gate.DELETE("/assistants/:assistant_id", gatewayhandler.OpenAIAssistantsDelete(respDeps))

	gate.POST("/threads", gatewayhandler.OpenAIThreadsCreate(respDeps))
	gate.GET("/threads/:thread_id", gatewayhandler.OpenAIThreadsRetrieve(respDeps))
	gate.POST("/threads/:thread_id", gatewayhandler.OpenAIThreadsModify(respDeps))
	gate.DELETE("/threads/:thread_id", gatewayhandler.OpenAIThreadsDelete(respDeps))

	gate.POST("/threads/:thread_id/messages", gatewayhandler.OpenAIThreadMessagesCreate(respDeps))
	gate.GET("/threads/:thread_id/messages", gatewayhandler.OpenAIThreadMessagesList(respDeps))
	gate.GET("/threads/:thread_id/messages/:message_id", gatewayhandler.OpenAIThreadMessagesRetrieve(respDeps))
	gate.POST("/threads/:thread_id/messages/:message_id", gatewayhandler.OpenAIThreadMessagesModify(respDeps))

	gate.POST("/threads/:thread_id/runs/:run_id/submit_tool_outputs", gatewayhandler.OpenAIThreadRunsSubmitToolOutputs(respDeps))
	gate.POST("/threads/:thread_id/runs/:run_id/cancel", gatewayhandler.OpenAIThreadRunsCancel(respDeps))
	gate.GET("/threads/:thread_id/runs/:run_id/steps/:step_id", gatewayhandler.OpenAIThreadRunStepsRetrieve(respDeps))
	gate.GET("/threads/:thread_id/runs/:run_id/steps", gatewayhandler.OpenAIThreadRunStepsList(respDeps))
	gate.POST("/threads/:thread_id/runs/:run_id", gatewayhandler.OpenAIThreadRunsModify(respDeps))
	gate.GET("/threads/:thread_id/runs/:run_id", gatewayhandler.OpenAIThreadRunsRetrieve(respDeps))
	gate.POST("/threads/:thread_id/runs", gatewayhandler.OpenAIThreadRunsCreate(respDeps))
	gate.GET("/threads/:thread_id/runs", gatewayhandler.OpenAIThreadRunsList(respDeps))

	gate.POST("/vector_stores", gatewayhandler.OpenAIVectorStoresCreate(respDeps))
	gate.GET("/vector_stores", gatewayhandler.OpenAIVectorStoresList(respDeps))
	gate.GET("/vector_stores/:vector_store_id/file_batches/:batch_id/files/:file_id", gatewayhandler.OpenAIVectorStoreFileBatchFilesRetrieve(respDeps))
	gate.GET("/vector_stores/:vector_store_id/file_batches/:batch_id/files", gatewayhandler.OpenAIVectorStoreFileBatchFilesList(respDeps))
	gate.POST("/vector_stores/:vector_store_id/file_batches/:batch_id/cancel", gatewayhandler.OpenAIVectorStoreFileBatchesCancel(respDeps))
	gate.GET("/vector_stores/:vector_store_id/file_batches/:batch_id", gatewayhandler.OpenAIVectorStoreFileBatchesRetrieve(respDeps))
	gate.POST("/vector_stores/:vector_store_id/file_batches", gatewayhandler.OpenAIVectorStoreFileBatchesCreate(respDeps))
	gate.DELETE("/vector_stores/:vector_store_id/files/:file_id", gatewayhandler.OpenAIVectorStoreFilesDelete(respDeps))
	gate.GET("/vector_stores/:vector_store_id/files/:file_id", gatewayhandler.OpenAIVectorStoreFilesRetrieve(respDeps))
	gate.POST("/vector_stores/:vector_store_id/files", gatewayhandler.OpenAIVectorStoreFilesCreate(respDeps))
	gate.GET("/vector_stores/:vector_store_id/files", gatewayhandler.OpenAIVectorStoreFilesList(respDeps))
	gate.GET("/vector_stores/:vector_store_id", gatewayhandler.OpenAIVectorStoresRetrieve(respDeps))
	gate.POST("/vector_stores/:vector_store_id", gatewayhandler.OpenAIVectorStoresModify(respDeps))
	gate.DELETE("/vector_stores/:vector_store_id", gatewayhandler.OpenAIVectorStoresDelete(respDeps))

	if opt.Config.Site.OpenAIPassthrough.Enabled {
		gate.Any("/*gwOpenAICatchAll", gatewayhandler.OpenAIV1Passthrough(gwBase))
	}

	u := r.Group("/api/user")
	u.GET("/register/email-code-meta", userhandler.RegisterEmailCodeMeta(opt.Config, opt.Redis))
	u.POST("/register/email-code/send", userhandler.SendRegisterCode(opt.Client, opt.Config, opt.Redis))
	u.POST("/register", userhandler.Register(opt.Client, opt.Config, opt.Redis))
	u.GET("/login-captcha", userhandler.LoginCaptchaIssue(opt.Client, opt.Config, opt.Redis))
	u.POST("/login", userhandler.Login(opt.Client, opt.Config, jwtSecret, opt.Redis))
	u.POST("/login-code/send", userhandler.SendLoginCode(opt.Client, opt.Config, opt.Redis))
	u.POST("/login-code/verify", userhandler.VerifyLoginCode(opt.Client, opt.Config, jwtSecret, opt.Redis))
	u.POST("/refresh", userhandler.Refresh(opt.Client, jwtSecret))

	authUser := u.Group("")
	authUser.Use(middleware.JWTAuth(jwtSecret, opt.Client))
	authUser.GET("/me", userhandler.Me(opt.Client, opt.Config))
	authUser.GET("/balance", userhandler.Balance(opt.Client, opt.Config))
	authUser.GET("/usage-logs", userhandler.UsageLogs(opt.Client))
	authUser.GET("/group", userhandler.MyGroup(opt.Client))
	authUser.GET("/available-models", userhandler.AvailableModels(opt.Client))
	authUser.GET("/models", userhandler.AvailableModels(opt.Client))
	authUser.GET("/usage", userhandler.Usage(opt.Client, opt.Config))
	authUser.GET("/usage/export", userhandler.UsageExport(opt.Client))
	authUser.GET("/apikeys", userhandler.ListAPIKeys(opt.Client))
	authUser.POST("/apikeys", userhandler.CreateAPIKey(opt.Client, opt.Config, aesKey))
	authUser.POST("/apikeys/:id/reveal", userhandler.RevealAPIKey(opt.Client, aesKey))
	authUser.PUT("/apikeys/:id", userhandler.UpdateAPIKey(opt.Client))
	authUser.DELETE("/apikeys/:id", userhandler.DeleteAPIKey(opt.Client))
	authUser.POST("/redeem", userhandler.Redeem(opt.Client))
	authUser.PUT("/password", userhandler.ChangePassword(opt.Client))
	authUser.PUT("/bind", userhandler.BindProfile(opt.Client))
	authUser.PUT("/ip_whitelist", userhandler.SetIPWhitelist(opt.Client))
	authUser.GET("/announcements/latest", userhandler.LatestAnnouncement(opt.Client))
	authUser.GET("/announcements", userhandler.ListPublishedAnnouncements(opt.Client))

	chAdmin := adminhandler.ChannelDeps{Client: opt.Client, AESKey: aesKey}
	mcAdmin := adminhandler.ModelCatalogDeps{Client: opt.Client, AESKey: aesKey}

	a := r.Group("/api/admin")
	a.Use(middleware.JWTAuth(jwtSecret, opt.Client))
	a.Use(middleware.RequireRole("admin", "superadmin"))
	a.GET("/dashboard/summary", adminhandler.DashboardSummary(opt.Client, opt.Config))
	a.GET("/dashboard/overview", adminhandler.DashboardOverview(opt.Client))
	a.GET("/dashboard/trends", adminhandler.DashboardTrends(opt.Client))
	a.GET("/dashboard/channels", adminhandler.DashboardChannels(opt.Client))
	a.GET("/dashboard/insight", adminhandler.DashboardInsight(opt.Client, opt.Config))
	a.GET("/dashboard/snapshot", adminhandler.DashboardSnapshot(opt.Client))
	a.GET("/dashboard/ops", adminhandler.DashboardOps(opt.Client, opt.Redis, opt.Config))
	a.GET("/users", adminhandler.ListUsers(opt.Client))
	a.GET("/users/:id", adminhandler.GetUser(opt.Client))
	a.POST("/users/batch-balance", adminhandler.BatchAdjustBalance(opt.Client))
	a.PUT("/users/:id", adminhandler.UpdateUser(opt.Client))
	a.POST("/users/:id/balance", adminhandler.AdjustBalance(opt.Client))
	a.POST("/users/:id/ban", adminhandler.BanUser(opt.Client))
	a.POST("/users/:id/unban", adminhandler.UnbanUser(opt.Client))
	a.DELETE("/users/:id", adminhandler.DeleteUser(opt.Client))
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
	a.GET("/site-settings", adminhandler.GetAdminSiteSettings(opt.Client, opt.Config))
	a.POST("/site-upload/brand-icon", adminhandler.PostBrandIconUpload(opt.Client, opt.Config))
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
	a.GET("/announcements", adminhandler.ListAnnouncements(opt.Client))
	a.POST("/announcements", adminhandler.CreateAnnouncement(opt.Client))
	a.PUT("/announcements/:id", adminhandler.UpdateAnnouncement(opt.Client))
	a.DELETE("/announcements/:id", adminhandler.DeleteAnnouncement(opt.Client))
	a.POST("/announcements/:id/publish", adminhandler.PublishAnnouncement(opt.Client))
	a.POST("/announcements/:id/unpublish", adminhandler.UnpublishAnnouncement(opt.Client))

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
