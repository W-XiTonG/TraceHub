package main

import (
	"context"
	"database/sql"
	"flag"
	"path/filepath"

	entsql "entgo.io/ent/dialect/sql"
	"tracehub/internal/bootstrap"
	"tracehub/internal/ent"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/pginit"
	"tracehub/internal/pkg/pricingsyncmigrate"
	"tracehub/internal/pkg/usagelogmigrate"
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
	cfgAbs, err := filepath.Abs(*cfgPath)
	if err != nil {
		zlog.Fatal("failed to resolve config path", zap.Error(err), zap.String("path", *cfgPath))
	}

	ctx := context.Background()
	createdDB, createdDBName, err := pginit.EnsureDatabase(ctx, cfg.Database.DSN)
	if err != nil {
		zlog.Fatal("database preparation failed", zap.Error(err))
	}
	if createdDB {
		zlog.Info("created PostgreSQL database (schema will be applied by migration)", zap.String("database", createdDBName))
	}

	sqldb, err := sql.Open("postgres", cfg.Database.DSN)
	if err != nil {
		zlog.Fatal("database open failed", zap.Error(err))
	}
	defer func() { _ = sqldb.Close() }()

	client := ent.NewClient(ent.Driver(entsql.OpenDB("postgres", sqldb)))

	if err := client.Schema.Create(ctx); err != nil {
		zlog.Fatal("database migration failed", zap.Error(err))
	}
	if err := pricingsyncmigrate.FixLegacyPullPaths(ctx, sqldb); err != nil {
		zlog.Fatal("pricing sync pull_path backfill failed", zap.Error(err))
	}
	if err := usagelogmigrate.BackfillFailoverFlags(ctx, sqldb); err != nil {
		zlog.Fatal("usage_logs failover flag backfill failed", zap.Error(err))
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
		Config:         cfg,
		ConfigFilePath: cfgAbs,
		Client:         client,
		StdDB:          sqldb,
		Redis:          rdb,
		Log:            zlog,
	})
	if err != nil {
		zlog.Fatal("router init failed", zap.Error(err))
	}

	if err := runServers(ctx, cfg, client, r, zlog); err != nil {
		zlog.Fatal("HTTP server exited with error", zap.Error(err))
	}
}
