package adminhandler

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/sitecfg"

	"github.com/gin-gonic/gin"
)

const (
	tlsUploadMaxBytes     = 512 << 10 // 512 KiB
	tlsActiveCertFilename = "active-fullchain.pem"
	tlsActiveKeyFilename  = "active-privkey.pem"
)

// TLSUploadDir TLS 证书/私钥本地上传目录（绝对路径）；配置空时默认 data/tls。
func TLSUploadDir(cfg *config.Config) string {
	d := ""
	if cfg != nil {
		d = strings.TrimSpace(cfg.Server.TLS.TLSUploadDir)
	}
	if d == "" {
		d = "data/tls"
	}
	abs, err := filepath.Abs(d)
	if err != nil {
		return d
	}
	return abs
}

func upsertSystemConfigString(ctx context.Context, client *ent.Client, configKey, value, description string) error {
	ex, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(configKey)).Only(ctx)
	if ent.IsNotFound(err) {
		_, err = client.SystemConfig.Create().
			SetConfigKey(configKey).
			SetConfigValue(value).
			SetDescription(description).
			Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	up := client.SystemConfig.UpdateOne(ex).SetConfigValue(value)
	if description != "" {
		up.SetDescription(description)
	}
	return up.Exec(ctx)
}

func readUploadPEM(c *gin.Context, max int64) ([]byte, error) {
	if err := c.Request.ParseMultipartForm(max + 1); err != nil {
		return nil, errParseForm
	}
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		return nil, errMissingFile
	}
	if fh.Size > max {
		return nil, errFileTooLarge
	}
	src, err := fh.Open()
	if err != nil {
		return nil, errOpenFile
	}
	defer func() { _ = src.Close() }()
	data, err := io.ReadAll(io.LimitReader(src, max+1))
	if err != nil {
		return nil, errReadFile
	}
	if int64(len(data)) > max {
		return nil, errFileTooLarge
	}
	return data, nil
}

var (
	errParseForm    = &uploadErr{"无法解析上传表单"}
	errMissingFile  = &uploadErr{"缺少文件字段 file"}
	errFileTooLarge = &uploadErr{"文件过大（最大 512KB）"}
	errOpenFile     = &uploadErr{"无法读取上传文件"}
	errReadFile     = &uploadErr{"读取文件失败"}
)

type uploadErr struct{ msg string }

func (e *uploadErr) Error() string { return e.msg }

func validatePEMCertificate(data []byte) error {
	var found bool
	rest := data
	for {
		block, rem := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = rem
		if block.Type == "CERTIFICATE" {
			if _, err := x509.ParseCertificate(block.Bytes); err != nil {
				return err
			}
			found = true
		}
	}
	if !found {
		return errInvalidPEMCert
	}
	return nil
}

var errInvalidPEMCert = &uploadErr{"须为 PEM 格式证书（含 BEGIN CERTIFICATE）"}

func validatePEMPrivateKey(data []byte) error {
	block, _ := pem.Decode(data)
	if block == nil {
		return errInvalidPEMKey
	}
	switch block.Type {
	case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY":
		return nil
	default:
		return errInvalidPEMKey
	}
}

var errInvalidPEMKey = &uploadErr{"须为 PEM 格式私钥（含 BEGIN PRIVATE KEY 等）"}

func tryLoadTLSKeyPair(certPath, keyPath string) error {
	if certPath == "" || keyPath == "" {
		return nil
	}
	_, err := tls.LoadX509KeyPair(certPath, keyPath)
	return err
}

func postTLSMaterialUpload(client *ent.Client, cfg *config.Config, kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		data, err := readUploadPEM(c, tlsUploadMaxBytes)
		if err != nil {
			if ue, ok := err.(*uploadErr); ok {
				httpx.BadRequest(c, ue.msg)
				return
			}
			httpx.BadRequest(c, "无法解析上传表单")
			return
		}

		var configKey, filename, auditAction string
		var perm os.FileMode = 0o644
		switch kind {
		case "cert":
			if err := validatePEMCertificate(data); err != nil {
				if ue, ok := err.(*uploadErr); ok {
					httpx.BadRequest(c, ue.msg)
					return
				}
				httpx.BadRequest(c, "证书 PEM 无效")
				return
			}
			configKey = sitecfg.SystemConfigKeyTLSCertFile
			filename = tlsActiveCertFilename
			auditAction = "site.tls_cert_upload"
		case "key":
			if err := validatePEMPrivateKey(data); err != nil {
				if ue, ok := err.(*uploadErr); ok {
					httpx.BadRequest(c, ue.msg)
					return
				}
				httpx.BadRequest(c, "私钥 PEM 无效")
				return
			}
			configKey = sitecfg.SystemConfigKeyTLSKeyFile
			filename = tlsActiveKeyFilename
			auditAction = "site.tls_key_upload"
			perm = 0o600
		default:
			httpx.ServerError(c, "内部错误")
			return
		}

		root := TLSUploadDir(cfg)
		if err := os.MkdirAll(root, 0o700); err != nil {
			httpx.ServerError(c, "无法创建上传目录："+err.Error())
			return
		}

		dest := filepath.Join(root, filename)
		if err := os.WriteFile(dest, data, perm); err != nil {
			httpx.ServerError(c, "保存文件失败："+err.Error())
			return
		}
		absDest, err := filepath.Abs(dest)
		if err != nil {
			httpx.ServerError(c, "解析路径失败")
			return
		}
		if err := sitecfg.ValidateTLSCertFilePath(absDest); err != nil && kind == "cert" {
			_ = os.Remove(dest)
			httpx.ServerError(c, err.Error())
			return
		}
		if err := sitecfg.ValidateTLSKeyFilePath(absDest); err != nil && kind == "key" {
			_ = os.Remove(dest)
			httpx.ServerError(c, err.Error())
			return
		}

		ctx := c.Request.Context()
		if err := upsertSystemConfigString(ctx, client, configKey, absDest, configKey+" (uploaded)"); err != nil {
			httpx.ServerError(c, "更新配置失败")
			return
		}

		paths := sitecfg.EffectiveTLSCertKeyPaths(ctx, client, cfg)
		if loadErr := tryLoadTLSKeyPair(paths.CertFile, paths.KeyFile); loadErr != nil {
			auditLog(client, adm.ID, auditAction, "system_config", configKey, map[string]any{"path": absDest, "pair_ok": false}, c)
			httpx.OKMsg(c, "已保存；证书与私钥尚未配对成功，请上传匹配的另一项后重启服务", gin.H{
				"path":      absDest,
				"cert_file": paths.CertFile,
				"key_file":  paths.KeyFile,
				"pair_ok":   false,
			})
			return
		}

		auditLog(client, adm.ID, auditAction, "system_config", configKey, map[string]any{"path": absDest, "pair_ok": true}, c)
		httpx.OKMsg(c, "已保存；请重启 TraceHub 服务后内置 TLS 生效", gin.H{
			"path":      absDest,
			"cert_file": paths.CertFile,
			"key_file":  paths.KeyFile,
			"pair_ok":   true,
		})
	}
}

// PostTLSCertUpload POST /api/admin/site-upload/tls-cert
func PostTLSCertUpload(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return postTLSMaterialUpload(client, cfg, "cert")
}

// PostTLSKeyUpload POST /api/admin/site-upload/tls-key
func PostTLSKeyUpload(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return postTLSMaterialUpload(client, cfg, "key")
}
