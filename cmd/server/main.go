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
	cfgPath := flag.String("config", "configs/config.yaml", "path to YAML config file")
	flag.Parse()

	zlog, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() { _ = zlog.Sync() }()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		zlog.Fatal("failed to load config", zap.Error(err), zap.String("path", *cfgPath))
	}

	ctx := context.Background()
	createdDB, createdDBName, err := pginit.EnsureDatabase(ctx, cfg.Database.DSN)
	if err != nil {
		zlog.Fatal("database preparation failed", zap.Error(err))
	}
	if createdDB {
		zlog.Info("created PostgreSQL database (schema will be applied by migration)", zap.String("database", createdDBName))
	}

	client, err := ent.Open("postgres", cfg.Database.DSN)
	if err != nil {
		zlog.Fatal("database connection failed", zap.Error(err))
	}
	defer client.Close()

	if err := client.Schema.Create(ctx); err != nil {
		zlog.Fatal("database migration failed", zap.Error(err))
	}

	var rdb *redis.Client
	if cfg.Redis.Addr != "" {
		zlog.Info("connecting to Redis (using redis.addr from config)",
			zap.String("redis_addr", cfg.Redis.Addr),
			zap.Int("redis_db", cfg.Redis.DB),
			zap.Bool("redis_password_set", cfg.Redis.Password != ""),
		)
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		defer func() { _ = rdb.Close() }()
		if err := rdb.Ping(ctx).Err(); err != nil {
			zlog.Warn("Redis ping failed; running without Redis (email OTP login/register disabled)",
				zap.String("redis_addr", cfg.Redis.Addr),
				zap.Int("redis_db", cfg.Redis.DB),
				zap.Error(err),
			)
			rdb = nil
		} else {
			zlog.Info("Redis connected and ready",
				zap.String("redis_addr", cfg.Redis.Addr),
				zap.Int("redis_db", cfg.Redis.DB),
			)
		}
	} else {
		zlog.Info("Redis skipped (redis.addr is empty)")
	}

	if err := bootstrap.EnsureDefaults(ctx, client, cfg, zlog); err != nil {
		zlog.Fatal("bootstrap defaults failed", zap.Error(err))
	}

	r, err := server.NewRouter(server.Options{
		Config: cfg,
		Client: client,
		Redis:  rdb,
		Log:    zlog,
	})
	if err != nil {
		zlog.Fatal("router init failed", zap.Error(err))
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
	zlog.Info("TraceHub gateway listening", zap.String("addr", addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		zlog.Fatal("HTTP server exited with error", zap.Error(err))
	}
}
