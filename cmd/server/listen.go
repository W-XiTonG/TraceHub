package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/sitecfg"

	"go.uber.org/zap"
)

func runServers(ctx context.Context, cfg *config.Config, client *ent.Client, handler http.Handler, log *zap.Logger) error {
	readTO := 120 * time.Second
	writeTO := 120 * time.Second

	httpAddr := strings.TrimSpace(cfg.Server.Addr)
	if httpAddr == "" {
		httpAddr = ":8080"
	}

	tlsOn := sitecfg.TLSEnabledAtStartup(ctx, client, cfg)
	if !tlsOn {
		srv := &http.Server{
			Addr:         httpAddr,
			Handler:      handler,
			ReadTimeout:  readTO,
			WriteTimeout: writeTO,
		}
		log.Info("TraceHub gateway listening (HTTP)", zap.String("addr", httpAddr))
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}

	certFile := strings.TrimSpace(cfg.Server.TLS.CertFile)
	keyFile := strings.TrimSpace(cfg.Server.TLS.KeyFile)
	if certFile == "" || keyFile == "" {
		log.Fatal("server.tls.enabled 为 true 时须配置 server.tls.cert_file 与 server.tls.key_file")
	}
	if _, err := os.Stat(certFile); err != nil {
		log.Fatal("TLS 证书文件不可读", zap.String("cert_file", certFile), zap.Error(err))
	}
	if _, err := os.Stat(keyFile); err != nil {
		log.Fatal("TLS 私钥文件不可读", zap.String("key_file", keyFile), zap.Error(err))
	}

	tlsAddr := strings.TrimSpace(cfg.Server.TLS.Addr)
	if tlsAddr == "" {
		tlsAddr = ":443"
	}

	tlsSrv := &http.Server{
		Addr:         tlsAddr,
		Handler:      handler,
		ReadTimeout:  readTO,
		WriteTimeout: writeTO,
	}
	log.Info("TraceHub gateway listening (HTTPS)",
		zap.String("tls_addr", tlsAddr),
		zap.String("cert_file", certFile),
		zap.String("key_file", keyFile),
	)

	if httpAddr != tlsAddr {
		httpSrv := &http.Server{
			Addr:         httpAddr,
			Handler:      handler,
			ReadTimeout:  readTO,
			WriteTimeout: writeTO,
		}
		go func() {
			log.Info("TraceHub HTTP redirect listener", zap.String("addr", httpAddr))
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("HTTP listener exited", zap.Error(err))
			}
		}()
	}

	err := tlsSrv.ListenAndServeTLS(certFile, keyFile)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
