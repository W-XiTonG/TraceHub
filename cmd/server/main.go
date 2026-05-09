package main

import (
	"context"
	"flag"
	"net/http"
	"time"

	"tracehub/internal/bootstrap"
	"tracehub/internal/ent"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/pginit"
	"tracehub/internal/server"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	cfgPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	zlog, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() { _ = zlog.Sync() }()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		zlog.Fatal("加载配置失败", zap.Error(err), zap.String("path", *cfgPath))
	}

	ctx := context.Background()
	createdDB, createdDBName, err := pginit.EnsureDatabase(ctx, cfg.Database.DSN)
	if err != nil {
		zlog.Fatal("数据库准备失败", zap.Error(err))
	}
	if createdDB {
		zlog.Info("已自动创建 PostgreSQL 数据库（表结构将由迁移创建）", zap.String("database", createdDBName))
	}

	client, err := ent.Open("postgres", cfg.Database.DSN)
	if err != nil {
		zlog.Fatal("数据库连接失败", zap.Error(err))
	}
	defer client.Close()

	if err := client.Schema.Create(ctx); err != nil {
		zlog.Fatal("数据库迁移失败", zap.Error(err))
	}

	var rdb *redis.Client
	if cfg.Redis.Addr != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		defer func() { _ = rdb.Close() }()
		if err := rdb.Ping(ctx).Err(); err != nil {
			zlog.Warn("Redis 不可用，粘性会话与部分限流将降级", zap.Error(err))
			rdb = nil
		}
	}

	if err := bootstrap.EnsureDefaults(ctx, client, cfg, zlog); err != nil {
		zlog.Fatal("初始化数据失败", zap.Error(err))
	}

	r, err := server.NewRouter(server.Options{
		Config: cfg,
		Client: client,
		Redis:  rdb,
		Log:    zlog,
	})
	if err != nil {
		zlog.Fatal("路由初始化失败", zap.Error(err))
	}

	addr := cfg.Server.Addr
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}
	zlog.Info("TraceHub 网关启动", zap.String("addr", addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		zlog.Fatal("HTTP 服务异常退出", zap.Error(err))
	}
}
