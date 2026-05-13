package adminhandler

import (
	"bytes"
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
	// BrandIconPublicURLPrefix 本地上传图标对外 URL 前缀（与 gin Static 挂载一致）。
	BrandIconPublicURLPrefix = "/uploads/site-brand"
	brandIconDiskBase        = "site-brand-icon"
	brandIconMaxBytes        = 2 << 20 // 2 MiB
)

// BrandUploadDir 品牌图标本地上传目录（绝对路径）；配置空时默认 data/site_brand。
func BrandUploadDir(cfg *config.Config) string {
	d := strings.TrimSpace(cfg.Site.BrandUploadDir)
	if d == "" {
		d = "data/site_brand"
	}
	abs, err := filepath.Abs(d)
	if err != nil {
		return d
	}
	return abs
}

// detectBrandUploadExt 仅允许常见栅格/WebP，避免 SVG 脚本通过 <img> 同源加载风险。
func detectBrandUploadExt(data []byte) (ext string, ok bool) {
	if len(data) >= 4 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return ".png", true
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return ".jpg", true
	}
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return ".gif", true
	}
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) {
		if len(data) >= 12 && bytes.Equal(data[8:12], []byte("WEBP")) {
			return ".webp", true
		}
	}
	return "", false
}

func brandIconPublicURL(ext string) string {
	return BrandIconPublicURLPrefix + "/" + brandIconDiskBase + ext
}

// PostBrandIconUpload POST /api/admin/site-upload/brand-icon — multipart 字段 file；写入磁盘并更新 site.brand.icon_url。
func PostBrandIconUpload(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		if err := c.Request.ParseMultipartForm(brandIconMaxBytes + 1); err != nil {
			httpx.BadRequest(c, "无法解析上传表单")
			return
		}
		fh, err := c.FormFile("file")
		if err != nil || fh == nil {
			httpx.BadRequest(c, "缺少文件字段 file")
			return
		}
		if fh.Size > brandIconMaxBytes {
			httpx.BadRequest(c, "文件过大（最大 2MB）")
			return
		}
		src, err := fh.Open()
		if err != nil {
			httpx.BadRequest(c, "无法读取上传文件")
			return
		}
		defer func() { _ = src.Close() }()

		data, err := io.ReadAll(io.LimitReader(src, brandIconMaxBytes+1))
		if err != nil {
			httpx.ServerError(c, "读取文件失败")
			return
		}
		if len(data) > brandIconMaxBytes {
			httpx.BadRequest(c, "文件过大（最大 2MB）")
			return
		}
		ext, ok := detectBrandUploadExt(data)
		if !ok {
			httpx.BadRequest(c, "仅支持 PNG、JPEG、GIF、WebP 图片")
			return
		}

		root := BrandUploadDir(cfg)
		if err := os.MkdirAll(root, 0o755); err != nil {
			httpx.ServerError(c, "无法创建上传目录："+err.Error())
			return
		}

		matches, _ := filepath.Glob(filepath.Join(root, brandIconDiskBase+".*"))
		for _, m := range matches {
			_ = os.Remove(m)
		}

		dest := filepath.Join(root, brandIconDiskBase+ext)
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			httpx.ServerError(c, "保存文件失败："+err.Error())
			return
		}

		url := brandIconPublicURL(ext)
		if err := sitecfg.ValidateBrandIconURL(url); err != nil {
			httpx.ServerError(c, "内部错误：生成 URL 无效")
			return
		}

		ctx := c.Request.Context()
		ex, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(sitecfg.SystemConfigKeyBrandIconURL)).Only(ctx)
		if ent.IsNotFound(err) {
			_, err = client.SystemConfig.Create().
				SetConfigKey(sitecfg.SystemConfigKeyBrandIconURL).
				SetConfigValue(url).
				SetDescription("Brand icon (uploaded)").
				Save(ctx)
		} else if err != nil {
			httpx.ServerError(c, "更新配置失败")
			return
		} else {
			err = client.SystemConfig.UpdateOne(ex).SetConfigValue(url).Exec(ctx)
		}
		if err != nil {
			httpx.ServerError(c, "更新配置失败")
			return
		}

		auditLog(client, adm.ID, "site.brand_icon_upload", "system_config", sitecfg.SystemConfigKeyBrandIconURL, map[string]any{"url": url}, c)
		httpx.OK(c, gin.H{"url": url})
	}
}
